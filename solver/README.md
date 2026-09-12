# chatgpt2API — Solver Sidecar

Sidecar opsional (`solver/server.cjs`) buat dua hal yang gak bisa lewat HTTP murni:

1. **`/solve`** — lewatin Cloudflare managed challenge pakai headless Chromium asli, ambil `cf_clearance` + cookies, plus **JSON passthrough**: fetch endpoint JSON mana pun di konteks browser (mis. `chatgpt.com/api/auth/csrf` yang cuma mau dibaca pas challenge).
2. **`/login`** — auto-login **email + password** ke ChatGPT via browser:
   - isi form email (chatgpt.com/auth/login) → submit
   - isi form password (auth.openai.com) → submit
   - deteksi OTP email (akun belum pernah verifikasi), Arkose, Turnstile
   - kalau sukses: ambil `__Secure-next-auth.session-token` + `accessToken` dari `/api/auth/session`

## Hasil test nyata (diverifikasi live)

| Test | Hasil |
|---|---|
| Cloudflare challenge chatgpt.com | ✅ Lewat (±10 dtk, DOM asli + `oai-sc` dsb.) |
| `POST /solve` → csrf JSON passthrough | ✅ `{"csrfToken":"3da5...9850"}` |
| Halaman login | ✅ **Tanpa Arkose & tanpa Turnstile** |
| Submit email akun baru | ✅ Masuk `auth.openai.com/email-verification` (OTP) |
| Login akun existing (password) | ⏳ Butuh kredensial asli — alur sudah siap |
| OTP email otomatis | ❌ Belum (butuh akses inbox) |

## Jalankan

```bash
CHROME_PATH=/root/.cache/ms-playwright/chromium-1234/chrome-linux/chrome \
  node solver/server.cjs 7900
```

## Contoh

```bash
# lewatin challenge + ambil csrf
curl -X POST http://127.0.0.1:7900/solve \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://chatgpt.com/api/auth/csrf","timeoutSec":80}'

# auto-login email+password
curl -X POST http://127.0.0.1:7900/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"lu@contoh.com","password":"rahasia","timeoutSec":150}'
```

## Integrasi backend Go

`config.yaml`:

```yaml
solver:
  enabled: true
  url: http://127.0.0.1:7900
```

Backend panggil `/login` buat menambah akun tanpa paste token, dan `/solve`
buat mengisi ulang `cf_clearance` saat akun kena 403 challenge. Slot sudah
disiapkan; wiring otomatis di backend menyusul (manual dulu via dashboard).

## Batasan (jujur)

- OTP email **belum** dijawab otomatis — kalau ChatGPT minta kode verifikasi
  (akun baru / login perangkat baru), proses berhenti di `stage: otp_email`.
- `cf_clearance` kadang gak muncul walau challenge lewat (Cloudflare tidak
  selalu menetapkan cookie di headless). Yang penting konteks browser-nya
  lolos — JSON passthrough tetap jalan.
- RAM: 1 Chromium headless ± 200-400 MB. Sidecar hanya hidup saat dipanggil.
