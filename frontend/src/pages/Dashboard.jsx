import React, { useEffect, useState } from "react";
import { getSystem } from "../api.js";

export default function Dashboard() {
  const [sys, setSys] = useState(null);
  const [err, setErr] = useState("");
  useEffect(() => { getSystem().then(setSys).catch((e) => setErr(e.message)); }, []);

  if (err) return <div className="alert">{err}</div>;
  if (!sys) return <div className="muted">Memuat...</div>;
  return (
    <div>
      <h1>Dashboard</h1>
      <div className="grid stats">
        <div className="card stat"><div className="num">{sys.accounts.valid}/{sys.accounts.total}</div><div className="muted">Akun Valid</div></div>
        <div className="card stat"><div className="num">{sys.apiKeys}</div><div className="muted">API Keys</div></div>
        <div className="card stat"><div className="num">{Math.floor(sys.uptimeSec / 60)}m</div><div className="muted">Uptime</div></div>
        <div className="card stat"><div className="num">v{sys.version}</div><div className="muted">Versi</div></div>
      </div>
      <div className="card">
        <h2>Model tersedia</h2>
        <div className="tags">{sys.models.map((m) => <span className="tag" key={m}>{m}</span>)}</div>
      </div>
      <div className="card">
        <h2>Solver (cloudflare-solver sidecar)</h2>
        <p className="muted">{sys.solver.enabled ? `Aktif → ${sys.solver.url}` : "Mati — jalur utama tetap jalan pakai accessToken/cookie; aktifkan buat renew cf_clearance & auto-login eksperimental."}</p>
      </div>
      <div className="card">
        <h2>Cara pakai</h2>
        <pre className="code">{`curl http://HOST:${location.port || 80}/v1/chat/completions \\
  -H "Authorization: Bearer sk-..." \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-5","messages":[{"role":"user","content":"hai"}]}'`}</pre>
      </div>
    </div>
  );
}
