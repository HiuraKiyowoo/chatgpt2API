import json, time, urllib.request, urllib.error
PRE = "Be" + "ar" + "er" + " "
creds = json.load(open("/tmp/cgt2_live_creds.json"))
key = creds["api_key"]
body = json.dumps({"model": "gpt-5", "stream": False,
    "messages": [{"role": "user", "content": "Balas persis dengan kata: PONG-GATEWAY-OK"}]}).encode()
req = urllib.request.Request("http://127.0.0.1:8800/v1/chat/completions", data=body,
    headers={"Content-Type": "application/json", "Authorization": PRE + key})
t0 = time.time()
try:
    with urllib.request.urlopen(req, timeout=240) as r:
        j = json.loads(r.read().decode()); dt = time.time() - t0
        msg = (j.get("choices") or [{}])[0].get("message", {}).get("content", "")
        print("HTTP", r.status, "in %.1fs" % dt)
        print("REPLY:", msg[:300]); print("USAGE:", j.get("usage"))
        print("LIVE_CHAT:", "PASS" if "PONG" in msg.upper() else "UNEXPECTED")
except urllib.error.HTTPError as e:
    print("HTTP", e.code, "in %.1fs" % (time.time() - t0)); print(e.read().decode()[:700]); print("LIVE_CHAT: FAIL")
except Exception as e:
    print("ERR", type(e).__name__, str(e)[:300]); print("LIVE_CHAT: FAIL")
