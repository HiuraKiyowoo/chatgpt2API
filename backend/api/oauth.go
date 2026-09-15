package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// saveOAuthTokens nyimpen hasil tukar token ke satu akun:
//   - access_token_enc: selalu diperbarui
//   - credential_enc (refresh_token): HANYA kalau upstream ngasih nilai baru.
//
// Why: OpenAI me-rotasi refresh_token. Kalau rotasi dibuang, refresh_token di DB
// jadi basi dan akun mati permanen begitu yang lama hangus — persis kegagalan yang
// udah kejadian di gateway lain. Satu UPDATE = tulis atomik; kegagalan di-log,
// jangan di swallow.
func (h *Handler) saveOAuthTokens(accID string, toks *upstream.OAuthTokens) error {
	if toks == nil || toks.AccessToken == "" {
		return fmt.Errorf("saveOAuthTokens: access_token kosong")
	}
	key := h.App.Config.Security.CredentialEncryptionKey
	tokEnc, err := core.EncryptCredential(key, toks.AccessToken)
	if err != nil {
		return fmt.Errorf("enkripsi access_token: %w", err)
	}
	if toks.RefreshToken != "" {
		credEnc, err := core.EncryptCredential(key, toks.RefreshToken)
		if err != nil {
			return fmt.Errorf("enkripsi refresh_token: %w", err)
		}
		if _, err := h.App.DB.Exec(
			`UPDATE accounts SET access_token_enc=?, credential_enc=?, status='valid', last_error='', updated_at=? WHERE id=?`,
			tokEnc, credEnc, core.Now(), accID); err != nil {
			return fmt.Errorf("simpan token berotasi: %w", err)
		}
		h.App.Logs.Add("info", "oauth", "refresh_token hasil rotasi ikut disimpan", accID)
		return nil
	}
	if _, err := h.App.DB.Exec(
		`UPDATE accounts SET access_token_enc=?, status='valid', last_error='', updated_at=? WHERE id=?`,
		tokEnc, core.Now(), accID); err != nil {
		return fmt.Errorf("simpan access_token: %w", err)
	}
	return nil
}

// OAuthHarvest — POST /api/accounts/oauth-harvest
// Body: {cookies: "<array JSON Cookie-Editor atau cookie-string>", label?: ""}
//
// Alur (jalur A):
//  1. bikin PKCE + auth URL
//  2. sidecar: load cookie Google -> buka authorize -> tangkap code
//  3. tukar code -> refresh_token (HTTP murni, ~1 detik)
//  4. simpan refresh_token ke DB; access_token di-refresh dari situ selamanya
func (h *Handler) OAuthHarvest(w http.ResponseWriter, r *http.Request) {
	if !h.App.Config.Solver.Enabled {
		jsonErr(w, 400, "solver mati — set solver.enabled=true di config.yaml")
		return
	}
	var req struct {
		Cookies string `json:"cookies"`
		Label   string `json:"label"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalid")
		return
	}
	if strings.TrimSpace(req.Cookies) == "" {
		jsonErr(w, 400, "cookies kosong — paste array JSON dari Cookie-Editor (domain .google.com)")
		return
	}

	// 1. PKCE + URL
	verifier := upstream.PKCEVerifier()
	challenge := upstream.PKCEChallenge(verifier)
	state := upstream.RandomState()
	authURL := upstream.BuildAuthURL(challenge, state)

	// 2. panen code via sidecar
	res, err := h.App.Solver.HarvestOAuthCode(req.Cookies, authURL, 120)
	if err != nil {
		h.App.Logs.Add("error", "oauth", "harvest error: "+err.Error(), "")
		jsonErr(w, 502, err.Error())
		return
	}
	if !res.OK || res.Code == "" {
		h.App.Logs.Add("warn", "oauth", "harvest gagal: "+res.Error, "")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "stage": "harvest_fail", "detail": res.Error, "finalUrl": res.FinalURL,
		})
		return
	}

	// 3. tukar code -> token (HTTP, tanpa browser)
	toks, err := upstream.ExchangeCode(res.Code, res.Verifier)
	if err != nil {
		h.App.Logs.Add("error", "oauth", "exchange code gagal: "+err.Error(), "")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "stage": "exchange_fail", "detail": err.Error()})
		return
	}
	if toks.RefreshToken == "" {
		h.App.Logs.Add("warn", "oauth", "token OK tapi tanpa refresh_token", "")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(502)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": false, "stage": "no_refresh_token",
			"detail": "token didapat tapi tanpa refresh_token — authorize ulang dengan scope offline_access",
		})
		return
	}

	// 4. simpan sebagai akun
	id := "acc_" + core.RandomHex(8)
	key := h.App.Config.Security.CredentialEncryptionKey
	tokEnc, _ := core.EncryptCredential(key, toks.AccessToken)
	credEnc, _ := core.EncryptCredential(key, toks.RefreshToken)
	label := req.Label
	if label == "" {
		label = "oauth-google"
	}
	now := core.Now()
	if _, err := h.App.DB.Exec(`INSERT INTO accounts (id, label, credential_type, credential_enc, access_token_enc,
		cookies_enc, cf_clearance, user_agent, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, label, "oauth_refresh", credEnc, tokEnc, "", "", "", "valid", now, now); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	h.App.Logs.Add("info", "oauth", "refresh_token tersimpan (akses permanen)", label)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "id": id, "label": label,
		"hasRefreshToken": true, "hasAccessToken": toks.AccessToken != "",
		"expiresIn": toks.ExpiresIn, "harvestSec": res.TimeSec,
		"note": "selesai — browser gak dipake lagi setelah ini; runtime murni HTTP refresh_token",
	})
}

