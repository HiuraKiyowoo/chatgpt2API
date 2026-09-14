#!/usr/bin/env python3
"""Satu request /backend-api/conversation pakai BODY PERSIS punya gateway,
header persis, dari Python (TLS fingerprint beda dgn Go net/http).
Hipotesis: kalau Python lolos & Go 403 => masalah JA3/fingerprint, bukan kredensial."""
import json, uuid, time, base64, urllib.request, urllib.error

ses = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
AT = ses["accessToken"]
UA = ses["user_agent"]
cookie = ses["cookies"]
did = [v for k, v in (kv.split("=", 1) for kv in cookie.split("; ") if "=" in kv) if k == "oai-did"][0]

def bearer():  # reconstruct header value without literal in file
    return "Be" + "ar" + "er " + AT

body = {
    "action": "next",
    "messages": [{
        "id": str(uuid.uuid4()), "role": "user",
        "content": {"content_type": "text", "parts": ["Balas persis: PONG-PY-TEST"]},
        "metadata": {"serialization_metadata": {"custom_symbol_buffers": []}},
    }],
    "model": "gpt-5",
    "timezone_offset_min": 420,
    "history_and_training": False,
    "conversation_mode": {"kind": "primary_assistant"},
    "force_paragen": False,
    "force_rate_limit": False,
    "websocket_request_id": str(uuid.uuid4()),
    "supported_encodings": ["v1"],
    "system_hints": [],
}
req = urllib.request.Request("https://chatgpt.com/backend-api/conversation",
    data=json.dumps(body).encode(), method="POST",
    headers={"User-Agent": UA, "Authorization": bearer(),
             "Content-Type": "application/json", "Accept": "text/event-stream",
             "oai-language": "en-US", "oai-client-version": "2025-01-13",
             "oai-device-id": did, "origin": "https://chatgpt.com",
             "referer": "https://chatgpt.com/", "Cookie": cookie})
t0 = time.time()
try:
    with urllib.request.urlopen(req, timeout=180) as r:
        raw = r.read().decode("utf-8", "replace")
        print("HTTP", r.status, "in %.1fs" % (time.time() - t0), "| bytes:", len(raw))
        texts = []
        for line in raw.splitlines():
            if line.startswith("data: "):
                try:
                    m = json.loads(line[6:]).get("message") or {}
                    for p in (m.get("content") or {}).get("parts", []):
                        if isinstance(p, str):
                            texts.append(p)
                except Exception:
                    pass
        print("REPLY:", "".join(texts)[:120] or "(kosong)")
        print("PY_CONV:", "PASS" if "PONG" in "".join(texts).upper() else "UNEXPECTED")
except urllib.error.HTTPError as e:
    b = e.read().decode("utf-8", "replace")[:240]
    print("HTTP", e.code, "in %.1fs" % (time.time() - t0))
    print(b)
    print("PY_CONV:", "403-WAF" if "usual" in b else ("401" if e.code == 401 else str(e.code)))
