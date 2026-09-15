import React, { useEffect, useState } from "react";
import { getKeys, addKey, deleteKey } from "../api.js";
import { useDocPanel, Code } from "../docpanel.jsx";

const AUTH_EX = `# semua request inference pakai header ini
Authorization: Bearer sk-••••••••••••••••

# contoh
curl http://localhost:8800/v1/models \\
  -H "Authorization: Bearer sk-••••••••••••••••"`;

const KEY_SHAPE = `sk-<24 hex>
contoh: sk-181f9c2e7b04a5d83f6e1092`;

export default function Keys() {
  const [list, setList] = useState([]);
  const [newKey, setNewKey] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState("");
  const [copied, setCopied] = useState(false);

  const load = () => getKeys().then((d) => setList(d.keys)).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

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
      setName("");
      load();
    } catch (ex) { setErr(ex.message); }
  };

  const copy = async () => {
    try { await navigator.clipboard.writeText(newKey); setCopied(true); setTimeout(() => setCopied(false), 1800); } catch {}
  };
  const del = async (id) => { await deleteKey(id); load(); };

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
            <div className="hint" style={{ marginBottom: 6 }}>Key baru — salin sekarang, cuma tampil sekali</div>
            <code>{newKey}</code>
            <div style={{ marginTop: 9 }}>
              <button className="btn small" onClick={copy}>{copied ? "✓ Tersalin" : "Salin"}</button>
            </div>
          </div>
        )}
      </div>

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
                    <td className="k">{k.name || "default"}</td>
                    <td className="ty">{k.keyPreview}</td>
                    <td className="desc">
                      {k.createdAt ? new Date(k.createdAt * 1000).toLocaleDateString("id-ID") : "—"}
                    </td>
                    <td style={{ textAlign: "right" }}>
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
