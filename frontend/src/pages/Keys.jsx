import React, { useEffect, useState } from "react";
import { getKeys, addKey, deleteKey } from "../api.js";

export default function Keys() {
  const [list, setList] = useState([]);
  const [newKey, setNewKey] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState("");

  const load = () => getKeys().then((d) => setList(d.keys)).catch((e) => setErr(e.message));
  useEffect(() => { load(); }, []);

  const create = async () => {
    setErr("");
    try {
      const r = await addKey(name || "default");
      setNewKey(r.key);
      setName("");
      load();
    } catch (ex) { setErr(ex.message); }
  };
  const copy = async () => { try { await navigator.clipboard.writeText(newKey); } catch {} };
  const del = async (id) => { await deleteKey(id); load(); };

  return (
    <div>
      <h1>API Keys</h1>
      {err && <div className="alert">{err}</div>}
      <div className="card">
        <h2>Buat key baru</h2>
        <div className="form inline">
          <input placeholder="Nama (opsional)" value={name} onChange={(e) => setName(e.target.value)} />
          <button className="btn primary" onClick={create}>Generate</button>
        </div>
        {newKey && (
          <div className="keybox">
            <code>{newKey}</code>
            <button className="btn small" onClick={copy}>Copy</button>
            <p className="muted small">⚠️ Simpan sekarang — plaintext cuma ditampilkan sekali.</p>
          </div>
        )}
      </div>
      <div className="card">
        <h2>Daftar ({list.length})</h2>
        {list.length === 0 && <p className="muted">Belum ada key.</p>}
        {list.map((k) => (
          <div className="row" key={k.id}>
            <div><b>{k.name || "default"}</b> <span className="muted small">{k.keyPreview}</span></div>
            <button className="btn small danger" onClick={() => del(k.id)}>Hapus</button>
          </div>
        ))}
      </div>
    </div>
  );
}
