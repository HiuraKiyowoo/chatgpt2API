#!/usr/bin/env python3
"""PROBE fitur via jalur ANDROID: thinking, web_search (dan opsional vision).
Baca kredensial dari pool live. Cetak HTTP code + penanda di SSE."""
import json, time, re, uuid, base64, urllib.request, urllib.error

s = json.load(open("/root/chatgpt2API/data/session_fetched.json"))
tok = s["accessToken"]

def claims(t):
    p = t.split(".")[1]; p += "=" * (-len(p) % 4)
    return json.loads(base64.urlsafe_b64decode(p))

cl = claims(tok)
acct = cl.get("chatgpt_account_id") or cl.get("https://api.openai.com/account_id") or ""
user = cl.get("sub") or cl.get("https://api.openai.com/profile", {}).get("sub") or ""
plan = ""
for k, v in cl.items():
    if "plan" in k.lower(): plan = v
print("acct=", acct[:12], "user=", str(user)[:12], "plan=", plan or "?")

did = ""
for c in json.load(open("/root/chatgpt2API/data/cookies_local.json")):
    if c["name"] == "oai-did": did = c["value"]
print("device_id from cookies:", did or "(kosong -> pakai uuid)")
if not did: did = str(uuid.uuid4())
ck = "; ".join(f"{c['name']}={c['value']}" for c in json.load(open("/root/chatgpt2API/data/cookies_local.json")))

def probe(label, wmo, content):
    uid, aid = str(uuid.uuid4()), str(uuid.uuid4())
    body = {
        "action": "post_message",
        "messages": [{
            "id": uid, "author": {"role": "user", "metadata": {}},
            "content": {"content_type": "text", "parts": [content]},
            "status": "finished_successfully", "metadata": {},
            "parent_id": None, "weight": 0.0,
        }],
        "model": "gpt-5", "version": "20241221",
        "system_harm_messages": [], "system_character_messages": [],
        "conversation_id": None,
        "with_message_options": wmo or {},
        "params": {"time_of_day": int(time.time()), "timezone": "Asia/Jakarta"},
    }
    req = urllib.request.Request(
        "https://android.chat.openai.com/backend-api/conversation",
        data=json.dumps(body).encode(), method="POST", headers={
        "User-Agent": "ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)",
        "Oai-Package-Name": "com.openai.chatgpt", "Oai-Client-Type": "android",
        "Oai-Device-Id": did, "Accept-Language": "id-ID,id;q=0.9",
        "Content-Type": "application/json", "Accept": "text/event-stream",
        "Authorization": "***" + tok,
        "Chatgpt-Account-Id": acct or "default",
        "Cookie": ck})
    try:
        with urllib.request.urlopen(req, timeout=240) as r:
            raw = r.read().decode(errors="replace")
            print(f"[{label}] HTTP {r.status}")
    except urllib.error.HTTPError as e:
        print(f"[{label}] HTTP {e.code}: {e.read().decode(errors='replace')[:240]}")
        return
    evs = [l for l in raw.splitlines() if l.startswith("data:") and len(l) > 6]
    rsn = sum(1 for l in evs if "reasoning" in l.lower() or '"summary"' in l)
    cit = sum(1 for l in evs if "citation" in l.lower() or "\ue200" in l)
    txt = "".join(re.findall(r'"o":"append","v":"((?:[^"\\]|\\.)*)"', raw))
    print(f"   ev={len(evs)} reasoning_ev={rsn} citation_markers={cit}")
    print(f"   teks: {txt[:160].replace(chr(10),' ') or '(kosong)'}")

print("\n== PROBE thinking ==")
probe("thinking", {"reasoning": {"effort": "high"}},
      "17*23 berapa? Jawab setelah mikir.")
print("\n== PROBE web_search ==")
time.sleep(2)
probe("websearch", {"tools": [{"type": "web_search"}]},
      "Berita AI terbaru hari ini? Satu kalimat.")
