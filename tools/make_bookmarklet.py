#!/usr/bin/env python3
"""Bikin bookmarklet satu-klik dari tools/grabber-bookmarklet.js.

Bookmarklet = file JS yang di-compact + di-URL-encode + di-prefix 'javascript:',
jadi bisa ditempel jadi bookmark browser. Output:
  tools/bookmarklet.txt       -> baris 'javascript:...' siap copy
  tools/bookmarklet.html      -> halaman dengan <a> draggable (tombol install)
"""
import pathlib
import re
import urllib.parse

HERE = pathlib.Path(__file__).resolve().parent
src = (HERE / "grabber-bookmarklet.js").read_text()

# strip // comments (none of them are inside string literals that matter, but we
# only strip lines whose comment is outside quotes; simple line-based removal)
lines = []
for ln in src.splitlines():
    m = re.match(r"^(\s*)//.*$", ln)
    if m:
        continue
    lines.append(ln.strip())
code = " ".join(l for l in lines if l)
code = re.sub(r"\s{2,}", " ", code).strip()

bookmarklet = "javascript:" + urllib.parse.quote(code, safe="')!\";,:/<>=%&.{}[]+-*~$#@^|? ")
(HERE / "bookmarklet.txt").write_text(bookmarklet + "\n")

page = f"""<!doctype html><html><head><meta charset=utf8><title>chatgpt2API grabber</title>
<style>body{{font:14px/1.6 system-ui;max-width:760px;margin:40px auto;padding:0 16px}}
a.bk{{display:inline-block;padding:14px 22px;background:#00c85a;color:#000;font-weight:700;border-radius:10px;text-decoration:none}}
code,textarea{{background:#111;color:#0f0;padding:10px;border-radius:8px;display:block;width:100%;box-sizing:border-box;font:11px monospace}}</style></head>
<body><h2>chatgpt2API — cookie/token grabber</h2>
<p><b>Install (desktop browser yang login ChatGPT):</b> drag tombol hijau ini ke Bookmarks bar.</p>
<p><a class="bk" href="{bookmarklet}">chatgpt2API grabber</a></p>
<p>Atau salin manual:</p>
<textarea rows="7" readonly>{urllib.parse.unquote(bookmarklet).replace("<", "&lt;")}</textarea>
<p><b>Pakai:</b> buka <code>chatgpt.com</code> (harus sudah login) &rarr; klik bookmark itu
&rarr; muncul overlay berisi <code>ACCESS_TOKEN</code>, <code>USER_AGENT</code>, dll &rarr;
tombol COPY SEMUA &rarr; tempel ke agent.</p>
<p>Catatan: <code>__Secure-next-auth.session-token</code> itu <b>httpOnly</b> — tidak bisa
dibaca JS mana pun. Yang diambil di sini <code>accessToken</code> dari
<code>/api/auth/session</code> (cukup untuk menjalankan gateway); kalau nanti butuh
refresh via cookie, ambil dari DevTools &gt; Application &gt; Cookies.</p>
</body></html>
"""
(HERE / "bookmarklet.html").write_text(page)
print("OK  bookmarklet.txt (%d chars)  bookmarklet.html" % len(bookmarklet))
