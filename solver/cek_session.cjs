'use strict';
// cek_session.cjs — diagnosa cookie: buka chatgpt.com, dump hasil fetch beberapa endpoint.
const { spawn } = require('child_process');
const fs = require('fs'); const os = require('os'); const path = require('path');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const RAW = fs.readFileSync('/tmp/cookie_raw.txt', 'utf8').trim();

function parseCookies(raw) {
  return raw.split(';').map(s => s.trim()).filter(Boolean).map(kv => {
    const i = kv.indexOf('=');
    return { name: kv.slice(0, i), value: kv.slice(i + 1), domain: '.chatgpt.com', path: '/' };
  }).filter(c => c.name);
}

let seq = 0;
function mkClient(ws) {
  const pending = new Map();
  ws.onmessage = (ev) => {
    let m; try { m = JSON.parse(ev.data.toString()); } catch { return; }
    if (m.id && pending.has(m.id)) {
      const p = pending.get(m.id); pending.delete(m.id);
      m.error ? p.reject(new Error(JSON.stringify(m.error))) : p.resolve(m.result);
    }
  };
  return {
    send(method, params = {}, sid) {
      const id = ++seq;
      return new Promise((res, rej) => {
        pending.set(id, { resolve: res, reject: rej });
        ws.send(JSON.stringify(sid ? { id, method, params, sessionId: sid } : { id, method, params }));
        setTimeout(() => { if (pending.has(id)) { pending.delete(id); rej(new Error('timeout ' + method)); } }, 40000);
      });
    },
  };
}

(async () => {
  const cookies = parseCookies(RAW);
  console.log('cookies dikirim:', cookies.map(c => c.name).join(', '));
  const udd = fs.mkdtempSync(path.join(os.tmpdir(), 'cg-cek-'));
  const child = spawn(CHROME, ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage',
    '--remote-debugging-port=0', `--user-data-dir=${udd}`, `--user-agent=${UA}`, '--window-size=1280,900', 'about:blank'],
    { stdio: ['ignore', 'ignore', 'pipe'] });

  const wsURL = await new Promise((res, rej) => {
    let buf = '';
    child.stderr.on('data', d => { buf += d.toString(); const m = buf.match(/ws:\/\/\S+?\/devtools\/browser\/[0-9a-f-]+/i); if (m) res(m[0]); });
    child.on('exit', c => rej(new Error('chromium exit ' + c)));
    setTimeout(() => rej(new Error('timeout chromium')), 20000);
  });

  const ws = new WebSocket(wsURL);
  await new Promise((r, j) => { ws.onopen = r; ws.onerror = () => j(new Error('ws gagal')); });
  const c = mkClient(ws);
  const { targetId } = await c.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await c.send('Target.attachToTarget', { targetId, flatten: true });
  await c.send('Network.enable', {}, sessionId);
  await c.send('Page.enable', {}, sessionId);
  const set = await c.send('Network.setCookies', { cookies }, sessionId);
  console.log('setCookies:', JSON.stringify(set));
  await c.send('Page.navigate', { url: 'https://chatgpt.com/' }, sessionId);
  await new Promise(r => setTimeout(r, 9000));

  const ev = async (expr, label) => {
    try {
      const r = await c.send('Runtime.evaluate', { expression: `(async()=>{try{const r=await fetch(${JSON.stringify(expr)},{credentials:"include"});return r.status+" | "+((await r.text())||"").slice(0,400)}catch(e){return "ERR "+e.message}})()`, awaitPromise: true, returnByValue: true }, sessionId);
      console.log(`\n[${label}]`, r.result && r.result.value);
    } catch (e) { console.log(`\n[${label}] GAGAL:`, e.message); }
  };
  await ev('/api/auth/session', 'session');
  await ev('/backend-api/me', 'me');
  const who = await c.send('Runtime.evaluate', { expression: 'location.href + " || " + document.title', returnByValue: true }, sessionId);
  console.log('\n[halaman]', who.result.value);

  console.log('\n--- cookies di browser setelah load ---');
  const all = await c.send('Network.getAllCookies');
  const names = (all.cookies || []).filter(x => /chatgpt|openai|auth|oai|cf_/.test(x.domain + x.name)).map(x => `${x.name}(${x.domain})`);
  console.log(names.join('\n'));

  ws.close(); child.kill('SIGKILL'); fs.rmSync(udd, { recursive: true, force: true });
})().catch(e => { console.log('FATAL:', e.message); process.exit(0); });
