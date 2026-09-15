import React, { useState } from "react";
import { login as adminLogin } from "../api.js";

export default function Login() {
  const [u, setU] = useState("admin");
  const [p, setP] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true); setErr("");
    try {
      const r = await adminLogin(u, p);
      localStorage.setItem("admin_token", r.token);
      location.href = "/";
    } catch (ex) {
      setErr(ex.message);
    }
    setBusy(false);
  };

  return (
    <div className="center-screen">
      <form className="login" onSubmit={submit}>
        <div className="logo"><span className="mark">C2</span> chatgpt2API</div>
        <p className="muted small" style={{ margin: "0 0 6px" }}>
          Masuk ke konsol admin untuk mengelola akun, API key, dan memantau trafik.
        </p>
        {err && <div className="alert" style={{ marginBottom: 0 }}>{err}</div>}
        <label className="field">
          <span className="lab">Username</span>
          <input value={u} onChange={(e) => setU(e.target.value)} autoComplete="username" />
        </label>
        <label className="field">
          <span className="lab">Password</span>
          <input type="password" value={p} onChange={(e) => setP(e.target.value)} autoComplete="current-password" />
        </label>
        <button className="btn primary" disabled={busy}>{busy ? "Masuk…" : "Masuk"}</button>
        <div className="note">
          Kredensial default ada di <code>config.yaml</code> → <code>admin</code>.
        </div>
      </form>
    </div>
  );
}
