'use strict';
/**
 * server.cjs — HTTP sidecar solver buat chatgpt2API.
 *
 * Endpoint:
 *   GET  /health            -> {ok:true}
 *   POST /solve             -> {cf_clearance, cookies, userAgent}  (challenge CF saja)
 *   POST /login             -> {ok, accessToken, cookies, userAgent, stage}  (email+password)
 *
 * Pakai: CHROME_PATH=... node server.cjs [port=7900]
 */
const { spawn } = require('child_process');
const http = require('http');
const path = require('path');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const PORT = parseInt(process.argv[2] || process.env.PORT || '7900', 10);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---- shared CDP helpers (dari driver.cjs, inline biar single-file) ----
async function launch() {
  const proc = spawn(CHROME, [
    '--headless=new', '--no-sandbox', '--disable-gpu',
    '--remote-debugging-port=0', '--window-size=1280,900',
    '--user-agent=' + UA, '--disable-blink-features=AutomationControlled',
    'about:blank',
  ], { stdio: ['ignore', 'ignore', 'pipe'] });
  const wsUrl = await new Promise((res, rej) => {
    let buf = '';
    const t = setTimeout(() => rej(new Error('no devtools')), 20000);
    proc.stderr.on('data', (d) => {
      buf += d.toString();
      const m = buf.match(/DevTools listening on (ws:\/\/\S+)/);
      if (m) { clearTimeout(t); res(m[1]); }
    });
    proc.on('exit', (c) => { clearTimeout(t); rej(new Error('chrome exit ' + c)); });
  });
  const debugPort = wsUrl.match(/:(\d+)\//)[1];
  const target = await new Promise((res, rej) => {
    http.get({ host: '127.0.0.1', port: debugPort, path: '/json/list' }, (r) => {
      let b = '';
      r.on('data', (c) => (b += c));
      r.on('end', () => {
        const pages = JSON.parse(b).filter((t) => t.type === 'page');
        pages.length ? res(pages[0]) : rej(new Error('no page target'));
      });
    }).on('error', rej);
  });
  return { proc, wsUrl: target.webSocketDebuggerUrl };
}

function connect(wsUrl) {
  const ws = new WebSocket(wsUrl);
  let id = 0;
  const pending = new Map();
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id); }
  };
  const send = (method, params = {}) => new Promise((res, rej) => {
    const mid = ++id;
    pending.set(mid, (m) => (m.error ? rej(new Error(m.error.message)) : res(m.result)));
    ws.send(JSON.stringify({ id: mid, method, params }));
  });
  const ready = new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error('WS')); });
  return { ws, send, ready };
}

async function evalJSON(c, expr) {
  const r = await c.send('Runtime.evaluate', { expression: expr, returnByValue: true });
  return JSON.parse(r.result.value || '{}');
}

const SNAP = `JSON.stringify({
  url: location.href, title: document.title,
  email: !!document.querySelector('input[name="email"],input[type="email"]'),
  password: !!document.querySelector('input[type="password"]'),
  otp: !!document.querySelector('input[inputmode="numeric"],input[name="code"],input[autocomplete="one-time-code"]'),
  arkose: !!document.querySelector('iframe[src*="arkoselabs"],iframe[src*="funcaptcha"],iframe[title*="challenge"]'),
  turnstile: !!document.querySelector('iframe[src*="challenges.cloudflare"]'),
  err: (document.querySelector('[role="alert"],.error')||{}).innerText||'',
  body: document.body.innerText.slice(0, 250)})`;

async function fillInput(c, sel, val) {
  await c.send('Runtime.evaluate', { expression: `
    const el=document.querySelector(${JSON.stringify(sel)});
    if(!el) throw new Error('input tidak ketemu: ${''}${JSON.stringify(sel)}');
    const set=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;
    set.call(el, ${JSON.stringify(val)});
    el.dispatchEvent(new Event('input',{bubbles:true}));
    el.dispatchEvent(new Event('change',{bubbles:true}));` });
}

