import React, { useEffect, useState } from "react";
import { getLogs } from "../api.js";
import { useDocPanel, Code } from "../docpanel.jsx";

const ERRCODES = `401  token mati / akun invalid
403  challenge WAF (bukan kredensial mati)
429  rate limit upstream
502  upstream tidak merespons
503  pool kosong / akun tidak tersedia`;

const SOURCES = `api     request inference masuk
admin   perubahan akun / key
oauth   refresh token
upstream jalur ChatGPT (android / web)`;

export default function Logs() {
  const [logs, setLogs] = useState([]);
  const [err, setErr] = useState("");
  const [filter, setFilter] = useState("semua");

  useEffect(() => {
    const load = () => getLogs().then((d) => setLogs(d.logs)).catch((e) => setErr(e.message));
    load();
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, []);

  useDocPanel("Referensi log", [
    <div key="a">
      <div className="h">Kode error</div>
      <Code label="artinya">{ERRCODES}</Code>
    </div>,
    <div key="b">
      <div className="h">Sumber</div>
      <Code label="source">{SOURCES}</Code>
    </div>,
    <div key="c">
      <div className="h">Catatan</div>
      <p className="hint" style={{ margin: 0 }}>
        Halaman ini refresh otomatis tiap 5 detik. Log cuma menyimpan beberapa ratus
        entri terakhir supaya DB tetap ringan.
      </p>
    </div>,
  ]);

  const shown = logs.filter((l) => filter === "semua" || l.level === filter);

  return (
    <div className="content">
      <h1>Logs</h1>
      <p className="lede">
        Aktivitas gateway secara real-time. Auto-refresh tiap 5 detik —{" "}
        <span className="muted">{logs.length} entri dimuat</span>.
      </p>

      {err && <div className="alert">{err}</div>}

      <div className="card flush">
        <div className="card-head">
          <h2>Activity</h2>
          <div className="right">
            <div className="tabs" style={{ border: "none", marginBottom: 0 }}>
              {["semua", "info", "error", "warn"].map((lv) => (
                <button
                  key={lv}
                  className={"tab" + (filter === lv ? " on" : "")}
                  style={{ padding: "4px 9px", borderBottom: "none" }}
                  onClick={() => setFilter(lv)}
                >
                  {lv}
                </button>
              ))}
            </div>
          </div>
        </div>
        <div className="card-body" style={{ padding: 0 }}>
          {shown.length === 0 && (
            <p className="muted" style={{ padding: 16, margin: 0 }}>Belum ada log untuk filter ini.</p>
          )}
          {shown.map((l) => (
            <div className="row log" key={l.id} style={{ padding: "9px 16px" }}>
              <span className="mono tiny" style={{ color: "var(--tx-faint)", minWidth: 62 }}>
                {new Date(l.ts * 1000).toLocaleTimeString("id-ID", { hour12: false })}
              </span>
              <span className={`badge ${l.level}`}>{l.level}</span>
              <span className="mono tiny" style={{ color: "var(--tx-dim)", minWidth: 62 }}>{l.source}</span>
              <span style={{ flex: 1, minWidth: 0, wordBreak: "break-word" }}>{l.message}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
