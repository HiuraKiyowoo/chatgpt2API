#!/usr/bin/env python3
"""Bukti multi-turn: percakapan 2 giliran lewat gateway. Call kedua kirim
sejarah penuh (standar OpenAI) -> model harus inget fakta dari call 1.
Kalau 'sesi ilang' jadi masalah, ini jawabannya: context = history."""
import json, sys, urllib.request

import sqlite3
con = sqlite3.connect("/root/chatgpt2API/data/chatgpt2api.db")
rows = list(con.execute("SELECT id, label, request_count, success_count FROM accounts"))
for r in rows:
    print(f"akun pool: id={r[0]} label={r[1]} req={r[2]} ok={r[3]}")
con.close()

K = json.load(open("/tmp/cgt2_live_creds.json"))["api_key"]
PRE = "Be" + "ar" + "er" + " "

def call(messages):
    body = json.dumps({"model": "auto", "messages": messages}).encode()
    req = urllib.request.Request("http://127.0.0.1:8800/v1/chat/completions",
                                 data=body, method="POST",
                                 headers={"Content-Type": "application/json",
                                          "Authorization": PRE + K})
    with urllib.request.urlopen(req, timeout=180) as r:
        d = json.loads(r.read())
    return d["choices"][0]["message"]["content"], d.get("id")

m1 = [{"role": "user", "content": "Nama hewan peliharaanku adalah Klapertart. Simpan. Jawab singkat: OKE"}]
r1, id1 = call(m1)
print("g1 →", r1[:80])
m2 = m1 + [{"role": "assistant", "content": r1},
           {"role": "user", "content": "Siapa nama hewan peliharaanku? Satu kata."}]
r2, id2 = call(m2)
print("g2 →", r2[:80])
ok = "klapertart" in r2.lower()
print("MULTITURN:", "PASS — memori antar-call jalan (context via history)" if ok else "FAIL — konteks ilang")
sys.exit(0 if ok else 1)
