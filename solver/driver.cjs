'use strict';
/**
 * CDP driver minim (zero-dep, Node 22 built-in WebSocket).
 * Launch Chromium headless baru, navigasi, tunggu challenge Cloudflare lewat,
 * dump cookies via CDP.
 *
 * Pakai: node driver.js <url> [timeoutDetik]
 * Keluar: JSON {ok, title, cookies, error}
 */
const { spawn } = require('child_process');
const http = require('http');

const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

async function launch() {
  const proc = spawn(CHROME, [
    '--headless=new', '--no-sandbox', '--disable-gpu',
    '--remote-debugging-port=0',
    '--window-size=1280,900',
    '--user-agent=' + UA,
    '--disable-blink-features=AutomationControlled',
    'about:blank',
  ], { stdio: ['ignore', 'ignore', 'pipe'] });
  const wsUrl = await new Promise((res, rej) => {
    let buf = '';
    const t = setTimeout(() => rej(new Error('chrome tak expose DevTools (timeout)')), 20000);
    proc.stderr.on('data', (d) => {
      buf += d.toString();
      const m = buf.match(/DevTools listening on (ws:\/\/\S+)/);
      if (m) { clearTimeout(t); res(m[1]); }
    });
    proc.on('exit', (c) => { clearTimeout(t); rej(new Error('chrome exit ' + c)); });
  });
  // wsUrl menunjuk browser endpoint; ambil page target
  const debugPort = wsUrl.match(/:(\d+)\//)[1];
  const target = await new Promise((res, rej) => {
    http.get({ host: '127.0.0.1', port: debugPort, path: '/json/list' }, (r) => {
      let b = '';
      r.on('data', (c) => (b += c));
      r.on('end', () => {
        const list = JSON.parse(b).filter((t) => t.type === 'page');
        res(list[0]);
      });
    }).on('error', rej);
  });
  return { proc, wsUrl: target.webSocketDebuggerUrl, port: debugPort };
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
  const ready = new Promise((res, rej) => { ws.onopen = res; ws.onerror = () => rej(new Error('WS error')); });
  return { ws, send, ready };
}

async function main() {
  const url = process.argv[2] || 'https://chatgpt.com/';
  const timeoutMs = (parseInt(process.argv[3] || '90', 10)) * 1000;
  const t0 = Date.now();
  const { proc, wsUrl } = await launch();
  const c = connect(wsUrl);
  try {
    await c.ready;
    await c.send('Page.enable');
    await c.send('Network.enable');
    await c.send('Runtime.enable');
    await c.send('Page.navigate', { url });
    let passed = false;
    let title = '';
    while (Date.now() - t0 < timeoutMs) {
      await sleep(3000);
      try {
        const r = await c.send('Runtime.evaluate', { expression: 'document.title + "|" + document.body.innerText.slice(0,200)', returnByValue: true });
        const txt = (r.result && r.result.value) || '';
        title = txt.split('|')[0] || '';
        const challenge = /just a moment|checking your browser|attention required/i.test(txt) || /cf_chl/.test(txt);
        if (!challenge && title) { passed = true; break; }
      } catch (e) { /* page navigating */ }
    }
    const ck = await c.send('Network.getAllCookies').catch(() => null);
    const cookies = ck ? ck.cookies : [];
    const cf = cookies.find((x) => x.name === 'cf_clearance');
    console.log(JSON.stringify({
      ok: passed,
      title,
      timeSec: ((Date.now() - t0) / 1000).toFixed(1),
      cf_clearance: cf ? cf.value : null,
      cookieCount: cookies.length,
      cookieString: cookies.map((x) => x.name + '=' + x.value).join('; '),
      error: passed ? null : 'challenge belum lewat dalam batas waktu',
    }, null, 1));
  } finally {
    try { c.ws.close(); } catch {}
    proc.kill('SIGKILL');
  }
}

main().catch((e) => { console.log(JSON.stringify({ ok: false, error: e.message })); process.exit(1); });
