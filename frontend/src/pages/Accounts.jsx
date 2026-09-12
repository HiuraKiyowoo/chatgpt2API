import React, { useEffect, useState } from "react";
import { getAccounts, addAccount, deleteAccount, checkAccount, autoLogin, oauthHarvest, oauthRefresh } from "../api.js";

export default function Accounts() {
  const [list, setList] = useState([]);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({ label: "", accessToken: "", cookies: "", cfClearance: "", userAgent: "", checkNow: false });
  const [loginForm, setLoginForm] = useState({ email: "", password: "", label: "" });
  const [oauthForm, setOauthForm] = useState({ cookies: "", label: "" });
  const [busy, setBusy] = useState(false);
  const [loginBusy, setLoginBusy] = useState(false);
  const [oauthBusy, setOauthBusy] = useState(false);
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

  const doOAuth = async (e) => {
    e.preventDefault();
    setOauthBusy(true); setErr(""); setNotice("");
    setNotice("Panen OAuth jalan — browser headless buka authorize, ~15-30 detik...");
    try {
      const r = await oauthHarvest(oauthForm);
      setNotice(r.ok
        ? `SUKSES — refresh_token tersimpan (${r.id}), panen ${r.harvestSec}s. Mulai sekarang browser GAK dipake lagi, runtime murni HTTP.`
        : `Gagal (stage: ${r.stage}) — ${r.detail || "cookie gak cukup"}`);
      if (r.ok) setOauthForm({ cookies: "", label: "" });
      load();
    } catch (ex) { setErr(ex.message); }
    setOauthBusy(false);
  };

  const doRefresh = async (id) => {
    setNotice("Refresh token via HTTP...");
    try {
      const r = await oauthRefresh(id);
      setNotice(r.ok ? `Refresh OK — access_token baru (${r.expiresIn}s), murni HTTP tanpa browser.` : `Refresh gagal: ${r.detail}`);
      load();
    } catch (ex) { setErr(ex.message); }
  };

  const doAutoLogin = async (e) => {
    e.preventDefault();
    setLoginBusy(true); setErr(""); setNotice("");
    setNotice("Auto-login jalan — browser headless lagi ngetik, 1-2 menit jangan close tab...");
    try {
      const r = await autoLogin(loginForm);
      setNotice(r.ok ? `Login SUKSES — akun ${r.id} tersimpan (stage: ${r.stage})` : `Login gagal (stage: ${r.stage}) — ${r.detail || "cek email/password, akun ini mungkin via Google/Apple"}`);
      setLoginForm({ email: "", password: "", label: "" });
      load();
    } catch (ex) { setErr(ex.message); }
    setLoginBusy(false);
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
        <h2>Auto-login (email + password)</h2>
        <p className="muted">Pakai solver sidecar + browser headless. Coba ini kalau akun lu didaftarkan pakai email/password langsung (bukan "Continue with Google"). Butuh 1-2 menit.</p>
        <form onSubmit={doAutoLogin} className="form">
          <input placeholder="Email" type="email" value={loginForm.email} onChange={(e) => setLoginForm({ ...loginForm, email: e.target.value })} />
          <input placeholder="Password" type="password" value={loginForm.password} onChange={(e) => setLoginForm({ ...loginForm, password: e.target.value })} />
          <input placeholder="Label (opsional, default = email)" value={loginForm.label} onChange={(e) => setLoginForm({ ...loginForm, label: e.target.value })} />
          <button className="btn primary" disabled={loginBusy || !loginForm.email || !loginForm.password}>{loginBusy ? "Ngetik di browser..." : "Auto-login"}</button>
        </form>
      </div>
      <div className="card">
        <h2>🔑 OAuth Google (jalur cepat — sekali pakai selamanya)</h2>
        <p className="muted">
          Paste cookie Google dari extension Cookie-Editor (login Google di browser lu dulu, export cookie domain <code>.google.com</code>).
          Sidecar pakai cookie itu sekali buat dapetin <b>refresh_token</b> — setelah itu cookie boleh expired, browser gak dipake lagi,
          runtime chat murni HTTP (secepat qwen/deepseek). 15-30 detik.
        </p>
        <form onSubmit={doOAuth} className="form">
          <textarea rows={4} placeholder='Tempel array JSON dari Cookie-Editor: [{"name":"SID","value":"...","domain":".google.com"}, ...]'
            value={oauthForm.cookies} onChange={(e) => setOauthForm({ ...oauthForm, cookies: e.target.value })} />
          <input placeholder="Label (opsional)" value={oauthForm.label} onChange={(e) => setOauthForm({ ...oauthForm, label: e.target.value })} />
          <button className="btn primary" disabled={oauthBusy || !oauthForm.cookies.trim()}>{oauthBusy ? "Panen OAuth..." : "Panen refresh_token"}</button>
        </form>
      </div>
      <div className="card">
        <h2>Daftar ({list.length})</h2>
        {list.length === 0 && <p className="muted">Belum ada akun.</p>}
        {list.map((a) => (
          <div className="row" key={a.id}>
            <div>
              <b>{a.label || "(tanpa label)"}</b> <span className={`badge ${a.status}`}>{a.status}</span>
              {a.credentialType === "oauth_refresh" && <span className="badge info">refresh_token</span>}
              <div className="muted small">{a.requestCount} req · {a.successCount} ok · {a.errorCount} err{a.lastError ? ` · ${a.lastError.slice(0, 80)}` : ""}</div>
            </div>
            <div className="actions">
              {a.credentialType === "oauth_refresh" && <button className="btn small" onClick={() => doRefresh(a.id)}>Refresh</button>}
              <button className="btn small" onClick={() => check(a.id)}>Cek</button>
              <button className="btn small danger" onClick={() => del(a.id)}>Hapus</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
