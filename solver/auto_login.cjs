'use strict';
/**
 * auto_login.cjs — full login email+password via browser CDP.
 * Kembalikan JSON: {ok, sessionToken, accessToken, cookies, error}
 * Pakai: CHROME_PATH=... node auto_login.cjs <email> <password> [timeoutSec]
 */
const { spawn } = require('child_process');
const http = require('http');
const fs = require('fs');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

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
  });
  const debugPort = wsUrl.match(/:(\d+)\//)[1];
  const target = await new Promise((res, rej) => {
    http.get({ host: '127.0.0.1', port: debugPort, path: '/json/list' }, (r) => {
      let b = '';
      r.on('data', (c) => (b += c));
      r.on('end', () => res(JSON.parse(b).filter((t) => t.type === 'page')[0]));
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
  arkose: !!document.querySelector('iframe[src*="arkoselabs"],iframe[src*="funcaptcha"],iframe[title*="challenge"]'),
  turnstile: !!document.querySelector('iframe[src*="challenges.cloudflare"]'),
  err: (document.querySelector('[role="alert"],.error,.text-red')||{}).innerText||'',
  body: document.body.innerText.slice(0, 250)})`;

async function fillInput(c, sel, val) {
  await c.send('Runtime.evaluate', { expression: `
    const el=document.querySelector(${JSON.stringify(sel)});
    const set=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,'value').set;
    set.call(el, ${JSON.stringify(val)});
    el.dispatchEvent(new Event('input',{bubbles:true}));
    el.dispatchEvent(new Event('change',{bubbles:true}));` });
}

async function main() {
  const [email, password] = [process.argv[2], process.argv[3]];
  const timeoutMs = (parseInt(process.argv[4] || '150', 10)) * 1000;
  if (!email || !password) { console.log(JSON.stringify({ ok: false, error: 'pakai: node auto_login.cjs <email> <password>' })); return; }
  const t0 = Date.now();
  const { proc, wsUrl } = await launch();
  const c = connect(wsUrl);
  const log = [];
  try {
    await c.ready;
    await c.send('Page.enable'); await c.send('Runtime.enable'); await c.send('Network.enable');
    await c.send('Page.navigate', { url: 'https://chatgpt.com/auth/login' });

    let stage = 'loading';
    while (Date.now() - t0 < timeoutMs) {
      await sleep(3000);
      let st;
      try { st = await evalJSON(c, SNAP); } catch (e) { continue; }
      log.push(st);

      if (st.arkose) { stage = 'arkose'; break; }
      if (st.turnstile && !st.email && !st.password) { stage = 'turnstile'; continue; }

      // STEP 1: form email — pakai tombol submit, BUKAN "Continue with Google"
      if (st.email && !st.password) {
        await fillInput(c, 'input[name="email"],input[type="email"]', email);
        await sleep(700);
        await c.send('Runtime.evaluate', { expression: `
          const f=document.querySelector('form');
          const b=[...document.querySelectorAll('button[type="submit"]')].find(x=>/continue/i.test(x.innerText.trim()));
          if(b) b.click(); else if(f) f.requestSubmit? f.requestSubmit(): f.submit();` });
        await sleep(3500);
        continue;
      }

      // STEP 2: form password — sama, tombol submit saja
      if (st.password) {
        await fillInput(c, 'input[type="password"]', password);
        await sleep(700);
        await c.send('Runtime.evaluate', { expression: `
          const f=document.querySelector('form');
          const b=[...document.querySelectorAll('button[type="submit"]')].find(x=>/continue|log in|sign in/i.test(x.innerText.trim()));
          if(b) b.click(); else if(f) f.requestSubmit? f.requestSubmit(): f.submit();` });
        await sleep(4000);
        continue;
      }

      // STEP 3: logged-in detection
      if (st.url.indexOf('chatgpt.com/') === 8 && st.url.indexOf('/auth') === -1 && !st.email && !st.password && st.title && !/log in|get started/i.test(st.title)) {
        stage = 'loggedin'; break;
      }
    }

    // ambil kredensial hasil login
    let sessionToken = null, accessToken = null;
    const sess = await c.send('Runtime.evaluate', {
      expression: `fetch('/api/auth/session',{headers:{'Accept':'application/json'}}).then(r=>r.json()).then(j=>JSON.stringify(j))`,
      returnByValue: true, awaitPromise: true,
    }).catch(() => null);
    if (sess && sess.result && sess.result.value && sess.result.value !== 'null') {
      try { accessToken = JSON.parse(sess.result.value).accessToken || null; } catch {}
    }
    const ck = await c.send('Network.getAllCookies').catch(() => null);
    const cookies = ck ? ck.cookies : [];
    const st2 = cookies.find((x) => x.name === '__Secure-next-auth.session-token');
    if (st2) sessionToken = st2.value;
    const cf = cookies.find((x) => x.name === 'cf_clearance');

    console.log(JSON.stringify({
      ok: stage === 'loggedin' || !!accessToken || !!sessionToken,
      stage, timeSec: ((Date.now() - t0) / 1000).toFixed(0),
      accessToken: accessToken ? accessToken.slice(0, 40) + '...' : null,
      sessionToken: sessionToken ? sessionToken.slice(0, 24) + '...' : null,
      cf_clearance: cf ? cf.value : null,
      cookies: cookies.map((x) => x.name + '=' + x.value).join('; '),
      log: log.slice(-6),
    }, null, 1));
  } finally {
    try { c.ws.close(); } catch {}
    proc.kill('SIGKILL');
  }
}
main().catch((e) => { console.log(JSON.stringify({ ok: false, error: e.message })); process.exit(1); });