async function clickSubmit(c, re) {
  const r = await c.send('Runtime.evaluate', { expression: `
    (()=>{ const f=document.querySelector('form');
      const b=[...document.querySelectorAll('button[type="submit"]')].find(x=>/${re}/i.test(x.innerText.trim()));
      if(b){b.click();return 'clicked';}
      if(f){f.requestSubmit?f.requestSubmit():f.submit();return 'submitted';}
      return 'no-form'; })()`, returnByValue: true });
  return r.result.value;
}

async function getAllCookies(c) {
  const ck = await c.send('Network.getAllCookies').catch(() => null);
  return ck ? ck.cookies : [];
}

// ---- solver actions ----

async function solveCf(url, timeoutSec) {
  const t0 = Date.now();
  const { proc, wsUrl } = await launch();
  const c = connect(wsUrl);
  try {
    await c.ready;
    await c.send('Page.enable'); await c.send('Runtime.enable'); await c.send('Network.enable');
    await c.send('Page.navigate', { url: url || 'https://chatgpt.com/' });
    let passed = false, title = '';
    while (Date.now() - t0 < timeoutSec * 1000) {
      await sleep(3000);
      try {
        const r = await c.send('Runtime.evaluate', { expression: 'document.title+"|"+document.body.innerText.slice(0,120)', returnByValue: true });
        const txt = (r.result && r.result.value) || '';
        title = txt.split('|')[0];
        const challenge = /just a moment|checking your browser|attention required/i.test(txt) || /cf_chl/.test(txt);
        if (!challenge && title) { passed = true; break; }
      } catch {}
    }
    const cookies = await getAllCookies(c);
    const cf = cookies.find((x) => x.name === 'cf_clearance');
    // JSON passthrough: buka URL JSON (mis. /api/auth/csrf) di konteks yang sama
    let jsonBody = null;
    const jurl = (url || '') + (url && url.includes('?') ? '&' : '?') + '_=' + Date.now();
    const jr = await c.send('Runtime.evaluate', {
      expression: `fetch(${JSON.stringify(jurl)}, {headers:{'Accept':'application/json'}}).then(r=>r.text()).then(t=>t.slice(0,2000))`,
      returnByValue: true, awaitPromise: true,
    }).catch(() => null);
    if (jr && jr.result && typeof jr.result.value === 'string' && jr.result.value.trim().startsWith('{')) {
      jsonBody = jr.result.value;
    }
    return {
      ok: !!(passed || cf),
      cf_clearance: cf ? cf.value : null,
      cookies: cookies.map((x) => x.name + '=' + x.value).join('; '),
      userAgent: UA,
      jsonBody,
      timeSec: ((Date.now() - t0) / 1000).toFixed(1),
      error: (passed || cf) ? null : 'challenge tidak lewat dalam batas waktu',
    };
  } finally { try { c.ws.close(); } catch {} proc.kill('SIGKILL'); }
}

