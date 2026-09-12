'use strict';
/**
 * harvest_manual.cjs — panen code OAuth TANPA lewat backend.
 * Pakai cookie Google dari /tmp/google_cookies.json, generate PKCE sendiri,
 * print code + verifier biar bisa ditukar manual.
 */
const { spawn } = require('child_process');
const fs = require('fs'); const os = require('os'); const path = require('path'); const crypto = require('crypto');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const CLIENT = 'app_EMoamEEZ73f0CkXaXp7hrann';
const REDIRECT = 'http://localhost:1455/auth/callback';

const b64 = (b) => Buffer.from(b).toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
const verifier = b64(crypto.randomBytes(48));
const challenge = b64(crypto.createHash('sha256').update(verifier).digest());
const state = 'cgt2api-' + b64(crypto.randomBytes(8));

const p = new URLSearchParams({
  client_id: CLIENT, response_type: 'code', redirect_uri: REDIRECT,
  scope: 'openid profile email offline_access',
  code_challenge: challenge, code_challenge_method: 'S256',
  state, prompt: 'consent',
});
const AUTH_URL = 'https://auth.openai.com/oauth/authorize?' + p.toString();

function normalize(raw) {
  const arr = JSON.parse(raw);
  return arr.map((c) => {
    const o = { name: c.name, value: String(c.value ?? ''), domain: c.domain || '.google.com',
                path: c.path || '/', secure: !!c.secure, httpOnly: !!c.httpOnly };
    if (/^__Secure-|^__Host-/.test(c.name)) o.secure = true;
    const e = c.expires != null ? c.expires : c.expirationDate;
    if (e != null && !isNaN(Number(e))) o.expires = Math.floor(Number(e));
    const ss = c.sameSite;
    if (ss === 'strict' || ss === 'lax' || ss === 'none') o.sameSite = ss;
    else if (ss === 'no_restriction') o.sameSite = 'none';
    return o;
  });
}

let seq = 0;
function mk(ws) {
  const pend = new Map();
  ws.onmessage = (ev) => {
    let m; try { m = JSON.parse(ev.data.toString()); } catch { return; }
    if (m.id && pend.has(m.id)) { const x = pend.get(m.id); pend.delete(m.id); m.error ? x.reject(new Error(JSON.stringify(m.error))) : x.resolve(m.result); }
  };
  return { send(method, params = {}, sid) { const id = ++seq; return new Promise((res, rej) => { pend.set(id, { resolve: res, reject: rej }); ws.send(JSON.stringify(sid ? { id, method, params, sessionId: sid } : { id, method, params })); setTimeout(() => { if (pend.has(id)) { pend.delete(id); rej(new Error('timeout ' + method)); } }, 40000); }); } };
}

(async () => {
  const cookies = normalize(fs.readFileSync(process.argv[2] || '/tmp/google_cookies.json', 'utf8'));
  const udd = fs.mkdtempSync(path.join(os.tmpdir(), 'cg-man-'));
  const child = spawn(CHROME, ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage',
    '--remote-debugging-port=0', `--user-data-dir=${udd}`, `--user-agent=${UA}`, '--window-size=1280,900', 'about:blank'],
    { stdio: ['ignore', 'ignore', 'pipe'] });
  const wsURL = await new Promise((res, rej) => {
    let b = '';
    child.stderr.on('data', d => { b += d.toString(); const m = b.match(/ws:\/\/\S+?\/devtools\/browser\/[0-9a-f-]+/i); if (m) res(m[0]); });
    child.on('exit', c => rej(new Error('chromium exit ' + c)));
    setTimeout(() => rej(new Error('timeout chromium')), 20000);
  });
  const ws = new WebSocket(wsURL);
  await new Promise((r, j) => { ws.onopen = r; ws.onerror = () => j(new Error('ws gagal')); });
  const c = mk(ws);
  const { targetId } = await c.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await c.send('Target.attachToTarget', { targetId, flatten: true });
  await c.send('Network.enable', {}, sessionId);
  await c.send('Page.enable', {}, sessionId);

  const set = await c.send('Network.setCookies', { cookies }, sessionId);
  console.log('setCookies:', JSON.stringify(set).slice(0, 120));

  // bukti cookie Google beneran login: buka akun Google
  await c.send('Page.navigate', { url: 'https://myaccount.google.com/' }, sessionId);
  await new Promise(r => setTimeout(r, 7000));
  const acc = await c.send('Runtime.evaluate', { expression: 'location.href.slice(0,90) + " || " + document.title.slice(0,80)', returnByValue: true }, sessionId);
  console.log('cekJadiGoogle:', acc.result && acc.result.value);

  const all = await c.send('Network.getAllCookies');
  const gc = (all.cookies || []).filter(x => /google/.test(x.domain));
  console.log('cookie google kepasang:', gc.length, gc.map(x => x.name).join(',').slice(0, 160));

  // authorize
  await c.send('Page.navigate', { url: AUTH_URL }, sessionId);
  let code = null, finalUrl = '', hops = [];
  const t0 = Date.now();
  while (Date.now() - t0 < 90000) {
    await new Promise(r => setTimeout(r, 1500));
    let cur = '';
    try { const r = await c.send('Runtime.evaluate', { expression: 'location.href', returnByValue: true }, sessionId); cur = (r.result && r.result.value) || ''; } catch { continue; }
    if (cur && cur !== hops[hops.length - 1]) { hops.push(cur); console.log('  hop:', cur.slice(0, 110)); }
    finalUrl = cur;
    const m = cur.match(/[?&]code=([^&]+)/);
    if (m) { code = decodeURIComponent(m[1]); break; }
    if (/[?&]error=/.test(cur)) { console.log('  OAuth error:', cur.slice(0, 200)); break; }
    if (/auth\.openai\.com\/log-in/.test(cur)) break;
  }

  console.log('\nfinalUrl:', finalUrl.slice(0, 140));
  if (code) {
    console.log('CODE DIDAPAT:', code.slice(0, 60) + '...');
    console.log('VERIFIER:', verifier);
    fs.writeFileSync('/tmp/oauth_result.json', JSON.stringify({ code, verifier, state }, null, 2));
  } else {
    const body = await c.send('Runtime.evaluate', { expression: 'document.body.innerText.slice(0,250).replace(/\\n/g," | ")', returnByValue: true }, sessionId).catch(() => null);
    console.log('TANPA CODE. Halaman:', body && body.result && body.result.value);
  }

  ws.close(); child.kill('SIGKILL'); fs.rmSync(udd, { recursive: true, force: true });
})().catch(e => { console.log('FATAL:', e.message); process.exit(0); });
