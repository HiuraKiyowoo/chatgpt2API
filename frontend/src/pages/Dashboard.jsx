import React, { useEffect, useState } from "react";
import { getSystem } from "../api.js";
import { useDocPanel, Code, Endpoint } from "../docpanel.jsx";

const REQ = `curl http://localhost:8800/v1/chat/completions \\
  -H "Authorization: Bearer sk-••••••••" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5-web",
    "messages": [
      { "role": "user", "content": "harga saham BBCA hari ini?" }
    ],
    "stream": false
  }'`;

const RESP = `{
  "id": "chatcmpl-9f2c1a",
  "object": "chat.completion",
  "created": 1789466093,
  "model": "gpt-5-web",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "BBCA ditutup di 6.325 (naik 0,4%)..."
    },
    "finish_reason": "stop"
  }],
  "usage": { "prompt_tokens": 0, "completion_tokens": 0 }
}`;

const ERR = `{
  "error": {
    "code": "upstream_error",
    "message": "upstream 401: token_revoked",
    "type": "upstream_error"
  }
}`;

export default function Dashboard() {
  const [sys, setSys] = useState(null);
  const [err, setErr] = useState("");

  useEffect(() => { getSystem().then(setSys).catch((e) => setErr(e.message)); }, []);

  useDocPanel("Chat Completions", [
    <React.Fragment key="ep">
      <div className="h">Endpoint<span className="right badge GET">POST</span></div>
      <Endpoint method="POST" path="/v1/chat/completions" />
    </React.Fragment>,
    <div key="req">
      <div className="h">Request</div>
      <Code label="bash · curl">{REQ}</Code>
    </div>,
    <div key="res">
      <div className="h">Response <span className="right status s2">200</span></div>
      <Code label="application/json" scroll>{RESP}</Code>
    </div>,
    <div key="err">
      <div className="h">Error <span className="right status s5">401</span></div>
      <Code label="application/json" scroll>{ERR}</Code>
    </div>,
  ]);

  if (err) return <div className="content"><div className="alert">{err}</div></div>;
  if (!sys) return <div className="content"><div className="muted">Memuat…</div></div>;

  const port = location.port || 80;
  const variants = sys.modelVariants || [];

  return (
    <div className="content">
      <h1>Dashboard</h1>
      <p className="lede">
        Gateway OpenAI-compatible untuk ChatGPT web. Endpoint-nya sama seperti API resmi —
        cukup ganti <code>base_url</code> ke <code>http://localhost:{port}/v1</code>.
      </p>

      <div className="grid stats">
        <div className="stat">
          <div className="k">Akun valid</div>
          <div className={"num" + (sys.accounts.valid > 0 ? " ok" : " err")}>
            {sys.accounts.valid}<span className="muted" style={{ fontSize: 15 }}>/{sys.accounts.total}</span>
          </div>
        </div>
        <div className="stat"><div className="k">API keys</div><div className="num">{sys.apiKeys}</div></div>
        <div className="stat"><div className="k">Uptime</div><div className="num">{Math.floor(sys.uptimeSec / 60)}<span className="muted" style={{ fontSize: 15 }}>m</span></div></div>
        <div className="stat"><div className="k">Versi</div><div className="num">{sys.version}</div></div>
      </div>

      <h3>Base URL</h3>
      <div className="card flush">
        <div className="card-body" style={{ paddingTop: 14 }}>
          <div className="endpoint" style={{ marginBottom: 10 }}>
            <span className="m POST">BASE</span>
            http://localhost:{port}/v1
          </div>
          <table className="t">
            <thead>
              <tr><th>Endpoint</th><th>Kegunaan</th></tr>
            </thead>
            <tbody>
              <tr><td className="k">/v1/chat/completions</td><td className="desc">Chat + streaming (SSE)</td></tr>
              <tr><td className="k">/v1/models</td><td className="desc">Daftar model &amp; varian fitur</td></tr>
              <tr><td className="k">/api/health</td><td className="desc">Health check gateway</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <h3>Model</h3>
      <div className="card">
        <div className="tags">
          {sys.models.map((m) => <span className="tag" key={m}>{m}</span>)}
        </div>
        <p className="hint" style={{ marginTop: 12 }}>
          Suffix fitur bisa ditempel ke semua model: <code>-web</code> (cari web + sitasi),{" "}
          <code>-thinking</code> (reasoning effort tinggi), <code>-research</code> (riset 3 fase).
          Contoh: <code>gpt-5-web-thinking</code>.
        </p>
        {variants.length > 0 && (
          <div className="tags" style={{ marginTop: 10 }}>
            {variants.map((v) => <span className="tag accent" key={v}>{v}</span>)}
          </div>
        )}
      </div>

      <h3>Fitur</h3>
      <div className="card flush">
        <div className="card-head"><h2>Jalur upstream</h2></div>
        <div className="card-body" style={{ padding: 0 }}>
          <table className="t">
            <thead>
              <tr><th>Fitur</th><th>Slug</th><th>Catatan</th></tr>
            </thead>
            <tbody>
              <tr>
                <td className="k">Web search</td>
                <td className="ty">gpt-5-web</td>
                <td className="desc">Browsing otomatis, markup sitasi dibersihkan</td>
              </tr>
              <tr>
                <td className="k">Thinking</td>
                <td className="ty">gpt-5-thinking</td>
                <td className="desc">Reasoning effort tinggi</td>
              </tr>
              <tr>
                <td className="k">Riset</td>
                <td className="ty">gpt-5-research</td>
                <td className="desc">3 fase: rencana → cari paralel → sintesis</td>
              </tr>
              <tr>
                <td className="k">Transport</td>
                <td className="ty">android → web</td>
                <td className="desc">Jalur app Android dulu, fallback web</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <h3>Solver</h3>
      <div className="card">
        <p className="muted" style={{ margin: 0 }}>
          {sys.solver.enabled
            ? <>Aktif → <code>{sys.solver.url}</code></>
            : <>Mati. Jalur utama tetap jalan pakai cookie/accessToken; solver cuma buat renew{" "}
                <code>cf_clearance</code>.</>}
        </p>
      </div>

      <h3>Mulai cepat</h3>
      <Code label="bash · curl">{`curl http://localhost:${port}/v1/chat/completions \\
  -H "Authorization: Bearer sk-••••••••" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-5","messages":[{"role":"user","content":"hai"}]}'`}</Code>
      <p className="hint" style={{ marginTop: 8 }}>
        Butuh API key? Buat di <a href="/keys">API Keys</a>.
      </p>
    </div>
  );
}
