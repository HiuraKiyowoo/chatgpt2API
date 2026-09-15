import React, { useEffect, useState } from "react";
import { getAccounts, addAccount, deleteAccount, checkAccount } from "../api.js";

// ---- helper konversi input mentah -> cookie string siap pakai ----
// Menerima 3 format:
//  1. Export Cookie-Editor (array JSON)  -> gabung .0/.1, buang yang kadaluarsa
//  2. String cookie polos (document.cookie / Copy as cURL) -> dirapikan
//  3. accessToken JWT doang ("eyJ...") -> masuk kolom accessToken
export function parseCookieInput(raw) {
  const t = raw.trim();
  if (!t) return { error: "Kosong" };
  // format 3: JWT polos
  if (/^eyJ[\w-]+\./.test(t) && !t.includes("=")) {
    return { accessToken: t };
  }
  let pairs = [];
  try {
    const j = JSON.parse(t);
    if (Array.isArray(j)) {
      // Cookie-Editor: skip yang expired
      const now = Date.now() / 1000;
      pairs = j
        .filter((c) => c && c.name && (!c.expirationDate || c.expirationDate > now))
        .map((c) => [c.name, String(c.value)]);
    } else if (j && typeof j === "object") {
      // output snippet DevTools: {accessToken, cookies} — cookies bisa string
      // "a=b; c=d" atau array JSON. Rekursi buat bagian cookies-nya.
      const out = {};
      const at = j.accessToken || j.access_token || j.token;
      if (typeof at === "string" && at.trim()) out.accessToken = at.trim();
      if (j.cookies) {
        const sub = parseCookieInput(
          typeof j.cookies === "string" ? j.cookies : JSON.stringify(j.cookies)
        );
        if (sub.cookies) {
          out.cookies = sub.cookies;
          out.hasSession = sub.hasSession;
          out.hasDid = sub.hasDid;
        } else if (sub.error) {
          out.cookieWarn = sub.error + " (bagian cookies dilewati)";
        }
      }
      if (!out.accessToken && !out.cookies) return { error: "Objek tidak punya accessToken/cookies" };
      return out;
    }
  } catch {
    // format 2: "a=b; c=d"
    pairs = t
      .split(/;\s*/)
      .map((kv) => kv.replace(/^Cookie:\s*/i, ""))
      .filter((kv) => kv.includes("="))
      .map((kv) => {
        const i = kv.indexOf("=");
        return [kv.slice(0, i).trim(), kv.slice(i + 1).trim()];
      });
  }
  if (!pairs.length) return { error: "Format tidak dikenali" };
  // PENTING: chunk NextAuth (.0/.1) TIDAK BOLEH digabung — server cuma ngenalin
  // bentuk chunk terpisah (terbukti: gabungan -> 403 WAF, chunk -> 200 + JWT).
  const map = new Map();
  for (const [k, v] of pairs) if (!map.has(k)) map.set(k, v);
  const out = [...map.entries()].map(([k, v]) => `${k}=${v}`).join("; ");
  const hasSession = [...map.keys()].some(
    (k) => k.startsWith("__Secure-next-auth.session-token") || k.startsWith("next-auth.session-token")
  );
  const hasDid = map.has("oai-did");
  return { cookies: out, hasSession, hasDid };
}

const SNIPPET = `(async () => {
  const s = await (await fetch('/api/auth/session', {credentials:'include'})).json();
  const out = { accessToken: s?.accessToken || '' , cookies: document.cookie };
  if (!out.accessToken) return alert('Tidak ada sesi — login dulu di tab ini.');
  console.log('accessToken umur ~9 hari:', out.accessToken.slice(0, 24) + '...');
  console.log('PENTING: cookie HttpOnly TIDAK kebaca JS. Untuk seed permanen pakai export Cookie-Editor; untuk cepat pakai accessToken saja.');
  copy(JSON.stringify(out));
  return 'COPIED — tempel di kotak paste halaman Accounts, klik Konversi';
})()`;

