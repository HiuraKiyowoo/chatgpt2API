import React, { useEffect, useState } from "react";
import { Outlet, NavLink, useLocation } from "react-router-dom";
import "./styles.css";

const NAV = [
  { group: "Overview", items: [
    { to: "/", ico: "◈", label: "Dashboard", end: true },
  ]},
  { group: "Config", items: [
    { to: "/accounts", ico: "◉", label: "Accounts" },
    { to: "/keys", ico: "⌘", label: "API Keys" },
  ]},
  { group: "Tools", items: [
    { to: "/playground", ico: "▶", label: "Playground" },
    { to: "/logs", ico: "≡", label: "Logs" },
  ]},
];

const CRUMB = {
  "/": "dashboard",
  "/accounts": "accounts",
  "/keys": "api-keys",
  "/playground": "playground",
  "/logs": "logs",
};

const linkCls = ({ isActive }) => "navlink" + (isActive ? " active" : "");

export default function App() {
  const [ver, setVer] = useState("1.0.0");
  const loc = useLocation();
  const [panel, setPanel] = useState(null);

  // Panel contoh request/response di kanan: halaman bisa "mendaftarkan" isinya
  // lewat window event, jadi tidak perlu prop-drilling ke tiap page.
  useEffect(() => {
    const on = (e) => setPanel(e.detail || null);
    window.addEventListener("docs:panel", on);
    return () => window.removeEventListener("docs:panel", on);
  }, []);

  useEffect(() => {
    fetch("/api/system")
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => d && d.version && setVer(d.version))
      .catch(() => {});
  }, []);

  if (!localStorage.getItem("admin_token")) {
    location.href = "/login";
    return null;
  }

  const cur = CRUMB[loc.pathname] || loc.pathname.replace("/", "");

  return (
    <div className={"shell" + (panel ? " with-panel" : "")}>
      <aside>
        <div className="brand">
          <span className="mark">C2</span>
          chatgpt2API
          <span className="ver">v{ver}</span>
        </div>

        <nav>
          {NAV.map((g) => (
            <div key={g.group}>
              <div className="navgroup">{g.group}</div>
              {g.items.map((it) => (
                <NavLink key={it.to} to={it.to} end={it.end} className={linkCls}>
                  <span className="ico">{it.ico}</span>
                  {it.label}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        <div className="sidefoot">
          <button
            className="btn ghost small"
            onClick={() => {
              localStorage.removeItem("admin_token");
              location.href = "/login";
            }}
          >
            ↪ Keluar
          </button>
        </div>
      </aside>

      <main>
        <div className="topbar">
          <div className="crumbs">
            <span>chatgpt2api</span>
            <span className="sep">/</span>
            <span className="cur">{cur}</span>
          </div>
          <div className="right">
            <span className="badge accent mono">OpenAI-compatible</span>
          </div>
        </div>

        <Outlet />

        <footer className="docfoot">
          <span>chatgpt2API v{ver}</span>
          <span className="sep">·</span>
          <a href="/v1/models" target="_blank" rel="noreferrer">/v1/models</a>
          <span className="sep">·</span>
          <a href="/api/health" target="_blank" rel="noreferrer">/api/health</a>
          <span className="sep">·</span>
          <span>jalan lokal, data kredensial disimpan terenkripsi</span>
        </footer>
      </main>

      {panel && (
        <div className="panel">
          <div className="panel-head">
            <span className="t">{panel.title || "Referensi"}</span>
            <span className="right" />
            <button className="btn small" onClick={() => setPanel(null)} title="Tutup panel">✕</button>
          </div>
          {panel.sections.map((s, i) => (
            <div className="panel-sec" key={i}>{s}</div>
          ))}
        </div>
      )}
    </div>
  );
}
