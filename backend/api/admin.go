package api

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// ---- admin auth (JWT-lite: HMAC token di cookie/localStorage) ----

func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalid")
		return
	}
	user := h.App.Config.Admin.Username
	pass := h.App.Config.Admin.Password
	if user == "" || pass == "" {
		jsonErr(w, 503, "admin belum di-setup — isi security.admin di config.yaml")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.Username), []byte(user)) != 1 ||
		subtle.ConstantTimeCompare([]byte(req.Password), []byte(pass)) != 1 {
		jsonErr(w, 401, "username atau password salah")
		return
	}
	tok := core.AdminToken(h.App.Config.Security.JWTSecret, req.Username)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tok})
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// AuthAdmin middleware: validasi "Authorization: Bearer <token>".
func (h *Handler) AuthAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			jsonErr(w, 401, "tidak ada token")
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		if !core.VerifyAdminToken(h.App.Config.Security.JWTSecret, tok) {
			jsonErr(w, 401, "token invalid/kedaluwarsa")
			return
		}
		next(w, r)
	}
}

// AuthAPIKey middleware buat endpoint /v1/*: header "Authorization: Bearer sk-...".
func (h *Handler) AuthAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer sk-") {
			jsonErr(w, 401, "API key missing/invalid format (harus sk-...)")
			return
		}
		key := strings.TrimPrefix(auth, "Bearer ")
		hash := core.HashAPIKey(key)
		var id string
		err := h.App.DB.QueryRow(`SELECT id FROM api_keys WHERE key_hash = ? AND enabled = 1`, hash).Scan(&id)
		if err != nil {
			jsonErr(w, 401, "API key tidak dikenal")
			return
		}
		h.App.DB.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, core.Now(), id)
		next(w, r)
	}
}

// ---- accounts CRUD ----

func (h *Handler) AccountsList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.App.DB.Query(`SELECT id, label, credential_type, status, proxy_url, last_error,
		cooldown_until, request_count, success_count, error_count, last_used_at, created_at FROM accounts ORDER BY created_at ASC`)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	list := make([]Account, 0)
	for rows.Next() {
		var a Account
		var credType string
		if err := rows.Scan(&a.ID, &a.Label, &credType, &a.Status, &a.ProxyURL, &a.LastError,
			&a.CooldownUntil, &a.RequestCount, &a.SuccessCount, &a.ErrorCount, &a.LastUsedAt, &a.CreatedAt); err != nil {
			continue
		}
		a.CredType = credType
		list = append(list, a)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"accounts": list})
}

func (h *Handler) AccountsAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label        string `json:"label"`
		AccessToken  string `json:"accessToken"`
		Cookies      string `json:"cookies"`
		CFClearance  string `json:"cfClearance"`
		UserAgent    string `json:"userAgent"`
		ProxyURL     string `json:"proxyUrl"`
		CheckNow     bool   `json:"checkNow"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		jsonErr(w, 400, "JSON invalid")
		return
	}
	if strings.TrimSpace(req.AccessToken) == "" && strings.TrimSpace(req.Cookies) == "" {
		jsonErr(w, 400, "accessToken atau cookies wajib diisi minimal satu")
		return
	}
	key := h.App.Config.Security.CredentialEncryptionKey
	if key == "" {
		jsonErr(w, 503, "security.credentialEncryptionKey kosong di config.yaml")
		return
	}
	tokEnc, err := core.EncryptCredential(key, req.AccessToken)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	ckEnc, _ := core.EncryptCredential(key, req.Cookies)
	id := "acc_" + core.RandomHex(8)
	now := core.Now()
	ctype := "access_token"
	if req.AccessToken == "" {
		ctype = "session_token"
	}
	preview := core.MaskCredential(req.AccessToken)
	_ = preview
	if _, err := h.App.DB.Exec(`INSERT INTO accounts (id, label, credential_type, credential_enc, access_token_enc,
		cookies_enc, cf_clearance, user_agent, status, proxy_url, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, req.Label, ctype, tokEnc, tokEnc, ckEnc, req.CFClearance, req.UserAgent, "unknown", req.ProxyURL, now, now); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	h.App.Logs.Add("info", "admin", "akun ditambahkan: "+req.Label, id)

	status := "unknown"
	detail := ""
	if req.CheckNow {
		ok, msg, _ := h.App.Upstream.HealthCheck(upstream.Credential{
			AccessToken: req.AccessToken, Cookies: req.Cookies, CFClearance: req.CFClearance, UserAgent: req.UserAgent,
		})
		if ok {
			status = "valid"
		} else {
			status = "invalid"
			detail = msg
			h.App.DB.Exec(`UPDATE accounts SET status='invalid', last_error=? WHERE id=?`, core.Truncate(msg, 300), id)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "status": status, "detail": detail})
}

func (h *Handler) AccountsDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	if id == "" {
		jsonErr(w, 400, "id kosong")
		return
	}
	res, err := h.App.DB.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonErr(w, 404, "akun tidak ditemukan")
		return
	}
	h.App.Logs.Add("info", "admin", "akun dihapus", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// HealthCheck akun ulang (POST /api/accounts/{id}/check).
func (h *Handler) AccountCheck(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/accounts/"), "/")
	id = strings.TrimSuffix(id, "/check")
	accID, cred, err := h.poolCred(id)
	if err != nil {
		jsonErr(w, 404, err.Error())
		return
	}
	ok, msg, _ := h.App.Upstream.HealthCheck(cred)
	status := "invalid"
	if ok {
		status = "valid"
	}
	h.App.DB.Exec(`UPDATE accounts SET status=?, last_error=?, updated_at=? WHERE id=?`,
		status, core.Truncate(msg, 300), core.Now(), accID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": status, "detail": msg})
}

func (h *Handler) poolCred(id string) (string, upstream.Credential, error) {
	var tokEnc, ckEnc, cf, ua string
	err := h.App.DB.QueryRow(`SELECT access_token_enc, cookies_enc, cf_clearance, user_agent FROM accounts WHERE id = ?`, id).
		Scan(&tokEnc, &ckEnc, &cf, &ua)
	if err != nil {
		return "", upstream.Credential{}, err
	}
	key := h.App.Config.Security.CredentialEncryptionKey
	tok, err := core.DecryptCredential(key, tokEnc)
	if err != nil {
		return "", upstream.Credential{}, err
	}
	ck, _ := core.DecryptCredential(key, ckEnc)
	return id, upstream.Credential{AccessToken: tok, Cookies: ck, CFClearance: cf, UserAgent: ua}, nil
}
