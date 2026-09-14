#!/usr/bin/env python3
"""PROBE fitur dengan body PERSIS buildConvRequest gateway (yang 200 di android),
plus varian with_message_options. Yang 422 = field ditolak, catat mana yang 200."""
import json, time, re, uuid, base64, urllib.request, urllib.error

s = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
tok = s["accessToken"]
def claims(t):
    p = t.split(".")[1]; p += "="*(-len(p)%4)
    return json.loads(base64.urlsafe_b64decode(p))
cl = claims(tok)
acct = cl.get("chatgpt_account_id","")
ck = "; ".join(f"{c['name']}={c['value']}" for c in json.load(open("/root/chatgpt2API/data/cookies_local.json")))
did = (re.search(r"oai-did=([0-9a-f-]{36})", ck) or [None,"3539d932-0564-4f11-b0d0-df45f5a3f9a6"])[1]

def body_for(content, extra):
    b = {
        "action": "post_message",
        "messages": [{
            "id": str(uuid.uuid4()), "role": "user",
            "content": {"content_type":"text","parts":[content]},
            "metadata": {"serialization_metadata":{"custom_symbol_offsets":[]}}
        }],
        "model": "gpt-5", "timezone_offset_min": -420,
        "history_and_training": True,
        "conversation_mode": {"kind":"primary"},
        "force_paragen": False, "force_rate_limit": False,
        "websocket_request_id": str(uuid.uuid4()),
        "supported_encodings": ["sig"], "system_hints": [],
    }
    b.update(extra or {})
    return b

def probe(label, extra, content):
    req = urllib.request.Request(
        "https://android.chat.openai.com/backend-api/conversation",
        data=json.dumps(body_for(content, extra)).encode(), method="POST", headers={
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
        print(f"[{label}] HTTP {e.code}: {e.read().decode(errors='replace')[:200]}")
        return
    evs = [l for l in raw.splitlines() if l.startswith("data:") and len(l) > 6]
    rsn = sum(1 for l in evs if "reasoning" in l.lower() or '"summary_text"' in l)
    cit = sum(1 for l in evs if "citation" in l.lower()) + sum(1 for l in evs if "\ue200" in l)
    txt = "".join(re.findall(r'"o":"append","v":"((?:[^"\\]|\\.)*)"', raw))
    print(f"[{label}] HTTP {code} ev={len(evs)} reasoning={rsn} cite/mark={cit} | {txt[:140].replace(chr(10),' ')}")

print("== 1: reasoning effort ==")
probe("reasoning", {"with_message_options": {"reasoning": {"effort": "high"}}},
      "17*23? Jawab singkat setelah mikir.")
print("== 2: web_search tool ==")
time.sleep(2)
probe("websearch", {"with_message_options": {"tools": [{"type": "web_search"}]}},
      "Berita AI hari ini 1 kalimat.")
print("== 3: model direct gpt-5-thinking ==")
time.sleep(2)
b = body_for("17*23?", None)
def probe2(label, body):
    req = urllib.request.Request(
        "https://android.chat.openai.com/backend-api/conversation",
        data=json.dumps(body).encode(), method="POST", headers={
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
        print(f"[{label}] HTTP {e.code}: {e.read().decode(errors='replace')[:200]}"); return
    evs = [l for l in raw.splitlines() if l.startswith("data:") and len(l) > 6]
    txt = "".join(re.findall(r'"o":"append","v":"((?:[^"\\]|\\.)*)"', raw))
    print(f"[{label}] HTTP {code} ev={len(evs)} | {txt[:140].replace(chr(10),' ')}")
time.sleep(2)
b["model"] = "gpt-5-thinking"
probe2("gpt-5-thinking", b)