async function loginEmailPw(email, password, timeoutSec) {
  const t0 = Date.now();
  const { proc, wsUrl } = await launch();
  const c = connect(wsUrl);
  const log = [];
  let stage = 'loading';
  try {
    await c.ready;
    await c.send('Page.enable'); await c.send('Runtime.enable'); await c.send('Network.enable');
    await c.send('Page.navigate', { url: 'https://chatgpt.com/auth/login' });
    while (Date.now() - t0 < timeoutSec * 1000) {
      await sleep(3000);
      let st;
      try { st = await evalJSON(c, SNAP); } catch { continue; }
      // page lagi navigasi (mis. pindah ke auth.openai.com) -> skip, jangan dianggap state
      if (!st.url || !(st.body || st.title)) continue;
      log.push(st);
      if (st.arkose) { stage = 'arkose'; break; }

      // auth ditolak upstream -> berhenti, jangan loop buang waktu
      if (/incorrect|invalid|wrong|didn.?t match|try again/i.test(st.err || st.body || '')) {
        stage = 'auth_fail'; break;
      }

      if (st.otp) { stage = 'otp_email'; break; }

      if (st.email && !st.password) {
        await fillInput(c, 'input[name="email"],input[type="email"]', email);
        await sleep(700);
        await clickSubmit(c, 'continue');
        await sleep(3500);
        continue;
      }
      if (st.password) {
        await fillInput(c, 'input[type="password"]', password);
        await sleep(700);
        await clickSubmit(c, 'continue|log in|sign in');
        await sleep(4000);
        continue;
      }
      if ((st.url || '').indexOf('chatgpt.com/') === 8 && (st.url || '').indexOf('/auth') === -1 && !st.email && !st.password && st.title && !/log in|get started/i.test(st.title || '')) {
        stage = 'loggedin'; break;
      }
    }
    let accessToken = null;
    if (stage === 'loggedin') {
      const sess = await c.send('Runtime.evaluate', {
        expression: `fetch('/api/auth/session',{headers:{'Accept':'application/json'}}).then(r=>r.json()).then(j=>JSON.stringify(j))`,
        returnByValue: true, awaitPromise: true,
      }).catch(() => null);
      if (sess && sess.result && sess.result.value && sess.result.value !== 'null') {
        try { accessToken = JSON.parse(sess.result.value).accessToken || null; } catch {}
      }
    }
    const cookies = await getAllCookies(c);
    const stTok = cookies.find((x) => x.name === '__Secure-next-auth.session-token');
    return {
      ok: stage === 'loggedin' && !!(accessToken || stTok),
      stage, accessToken,
      sessionToken: stTok ? stTok.value : null,
      cookies: cookies.map((x) => x.name + '=' + x.value).join('; '),
      userAgent: UA,
      timeSec: ((Date.now() - t0) / 1000).toFixed(1),
      error: stage === 'loggedin' ? null : 'berhenti di tahap: ' + stage,
      log: log.slice(-4),
    };
  } finally { try { c.ws.close(); } catch {} proc.kill('SIGKILL'); }
}

// ---- OAuth harvest (jalur A: cookie Google -> authorization code) ----

const OAuthRedirectURI = 'http://localhost:1455/auth/callback';

