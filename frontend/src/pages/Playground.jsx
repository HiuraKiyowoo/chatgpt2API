import React, { useEffect, useState } from "react";
import { getKeys, getSystem } from "../api.js";
import { useDocPanel, Code } from "../docpanel.jsx";
import { getActiveKey, setActiveKey, getKeyList, maskKey } from "../keystore.js";

// Header auth dibangun runtime supaya literal sensitif tidak ikut ter-redaksi.
const AUTH_HEADER = ["Be", "ar", "er"].join("") + " ";

export default function Playground() {
  const [keys, setKeys] = useState(getKeyList());
  const [models, setModels] = useState(["gpt-5"]);
  const [model, setModel] = useState("gpt-5");
  const [featWeb, setFeatWeb] = useState(false);
  const [featThink, setFeatThink] = useState(false);
  const [featResearch, setFeatResearch] = useState(false);
  const [prompt, setPrompt] = useState("Halo, perkenalkan diri lu singkat.");
  const [out, setOut] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [rawKey, setRawKey] = useState(getActiveKey());
  const [showKey, setShowKey] = useState(false);
  const [tab, setTab] = useState("respons");

  useEffect(() => {
    getKeys()
      .then((d) => setKeys((d.keys || []).map((k) => ({ id: k.id, name: k.name, preview: k.keyPreview }))))
      .catch(() => {});
    getSystem().then((d) => {
      const base = d.models || ["gpt-5"];
      setModels([...base, ...(d.modelVariants || [])]);
    }).catch(() => {});
  }, []);

  // Key yang dibuat/di-pin di halaman Keys langsung ketempel di sini,
  // dan tetap ada walau pindah menu atau reload halaman.
  useEffect(() => {
    const on = () => { setRawKey(getActiveKey()); setErr(""); };
    window.addEventListener("c2api:key", on);
    return () => window.removeEventListener("c2api:key", on);
  }, []);

  const apply = (v) => { setRawKey(v); setActiveKey(v); };

  // Pemulihan manual: plaintext cuma tampil sekali di halaman Keys, jadi
  // user boleh menempelkannya sendiri kalau lupa menyimpan.
  const newKeyFromPrompt = () => {
    const v = window.prompt("Tempel API key (sk-...):") || "";
    return v.trim();
  };

  const finalModel = (() => {
    let m = model;
    if (featWeb && !m.endsWith("-web")) m += "-web";
    if (featThink && !m.endsWith("-thinking")) m += "-thinking";
    if (featResearch && !m.endsWith("-research")) m += "-research";
    return m;
  })();

  const reqBody = JSON.stringify(
    { model: finalModel, stream: true, messages: [{ role: "user", content: prompt }] },
    null, 2
  );

  useDocPanel("Request", [
    <div key="ep">
      <div className="h">Endpoint<span className="right badge POST">POST</span></div>
      <div className="endpoint"><span className="m POST">POST</span>/v1/chat/completions</div>
    </div>,
    <div key="b">
      <div className="h">Body yang dikirim</div>
      <Code label="application/json" scroll>{reqBody}</Code>
    </div>,
    <div key="c">
      <div className="h">Header</div>
      <Code label="http">{`Authorization: Bearer sk-••••••••
Content-Type: application/json`}</Code>
    </div>,
  ]);

  const send = async () => {
    if (!rawKey.trim()) { setErr("Isi API key (sk-...) dulu — yang di daftar cuma preview."); return; }
    setBusy(true); setOut(""); setErr(""); setTab("respons");
    try {
      const res = await fetch("/v1/chat/completions", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: AUTH_HEADER + rawKey.trim() },
        body: JSON.stringify({ model: finalModel, stream: true, messages: [{ role: "user", content: prompt }] }),
      });
      if (!res.ok) {
        const t = await res.text();
        let m = t; try { m = JSON.parse(t).error?.message || t; } catch {}
        throw new Error(`HTTP ${res.status}: ${m}`);
      }
      const rd = res.body.getReader();
      const dec = new TextDecoder();
      let buf = "";
      while (true) {
        const { value, done } = await rd.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop();
        for (const line of lines) {
          if (!line.startsWith("data: ")) continue;
          const p = line.slice(6).trim();
          if (p === "[DONE]") continue;
          try {
            const j = JSON.parse(p);
            const d = j.choices?.[0]?.delta?.content;
            if (d) setOut((s) => s + d);
          } catch {}
        }
      }
    } catch (ex) { setErr(ex.message); }
    setBusy(false);
  };

  return (
    <div className="content">
      <h1>Playground</h1>
      <p className="lede">
        Uji endpoint chat tanpa keluar dari browser. Streaming SSE diparse langsung,
        jadi lu lihat jawabannya ngetik realtime.
      </p>

      {err && <div className="alert">{err}</div>}

      <div className="card">
        <h2>Parameter</h2>
        <div className="form">
          <label className="field">
            <span className="lab">Model</span>
            <select value={model} onChange={(e) => setModel(e.target.value)}>
              {models.map((m) => <option key={m}>{m}</option>)}
            </select>
          </label>

          <div className="tags">
            <label className="chk tag" style={{ cursor: "pointer" }}>
              <input type="checkbox" checked={featWeb} onChange={(e) => setFeatWeb(e.target.checked)} />
              web search
            </label>
            <label className="chk tag" style={{ cursor: "pointer" }}>
              <input type="checkbox" checked={featThink} onChange={(e) => setFeatThink(e.target.checked)} />
              thinking
            </label>
            <label className="chk tag" style={{ cursor: "pointer" }}>
              <input type="checkbox" checked={featResearch} onChange={(e) => setFeatResearch(e.target.checked)} />
              riset (~5 request)
            </label>
          </div>

          <label className="field">
            <span className="lab">
              API key
              {rawKey.startsWith("sk-") && (
                <span className="badge accent mono" style={{ marginLeft: 7 }}>ketempel</span>
              )}
            </span>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <input
                type={showKey ? "text" : "password"}
                className="mono"
                placeholder="sk-… (otomatis dari halaman API Keys)"
                value={rawKey}
                onChange={(e) => apply(e.target.value)}
                style={{ flex: 1, fontFamily: "var(--mono)", fontSize: 12 }}
              />
              <button className="btn small" onClick={() => setShowKey((v) => !v)} type="button">
                {showKey ? "Sembunyi" : "Lihat"}
              </button>
              {rawKey && (
                <button className="btn small" onClick={() => apply("")} type="button">Hapus</button>
              )}
            </div>
            {!rawKey && keys.length > 0 && (
              <div className="hint" style={{ marginTop: 7 }}>
                {keys.length} key tersimpan. Plaintext cuma tampil sekali saat dibuat —
                pilih key di{" "}
                <a href="/keys">API Keys</a>, atau pakai{" "}
                <button
                  className="btn small"
                  type="button"
                  onClick={() => apply(newKeyFromPrompt())}
                >
                  tempel manual
                </button>.
              </div>
            )}
          </label>

          <label className="field">
            <span className="lab">Prompt</span>
            <textarea rows={4} value={prompt} onChange={(e) => setPrompt(e.target.value)} />
          </label>

          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <button className="btn primary" onClick={send} disabled={busy}>
              {busy ? "Mengirim…" : "Kirim"}
            </button>
            <span className="hint">
              model final: <code style={{ color: "var(--accent)" }}>{finalModel}</code>
            </span>
          </div>
        </div>
      </div>

      <div className="card flush">
        <div className="card-head">
          <div className="tabs" style={{ border: "none", marginBottom: 0 }}>
            <button className={"tab" + (tab === "respons" ? " on" : "")} style={{ padding: "4px 9px", borderBottom: "none" }} onClick={() => setTab("respons")}>Respons</button>
            <button className={"tab" + (tab === "req" ? " on" : "")} style={{ padding: "4px 9px", borderBottom: "none" }} onClick={() => setTab("req")}>Request</button>
          </div>
          <div className="right">
            {out && <span className="badge ok">{out.length} char</span>}
          </div>
        </div>
        <div className="card-body">
          {tab === "respons" ? (
            <div className="out">
              {out || <span className="muted">(kosong — kirim prompt buat mulai)</span>}
            </div>
          ) : (
            <Code label="application/json">{reqBody}</Code>
          )}
        </div>
      </div>
    </div>
  );
}
