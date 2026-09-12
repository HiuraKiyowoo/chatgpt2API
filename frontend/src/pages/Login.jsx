import React, { useState } from "react";
import { login } from "../api.js";

export default function Login() {
  const [u, setU] = useState("");
  const [p, setP] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e) => {
    e.preventDefault();
    setBusy(true); setErr("");
    try {
      const { token } = await login(u, p);
      localStorage.setItem("admin_token", token);
      location.href = "/";
    } catch (ex) { setErr(ex.message); }
    setBusy(false);
  };

  return (
    <div className="center-screen">
      <form className="card login" onSubmit={submit}>
        <h1>⚡ chatgpt2API</h1>
        <p className="muted">Masuk admin dashboard</p>
        <input placeholder="Username" value={u} onChange={(e) => setU(e.target.value)} autoFocus />
        <input placeholder="Password" type="password" value={p} onChange={(e) => setP(e.target.value)} />
        {err && <div className="alert">{err}</div>}
        <button className="btn primary" disabled={busy}>{busy ? "Memeriksa..." : "Masuk"}</button>
      </form>
    </div>
  );
}
