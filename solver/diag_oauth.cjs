'use strict';
/**
 * diag_oauth.cjs — diagnosa: kenapa authorize OpenAI gak lempar ke Google.
 * Cek cookie Google kepasang apa gak di browser, dan apa yang Google balikin.
 */
const { spawn } = require('child_process');
const fs = require('fs'); const os = require('os'); const path = require('path');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const cookies = JSON.parse(fs.readFileSync('/tmp/google_cookies.json', 'utf8'))
  .map(c => ({ name: c.name, value: c.value, domain: c.domain || '.google.com', path: c.path || '/' }));

let seq = 0;
function mk(ws) {
  const p = new Map();
  ws.onmessage = (ev) => {
    let m; try { m = JSON.parse(ev.data.toString()); } catch { return; }
    if (m.id && p.has(m.id)) { const x = p.get(m.id); p.delete(m.id); m.error ? x.reject(new Error(JSON.stringify(m.error))) : x.resolve(m.result); }
  };
  return { send(method, params = {}, sid) { const id = ++seq; return new Promise((res, rej) => { p.set(id, { resolve: res, reject: rej }); ws.send(JSON.stringify(sid ? { id, method, params, sessionId: sid } : { id, method, params })); setTimeout(() => { if (p.has(id)) { p.delete(id); rej(new Error('timeout ' + method)); } }, 30000); }); } };
}

(async () => {
  const udd = fs.mkdtempSync(path.join(os.tmpdir(), 'cg-diag-'));
  const child = spawn(CHROME, ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage',
    '--remote-debugging-port=0', `--user-data-dir=${udd}`, `--user-agent=${UA}`, '--window-size=1280,900', 'about:blank'],
    { stdio: ['ignore', 'ignore', 'pipe'] });
  const wsURL = await new Promise((res, rej) => {
    let b = '';
    child.stderr.on('data', d => { b += d.toString(); const m = b.match(/ws:\/\/\S+?\/devtools\/browser\/[0-9a-f-]+/i); if (m) res(m[0]); });
    child.on('exit', c => rej(new Error('chromium exit ' + c)));
    setTimeout(() => rej(new Error('timeout')), 20000);
  });
  const ws = new WebSocket(wsURL);
  await new Promise((r, j) => { ws.onopen = r; ws.onerror = () => j(new Error('ws gagal')); });
  const c = mk(ws);
  const { targetId } = await c.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await c.send('Target.attachToTarget', { targetId, flatten: true });
  await c.send('Network.enable', {}, sessionId);
  await c.send('Page.enable', {}, sessionId);

  // 1. set cookie Google
  const set = await c.send('Network.setCookies', { cookies }, sessionId);
  console.log('1) setCookies:', JSON.stringify(set));

  // 2. tes cookie kepasang gak: buka google.com/myaccount
  await c.send('Page.navigate', { url: 'https://www.google.com/' }, sessionId);
  await new Promise(r => setTimeout(r, 6000));
  const all = await c.send('Network.getAllCookies');
  const gc = (all.cookies || []).filter(x => /google/.test(x.domain));
  console.log('2) cookie google di browser:', gc.length, '->', gc.map(x => x.name).join(',').slice(0, 200));

  const gp = await c.send('Runtime.evaluate', { expression: 'fetch("/search?q=test",{credentials:"include"}).then(r=>r.status+" | "+(r.url||"").slice(0,120))', awaitPromise: true, returnByValue: true }, sessionId);
  console.log('3) google search status:', gp.result && gp.result.value);

  // 3. buka authorize OpenAI, lihat ke mana
  const authURL = process.env.AUTH_URL;
  if (authURL) {
    await c.send('Page.navigate', { url: authURL }, sessionId);
    await new Promise(r => setTimeout(r, 12000));
    const where = await c.send('Runtime.evaluate', { expression: 'location.href + " || " + document.title + " || " + document.body.innerText.slice(0,300)', returnByValue: true }, sessionId);
    console.log('4) setelah authorize ->', where.result && where.result.value);
  }

  ws.close(); child.kill('SIGKILL'); fs.rmSync(udd, { recursive: true, force: true });
})().catch(e => { console.log('FATAL:', e.message); process.exit(0); });
