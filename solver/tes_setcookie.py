#!/usr/bin/env python3
"""tes_setcookie.py — cari field cookie mana yang bikin CDP nolak.
Jalanin Chromium dgn remote-debugging, coba Network.setCookies satu per satu.
"""
import json, subprocess, os, tempfile, time, urllib.request, socket, sys

CHROME = os.environ.get("CHROME_PATH", "/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome")
UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

# port bebas
s = socket.socket(); s.bind(("127.0.0.1", 0)); PORT = s.getsockname()[1]; s.close()
udd = tempfile.mkdtemp(prefix="cgt2-")

p = subprocess.Popen([CHROME, "--headless=new", "--no-sandbox", "--disable-gpu",
                      "--disable-dev-shm-usage", f"--remote-debugging-port={PORT}",
                      f"--user-data-dir={udd}", f"--user-agent={UA}", "about:blank"],
                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

ws_url = None
for _ in range(60):
    try:
        d = json.loads(urllib.request.urlopen(f"http://127.0.0.1:{PORT}/json/version", timeout=2).read())
        ws_url = d.get("webSocketDebuggerUrl"); break
    except Exception:
        time.sleep(0.5)

if not ws_url:
    print("FAIL: chromium gak nyala"); p.kill(); sys.exit(1)

try:
    from websockets.sync.client import connect as ws_connect
except ImportError:
    print("need websockets: pip install websockets"); p.kill(); sys.exit(1)

cookies = json.load(open("/tmp/gc_norm.json"))

def call(ws, method, params=None, sid=None):
    msg = {"id": call.n, "method": method, "params": params or {}}
    if sid: msg["sessionId"] = sid
    ws.send(json.dumps(msg)); call.n += 1
    while True:
        r = json.loads(ws.recv(timeout=30))
        if r.get("id") == msg["id"]:
            return r
call.n = 0

with ws_connect(ws_url, max_size=64 * 1024 * 1024) as ws:
    t = call(ws, "Target.createTarget", {"url": "about:blank"})["result"]["targetId"]
    sid = call(ws, "Target.attachToTarget", {"targetId": t, "flatten": True})["result"]["sessionId"]
    call(ws, "Network.enable", {}, sid)

    print("=== tes per-cookie ===")
    bad = []
    for c in cookies:
        r = call(ws, "Network.setCookies", {"cookies": [c]}, sid)
        if "error" in r:
            bad.append((c["name"], r["error"].get("message")))
            print(f"  GAGAL  {c['name']:22} {r['error'].get('message')}")
        else:
            print(f"  ok     {c['name']}")
    print()
    print("=== tes pakai expires dihilangkan ===")
    stripped = [{k: v for k, v in c.items() if k not in ("expires", "sameSite")} for c in cookies]
    r = call(ws, "Network.setCookies", {"cookies": stripped}, sid)
    print("tanpa expires+sameSite:", "OK semua masuk" if "error" not in r else r["error"].get("message"))

    print("=== tes SATU cookie SID doang ===")
    sid_c = [c for c in cookies if c["name"] == "SID"]
    r = call(ws, "Network.setCookies", {"cookies": sid_c}, sid)
    print("SID doang:", "OK" if "error" not in r else r["error"].get("message"))

    # dump cookie yang beneran kepasang
    got = call(ws, "Network.getAllCookies")["result"]["cookies"]
    print(f"\n=== cookie di browser: {len(got)} ===")
    for c in got:
        print(f"  {c['name']}  dom={c['domain']}  secure={c.get('secure')}  len={len(c.get('value',''))}")

p.kill()
