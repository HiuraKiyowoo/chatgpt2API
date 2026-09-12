import React, { useEffect, useState } from "react";
import { getLogs } from "../api.js";

export default function Logs() {
  const [logs, setLogs] = useState([]);
  const [err, setErr] = useState("");
  useEffect(() => {
    const load = () => getLogs().then((d) => setLogs(d.logs)).catch((e) => setErr(e.message));
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, []);
  if (err) return <div className="alert">{err}</div>;
  return (
    <div>
      <h1>Logs</h1>
      <div className="card">
        {logs.length === 0 && <p className="muted">Belum ada log.</p>}
        {logs.map((l) => (
          <div className="row log" key={l.id}>
            <span className="muted small">{new Date(l.ts * 1000).toLocaleTimeString()}</span>
            <span className={`badge ${l.level}`}>{l.level}</span>
            <span className="small">{l.source}</span>
            <span>{l.message}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
