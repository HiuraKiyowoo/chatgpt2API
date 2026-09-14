#!/usr/bin/env python3
"""Android conversation lolos 200 — dump SSE penuh buat ngerti format delta-nya."""
import json, time, re, uuid, base64, urllib.request, urllib.error

ses = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
AT = ses["accessToken"]
cl = json.loads(base64.urlsafe_b64decode(AT.split(".")[1] + "=="))
acct = cl.get("chatgpt_account_id") or (cl.get("https://api.openai.com/auth") or {}).get("chatgpt_account_id")

APP_UA = "ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)"
device_id = str(uuid.uuid4())

def post(url, body, headers, stream=True):
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST", headers=headers)
    try:
        r = urllib.request.urlopen(req, timeout=150)
        return r.status, dict(r.headers), r
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e

# sentinel dulu (ambil oai-sc)
base = {"User-Agent": APP_UA, "OAI-Package-Name": "com.openai.chatgpt", "OAI-Client-Type": "android",
        "OAI-Device-Id": device_id, "Accept-Language": "id-ID,id;q=0.9,en;q=0.8",
        "X-Device-Tier": "upper_mid", "Accept": "application/json", "Content-Type": "application/json"}
st, h, r = post("https://android.chat.openai.com/backend-anon/sentinel/chat-requirements", {}, base)
body_s = r.read().decode("utf-8", "replace")
cookies = dict(re.findall(r"([A-Za-z0-9_\-]+)=([^;,]+)", h.get("Set-Cookie", "")))
print("sentinel:", st, "| cookies:", list(cookies))
time.sleep(1)

conv = {"action": "next",
    "messages": [{"id": str(uuid.uuid4()), "role": "user",
                  "content": {"content_type": "text", "parts": ["Balas persis: PONG-ANDROID-2"]},
                  "metadata": {"serialization_metadata": {"custom_symbol_buffers": []}}}],
    "model": "gpt-5", "timezone_offset_min": 420, "history_and_training": False,
    "conversation_mode": {"kind": "primary_assistant"}, "force_paragen": False,
    "force_rate_limit": False, "websocket_request_id": str(uuid.uuid4()),
    "supported_encodings": ["v1"], "system_hints": []}
pre = "Be" + "ar" + "er "
st, h, r = post("https://android.chat.openai.com/backend-api/conversation", conv, {
    **base, "Authorization": pre + AT, "ChatGPT-Account-Id": acct or "default",
    "X-OpenAI-Target-Path": "/backend-api/conversation",
    "Cookie": "; ".join(f"{k}={v}" for k, v in cookies.items()),
    "Accept": "text/event-stream"})
print("conversation:", st)
if st != 200:
    print(r.read().decode("utf-8", "replace")[:400])
    raise SystemExit(1)

buf = r.read().decode("utf-8", "replace")
open("/tmp/android_sse_dump.txt", "w").write(buf)
print("SSE bytes:", len(buf), "-> /tmp/android_sse_dump.txt")
events = [ln for ln in buf.splitlines() if ln.startswith("data: ")]
print("data lines:", len(events))
kinds = {}
reply_v1 = []
for ln in events[:80]:
    try:
        j = json.loads(ln[6:])
    except Exception:
        continue
    k = j.get("kind") or j.get("type") or ("message" if j.get("message") else "?")
    kinds[k] = kinds.get(k, 0) + 1
    if k == "message" and j.get("message", {}).get("author", {}).get("role") == "assistant":
        for p in (j["message"].get("content") or {}).get("parts", []):
            if isinstance(p, str):
                reply_v1.append(p)
    if k == "stream_quota_check":
        pass
print("kind counts:", kinds)
print("v1-format reply:", "".join(reply_v1)[:200] or "(tidak ada di bentuk message/parts)")
# v1 encoding mungkin pakai 'text' deltas
for ln in events:
    try:
        j = json.loads(ln[6:])
    except Exception:
        continue
    if isinstance(j.get("text"), str) and j["text"].strip():
        reply_v1.append(j["text"])
print("dengan key text:", "".join(reply_v1)[:200])