function b64url(buf) {
  return Buffer.from(buf).toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function buildAuthorizeURL(challenge, state) {
  const p = new URLSearchParams({
    client_id: process.env.OAUTH_CLIENT_ID || 'app_EMoamEEZ73f0CkXaXp7hrann',
    response_type: 'code',
    redirect_uri: OAuthRedirectURI,
    scope: 'openid profile email offline_access',
    code_challenge: challenge,
    code_challenge_method: 'S256',
    state: state,
    prompt: 'consent',
  });
  return 'https://auth.openai.com/oauth/authorize?' + p.toString();
}

function normalizeCookies(raw) {
  const out = [];
  const push = (name, value, domain) => {
    if (!name) return;
    out.push({ name, value: value == null ? '' : String(value), domain: domain || '.google.com', path: '/', secure: true });
  };
  const t = String(raw || '').trim();
  if (t.startsWith('[')) {
    let arr;
    try { arr = JSON.parse(t); } catch (e) { throw new Error('JSON cookie gak valid: ' + e.message); }
    for (const c of arr) push(c.name, c.value, c.domain || '.google.com');
    return out;
  }
  for (const part of t.split(';')) {
    const i = part.indexOf('=');
    if (i > 0) push(part.slice(0, i).trim(), part.slice(i + 1).trim(), '.google.com');
  }
  if (!out.length) throw new Error('cookie kosong / format gak kebaca');
  return out;
}

async function harvestOAuthCode(cookiesRaw, authUrl, timeoutSec) {
  const t0 = Date.now();
  let cookies;
  try { cookies = normalizeCookies(cookiesRaw); }
  catch (e) { return { ok: false, error: e.message, timeSec: '0' }; }

  const verifier = b64url(require('crypto').randomBytes(48));
  const challenge = b64url(require('crypto').createHash('sha256').update(verifier).digest());
  const state = 'cgt2api-' + b64url(require('crypto').randomBytes(8));
  const url = authUrl && authUrl.length > 10 ? authUrl : buildAuthorizeURL(challenge, state);

  const { proc, wsUrl } = await launch();
  const c = connect(wsUrl);
  let code = null, finalUrl = null, errText = '', sawConsent = false;
  try {
    await c.ready;
    await c.send('Page.enable'); await c.send('Runtime.enable'); await c.send('Network.enable');
    await c.send('Network.setCookies', { cookies });
    await c.send('Page.navigate', { url });
    while (Date.now() - t0 < timeoutSec * 1000) {
      await sleep(1500);
      let cur = '';
      try {
        const r = await c.send('Runtime.evaluate', { expression: 'location.href', returnByValue: true });
        cur = (r.result && r.result.value) || '';
      } catch { continue; }
      finalUrl = cur;
      if (cur.includes('code=')) {
        const m = cur.match(/[?&]code=([^&]+)/);
        if (m) { code = decodeURIComponent(m[1]); break; }
      }
      if (/[?&]error=/.test(cur)) {
        const m = cur.match(/[?&]error=([^&]+)/);
        errText = 'OAuth error: ' + decodeURIComponent(m[1]);
        break;
      }
      if (/auth\.openai\.com\/log-in/.test(cur)) {
        const r = await c.send('Runtime.evaluate', { expression: `document.body.innerText.slice(0,180).replace(/\\n/g,' | ')`, returnByValue: true }).catch(() => null);
        errText = 'OpenAI minta login — cookie Google gak bikin sesi OAuth. Halaman: ' + ((r && r.result && r.result.value) || cur);
        break;
      }
      if (/accounts\.google\.com/.test(cur) && !/oauth2/.test(cur)) {
        const r = await c.send('Runtime.evaluate', { expression: `JSON.stringify({login:!!document.querySelector('input[type="password"],input[name="identifier"],input[type="email"]'),body:document.body.innerText.slice(0,150)})`, returnByValue: true }).catch(() => null);
        const st = r ? JSON.parse(r.result.value || '{}') : {};
        if (st.login) { errText = 'cookie Google expired — Google minta login: ' + (st.body || ''); break; }
      }
      if (/consent|authorize/i.test(cur)) {
        sawConsent = true;
        const r = await c.send('Runtime.evaluate', {
          expression: `(()=>{const b=[...document.querySelectorAll('button')].find(x=>/allow|authorize|continue|accept/i.test(x.innerText.trim()));if(b){b.click();return b.innerText.trim();}return '';})()`,
          returnByValue: true });
        if (r.result.value) { await sleep(2000); continue; }
      }
    }
    return {
      ok: !!code, code, state, verifier, finalUrl, sawConsent,
      error: code ? null : (errText || 'code gak ketangkep dalam batas waktu'),
      timeSec: ((Date.now() - t0) / 1000).toFixed(1),
    };
  } finally { try { c.ws.close(); } catch {} proc.kill('SIGKILL'); }
}

// ---- HTTP server ----

function readBody(req) {
  return new Promise((res) => {
    let b = '';
    req.on('data', (c) => (b += c));
    req.on('end', () => { try { res(JSON.parse(b || '{}')); } catch { res({}); } });
  });
}

const server = http.createServer(async (req, res) => {
  const url = req.url.split('?')[0];
  try {
    if (req.method === 'GET' && url === '/health') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ ok: true, chrome: CHROME }));
      return;
    }
    if (req.method === 'POST' && url === '/solve') {
      const body = await readBody(req);
      const r = await solveCf(body.url || 'https://chatgpt.com/', body.timeoutSec || 90);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(r));
      return;
    }
    if (req.method === 'POST' && url === '/login') {
      const body = await readBody(req);
      if (!body.email || !body.password) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ ok: false, error: 'email & password wajib' }));
        return;
      }
      const r = await loginEmailPw(body.email, body.password, body.timeoutSec || 150);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(r));
      return;
    }
    if (req.method === 'POST' && url === '/oauth-harvest') {
      const body = await readBody(req);
      if (!body.cookies || !body.authUrl) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ ok: false, error: 'cookies & authUrl wajib' }));
        return;
      }
      const r = await harvestOAuthCode(body.cookies, body.authUrl, body.timeoutSec || 120);
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(r));
      return;
    }
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'not found' }));
  } catch (e) {
    res.writeHead(500, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ ok: false, error: e.message }));
  }
});

server.listen(PORT, '127.0.0.1', () => console.log('solver sidecar jalan di 127.0.0.1:' + PORT));
