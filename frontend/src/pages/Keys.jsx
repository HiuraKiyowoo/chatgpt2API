import React, { useEffect, useState } from "react";
import { getKeys, addKey, deleteKey } from "../api.js";
import { useDocPanel, Code } from "../docpanel.jsx";
import { setActiveKey, getActiveKey, setKeyList, maskKey } from "../keystore.js";

const AUTH_EX = `# semua request inference pakai header ini
Authorization: Bearer sk-••••••••••••••••

# contoh
curl http://localhost:8800/v1/models \\
  -H "Authorization: Bearer sk-••••••••••••••••"`;

const KEY_SHAPE = `sk-<24 hex>`;

export default function Keys() {
  const [list, setList] = useState([]);
  const [newKey, setNewKey] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState("");
  const [copied, setCopied] = useState(false);
  const [pinned, setPinned] = useState(getActiveKey());

  const load = () =>
    getKeys()
      .then((d) => {
        setList(d.keys);
        setKeyList((d.keys || []).map((k) => ({ id: k.id, name: k.name, preview: k.keyPreview })));
      })
      .catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  useEffect(() => {
    const on = () => setPinned(getActiveKey());
    window.addEventListener("c2api:key", on);
    return () => window.removeEventListener("c2api:key", on);
  }, []);

  useDocPanel("Autentikasi", [
    <div key="a">
      <div className="h">Header</div>
      <Code label="http">{AUTH_EX}</Code>
    </div>,
    <div key="b">
      <div className="h">Format key</div>
      <Code label="plaintext">{KEY_SHAPE}</Code>
    </div>,
    <div key="c">
      <div className="h">Catatan</div>
      <p className="hint" style={{ margin: 0 }}>
        Key disimpan sebagai hash — plaintext cuma tampil sekali saat dibuat.
        Kalau hilang, hapus dan buat baru. Beda dengan akun upstream: key ini yang
        dipakai <i>klien</i> lu (9router, SDK, dsb).
      </p>
    </div>,
  ]);

  const create = async () => {
    setErr("");
    try {
      const r = await addKey(name || "default");
      setNewKey(r.key);
      setActiveKey(r.key);   // langsung ketempel ke Playground
      setPinned(r.key);
      setName("");
      load();
    } catch (ex) { setErr(ex.message); }
  };

  const copy = async () => {
    try { await navigator.clipboard.writeText(newKey); setCopied(true); setTimeout(() => setCopied(false), 1800); } catch {}
  };
  const del = async (id) => { await deleteKey(id); load(); };
  const pin = (k) => { setActiveKey(k); setPinned(k); };
  const unpin = () => { setActiveKey(""); setPinned(""); };

  return (
    <div className="content">
      <h1>API Keys</h1>
      <p className="lede">
        Key buat klien yang manggil gateway. Tiap key bisa dicabut kapan saja tanpa
        mengganggu akun upstream.
      </p>

      {err && <div className="alert">{err}</div>}

      <div className="card">
        <h2>Buat key</h2>
        <div className="form inline">
          <input
            placeholder="Nama — cth: 9router, laptop, bot"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <button className="btn primary" onClick={create}>Generate</button>
        </div>

        {newKey && (
          <div className="keybox">
            <div className="hint" style={{ marginBottom: 6 }}>
              Key baru — <b>sudah otomatis ketempel di Playground</b>. Salin juga buat klien luar, cuma tampil sekali ini.
            </div>
            <code>{newKey}</code>
            <div style={{ marginTop: 9, display: "flex", gap: 6 }}>
              <button className="btn small" onClick={copy}>{copied ? "✓ Tersalin" : "Salin"}</button>
              <a className="btn small" href="/playground">Buka Playground →</a>
            </div>
          </div>
        )}
      </div>

      {pinned && (
        <div className="notice" style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span className="badge accent mono">AKTIF</span>
          <span style={{ flex: 1 }}>
            Key aktif di Playground: <code>{maskKey(pinned)}</code>
          </span>
          <button className="btn small" onClick={unpin}>Lepas</button>
        </div>
      )}

      <div className="card flush">
        <div className="card-head">
          <h2>Key aktif</h2>
          <span className="right badge">{list.length}</span>
        </div>
        <div className="card-body" style={{ padding: list.length ? 0 : 16 }}>
          {list.length === 0 && <p className="muted" style={{ margin: 0 }}>Belum ada key. Buat satu di atas.</p>}
          {list.length > 0 && (
            <table className="t">
              <thead>
                <tr><th>Nama</th><th>Preview</th><th>Dibuat</th><th></th></tr>
              </thead>
              <tbody>
                {list.map((k) => (
                  <tr key={k.id}>
                    <td className="k">
                      {k.name || "default"}
                      {k.keyPreview && pinned && pinned.startsWith(k.keyPreview.slice(0, 7)) && (
                        <span className="badge accent mono" style={{ marginLeft: 6 }}>aktif</span>
                      )}
                    </td>
                    <td className="ty">{k.keyPreview}</td>
                    <td className="desc">
                      {k.createdAt ? new Date(k.createdAt * 1000).toLocaleDateString("id-ID") : "—"}
                    </td>
                    <td style={{ textAlign: "right", whiteSpace: "nowrap" }}>
                      <button className="btn small" onClick={() => pin(newKey || "")} disabled={!newKey}>
                        Pakai
                      </button>
                      <button className="btn small danger" onClick={() => del(k.id)}>Hapus</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
}
