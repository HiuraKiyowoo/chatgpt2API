#!/bin/bash
# start-solver.sh — nyalain sidecar solver (browser headless)
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$DIR"

CHROME_PATH="${CHROME_PATH:-/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome}"
PORT="${SOLVER_PORT:-7900}"
PIDFILE="./data/solver.pid"

if [ ! -x "$CHROME_PATH" ]; then
  echo "[!] Chromium gak ketemu di $CHROME_PATH"
  echo "    set CHROME_PATH=/path/ke/chrome terus ulangi"
  echo "    daftar kandidat:"
  ls -d /root/.cache/ms-playwright/*/chrome-linux/chrome 2>/dev/null | sed 's/^/      /'
  exit 1
fi

if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "[-] solver sudah jalan (PID $(cat "$PIDFILE"))"
  exit 1
fi

mkdir -p logs data
nohup env CHROME_PATH="$CHROME_PATH" node solver/server.cjs "$PORT" > logs/solver.log 2>&1 &
echo $! > "$PIDFILE"
sleep 2
if kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "[+] solver jalan di 127.0.0.1:$PORT (PID $(cat "$PIDFILE")) — log: logs/solver.log"
  curl -sS -m 3 "http://127.0.0.1:$PORT/health"; echo
else
  echo "[!] gagal start — cek logs/solver.log"; tail -5 logs/solver.log
  exit 1
fi
