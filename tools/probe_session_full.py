#!/usr/bin/env python3
"""Gabung chunk session-token (.0,.1,...) persis NextAuth, probe /api/auth/session."""
import json, base64, urllib.request, urllib.error, sys

SRC = sys.argv[1] if len(sys.argv) > 1 else "/root/.hermes/cache/documents/doc_ed8033247638_Cookie-gpt.json"
C = json.load(open(SRC))

# kumpulkan per-name, urutkan chunk
chunks = {}
others = {}
for c in C:
    n, v = c["name"], c["value"]
    if n.startswith("__Secure-next-auth.session-token"):
        suffix = n.split(".")[-1] if "." in n else "0"
        try:
            idx = int(suffix)
        except ValueError:
            idx = 0
        chunks.setdefault(idx, v)
    else:
        others[n] = v

sess_joined = "".join(chunks[i] for i in sorted(chunks))
print("session-token: %d chunk(s), joined len=%d, head=%s" % (len(chunks), len(sess_joined), sess_joined[:24]))

cookie = "; ".join(f"{k}={v}" for k, v in others.items())
if sess_joined:
    for i in sorted(chunks):
        cookie += f"; __Secure-next-auth.session-token.{i}={chunks[i]}"

UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
req = urllib.request.Request("https://chatgpt.com/api/auth/session", headers={
    "User-Agent": UA, "Accept": "application/json", "Cookie": cookie,
    "Referer": "https://chatgpt.com/", "Origin": "https://chatgpt.com",
})
try:
    r = urllib.request.urlopen(req, timeout=60)
    code, raw = r.status, r.read().decode("utf-8", "replace")
except urllib.error.HTTPError as e:
    code, raw = e.code, e.read().decode("utf-8", "replace")
except Exception as e:
    print("NET-ERR", type(e).__name__, str(e)[:200]); sys.exit(2)

print("HTTP", code)
if raw[:1] not in "{[":
    print("NOT-JSON:", raw[:300].replace("\n", " ")); sys.exit(1)
j = json.loads(raw)
u = j.get("user") or {}
at = j.get("accessToken") or ""
print("user:", u.get("name"), "|", u.get("email"), "| plan:", u.get("plan_type") or u.get("intercom_grou_p_plan_type") or "?")
print("accessToken len:", len(at), "| expires:", j.get("expires"))
if at:
    p = at.split(".")[1]; p += "=" * (-len(p) % 4)
    cl = json.loads(base64.urlsafe_b64decode(p))
    import time
    print("JWT: exp=", cl.get("exp"), "(", int((cl.get("exp", 0) - time.time()) / 60), "menit lagi )",
          "| chatgpt_account_is_fraud_retry=", cl.get("chatgpt_account_is_fraud_retry"),
          "| act=", cl.get("act"), "| scope=", cl.get("s"))
    out = {"accessToken": at, "cookies": cookie, "user_agent": UA,
           "email": u.get("email"), "name": u.get("name")}
    with open("/root/chatgpt2API/data/session_fetched.json", "w") as f:
        json.dump(out, f)
    import os
    os.chmod("/root/chatgpt2API/data/session_fetched.json", 0o600)
    print("saved -> data/session_fetched.json (0600, gitignored)")