// OAuthImport — POST /api/accounts/import-tokens
// Jalur manual: authorize dilakukan manusia di browser, code ditukar di luar
// (oauth_manual.py), hasilnya di-import ke sini. Tanpa browser, tanpa solver.
//
// Menerima DUA bentuk body (yang lama bikin script manual selamanya gagal 400):
//
//	flat   : {accessToken?, refreshToken?, idToken?, expiresIn?, label?}
//	oauth  : {access_token?, refresh_token?, id_token?, expires_in?} + {tokens:{...}}
//
// Alias camelCase dan snake_case diterima; label top-level atau label/nama.
func (h *Handler) OAuthImport(w http.ResponseWriter, r *http.Request) {
	var raw map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&raw); err != nil {
		jsonErr(w, 400, "JSON invalid")
		return
	}
	// unwrap {tokens: {...}} kalau ada
	src := raw
	if sub, ok := raw["tokens"].(map[string]interface{}); ok {
		src = sub
	}
	pick := func(keys ...string) string {
		for _, m := range []map[string]interface{}{src, raw} {
			for _, k := range keys {
				if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
		}
		return ""
	}
	num := func(keys ...string) int {
		for _, m := range []map[string]interface{}{src, raw} {
			for _, k := range keys {
				if v, ok := m[k].(float64); ok {
					return int(v)
				}
			}
		}
		return 0
	}
	req := struct {
		AccessToken, RefreshToken, IDToken, Label string
		ExpiresIn                                 int
	}{
		AccessToken:  pick("accessToken", "access_token"),
		RefreshToken: pick("refreshToken", "refresh_token"),
		IDToken:      pick("idToken", "id_token"),
		Label:        pick("label", "nama", "name"),
	}
	req.ExpiresIn = num("expiresIn", "expires_in")
	if req.RefreshToken == "" {
		jsonErr(w, 400, "refresh_token wajib ada (tanpa ini akun gak bisa di-refresh otomatis)"+
			" — kirim {access_token, refresh_token} hasil oauth_manual.py tukar")
		return
	}

	id := "acc_" + core.RandomHex(8)
	key := h.App.Config.Security.CredentialEncryptionKey
	tokEnc, errTok := core.EncryptCredential(key, req.AccessToken)
	credEnc, errCred := core.EncryptCredential(key, req.RefreshToken)
	if errTok != nil || errCred != nil {
		jsonErr(w, 500, "enkripsi kredensial gagal")
		return
	}
	label := req.Label
	if label == "" {
		label = "manual-oauth"
	}
	now := core.Now()
	if _, err := h.App.DB.Exec(`INSERT INTO accounts (id, label, credential_type, credential_enc, access_token_enc,
		cookies_enc, cf_clearance, user_agent, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, label, "oauth_refresh", credEnc, tokEnc, "", "", "", "pending", now, now); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	h.App.Logs.Add("info", "oauth", "import refresh_token manual", label)

	// verifikasi langsung: refresh_token ini beneran bisa ditukar?
	// Access token hasil verifikasi disimpan lewat saveOAuthTokens supaya
	// refresh_token ROTASI ikut kesimpen (bukan cuma access-nya).
	out := map[string]interface{}{"ok": true, "id": id, "label": label, "verify": "skip"}
	if req.AccessToken != "" {
		if _, err := h.App.DB.Exec(`UPDATE accounts SET status='valid', updated_at=? WHERE id=?`, core.Now(), id); err != nil {
			jsonErr(w, 500, err.Error())
			return
		}
	} else {
		toks, err := upstream.RefreshTokens(req.RefreshToken)
		if err != nil {
			h.App.DB.Exec(`UPDATE accounts SET status='invalid', last_error=?, updated_at=? WHERE id=?`,
				core.Truncate(err.Error(), 300), core.Now(), id)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(502)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "id": id, "verify": err.Error()})
			return
		}
		if err := h.saveOAuthTokens(id, toks); err != nil {
			h.App.DB.Exec(`UPDATE accounts SET status='invalid', last_error=?, updated_at=? WHERE id=?`,
				core.Truncate(err.Error(), 300), core.Now(), id)
			jsonErr(w, 500, "token valid tapi gagal disimpan: "+err.Error())
			return
		}
		out["verify"] = "ok"
		out["expiresIn"] = toks.ExpiresIn
		out["rotatedRefreshToken"] = toks.RefreshToken != "" && toks.RefreshToken != req.RefreshToken
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// OAuthRefresh — POST /api/accounts/{id}/oauth-refresh
// Tes jalur runtime: tukar refresh_token jadi access_token baru (milidetik, tanpa browser).
func (h *Handler) OAuthRefresh(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/accounts/"), "/")
	id = strings.TrimSuffix(id, "/oauth-refresh")
	if id == "" {
		jsonErr(w, 400, "id kosong")
		return
	}
	key := h.App.Config.Security.CredentialEncryptionKey
	var credEnc string
	if err := h.App.DB.QueryRow(`SELECT credential_enc FROM accounts WHERE id = ?`, id).Scan(&credEnc); err != nil {
		jsonErr(w, 404, "akun tidak ditemukan")
		return
	}
	refreshTok, err := core.DecryptCredential(key, credEnc)
	if err != nil || refreshTok == "" {
		jsonErr(w, 400, "akun ini gak punya refresh_token (bukan tipe oauth_refresh)")
		return
	}
	toks, err := upstream.RefreshTokens(refreshTok)
	if err != nil {
		h.App.DB.Exec(`UPDATE accounts SET status='invalid', last_error=?, updated_at=? WHERE id=?`,
			core.Truncate(err.Error(), 300), core.Now(), id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "detail": err.Error()})
		return
	}
	if err := h.saveOAuthTokens(id, toks); err != nil {
		h.App.Logs.Add("error", "oauth", "refresh OK tapi simpan gagal: "+err.Error(), id)
		jsonErr(w, 500, "token baru didapat tapi gagal disimpan: "+err.Error())
		return
	}
	h.App.Logs.Add("info", "oauth", "refresh_token -> access_token baru (HTTP, tanpa browser)", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "expiresIn": toks.ExpiresIn, "accessTokenLen": len(toks.AccessToken),
	})
}
