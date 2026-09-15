// Kunci API terpilih — dipakai bareng oleh halaman Keys dan Playground.
// Disimpan di localStorage supaya tetap ketempel waktu pindah menu / reload.
const K_ACTIVE = "c2api.active_key";
const K_KEYS = "c2api.keys_cache";

export function getActiveKey() {
  try { return localStorage.getItem(K_ACTIVE) || ""; } catch { return ""; }
}

export function setActiveKey(v) {
  try {
    if (v) localStorage.setItem(K_ACTIVE, v);
    else localStorage.removeItem(K_ACTIVE);
  } catch {}
  // kabari halaman lain (Playground) tanpa reload
  try { window.dispatchEvent(new Event("c2api:key")); } catch {}
}

// cache daftar key biar Playground bisa nampilin pilihan tanpa request ulang
export function getKeyList() {
  try { return JSON.parse(localStorage.getItem(K_KEYS) || "[]"); } catch { return []; }
}

export function setKeyList(list) {
  try { localStorage.setItem(K_KEYS, JSON.stringify(list || [])); } catch {}
  try { window.dispatchEvent(new Event("c2api:keylist")); } catch {}
}

export function maskKey(k) {
  if (!k) return "";
  if (k.length <= 12) return k;
  return k.slice(0, 7) + "…" + k.slice(-4);
}
