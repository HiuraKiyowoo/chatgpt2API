#!/usr/bin/env python3
"""Setup live gateway config + seed akun Robin dari hasil fetch session."""
import json, os, secrets, subprocess, sys, time, urllib.request, urllib.error

ROOT = "/root/chatgpt2API"
# config.yaml sudah ditulis make_live_config.py — baca password-nya, jangan bangun ulang
cfg = open(f"{ROOT}/config.yaml").read()
pw = [l.split('"')[1] for l in cfg.splitlines() if "password:" in l][0]
admin_user = [l.split(":")[1].strip() for l in cfg.splitlines() if "username:" in l][0].strip('"')
ses = json.load(open(f"{ROOT}/data/session_fetched.json"))

def api(path, obj, token=None, method="POST", base="http://127.0.0.1:8800"):
    data = json.dumps(obj).encode() if obj is not None else None
    req = urllib.request.Request(base + path, data=data, method=method,
                                 headers={"Content-Type": "application/json"})
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=120) as r:
            return r.status, json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode())
        except Exception:
            return e.code, {"raw": "non-json"}
    except Exception as e:
        return -1, {"err": str(e)[:200]}

# login admin
st, j = api("/api/admin/login", {"username": admin_user, "password": pw})
print("admin login:", st)
if st != 200:
    print("gateway belum siap / salah pass:", j); sys.exit(1)
tok = j["token"]

# seed akun: accessToken + cookies lengkap + UA (wajib sama dgn saat fetch)
st, j = api("/api/accounts", {
    "label": "Robin (robinvschina@gmail.com) — session cookie",
    "accessToken": ses["accessToken"],
    "cookies": ses["cookies"],
    "userAgent": ses["user_agent"],
    "checkNow": True,
}, tok)
print("add account:", st, {k: j.get(k) for k in ("id", "status", "detail")})
acc_id = j.get("id", "")

# create downstream api key
st, j = api("/api/keys", {"name": "merlin"}, tok)
api_key = j.get("key", "")
print("api key:", st, (api_key[:6] + "..." if api_key else j))

json.dump({"admin_pw_len": len(pw), "api_key": api_key, "account_id": acc_id},
          open("/tmp/cgt2_live_creds.json", "w"))
os.chmod("/tmp/cgt2_live_creds.json", 0o600)
print("creds saved to /tmp/cgt2_live_creds.json (600)")
