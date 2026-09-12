'use strict';
/* ketik_real.cjs — login pakai REAL keystroke (Input.dispatchKeyEvent), bukan JS value.
   Kalau tetap "Incorrect", berarti emang salah; kalau sukses, berarti JS-fill tadi gak kebaca React. */
const { spawn } = require('child_process');
const http = require('http');
const CHROME = process.env.CHROME_PATH || '/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome';
const UA = 'Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function launch() {
  const proc = spawn(CHROME, ['--headless=new','--no-sandbox','--disable-gpu','--remote-debugging-port=0','--window-size=1280,900','--user-agent='+UA,'--disable-blink-features=AutomationControlled','about:blank'], { stdio:['ignore','ignore','pipe'] });
  const wsUrl = await new Promise((res, rej) => { let buf=''; const t=setTimeout(()=>rej(new Error('no devtools')),20000);
    proc.stderr.on('data',(d)=>{buf+=d;const m=buf.match(/DevTools listening on (ws:\/\/\S+)/);if(m){clearTimeout(t);res(m[1]);}});});
  const debugPort = wsUrl.match(/:(\d+)\//)[1];
  const target = await new Promise((res, rej) => { http.get({host:'127.0.0.1',port:debugPort,path:'/json/list'},(r)=>{let b='';r.on('data',(c)=>(b+=c));r.on('end',()=>res(JSON.parse(b).filter(t=>t.type==='page')[0]));}).on('error',rej); });
  return { proc, wsUrl: target.webSocketDebuggerUrl };
}
function connect(wsUrl) {
  const ws = new WebSocket(wsUrl); let id=0; const pending=new Map();
  ws.onmessage=(ev)=>{const m=JSON.parse(ev.data);if(m.id&&pending.has(m.id)){pending.get(m.id)(m);pending.delete(m.id);}};
  const send=(method,params={})=>new Promise((res,rej)=>{const mid=++id;pending.set(mid,(m)=>(m.error?rej(new Error(m.error.message)):res(m.result)));ws.send(JSON.stringify({id:mid,method,params}));});
  const ready=new Promise((res,rej)=>{ws.onopen=res;ws.onerror=()=>rej(new Error('WS'));});
  return { ws, send, ready };
}

