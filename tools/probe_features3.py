#!/usr/bin/env python3
"""PROBE fitur — body KONTROL = tiruan persis buildConvRequest gateway (200),
varian nambah satu hal per tes: reasoning / web_search / mode search."""
import json, time, re, uuid, base64, urllib.request, urllib.error

s = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
tok = s["accessToken"]
def claims(t):
    p = t.split(".")[1]; p += "="*(-len(p)%4)
    return json.loads(base64.urlsafe_b64decode(p))
acct = claims(tok).get("chatgpt_account_id","")
ck = "; ".join(f"{c['name']}={c['value']}" for c in json.load(open("/root/chatgpt2API/data/cookies_local.json")))
did = (re.search(r"oai-did=([0-9a-f-]{36})", ck) or [None,"x"])[1]

def body(content, extra=None, kind="primary_assistant", model="gpt-5"):
    b = {
        "action": "next",
        "messages": [{
            "id": str(uuid.uuid4()), "role": "user",
            "content": {"content_type": "text", "parts": [content]},
            "metadata": {"serialization_metadata": {"custom_symbol_offsets": []}}
        }],
        "model": model,
        "timezone_offset_min": 420,
        "history_and_training": False,
        "conversation_mode": {"kind": kind},
        "force_paragen": False,
        "force_rate_limit": False,
        "supported_encodings": ["v1"],
        "system_hints": []
    }
    if extra: b.update(extra)
    return b

def probe(label, b):
    req = urllib.request.Request(
        "https://android.chat.openai.com/backend-api/conversation",
        data=json.dumps(b).encode(), method="POST", headers={
        "User-Agent":"ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)",
        "Oai-Package-Name":"com.openai.chatgpt","Oai-Client-Type":"android",
        "Oai-Device-Id":did,"Accept-Language":"id-ID,id;q=0.9",
        "Content-Type":"application/json","Accept":"text/event-stream",
        "Authorization":"***"+tok,"Chatgpt-Account-Id":acct or "default",
        "X-OpenAI-Target-Path":"/backend-api/conversation","Cookie":ck})
    try:
        with urllib.request.urlopen(req, timeout=240) as r:
            raw = r.read().decode(errors="replace"); code = r.status
    except urllib.error.HTTPError as e:
        print(f"[{label}] HTTP {e.code}: {e.read().decode(errors='replace')[:220]}")
        return
    evs = [l for l in raw.splitlines() if l.startswith("data:") and len(l) > 6]
    rsn = sum(1 for l in evs if "reasoning" in l.lower() or "summary_text" in l.lower())
    cit = sum(1 for l in evs if "citation" in l.lower()) + sum(1 for l in evs if "\ue200" in l)
    txt = "".join(re.findall(r'"o":"append","v":"((?:[^"\\]|\\.)*)"', raw))
    print(f"[{label}] HTTP {code} ev={len(evs)} reasoning={rsn} cite={cit} | {txt[:150]}")

print("== 0 KONTROL =="); probe("kontrol", body("Balas persis: KONTROL-OK"))
time.sleep(3)
print("== 1 reasoning =="); probe("reasoning", body("17*23?", {"with_message_options":{"reasoning":{"effort":"high"}}}))
time.sleep(3)
print("== 2 web_search via tools =="); probe("tools", body("Berita AI 1 kalimat", {"with_message_options":{"tools":[{"type":"web_search"}]}}))
time.sleep(3)
print("== 3 mode kind=search =="); probe("mode-search", body("Berita AI 1 kalimat", None, kind="search"))
