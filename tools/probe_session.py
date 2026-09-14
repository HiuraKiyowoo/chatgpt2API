#!/usr/bin/env python3
"""Probe chatgpt.com/api/auth/session pakai cookie yang di-ekspor user.
Cuma baca 1 endpoint (session); kalau kebuka, cetak siapa akunnya + panjang accessToken.
Tidak ada request ke /backend-api/conversation (nggak ngabisin quota)."""
import json, sys, urllib.request, urllib.error

C = json.load(open("/root/chatgpt2API/data/cookies_local.json"))
by_name = {}
for c in C:
    by_name[c["name"]] = c["value"]

# Gabung chunk session-token (.0 .1 ...) persis NextAuth split
sess = [v for k, v in by_name.items() if k.startswith("__Secure-next-auth.session-token")]
print("session-token chunks:", len(sess), "total len:", sum(len(s) for s in sess))

cookie = "; ".join(f"{k}={v}" for k, v in by_name.items()
                   if not k.startswith("__Secure-next-auth.session-token")) + "; " + "".join(sess)

UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
req = urllib.request.Request(
    "https://chatgpt.com/api/auth/session",
    headers={
        "User-Agent": UA,
        "Accept": "application/json",
        "Cookie": cookie,
        "Referer": "https://chatgpt.com/",
        "Origin": "https://chatgpt.com",
        "oai-language": "en-US",
    })
try:
    r = urllib.request.urlopen(req, timeout=60)
    code, raw = r.status, r.read().decode("utf-8", "replace")
except urllib.error.HTTPError as e:
    code, raw = e.code, e.read().decode("utf-8", "replace")
except Exception as e:
    print("NET-ERR", type(e).__name__, str(e)[:300]); sys.exit(2)

print("HTTP", code, "| content-type-ish:", "JSON" if raw[:1] in "{[" else "HTML/blocked")
if raw[:1] in "{[":
    try:
        j = json.loads(raw)
    except Exception as e:
        print("parse err", e, raw[:200]); sys.exit(1)
    u = j.get("user") or {}
    at = j.get("accessToken") or ""
    print("user:", u.get("name"), "|", u.get("email"), "| plan:", (u.get("plan_type") or u.get("plan") or "?"))
    print("accessToken len:", len(at), "| expires:", j.get("expires"))
    if at:
        import base64
        try:
            p = at.split(".")[1]
            p += "=" * (-len(p) % 4)
            claims = json.loads(base64.urlsafe_b64decode(p))
            print("JWT act:", claims.get("act"), "| chatgpt_did:", str(claims.get("chatgpt_did"))[:8],
                  "| scope:", claims.get("s"), "| exp:", claims.get("exp"))
            json.dump({"accessToken": at, "user": u, "expires": j.get("expires"),
                       "cookies_len": len(cookie)}, open("/root/chatgpt2API/data/session_probe.json", "w"))
            print("saved -> data/session_probe.json (gitignored)")
        except Exception as e:
            print("jwt decode err", e)
else:
    print(raw[:400].replace("\n", " "))
