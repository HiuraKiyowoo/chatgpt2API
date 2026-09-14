#!/usr/bin/env python3
"""Diagnose 403 'unusual activity': variasi cookie-set & oai-did terhadap
backend-api asli. models=kontrol auth; conversation=probe WAF (prompt kecil)."""
import json, uuid, urllib.request, urllib.error, time

ses = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
cookie_all = ses["cookies"]
UA = ses["user_agent"]
AT = ses["accessToken"]

def parse(cookie):
    d = {}
    for kv in cookie.split(";"):
        kv = kv.strip()
        if "=" in kv:
            k, v = kv.split("=", 1)
            d[k] = v
    return d

C = parse(cookie_all)
# gabung chunk session
sess = "".join(v for k, v in sorted(C.items()) if k.startswith("__Secure-next-auth.session-token"))
BASE_CORE = {
    "__Secure-next-auth.session-token.0": C.get("__Secure-next-auth.session-token.0", ""),
    "__Secure-next-auth.session-token.1": C.get("__Secure-next-auth.session-token.1", ""),
}
def ck(d): return "; ".join(f"{k}={v}" for k, v in d.items() if v)

SETS = {
    "A_semua": C,
    "B_tanpa_cf_cookies": {k: v for k, v in C.items() if k not in ("__cf_bm", "_cfuvid", "__cflb", "__oailb")},
    "C_hanya_session": dict(BASE_CORE),
    "D_session_did_baru": dict(BASE_CORE, **{"oai-did": str(uuid.uuid4())}),
}

def call(path, cookie, method="GET", body=None, extra=None):
    req = urllib.request.Request("https://chatgpt.com" + path, method=method,
                                 data=body.encode() if body else None,
                                 headers={"User-Agent": UA, "Accept": "application/json",
                                          "Cookie": cookie, "Referer": "https://chatgpt.com/",
                                          "Origin": "https://chatgpt.com",
                                          "Authorization": "***" + AT,
                                          "oai-client-version": "2025-01-13",
                                          "oai-language": "en-US", **(extra or {})})
    try:
        with urllib.request.urlopen(req, timeout=90) as r:
            return r.status, r.read().decode()[:160]
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()[:160]
    except Exception as e:
        return -1, type(e).__name__ + ":" + str(e)[:120]

conv_body = json.dumps({
    "action": "next", "messages": [{"id": str(uuid.uuid4()), "author": {"role": "user"},
     "content": {"content_type": "multimodal_text", "parts": [{"content_type": "text", "text": "say OK"}]},
     "metadata": {}}],
    "model": "gpt-5", "timezone_offset_min": -420, "history_and_training_disabled": False,
    "conversation_mode": {"conversation_kind": "primary", "publish_kind": "user_published_model"},
    "force_paragen": False, "force_rate_limit": False,
})

print("=== kontrol: /backend-api/models (auth check) ===")
for name, d in SETS.items():
    st, _b = call("/backend-api/models?history_and_training=false", ck(d))
    print(f"{name:22s} models -> {st}")

print("=== probe: /backend-api/conversation (WAF) ===")
for name, d in SETS.items():
    did = d.get("oai-did", C.get("oai-did", ""))
    st, b = call("/backend-api/conversation", ck(d), method="POST", body=conv_body,
                 extra={"oai-device-id": did, "Accept": "text/event-stream"})
    tag = "OK" if st == 200 else ("403-waf" if "usual" in b or st == 403 else str(st))
    print(f"{name:22s} did={'new' if 'did_baru' in name else 'orig'} -> {st} {tag} :: {b[:110]}")
    time.sleep(2)
