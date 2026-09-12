import React, { useEffect, useState } from "react";
import { getAccounts, addAccount, deleteAccount, checkAccount } from "../api.js";

export default function Accounts() {
  const [list, setList] = useState([]);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({ label: "", accessToken: "", cookies: "", cfClearance: "", userAgent: "", checkNow: false });
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  const load = () => getAccounts().then((d) => setList(d.accounts)).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true); setErr(""); setNotice("");
    try {
      const r = await addAccount(form);
      setNotice(`Akun ditambahkan (${r.status})${r.detail ? ": " + r.detail : ""}`);
      setForm({ label: "", accessToken: "", cookies: "", cfClearance: "", userAgent: "", checkNow: false });
      load();
    } catch (ex) { setErr(ex.message); }
    setBusy(false);
  };

  const del = async (id) => { await deleteAccount(id); load(); };
  const check = async (id) => { setNotice("Memeriksa..."); try { const r = await checkAccount(id); setNotice(`Status: ${r.status}${r.detail ? " — " + r.detail : ""}`); load(); } catch (ex) { setNotice(ex.message); } };

  return (
    <div>
      <h1>Accounts</h1>
      {err && <div className="alert">{err}</div>}
      {notice && <div className="notice">{notice}</div>}
      <div className="card">
        <h2>Tambah akun</h2>
        <p className="muted">Paste accessToken dari chatgpt.com (localStorage → <code>@@/auth</code> → accessToken) atau cookie string lengkap. cf_clearance opsional kalau IP kena challenge.</p>
        <form onSubmit={submit} className="form">
          <input placeholder="Label" value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })} />
          <input placeholder="accessToken (eyJhbGci...)" value={form.accessToken} onChange={(e) => setForm({ ...form, accessToken: e.target.value })} />
          <input placeholder='Cookies opsional ("__Secure-next-auth.session-token=...; ...")' value={form.cookies} onChange={(e) => setForm({ ...form, cookies: e.target.value })} />
          <input placeholder="cf_clearance opsional" value={form.cfClearance} onChange={(e) => setForm({ ...form, cfClearance: e.target.value })} />
          <input placeholder="User-Agent opsional (harus sama dengan browser ambil cf_clearance)" value={form.userAgent} onChange={(e) => setForm({ ...form, userAgent: e.target.value })} />
          <label className="chk"><input type="checkbox" checked={form.checkNow} onChange={(e) => setForm({ ...form, checkNow: e.target.checked })} /> Cek valid langsung setelah simpan</label>
          <button className="btn primary" disabled={busy}>{busy ? "Menyimpan..." : "Tambah"}</button>
        </form>
      </div>
      <div className="card">
        <h2>Daftar ({list.length})</h2>
        {list.length === 0 && <p className="muted">Belum ada akun.</p>}
        {list.map((a) => (
          <div className="row" key={a.id}>
            <div>
              <b>{a.label || "(tanpa label)"}</b> <span className={`badge ${a.status}`}>{a.status}</span>
              <div className="muted small">{a.requestCount} req · {a.successCount} ok · {a.errorCount} err{a.lastError ? ` · ${a.lastError.slice(0, 80)}` : ""}</div>
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
