// Helper panel dokumen kanan.
// Tiap halaman panggil useDocPanel(...) — isinya dirender di kolom ke-3.
import { useEffect } from "react";

export function Endpoint({ method, path }) {
  return (
    <div className="endpoint">
      <span className={"m " + method}>{method}</span>
      {path}
    </div>
  );
}

export function Code({ label, children }) {  return (
    <div className="code">
      {label && (
        <div className="code-head">
          <span>{label}</span>
        </div>
      )}
      <pre>{children}</pre>
    </div>
  );
}

let emit = null;

export function setPanelEmitter(fn) {
  emit = fn;
}

export function useDocPanel(title, sections) {
  useEffect(() => {
    if (!emit) return;
    emit({ title, sections });
    return () => emit(null);
  }, [title, JSON.stringify(sections?.map((s) => s?.key))]);
}
