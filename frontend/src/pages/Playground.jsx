import React, { useEffect, useState } from "react";
import { getKeys, getSystem } from "../api.js";

export default function Playground() {
  const [keys, setKeys] = useState([]);
  const [models, setModels] = useState(["gpt-5"]);
  const [model, setModel] = useState("gpt-5");
  const [featWeb, setFeatWeb] = useState(false);
  const [featThink, setFeatThink] = useState(false);
  const [prompt, setPrompt] = useState("Halo, perkenalkan diri lu singkat.");
  const [out, setOut] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [rawKey, setRawKey] = useState("");

  useEffect(() => {
    getKeys().then((d) => setKeys(d.keys || [])).catch(() => {});
    getSystem().then((d) => {
      const base = d.models || ["gpt-5"];
      setModels([...base, ...(d.modelVariants || [])]);
    }).catch(() => {});
  }, []);

  const finalModel = (() => {
    let m = model;
    // hindari dobel suffix kalau user pilih varian dari dropdown sekaligus ngecek toggle
    if (featWeb && !m.endsWith("-web")) m += "-web";
    if (featThink && !m.endsWith("-thinking")) m += "-thinking";
    return m;
  })();

  const send = async () => {
    if (!rawKey.trim()) { setErr("Isi API key (sk-...) dulu — yang di daftar cuma preview."); return; }
    setBusy(true); setOut(""); setErr("");
    try {
      const res = await fetch("/v1/chat/completions", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${rawKey.trim()}` },
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
    <div>
      <h1>Playground</h1>
      {err && <div className="alert">{err}</div>}
      <div className="card">
        <div className="form">
          <select value={model} onChange={(e) => setModel(e.target.value)}>
            {models.map((m) => <option key={m}>{m}</option>)}
          </select>
          <label className="chk">
            <input type="checkbox" checked={featWeb} onChange={(e) => setFeatWeb(e.target.checked)} />
            🔍 Web search
          </label>
          <label className="chk">
            <input type="checkbox" checked={featThink} onChange={(e) => setFeatThink(e.target.checked)} />
            🧠 Thinking
          </label>
          <input placeholder="API key sk-... (isi manual, preview gak bisa dipakai)" value={rawKey} onChange={(e) => setRawKey(e.target.value)} />
          <textarea rows={4} placeholder="Prompt" value={prompt} onChange={(e) => setPrompt(e.target.value)} />
          <button className="btn primary" onClick={send} disabled={busy}>{busy ? "Ngetik..." : `Kirim → ${finalModel}`}</button>
        </div>
        {keys.length > 0 && <p className="muted small">{keys.length} key terdaftar — buat test, bikin key baru biar plaintext-nya keliatan.</p>}
      </div>
      <div className="card">
        <h2>Respons</h2>
        <pre className="out">{out || <span className="muted">(kosong — butuh akun ChatGPT valid)</span>}</pre>
      </div>
    </div>
  );
}
