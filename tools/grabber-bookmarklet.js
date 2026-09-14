// chatgpt2API — grabber bookmarklet. Jalankan di tab chatgpt.com yang sudah login.
// Ambil accessToken via /api/auth/session (cookie httpOnly ikut terkirim otomatis),
// plus User-Agent & cookie yang visible. Tampil di overlay + tombol copy.
void (async () => {
  if (!/(^|\.)chatgpt\.com$/.test(location.hostname)) {
    alert("Buka dulu chatgpt.com, KLIK bookmarkletnya disitu.");
    return;
  }
  const box = document.createElement("div");
  box.style.cssText =
    "position:fixed;inset:0;z-index:2147483647;background:rgba(0,0,0,.88);padding:20px;box-sizing:border-box;font:12px/1.5 monospace;color:#eee;display:flex;flex-direction:column;gap:8px";
  const title = document.createElement("div");
  title.textContent = "chatgpt2API grabber";
  title.style.cssText = "font-weight:bold;font-size:14px;color:rgb(0,255,140)";
  const ta = document.createElement("textarea");
  ta.readOnly = true;
  ta.style.cssText = "flex:1;background:rgb(17,17,17);color:rgb(0,255,120);border:1px solid rgb(60,60,60);padding:10px;white-space:pre;font:12px monospace";
  const row = document.createElement("div");
  row.style.cssText = "display:flex;gap:8px";
  const btn = document.createElement("button");
  btn.textContent = "COPY SEMUA";
  btn.style.cssText = "flex:1;padding:12px;font:bold 14px monospace;background:rgb(0,200,90);color:rgb(0,0,0);border:0;border-radius:8px;cursor:pointer";
  const close = document.createElement("button");
  close.textContent = "TUTUP";
  close.style.cssText = "padding:12px 18px;font:12px monospace;background:rgb(60,60,60);color:#eee;border:0;border-radius:8px;cursor:pointer";
  btn.onclick = async () => {
    ta.select();
    try { await navigator.clipboard.writeText(ta.value); } catch (e) { document.execCommand("copy"); }
    btn.textContent = "TERSALIN";
  };
  close.onclick = () => box.remove();
  row.appendChild(btn); row.appendChild(close);
  box.appendChild(title); box.appendChild(ta); box.appendChild(row);
  document.documentElement.appendChild(box);

  const out = [];
  out.push("HANYA_BUAT_AKUN_CHATGPT2API — anggap sensitif, hapus pesan setelah dipakai");
  let j = null;
  try {
    const r = await fetch("/api/auth/session", { credentials: "include" });
    j = await r.json();
  } catch (e) {
    out.push("SESSION_FETCH_ERR=" + e);
  }
  if (j) {
    out.push("EMAIL=" + ((j.user && j.user.email) || "?"));
    out.push("EXPIRES=" + (j.expires || "?"));
    let exp = 0;
    try {
      const p = JSON.parse(atob((j.accessToken || "").split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));
      exp = p.exp || 0;
      out.push("JWT_EXP=" + (exp ? new Date(exp * 1000).toISOString() : "?"));
    } catch (e) { out.push("JWT_EXP=?"); }
    out.push("ACCESS_TOKEN=" + (j.accessToken || "KOSONG—login dulu"));
  }
  out.push("USER_AGENT=" + navigator.userAgent);
  const vis = document.cookie;
  out.push("VISIBLE_COOKIES=" + (vis || "(kosong)"));
  out.push("SESSION_TOKEN_JS=" + (/__Secure-next-auth\.session-token/.test(vis)
    ? "TERBACA (jarang!)"
    : "httpOnly — JS gak bisa; kalau perlu, copy dari DevTools>Application>Cookies atau Network>Request Headers>Cookie"));
  ta.value = out.join("\n\n");
})();
