// Helper panel dokumen kanan.
// Tiap halaman panggil useDocPanel(...) — isinya dirender di kolom ke-3.
import { useEffect } from "react";

export function useDocPanel(title, sections) {
  useEffect(() => {
    const detail = { title, sections };
    window.dispatchEvent(new CustomEvent("docs:panel", { detail }));
    return () => window.dispatchEvent(new CustomEvent("docs:panel", { detail: null }));
    // sections dibuat ulang tiap render; sengaja hanya bergantung pada title
    // agar tidak infinite-loop (isi panel statis per halaman).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [title]);
}

// Blok kode dengan header label + tombol salin.
export function Code({ label, children, scroll }) {
  return (
    <div>
      {label && (
        <div className="code-head">
          <span>{label}</span>
        </div>
      )}
      <pre className={"code" + (scroll ? " code-scroll" : "")}>{children}</pre>
    </div>
  );
}

export function Endpoint({ method, path }) {
  return (
    <div className="endpoint">
      <span className={"m " + method}>{method}</span>
      {path}
    </div>
  );
}