// ketik per karakter via CDP (keyDown+char+keyUp) — keliatan keyboard asli
async function typeText(c, text) {
  for (const ch of text) {
    const shift = /[A-Z!@#$%^&*()_+{}|:"<>?~]/.test(ch);
    const key = /[A-Za-z]/.test(ch) ? ch.toUpperCase() : ch;
    const base = { key: ch, code: /[A-Z]/.test(ch) ? 'Key'+ch : (/[a-z]/.test(ch)?'Key'+ch.toUpperCase():''), windowsVirtualKeyCode: 0 };
    if (/[A-Za-z0-9]/.test(ch)) {
      await c.send('Input.dispatchKeyEvent', { type:'keyDown', ...base, modifiers: shift?2:0 });
      await c.send('Input.dispatchKeyEvent', { type:'char', text: ch });
      await c.send('Input.dispatchKeyEvent', { type:'keyUp', ...base, modifiers: shift?2:0 });
    } else {
      // simbol non-alnum: char event saja (paling kompatibel)
      await c.send('Input.dispatchKeyEvent', { type:'keyDown', key: ch, text: ch, unmodifiedText: ch });
      await c.send('Input.dispatchKeyEvent', { type:'char', text: ch });
      await c.send('Input.dispatchKeyEvent', { type:'keyUp', key: ch });
    }
    await sleep(35 + Math.random()*55);
  }
}

// fokus input via koordinat box
async function clickEl(c, sel) {
  const r = await c.send('Runtime.evaluate', { expression: `
    (()=>{ const el=document.querySelector(${JSON.stringify(sel)});
      if(!el) return ''; el.scrollIntoView(); const b=el.getBoundingClientRect();
      return JSON.stringify({x:b.x+b.width/2, y:b.y+b.height/2}); })()`, returnByValue: true });
  if (!r.result.value) throw new Error('elemen tidak ada: ' + sel);
  const { x, y } = JSON.parse(r.result.value);
  await c.send('Input.dispatchMouseEvent', { type:'mousePressed', x, y, button:'left', clickCount:1 });
  await c.send('Input.dispatchMouseEvent', { type:'mouseReleased', x, y, button:'left', clickCount:1 });
}

async function snap(c) {
  const r = await c.send('Runtime.evaluate', { expression: `JSON.stringify({
    url:location.href, title:document.title,
    email:!!document.querySelector('input[name="email"],input[type="email"]'),
    password:!!document.querySelector('input[type="password"]'),
    otp:!!document.querySelector('input[inputmode="numeric"],input[autocomplete="one-time-code"]'),
    err:(document.querySelector('[role="alert"],[class*=rror]')||{}).innerText||'',
    body:document.body.innerText.slice(0,200)})`, returnByValue: true });
  return JSON.parse(r.result.value);
}

async function main() {
  const [email, password] = [process.argv[2], process.argv[3]];
  const { proc, wsUrl } = await launch();
  const c = connect(wsUrl);
  try {
    await c.ready;
    await c.send('Page.enable'); await c.send('Runtime.enable');
    await c.send('Page.navigate', { url: 'https://chatgpt.com/auth/login' });
    // tunggu form email (max 40s)
    let st = null;
    for (let i=0;i<14;i++){ await sleep(3000); st = await snap(c); if (st.email) break; }
    if (!st || !st.email) { console.log('FORM EMAIL GAK MUNCUL:', JSON.stringify(st)); return; }
    console.log('[a] form email muncul, klik + ketik...');
    await clickEl(c, 'input[name="email"],input[type="email"]');
    await sleep(400);
    await typeText(c, email);
    await sleep(600);
    // VERIFIKASI nilai beneran kebaca React
    const v1 = await c.send('Runtime.evaluate', { expression: `(document.querySelector('input[name="email"],input[type="email"]')||{}).value`, returnByValue: true });
    console.log('[b] nilai input email =', JSON.stringify(v1.result.value));
    await c.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('button[type="submit"]')].find(x=>/continue/i.test(x.innerText.trim())).click()` });
    // tunggu step password / otp / error
    let s2 = null;
    for (let i=0;i<12;i++){ await sleep(3000); s2 = await snap(c); if (s2.password || s2.otp || s2.err) break; }
    console.log('[c] step2:', s2.url.slice(0,60), '| pass:', !!s2.password, '| otp:', !!s2.otp, '| err:', (s2.err||'').slice(0,60));
    if (s2.password) {
      await clickEl(c, 'input[type="password"]');
      await sleep(400);
      await typeText(c, password);
      const v2 = await c.send('Runtime.evaluate', { expression: `(document.querySelector('input[type="password"]')||{}).value.length`, returnByValue: true });
      console.log('[d] panjang password terisi =', v2.result.value, '(harus', password.length + ')');
      await c.send('Runtime.evaluate', { expression: `[...document.querySelectorAll('button[type="submit"]')].find(x=>/continue|log in/i.test(x.innerText.trim())).click()` });
      let s3 = null;
      for (let i=0;i<10;i++){ await sleep(3000); s3 = await snap(c);
        if (s3.otp) { console.log('[e] OTP dibutuhkan:', s3.body.slice(0,120)); break; }
        if (/incorrect|invalid|lockout|too many/i.test(s3.err + s3.body)) { console.log('[e] DITOLAK:', (s3.err||'').slice(0,100) || s3.body.slice(0,140)); break; }
        if (s3.url.includes('chatgpt.com/') && !s3.url.includes('/auth')) { console.log('[e] LOGGED IN ->', s3.url.slice(0,60));
          const sess = await c.send('Runtime.evaluate', { expression:`fetch('/api/auth/session').then(r=>r.json()).then(j=>JSON.stringify({has:!!j.accessToken,user:(j.user&&j.user.emailAddress)||j.user&&j.user.name||''}))`, returnByValue:true, awaitPromise:true});
          console.log('[f] session:', sess.result.value); break; }
      }
      if (!s3) console.log('[e] timeout'); else if (!/OTP|DITOLAK|LOGGED/.test('')) {}
      else {}
      if (s3) console.log('[e] akhir:', s3.url.slice(0,60), '|', s3.body.slice(0,100).replace(/\n/g,' | '));
    }
  } finally { try{c.ws.close();}catch{} proc.kill('SIGKILL'); }
}
main().catch(e=>{console.log('ERR', e.message);process.exit(1);});
