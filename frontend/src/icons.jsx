// Ikon garis (stroke) — dipakai ganti emoji bawaan keyboard.
// Semua 16x16, currentColor, tanpa fill supaya ikut warna teks.
import React from "react";

const base = {
  width: 14,
  height: 14,
  viewBox: "0 0 16 16",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.4,
  strokeLinecap: "square",
  style: { flexShrink: 0, verticalAlign: "-2px" },
};

export function IconCheck(p) {
  return (
    <svg {...base} {...p}><path d="M3 8.5l3.2 3.2L13 4.8" /></svg>
  );
}

export function IconCross(p) {
  return (
    <svg {...base} {...p}><path d="M4 4l8 8M12 4l-8 8" /></svg>
  );
}

export function IconWarn(p) {
  return (
    <svg {...base} {...p}>
      <path d="M8 2.6l5.6 10.4H2.4L8 2.6z" />
      <path d="M8 6.6v3M8 11.2v.4" />
    </svg>
  );
}

export function IconTrash(p) {
  return (
    <svg {...base} {...p}>
      <path d="M3 4.6h10M6.4 4.6V3h3.2v1.6M5 4.6l.5 8.4h5l.5-8.4" />
    </svg>
  );
}

export function IconCopy(p) {
  return (
    <svg {...base} {...p}>
      <path d="M5.6 5.6V3.2h7.2v7.2h-2.4" />
      <path d="M3.2 5.6h7.2v7.2H3.2z" />
    </svg>
  );
}

export function IconMail(p) {
  return (
    <svg {...base} {...p}>
      <path d="M2.4 4h11.2v8H2.4z" />
      <path d="M2.4 4.4L8 8.6l5.6-4.2" />
    </svg>
  );
}

export function IconTool(p) {
  return (
    <svg {...base} {...p}>
      <path d="M10.4 2.6a3.4 3.4 0 00-4.2 4.2L2.6 10.4l3 3 3.6-3.6a3.4 3.4 0 004.2-4.2L11.2 8 8 4.8z" />
    </svg>
  );
}

export function IconGear(p) {
  return (
    <svg {...base} {...p}>
      <circle cx="8" cy="8" r="2.1" />
      <path d="M8 1.8v1.6M8 12.6v1.6M1.8 8h1.6M12.6 8h1.6M3.6 3.6l1.1 1.1M11.3 11.3l1.1 1.1M12.4 3.6l-1.1 1.1M4.7 11.3l-1.1 1.1" />
    </svg>
  );
}

export function IconExit(p) {
  return (
    <svg {...base} {...p}>
      <path d="M6.4 2.6H2.6v10.8h3.8" />
      <path d="M6.8 8h6.6M11 5.4L13.6 8 11 10.6" />
    </svg>
  );
}

export function IconDash(p) {
  return (
    <svg {...base} {...p}>
      <path d="M2.4 8h11.2" />
    </svg>
  );
}

export function IconBox(p) {
  return (
    <svg {...base} {...p}>
      <path d="M2.6 4.4h10.8v8H2.6z" />
      <path d="M2.6 7h10.8" />
    </svg>
  );
}

export function IconKey(p) {
  return (
    <svg {...base} {...p}>
      <circle cx="5.4" cy="5.4" r="2.6" />
      <path d="M7.3 7.3l6 6M11 10.3l1.4 1.4M9.4 11.9l1.4 1.4" />
    </svg>
  );
}

export function IconPlay(p) {
  return (
    <svg {...base} {...p}><path d="M5 3l7.4 5L5 13V3z" /></svg>
  );
}

export function IconList(p) {
  return (
    <svg {...base} {...p}>
      <path d="M5.6 4.2h8M5.6 8h8M5.6 11.8h8M2.6 4.2h.01M2.6 8h.01M2.6 11.8h.01" />
    </svg>
  );
}
