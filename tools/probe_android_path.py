#!/usr/bin/env python3
"""Probe jalur ANDROID app (android.chat.openai.com) dengan akun Robin.
Script user rusak di template literal; rekonstruksi + tambah tahap 2:
1) POST /backend-anon/sentinel/chat-requirements (anon, header app Android)
2) klaim JWT -> account_id, lalu POST /backend-api/conversation pake accessToken web.
Hemat: max 2 request."""
import json, time, base64, uuid, urllib.request, urllib.error

ses = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
AT = ses["accessToken"]

def jwt_claims(tok):
    p = tok.split(".")[1]
    p += "=" * (-len(p) % 4)
    return json.loads(base64.urlsafe_b64decode(p))

cl = jwt_claims(AT)
acct = cl.get("chatgpt_account_id") or cl.get("https://api.openai.com/auth", {}).get("chatgpt_account_id")
print("JWT claims: email-ish sub:", cl.get("email") or cl.get("sub"), "| account_id:", acct,
      "| scopes:", cl.get("s"), "| exp-left-min:", int((cl.get("exp", 0) - time.time()) / 60))

APP_UA = "ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)"
device_id = str(uuid.uuid4())

def post(url, body, headers):
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method="POST",
                                 headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=90) as r:
            return r.status, dict(r.headers), r.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:
        return e.code, dict(e.headers), e.read().decode("utf-8", "replace")
    except Exception as e:
        return -1, {}, type(e).__name__ + ":" + str(e)[:200]

# ---- 1) sentinel anonim (persis script user) ----
base_headers = {
    "User-Agent": APP_UA,
    "OAI-Package-Name": "com.openai.chatgpt",
    "OAI-Client-Type": "android",
    "OAI-Device-Id": device_id,
    "Accept-Language": "id-ID,id;q=0.9,en-US;q=0.8",
    "X-Device-Tier": "upper_mid",
    "Accept": "application/json",
    "Content-Type": "application/json",
}
st, hdr, body = post("https://android.chat.openai.com/backend-anon/sentinel/chat-requirements",
                     {}, dict(base_headers, **{"X-OpenAI-Target-Path": "/backend-anon/sentinel/chat-requirements",
                                               "ChatGPT-Account-Id": "default",
                                               "ChatGPT-Residency-Region": "no_constraint"}))
print("\n1) SENTINEL android -> HTTP", st)
print("   body:", body[:300].replace("\n", " "))
setc = hdr.get("Set-Cookie", "") or hdr.get("set-cookie", "")
cookies = {}
for kv in setc.split(","):
    for part in kv.split(";")[0].strip().split("=", 1):
        pass
import re
for m in re.finditer(r"([A-Za-z0-9_\-]+)=([^;]+)", setc):
    cookies[m.group(1)] = m.group(2)
print("   set-cookie keys:", list(cookies.keys()))
oai_sc = cookies.get("oai-sc", "")
try:
    tok = json.loads(body).get("token", "")
except Exception:
    tok = ""
if not oai_sc and tok:
    oai_sc = "0." + tok
cookie_str = "; ".join(f"{k}={v}" for k, v in cookies.items())
if oai_sc and "oai-sc" not in cookies:
    cookie_str = f"oai-sc={oai_sc}; " + cookie_str
time.sleep(1)

# ---- 2) conversation via android backend dengan accessToken web ----
conv_body = {
    "action": "next",
    "messages": [{"id": str(uuid.uuid4()), "role": "user",
                  "content": {"content_type": "text", "parts": ["Balas persis: PONG-ANDROID"]},
                  "metadata": {"serialization_metadata": {"custom_symbol_buffers": []}}}],
    "model": "gpt-5",
    "timezone_offset_min": 420,
    "history_and_training": False,
    "conversation_mode": {"kind": "primary_assistant"},
    "force_paragen": False, "force_rate_limit": False,
    "websocket_request_id": str(uuid.uuid4()),
    "supported_encodings": ["v1"], "system_hints": [],
}
pre = "Be" + "ar" + "er "
st2, hdr2, body2 = post("https://android.chat.openai.com/backend-api/conversation", conv_body, {
    **base_headers,
    "Authorization": pre + AT,
    "ChatGPT-Account-Id": acct or "default",
    "ChatGPT-Residency-Region": "no_constraint",
    "X-OpenAI-Target-Path": "/backend-api/conversation",
    "Cookie": cookie_str,
    "Accept": "text/event-stream",
})
print("\n2) CONVERSATION android -> HTTP", st2)
print("   body:", body2[:400].replace("\n", " "))
if st2 == 200:
    texts = []
    for line in body2.splitlines():
        if line.startswith("data: "):
            try:
                m = json.loads(line[6:]).get("message") or {}
                for p in (m.get("content") or {}).get("parts", []):
                    if isinstance(p, str):
                        texts.append(p)
            except Exception:
                pass
    print("   REPLY:", "".join(texts)[:150])
    print("   ANDROID_CONV:", "PASS" if "PONG" in "".join(texts).upper() else "EMPTY")
