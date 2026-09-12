const BASE = "";

export async function api(path, opts = {}) {
  const headers = { "Content-Type": "application/json", ...(opts.headers || {}) };
  const tok = localStorage.getItem("admin_token");
  if (tok) headers.Authorization = `Bearer ${tok}`;
  const res = await fetch(BASE + path, { ...opts, headers });
  let data = null;
  try { data = await res.json(); } catch {}
  if (res.status === 401 && !path.includes("/login")) {
    localStorage.removeItem("admin_token");
    location.href = "/login";
    throw new Error("sesi habis");
  }
  if (!res.ok) throw new Error((data && (data.error?.message || data.error)) || `HTTP ${res.status}`);
  return data;
}

export const getAccounts = () => api("/api/accounts");
export const addAccount = (body) => api("/api/accounts", { method: "POST", body: JSON.stringify(body) });
export const deleteAccount = (id) => api(`/api/accounts/${id}`, { method: "DELETE" });
export const checkAccount = (id) => api(`/api/accounts/${id}/check`, { method: "POST" });
export const getKeys = () => api("/api/keys");
export const addKey = (name) => api("/api/keys", { method: "POST", body: JSON.stringify({ name }) });
export const deleteKey = (id) => api(`/api/keys/${id}`, { method: "DELETE" });
export const getLogs = () => api("/api/logs");
export const getSystem = () => api("/api/system");
export const login = (username, password) => api("/api/admin/login", { method: "POST", body: JSON.stringify({ username, password }) });