export default function Accounts() {
  const [list, setList] = useState([]);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({ label: "", accessToken: "", cookies: "", cfClearance: "", userAgent: "", checkNow: true });
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [paste, setPaste] = useState("");
  const [copied, setCopied] = useState(false);

  const load = () => getAccounts().then((d) => setList(d.accounts)).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  const convert = () => {
    const r = parseCookieInput(paste);
    setErr(r.error || "");
    if (r.error) return;
    const parts = [];
    if (r.accessToken) {
      setForm((f) => ({ ...f, accessToken: r.accessToken }));
      parts.push("accessToken terisi (umur ±9 hari)");
    }
    if (r.cookies) {
      setForm((f) => ({ ...f, cookies: r.cookies }));
      parts.push(
        `Cookies ${r.cookies.length} char — session-token ${r.hasSession ? "ADA ✅" : "TIDAK KEPIKET ⚠️ (HttpOnly: pakai export Cookie-Editor)"} · oai-did ${r.hasDid ? "ADA ✅" : "(gak ada, device-id random)"}`
      );
    }
    if (r.cookieWarn) parts.push(r.cookieWarn);
    setNotice(parts.join(" · "));
    setPaste("");
  };

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true); setErr(""); setNotice("");
    try {
      const r = await addAccount(form);
      setNotice(`Akun ditambahkan → status ${r.status}${r.detail ? ": " + r.detail : ""}`);
      setForm({ label: "", accessToken: "", cookies: "", cfClearance: "", userAgent: "", checkNow: true });
      load();
    } catch (ex) { setErr(ex.message); }
    setBusy(false);
  };

  const del = async (id) => { await deleteAccount(id); load(); };
  const check = async (id) => {
    setNotice("Memeriksa...");
    try { const r = await checkAccount(id); setNotice(`Status: ${r.status}${r.detail ? " — " + r.detail : ""}`); load(); }
    catch (ex) { setNotice(ex.message); }
  };

  const copySnippet = async () => {
    try { await navigator.clipboard.writeText(SNIPPET); setCopied(true); setTimeout(() => setCopied(false), 2000); } catch {}
  };

  return (
    <div>
      <h1>Accounts</h1>
      {err && <div className="alert">{err}</div>}
      {notice && <div className="notice">{notice}</div>}

      <div className="card">
        <h2>Tambah akun ChatGPT</h2>
        <p className="muted">
          Cara paling gampang: login <code>chatgpt.com</code> di browser → extension
          <b> Cookie-Editor</b> → <i>Export</i> → tempel semuanya di kotak bawah → <b>Konversi</b> → <b>Tambah</b>.
          Gateway tukar cookie → JWT otomatis tiap request (±90 hari). Yang penting cuma
          <code> __Secure-next-auth.session-token</code> + <code>oai-did</code> — sisanya boleh ilang.
        </p>
        <div className="form">
          <textarea rows={4} placeholder={'Tempel DISINI apa saja: JSON export Cookie-Editor, string "a=b; c=d", atau accessToken JWT doang'}
            value={paste} onChange={(e) => setPaste(e.target.value)} />
          <button type="button" className="btn" onClick={convert} disabled={!paste.trim()}>⚙️ Konversi & isi form</button>
        </div>
        <details open={showAdvanced} onToggle={(e) => setShowAdvanced(e.target.open)}>
          <summary className="muted" style={{ cursor: "pointer" }}>…atau isi manual / lihat field lama</summary>
          <form onSubmit={submit} className="form" style={{ marginTop: 8 }}>
            <input placeholder="Label (cth: akun kantor)" value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })} />
            <input placeholder="accessToken (eyJ... — opsional, umur ±9 hari)" value={form.accessToken} onChange={(e) => setForm({ ...form, accessToken: e.target.value })} />
            <input placeholder="Cookies string (hasil konversi di atas)" value={form.cookies} onChange={(e) => setForm({ ...form, cookies: e.target.value })} />
            <input placeholder="cf_clearance (opsional — web fallback doang)" value={form.cfClearance} onChange={(e) => setForm({ ...form, cfClearance: e.target.value })} />
            <input placeholder="User-Agent (opsional)" value={form.userAgent} onChange={(e) => setForm({ ...form, userAgent: e.target.value })} />
            <label className="chk"><input type="checkbox" checked={form.checkNow} onChange={(e) => setForm({ ...form, checkNow: e.target.checked })} /> Cek valid langsung setelah simpan</label>
            <button className="btn primary" disabled={busy || (!form.accessToken.trim() && !form.cookies.trim())}>{busy ? "Menyimpan..." : "Tambah"}</button>
          </form>
        </details>
      </div>

      <div className="card">
        <h2>🛠️ Snippet DevTools (alternatif tanpa extension)</h2>
        <p className="muted">
          F12 → tab <b>Console</b> di halaman chatgpt.com yang udah login → paste → Enter.
          Hasil ke-clipboard. <b>Catatan:</b> cookie login (<code>session-token</code>) itu HttpOnly —
          JS browser NGAK bisa bacanya, jadi snippet cuma ngasih accessToken (umur ±9 hari, abis itu
          seed ulang). Mau yang 90 hari tanpa aksi → pakai Cookie-Editor.
        </p>
        <pre className="code" style={{ whiteSpace: "pre-wrap" }}>{SNIPPET}</pre>
        <button className="btn" onClick={copySnippet}>{copied ? "✅ Tersalin" : "Salin snippet"}</button>
      </div>

      <div className="card">
        <h2>Daftar ({list.length})</h2>
        {list.length === 0 && <p className="muted">Belum ada akun.</p>}
        {list.map((a) => (
          <div className="row" key={a.id}>
            <div>
              <b>{a.label || "(tanpa label)"}</b> <span className={`badge ${a.status}`}>{a.status}</span>
              <div className="muted small">{a.requestCount} req · {a.successCount} ok · {a.errorCount} err{a.lastError ? ` · ${a.lastError.slice(0, 80)}` : ""}</div>
              {a.email && <div className="muted small">✉️ {a.email} (terbaca otomatis dari token)</div>}
            </div>
            <div className="actions">
              <button className="btn small" onClick={() => check(a.id)}>Cek</button>
              <button className="btn small danger" onClick={() => del(a.id)}>Hapus</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
