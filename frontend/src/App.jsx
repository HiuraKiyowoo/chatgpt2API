import React from "react";
import { Outlet, NavLink } from "react-router-dom";
import "./styles.css";

const linkCls = ({ isActive }) => "navlink" + (isActive ? " active" : "");

export default function App() {
  if (!localStorage.getItem("admin_token")) location.href = "/login";
  return (
    <div className="shell">
      <aside>
        <div className="brand">⚡ chatgpt2API</div>
        <nav>
          <NavLink to="/" end className={linkCls}>Dashboard</NavLink>
          <NavLink to="/accounts" className={linkCls}>Accounts</NavLink>
          <NavLink to="/keys" className={linkCls}>API Keys</NavLink>
          <NavLink to="/playground" className={linkCls}>Playground</NavLink>
          <NavLink to="/logs" className={linkCls}>Logs</NavLink>
        </nav>
        <button className="btn ghost" onClick={() => { localStorage.removeItem("admin_token"); location.href = "/login"; }}>Keluar</button>
      </aside>
      <main><Outlet /></main>
    </div>
  );
}
