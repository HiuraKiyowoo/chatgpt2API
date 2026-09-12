'use strict';
// tes_cookie.cjs — tukar session cookie jadi accessToken via browser ber-CF-clearance.
// Input : JSON array cookie (Cookie-Editor format) ATAU cookie-string.
// Output: JSON {ok, accessToken, email, plan, expiresAt, cookies, error}

const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const RAW = process.argv[2] || '';
const TIMEOUT = parseInt(process.argv[3] || '90', 10);

function parseCookies(raw) {
  raw = (raw || '').trim();
  if (!raw) return [];
  if (raw.startsWith('[')) {
    let arr;
    try { arr = JSON.parse(raw); } catch { return []; }
    return arr.filter((c) => c && c.name).map((c) => ({
      name: c.name, value: c.value || '',
      domain: c.domain || '.chatgpt.com', path: c.path || '/',
    }));
  }
  return raw.split(';').map((s) => s.trim()).filter(Boolean).map((kv) => {
    const i = kv.indexOf('=');
    return { name: kv.slice(0, i), value: kv.slice(i + 1), domain: '.chatgpt.com', path: '/' };
  }).filter((c) => c.name);
}

let seq = 0;
function mkClient(ws) {
  const pending = new Map();
  const events = [];
  ws.addEventListener('message', (ev) => {
    let m; try { m = JSON.parse(ev.data.toString()); } catch { return; }
    if (m.id && pending.has(m.id)) {
      const { resolve, reject } = pending.get(m.id); pending.delete(m.id);
      m.error ? reject(new Error(JSON.stringify(m.error))) : resolve(m.result);
    } else if (m.method) events.push(m);
  });
  return {
    send(method, params = {}, sid) {
      const id = ++seq;
      return new Promise((resolve, reject) => {
        pending.set(id, { resolve, reject });
        ws.send(JSON.stringify(sid ? { id, method, params, sessionId: sid } : { id, method, params }));
        setTimeout(() => { if (pending.has(id)) { pending.delete(id); reject(new Error('timeout ' + method)); } }, 45000);
      });
    },
    events,
  };
}

(async () => {
  const cookies = parseCookies(RAW);
  if (!cookies.length) { console.log(JSON.stringify({ ok: false, error: 'cookie kosong / format gak kebaca' })); return; }

  const udd = fs.mkdtempSync(path.join(os.tmpdir(), 'cg-cookie-'));
  const child = spawn(CHROME, [
    '--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage',
    '--remote-debugging-port=0', `--user-data-dir=${udd}`,
    `--user-agent=${UA}`, '--window-size=1280,900', 'about:blank',
  ], { stdio: ['ignore', 'ignore', 'pipe'] });

  const wsURL = await new Promise((resolve, reject) => {
    let buf = '';
    child.stderr.on('data', (d) => {
      buf += d.toString();
      // DevTools ws dari stderr bisa multi-baris (regex [^\s]+ berhenti di newline -> Invalid URL)
      const m = buf.match(/ws:\/\/\S+?\/devtools\/browser\/[0-9a-f-]+/i) || buf.match(/ws:\/\/\S{10,}/);
      if (m) resolve(m[0].replace(/[\r\n].*$/, ''));
    });
    child.on('exit', (code) => reject(new Error('chromium mati, exit ' + code)));
    setTimeout(() => reject(new Error('chromium gak nyala (timeout)')), 25000);
  });

  const WebSocket = globalThis.WebSocket;
  const ws = new WebSocket(wsURL);
  await new Promise((r, j) => {
    ws.addEventListener('open', r);
    ws.addEventListener('error', (e) => j(new Error('ws error: ' + (e.message || 'gagal connect'))));
  });
  const c = mkClient(ws);

  const { targetId } = await c.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await c.send('Target.attachToTarget', { targetId, flatten: true });
  await c.send('Network.enable', {}, sessionId);
  await c.send('Page.enable', {}, sessionId);
  await c.send('Network.setCookies', { cookies }, sessionId);

  // buka chatgpt.com dulu (lewatin CF challenge), baru ambil session JSON
  await c.send('Page.navigate', { url: 'https://chatgpt.com/' }, sessionId);
  await new Promise((r) => setTimeout(r, 9000));

  let out = { ok: false, error: 'gak dapet session' };
  const r = await c.send('Runtime.evaluate', {
    expression: 'fetch("/api/auth/session",{credentials:"include"}).then(r=>r.text())',
    awaitPromise: true, returnByValue: true,
  }, sessionId);

  const txt = r && r.result && r.result.value;
  if (txt) {
    try {
      const s = JSON.parse(txt);
      if (s && s.accessToken) {
        out = {
          ok: true, accessToken: s.accessToken,
          email: (s.user && s.user.email) || '', plan: (s.user && s.user.planType) || '',
          expires: s.expires || '',
        };
      } else {
        out = { ok: false, error: 'session tanpa accessToken: ' + txt.slice(0, 300) };
      }
    } catch {
      out = { ok: false, error: 'bukan JSON: ' + String(txt).slice(0, 300) };
    }
  }

  // dump cookie hasil (isi ulang cf_clearance)
  try {
    const all = await c.send('Network.getAllCookies');
    out.cookies = (all.cookies || []).map((x) => `${x.name}=${x.value}`).join('; ');
    out.cookieCount = (all.cookies || []).length;
    out.hasCf = (all.cookies || []).some((x) => x.name === 'cf_clearance');
  } catch {}

  console.log(JSON.stringify(out));
  try { ws.close(); } catch {}
  try { child.kill('SIGKILL'); } catch {}
  try { fs.rmSync(udd, { recursive: true, force: true }); } catch {}
})().catch((e) => { console.log(JSON.stringify({ ok: false, error: String(e.message || e) })); process.exit(0); });
