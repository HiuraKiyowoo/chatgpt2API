package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// OAuthHarvest — POST /api/accounts/oauth-harvest
// Body: {cookies: "<array JSON Cookie-Editor atau cookie-string>", label?: ""}
//
// Alur (jalur A):
//   1. bikin PKCE + auth URL
//   2. sidecar: load cookie Google -> buka authorize -> tangkap code
//   3. tukar code -> refresh_token (HTTP murni, ~1 detik)
//   4. simpan refresh_token ke DB; access_token di-refresh dari situ selamanya
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
// Body: {accessToken, refreshToken, idToken?, label?}
// Jalur manual: authorize dilakukan manusia di browser, code ditukar di luar,
// hasilnya di-import ke sini. Tanpa browser, tanpa solver.
func (h *Handler) OAuthImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		IDToken      string `json:"idToken"`
		ExpiresIn    int    `json:"expiresIn"`
		Label        string `json:"label"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalid")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		jsonErr(w, 400, "refreshToken wajib (tanpa ini akun gak bisa di-refresh otomatis)")
		return
	}

	id := "acc_" + core.RandomHex(8)
	key := h.App.Config.Security.CredentialEncryptionKey
	tokEnc, _ := core.EncryptCredential(key, req.AccessToken)
	credEnc, _ := core.EncryptCredential(key, req.RefreshToken)
	label := req.Label
	if label == "" {
		label = "manual-oauth"
	}
	now := core.Now()
	if _, err := h.App.DB.Exec(`INSERT INTO accounts (id, label, credential_type, credential_enc, access_token_enc,
		cookies_enc, cf_clearance, user_agent, status, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, label, "oauth_refresh", credEnc, tokEnc, "", "", "", "valid", now, now); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	h.App.Logs.Add("info", "oauth", "import refresh_token manual", label)

	// verifikasi langsung: refresh_token ini benar-benar bisa ditukar?
	out := map[string]interface{}{"ok": true, "id": id, "label": label, "verify": "skip"}
	if req.AccessToken == "" {
		if toks, err := upstream.RefreshTokens(req.RefreshToken); err != nil {
			h.App.DB.Exec(`UPDATE accounts SET status='invalid', last_error=?, updated_at=? WHERE id=?`,
				core.Truncate(err.Error(), 300), core.Now(), id)
			w.WriteHeader(502)
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "id": id, "verify": err.Error()})
			return
		} else {
			te, _ := core.EncryptCredential(key, toks.AccessToken)
			h.App.DB.Exec(`UPDATE accounts SET access_token_enc=? WHERE id=?`, te, id)
			out["verify"] = "ok"
			out["expiresIn"] = toks.ExpiresIn
		}
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
	tokEnc, _ := core.EncryptCredential(key, toks.AccessToken)
	h.App.DB.Exec(`UPDATE accounts SET access_token_enc=?, status='valid', last_error='', updated_at=? WHERE id=?`,
		tokEnc, core.Now(), id)
	h.App.Logs.Add("info", "oauth", "refresh_token -> access_token baru (HTTP, tanpa browser)", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true, "expiresIn": toks.ExpiresIn, "accessTokenLen": len(toks.AccessToken),
	})
}
