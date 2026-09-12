#!/bin/bash
# chatgpt2API — start/stop/status
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

BIN="./bin/chatgpt2api-backend"
PIDFILE="./data/chatgpt2api.pid"
CONF="./config.yaml"

if [ ! -f "$CONF" ]; then
  echo "[!] config.yaml tidak ada — salin dulu: cp config.example.yaml config.yaml"
  exit 1
fi

if [ ! -x "$BIN" ]; then
  echo "[!] binary belum ada — build dulu: cd backend && go build -o ../bin/chatgpt2api-backend ./cmd/chatgpt2api"
  exit 1
fi

is_running() {
  [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null
}

if [ "$1" = "--daemon" ] || [ "$1" = "-d" ]; then
  if is_running; then
    echo "[-] chatgpt2API sudah jalan (PID $(cat "$PIDFILE"))"
    exit 1
  fi
  mkdir -p logs data
  nohup "$BIN" --config "$CONF" > logs/server.log 2>&1 &
  echo $! > "$PIDFILE"
  sleep 1
  if is_running; then
    echo "[+] chatgpt2API daemon jalan (PID $(cat "$PIDFILE")) — log: logs/server.log"
  else
    echo "[!] gagal start — cek logs/server.log"
    exit 1
  fi
  exit 0
fi

if [ "$1" = "stop" ]; then
  if is_running; then
    kill "$(cat "$PIDFILE")" && rm -f "$PIDFILE"
    echo "[+] chatgpt2API dihentikan"
  else
    echo "[-] tidak sedang jalan"
    rm -f "$PIDFILE"
  fi
  exit 0
fi

if [ "$1" = "status" ]; then
  if is_running; then
    echo "[+] jalan (PID $(cat "$PIDFILE"))"
    curl -sS -m 3 "http://127.0.0.1:$(grep -A1 'port:' "$CONF" | head -1 | awk '{print $2}')/api/health" || true
    echo
  else
    echo "[-] tidak jalan"
  fi
  exit 0
fi

if [ "$1" = "update" ]; then
  echo "[*] git pull..."
  git pull --ff-only || exit 1
  echo "[*] build backend..."
  (cd backend && go build -o ../bin/chatgpt2api-backend ./cmd/chatgpt2api) || exit 1
  echo "[*] build frontend..."
  (cd frontend && npm install --no-audit --no-fund && npm run build) || exit 1
  echo "[*] restart daemon..."
  "$0" stop; "$0" -d
  exit 0
fi

# foreground
exec "$BIN" --config "$CONF"
