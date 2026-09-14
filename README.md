# ⚡ chatgpt2API

Gateway API kompatibel OpenAI/Anthropic/Gemini untuk **ChatGPT web** (chatgpt.com/backend-api/conversation) — pola arsitektur sama dengan [qwen2API](https://github.com/Cxslin/qwen2API) & [deepseek2API](https://github.com/Cxslin/deepseek2API).

## Fitur

- **Backend Go murni** — single binary, SQLite embedded (modernc.org/sqlite, tanpa CGO)
- **Multi-format API**: OpenAI `/v1/chat/completions` (stream SSE + non-stream)
- **Model alias**: `gpt-5`, `gpt-5-mini`, `gpt-4o`, `gpt-4o-mini`, `o4-mini` (semua route ke upstream web yang sama)
- **Multi-account pool** — round-robin + least in-flight, cooldown otomatis (429 → 5 menit), status per akun
- **Auto-refresh JWT** — accessToken kedaluwarsa otomatis ditukar pakai cookie session (`/api/auth/session`)
- **Kredensial terenkripsi** AES-256-GCM (PBKDF2 20k iterasi) di SQLite
- **Dashboard React** — Login, Dashboard, Accounts, API Keys, Playground (stream SSE live), Logs
- **API key system** — `sk-...` hash SHA-256, plaintext ditampilkan sekali
- **Solver sidecar opsional** — slot buat [cloudflare-solver](https://github.com/GunturBalantara/cloudflare-solver): renew `cf_clearance` + auto-login email/pw (eksperimental, mati by default)

## Yang TIDAK ada (jujur)

- ❌ Auto-login email/pw — **belum diimplementasi**. ChatGPT auth-nya Auth0 + Arkose/Turnstile, beda level dengan Qwen/DeepSeek. Slot solver sudah disiapkan tapi logika login-nya belum ada.
- ❌ Tool calling — upstream web gak expose function calling; pesan tool di-request OpenAI di-skip.
- ❌ Endpoint Anthropic `/v1/messages` & Gemini `/v1beta` — belum ditambahkan (struktur sudah siap, tinggal adapter).

## Instalasi

```bash
git clone https://github.com/HiuraKiyowoo/chatgpt2API.git
cd chatgpt2API

# backend (Go 1.21+, tanpa CGO)
cd backend && go build -o ../bin/chatgpt2api-backend ./cmd/chatgpt2api && cd ..

# frontend (Node 18+)
cd frontend && npm install && npm run build && cd ..

# config
cp config.example.yaml config.yaml
# edit: jwtSecret, credentialEncryptionKey, admin.password

# jalan
./start.sh          # foreground
./start.sh -d       # daemon
```

Server jalan di `http://0.0.0.0:8800`. Dashboard: buka di browser.

## Menambah akun

### Jalur OAuth (disarankan) — refresh_token, bebas browser setelah sekali authorize

Akun tipe `oauth_refresh` gak pernah butuh browser lagi: runtime tukar `refresh_token`
→ `access_token` via HTTP murni (`auth.openai.com/oauth/token`, terbukti reachable tanpa
Cloudflare challenge). Sekali authorize di browser lu sendiri, selamanya HTTP.

```bash
# 1. cetak URL authorize (PKCE disimpan di data/pkce_state.json)
python3 oauth_manual.py url

# 2. BUKA URL ITU DI BROWSER YANG SUDAH LOGIN ChatGPT (HP/laptop, jangan profil baru
#    di server — profil asing pemancing gate verifikasi nomor). Setelah consent,
#    browser di-redirect ke http://localhost:1455/auth/callback?code=***
#    URL gak akan kebuka — itu normal. SALIN nilai code dari address bar.

# 3. tukar code -> token -> simpan ke gateway (login admin buat header auth)
GATEWAY_PASS=*** GATEWAY_URL=http://127.0.0.1:8800   python3 oauth_manual.py tukar '<code ATAU URL callback penuh>'
```

Token disimpan terenkripsi AES-256-GCM. Saat `access_token` mau kedaluwarsa, gateway
refresh sendiri **dan menyimpan refresh_token hasil rotasi** (OpenAI me-rotasi; kalau
rotasi dibuang akun mati permanen — sudah ditutup sama test di `backend/api/oauth_test.go`).
Kalau gateway gak reachable, script nulis `data/oauth_tokens.json` (0600) yang bisa
di-POST manual ke `/api/accounts/import-tokens`.

### Jalur cookie (lama) — accessToken + session cookie

Dashboard → **Accounts** → Tambah:
1. **accessToken** (JWT `eyJhbGci...`) — dari chatgpt.com login → DevTools → Application → Cookies → salin `__Secure-next-auth.session-token`, ATAU localStorage key `@@/auth` → ambil `accessToken`.
2. **Cookies** (opsional tapi disarankan) — cookie string lengkap, buat auto-refresh JWT.
3. **cf_clearance + User-Agent** (opsional) — kalau IP kena Cloudflare challenge; UA harus sama dengan browser saat ambil cookie.

## Pakai API

```bash
curl http://127.0.0.1:8800/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5","stream":true,"messages":[{"role":"user","content":"halo"}]}'
```

## Struktur

```
backend/
  cmd/chatgpt2api/   # entrypoint + routing
  api/               # handler: v1_chat, admin, pool, keys
  app/               # wiring dependensi
  core/              # config, db, crypto, auth token, logger
  upstream/          # client SSE chatgpt.com + refresh session
frontend/
  src/pages/         # Login, Dashboard, Accounts, Keys, Playground, Logs
solver/              # (opsional) wrapper cloudflare-solver sidecar
```

## Konfigurasi

| Key | Default | Keterangan |
|---|---|---|
| `server.port` | 8800 | Port HTTP |
| `security.jwtSecret` | — | Wajib, secret sesi admin |
| `security.credentialEncryptionKey` | — | Wajib, kunci AES kredensial |
| `admin.username` / `admin.password` | admin | Login dashboard |
| `solver.enabled` | false | Aktifkan sidecar solver |

## Lisensi

MIT
