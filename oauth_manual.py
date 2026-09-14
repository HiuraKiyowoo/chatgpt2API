#!/usr/bin/env python3
"""oauth_manual.py — jalur C: authorize manual di browser, code ditukar via HTTP.

Pakai:
  python3 oauth_manual.py url                 # cetak URL authorize (PKCE disimpan)
  python3 oauth_manual.py tukar "<code|url>"  # tukar code -> refresh_token, simpan akun
"""
import base64, hashlib, json, os, secrets, sys, time, urllib.parse, urllib.request, urllib.error

CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann"
TOKEN_URL = os.environ.get("OPENAI_AUTH_BASE", "https://auth.openai.com").rstrip("/") + "/oauth/token"
REDIRECT_URI = "http://localhost:1455/auth/callback"
SCOPE = "openid profile email offline_access"
AUTH_URL = "https://auth.openai.com/oauth/authorize"
STATE_FILE = "/root/chatgpt2API/data/pkce_state.json"


def b64url(b: bytes) -> str:
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode()


def cmd_url():
    verifier = b64url(secrets.token_bytes(48))
    challenge = b64url(hashlib.sha256(verifier.encode()).digest())
    state = "cgt2api-" + b64url(secrets.token_bytes(8))
    q = urllib.parse.urlencode({
        "client_id": CLIENT_ID,
        "response_type": "code",
        "redirect_uri": REDIRECT_URI,
        "scope": SCOPE,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
        "state": state,
        "prompt": "consent",
    })
    url = AUTH_URL + "?" + q
    os.makedirs(os.path.dirname(STATE_FILE), exist_ok=True)
    with open(STATE_FILE, "w") as f:
        json.dump({"verifier": verifier, "state": state, "ts": int(time.time())}, f)
    print("VERIFIER:", verifier)
    print("STATE   :", state)
    print()
    print("URL AUTHORIZE (buka di browser yang sudah login ChatGPT):")
    print()
    print(url)


def cmd_tukar(arg: str):
    # terima URL penuh atau code mentah
    code = arg
    if "code=" in arg:
        qs = urllib.parse.urlparse(arg).query or arg.split("?", 1)[-1]
        code = urllib.parse.parse_qs(qs).get("code", [""])[0]
    if not code:
        print("code kosong"); sys.exit(1)

    with open(STATE_FILE) as f:
        st = json.load(f)
    verifier = st["verifier"]

    form = urllib.parse.urlencode({
        "grant_type": "authorization_code",
        "code": code,
        "redirect_uri": REDIRECT_URI,
        "client_id": CLIENT_ID,
        "code_verifier": verifier,
    }).encode()
    req = urllib.request.Request(TOKEN_URL, data=form, headers={
        "Content-Type": "application/x-www-form-urlencoded",
        "Accept": "application/json",
        "User-Agent": "Mozilla/5.0",
    })
    try:
        r = urllib.request.urlopen(req, timeout=45)
        toks = json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        print("HTTP", e.code, e.read().decode()[:500]); sys.exit(1)

    print("access_token :", (toks.get("access_token") or "")[:40] + "...")
    print("refresh_token:", (toks.get("refresh_token") or "")[:40] + "...")
    print("expires_in   :", toks.get("expires_in"))
    if not toks.get("refresh_token"):
        print("\n[!] TANPA refresh_token — authorize ulang, pastikan scope offline_access")
        print(json.dumps(toks, indent=2)[:800]); sys.exit(1)

    # simpan via gateway (biar terenkripsi di DB)
    save_tokens(toks)


def admin_token(base, user, pw):
    """Login admin -> JWT pendek. Route import dibungkus AuthAdmin, tanpa
    header ini request ditolak 401 (itu bug yang bikin jalur manual gak pernah jalan)."""
    body = json.dumps({"username": user, "password": pw}).encode()
    req = urllib.request.Request(base + "/api/admin/login", data=body,
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read().decode())["token"]


def save_tokens(toks):
    base = os.environ.get("GATEWAY_URL", "http://127.0.0.1:8800").rstrip("/")
    user = os.environ.get("GATEWAY_USER") or "admin"
    pw = os.environ.get("GATEWAY_PASS")
    body = json.dumps({"tokens": toks, "label": os.environ.get("GATEWAY_LABEL", "manual-oauth")}).encode()
    headers = {"Content-Type": "application/json"}
    if pw:
        try:
            headers["Authorization"] = "Bearer " + admin_token(base, user, pw)
        except Exception as e:
            print("[!] login admin gagal:", e, "-- coba tanpa auth")
    req = urllib.request.Request(base + "/api/accounts/import-tokens", data=body, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            print("\n--- SIMPAN (via gateway) ---")
            print(r.read().decode()[:600])
            return
    except urllib.error.HTTPError as e:
        print("\n[!] gateway nolak (HTTP %s)" % e.code)
        print(e.read().decode()[:300])
    except Exception as e:
        print("\n[!] gateway gak reachable:", e)
    # fallback: tulis mentah (0600, jangan sampe kebaca user lain)
    out = os.path.join(os.path.dirname(STATE_FILE), "oauth_tokens.json")
    fd = os.open(out, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w") as f:
        json.dump(toks, f, indent=2)
    print("tokens ditulis ke %s (chmod 600) — import manual lewat dashboard/API" % out)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__); sys.exit(1)
    if sys.argv[1] == "url":
        cmd_url()
    elif sys.argv[1] == "tukar":
        cmd_tukar(sys.argv[2] if len(sys.argv) > 2 else "")
    else:
        print(__doc__); sys.exit(1)
