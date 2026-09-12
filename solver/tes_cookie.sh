#!/bin/bash
# tes_cookie.sh — jalanin tes_cookie.cjs pakai cookie dari file (biar gak kepotong shell)
cd "$(dirname "$0")"
CHROME_PATH="${CHROME_PATH:-/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome}" \
  node tes_cookie.cjs "$(cat /tmp/cookie_raw.txt)" "${1:-90}"
