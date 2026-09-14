#!/usr/bin/env python3
"""panen_oauth.py — kirim cookie Google ke endpoint oauth-harvest, tampilkan hasil."""
import json, urllib.request, urllib.error, time, sys

BASE = "http://127.0.0.1:8800"
COOKIE_FILE = "/tmp/google_cookies.json"

def post(path, data, token=None, timeout=240):
    h = {"Content-Type": "application/json"}
    if token:
        h["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, data=json.dumps(data).encode(), headers=h)
    t0 = time.time()
    try:
        r = urllib.request.urlopen(req, timeout=timeout)
        return r.status, json.loads(r.read().decode()), time.time() - t0
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        try:
            return e.code, json.loads(body), time.time() - t0
        except Exception:
            return e.code, {"raw": body[:400]}, time.time() - t0

st, b, _ = post("/api/admin/login", {"username": "admin", "password": "admin123"})
if st != 200 or "token" not in b:
    print("gagal login admin:", st, b); sys.exit(1)
tok = b["token"]

cookies = open(COOKIE_FILE).read().strip()
names = [c["name"] for c in json.loads(cookies)]
print("cookie dikirim:", ", ".join(names))
print("panen jalan (browser headless buka authorize)... tunggu sampai 2 menit\n")

st, b, el = post("/api/accounts/oauth-harvest",
                 {"cookies": cookies, "label": "akun-oauth-google"}, tok)
print("--- HASIL (%.1f detik, HTTP %s) ---" % (el, st))
print(json.dumps(b, indent=2, ensure_ascii=False)[:1500])
