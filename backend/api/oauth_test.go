package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chatgpt2api-go/app"
	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// ---- helpers ----

func newTestHandler(t *testing.T) (*Handler, *core.Config) {
	t.Helper()
	cfg := &core.Config{}
	cfg.Server.Port = 0
	cfg.Server.FrontendPath = "./dist"
	cfg.Database.Path = filepath.Join(t.TempDir(), "t.db")
	cfg.Security.JWTSecret = "test-jwt-secret"
	cfg.Security.CredentialEncryptionKey = "test-enc-key"
	cfg.Admin.Username = "admin"
	cfg.Admin.Password = "test-password"
	cfg.Solver.Enabled = false
	cfg.Solver.URL = "http://127.0.0.1:1"
	db, err := core.OpenDB(cfg.Database.Path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewHandler(app.New(cfg, db)), cfg
}

// jwtWithExp bikin JWT (alg none) dengan exp tertentu — cukup buat jalur
// TokenExpiredAtauHampir yang cuma baca klaim exp.
func jwtWithExp(exp int64) string {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d,"sub":"test"}`, exp)))
	return head + "." + body + ".sig"
}

func adminJWT(t *testing.T, h *Handler, cfg *core.Config) string {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/admin/login",
		bytes.NewBufferString(`{"username":"admin","password":"test-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.AdminLogin(rr, req)
	if rr.Code != 200 {
		t.Fatalf("admin login: want 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("login JSON: %v", err)
	}
	if out["token"] == "" {
		t.Fatal("login balikan token kosong")
	}
	return out["token"]
}

func seedOAuthAccount(t *testing.T, h *Handler, cfg *core.Config, accessToken, refreshToken string) string {
	t.Helper()
	key := cfg.Security.CredentialEncryptionKey
	tokEnc, err := core.EncryptCredential(key, accessToken)
	if err != nil {
		t.Fatal(err)
	}
	credEnc, err := core.EncryptCredential(key, refreshToken)
	if err != nil {
		t.Fatal(err)
	}
	id := "acc_test_" + core.RandomHex(4)
	if _, err := h.App.DB.Exec(`INSERT INTO accounts (id,label,credential_type,credential_enc,access_token_enc,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		id, "test", "oauth_refresh", credEnc, tokEnc, "valid", core.Now(), core.Now()); err != nil {
		t.Fatal(err)
	}
	return id
}

func storedRefreshToken(t *testing.T, h *Handler, cfg *core.Config, id string) string {
	t.Helper()
	var credEnc string
	if err := h.App.DB.QueryRow(`SELECT credential_enc FROM accounts WHERE id=?`, id).Scan(&credEnc); err != nil {
		t.Fatal(err)
	}
	v, err := core.DecryptCredential(cfg.Security.CredentialEncryptionKey, credEnc)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func seedAPIKey(t *testing.T, h *Handler) string {
	t.Helper()
	plain := "sk-test-" + core.RandomHex(8)
	if _, err := h.App.DB.Exec(`INSERT INTO api_keys (id,name,key_hash,enabled,created_at) VALUES (?,?,?,?,?)`,
		"key_test", "test", core.HashAPIKey(plain), 1, core.Now()); err != nil {
		t.Fatal(err)
	}
	return plain
}

// mockOAuth meniru POST /oauth/token: kasih access baru + (opsional) refresh rotasi.
func mockOAuth(t *testing.T, rotated string, reject bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		if reject {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"token_expired","code":"token_expired"}}`))
			return
		}
		resp := map[string]interface{}{
			"access_token": jwtWithExp(time.Now().Add(time.Hour).Unix()),
			"expires_in":   3600,
			"token_type":   "bearer",
		}
		if rotated != "" {
			resp["refresh_token"] = rotated
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// mockConv meniru POST /backend-api/conversation dengan SSE satu jawaban.
func mockConv(t *testing.T, hit *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/conversation" {
			http.NotFound(w, r)
			return
		}
		*hit++
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") || len(auth) < 20 {
			w.WriteHeader(401)
			return
		}
		msg := map[string]interface{}{
			"message": map[string]interface{}{
				"id":              "m1",
				"conversation_id": "c1",
				"model_slug":      "gpt-5",
				"status":          "finished_successfully",
				"content":         map[string]interface{}{"parts": []string{"hai dari mock"}},
				"metadata": map[string]interface{}{
					"finish_details": map[string]string{"type": "stop"},
					"usage":          map[string]int{"prompt_tokens": 5, "completion_tokens": 7},
				},
			},
		}
		b, _ := json.Marshal(msg)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
	}))
}

// ---- FIX #3: bentuk body import harus cocok dgn script manual + butuh auth ----

func TestImportTokensRequiresAdminAuth(t *testing.T) {
	h, cfg := newTestHandler(t)
	body := `{"tokens":{"access_token":"at","refresh_token":"rt"}}`

	pub := httptest.NewRequest("POST", "/api/accounts/import-tokens", strings.NewReader(body))
	pubRec := httptest.NewRecorder()
	h.AuthAdmin(h.OAuthImport)(pubRec, pub)
	if pubRec.Code != 401 {
		t.Fatalf("tanpa token admin: want 401 got %d (body=%s)", pubRec.Code, pubRec.Body.String())
	}

	auth := adminJWT(t, h, cfg)
	ok := httptest.NewRequest("POST", "/api/accounts/import-tokens", strings.NewReader(body))
	ok.Header.Set("Authorization", authHeader(auth))
	okRec := httptest.NewRecorder()
	h.AuthAdmin(h.OAuthImport)(okRec, ok)
	if okRec.Code != 200 {
		t.Fatalf(" dgn token admin: want 200 got %d body=%s", okRec.Code, okRec.Body.String())
	}
}

func TestImportTokensAcceptsScriptAndLegacyPayloads(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"nested snake_case (oauth_manual.py)", `{"tokens":{"access_token":"at-1","refresh_token":"rt-1","expires_in":3600},"label":"manual-oauth"}`},
		{"flat snake_case", `{"access_token":"at-2","refresh_token":"rt-2"}`},
		{"flat camelCase (legacy)", `{"accessToken":"at-3","refreshToken":"rt-3","expiresIn":3600}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, cfg := newTestHandler(t)
			_ = cfg
			req := httptest.NewRequest("POST", "/api/accounts/import-tokens", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.OAuthImport(rec, req)
			if rec.Code != 200 {
				t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
			}
			var out map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("bad JSON: %v", err)
			}
			if out["ok"] != true {
				t.Fatalf("want ok=true got %v", out)
			}
			if out["id"] == "" || out["id"] == nil {
				t.Fatal("id akun kosong")
			}
			var credType, status string
			if err := h.App.DB.QueryRow(`SELECT credential_type,status FROM accounts WHERE id=?`, out["id"]).Scan(&credType, &status); err != nil {
				t.Fatal(err)
			}
			if credType != "oauth_refresh" {
				t.Fatalf("credential_type want oauth_refresh got %s", credType)
			}
			if status != "valid" {
				t.Fatalf("status want valid got %s", status)
			}
		})
	}
}

func TestImportTokensRejectsMissingRefresh(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("POST", "/api/accounts/import-tokens", strings.NewReader(`{"access_token":"cuma-access"}`))
	rec := httptest.NewRecorder()
	h.OAuthImport(rec, req)
	if rec.Code != 400 {
		t.Fatalf("want 400 got %d body=%s", rec.Code, rec.Body.String())
	}
	var n int
	if err := h.App.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("akun gak boleh ke-seed saat refresh_token kosong, got %d", n)
	}
}

// ---- FIX #2: refresh_token hasil rotasi WAJIB ikut tersimpan ----

func TestImportVerifyStoresRotatedRefreshToken(t *testing.T) {
	h, cfg := newTestHandler(t)
	srv := mockOAuth(t, "rt-ROTATED-999", false)
	defer srv.Close()
	t.Setenv("OPENAI_AUTH_BASE", srv.URL)

	// tanpa access_token -> handler wajib verifikasi via refresh
	body := `{"tokens":{"refresh_token":"rt-ORIGINAL-000","label":"rot-test"}}`
	req := httptest.NewRequest("POST", "/api/accounts/import-tokens", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.OAuthImport(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["verify"] != "ok" {
		t.Fatalf("want verify=ok got %v", out)
	}
	if got := storedRefreshToken(t, h, cfg, out["id"].(string)); got != "rt-ROTATED-999" {
		t.Fatalf("BUG: refresh_token hasil rotasi kebuang — DB still %q want rt-ROTATED-999", got)
	}
	if out["rotatedRefreshToken"] != true {
		t.Fatalf("want rotatedRefreshToken=true got %v", out["rotatedRefreshToken"])
	}
}

func TestImportVerifyMarksInvalidWhenUpstreamRejects(t *testing.T) {
	h, cfg := newTestHandler(t)
	srv := mockOAuth(t, "", true)
	defer srv.Close()
	t.Setenv("OPENAI_AUTH_BASE", srv.URL)

	body := `{"refresh_token":"rt-DEAD"}`
	req := httptest.NewRequest("POST", "/api/accounts/import-tokens", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.OAuthImport(rec, req)
	if rec.Code != 502 {
		t.Fatalf("want 502 got %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	id, _ := out["id"].(string)
	if id == "" {
		t.Fatal("no id in response")
	}
	var status, lastErr string
	if err := h.App.DB.QueryRow(`SELECT status,last_error FROM accounts WHERE id=?`, id).Scan(&status, &lastErr); err != nil {
		t.Fatal(err)
	}
	if status != "invalid" {
		t.Fatalf("want status=invalid got %q", status)
	}
	if !strings.Contains(lastErr, "token_expired") {
		t.Fatalf("want last_error ngediak alasan upstream, got %q", lastErr)
	}
	_ = cfg
}

func TestChatPathStoresRotatedRefreshToken(t *testing.T) {
	h, cfg := newTestHandler(t)
	hits := 0
	authSrv := mockOAuth(t, "rt-ROTATED-FROM-CHAT", false)
	defer authSrv.Close()
	convSrv := mockConv(t, &hits)
	defer convSrv.Close()
	t.Setenv("OPENAI_AUTH_BASE", authSrv.URL)
	t.Setenv("CHATGPT_API_BASE", convSrv.URL)

	expired := jwtWithExp(time.Now().Add(-time.Hour).Unix())
	id := seedOAuthAccount(t, h, cfg, expired, "rt-ORIGINAL")
	key := seedAPIKey(t, h)

	body := `{"model":"gpt-5","stream":false,"messages":[{"role":"user","content":"halo"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", authHeader(key))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, req)

	if rec.Code != 200 {
		t.Fatalf("chat want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hai dari mock") {
		t.Fatalf("isi jawaban gak sesuai: %s", rec.Body.String())
	}
	if hits != 1 {
		t.Fatalf("want 1 hit ke conv upstream, got %d", hits)
	}
	if got := storedRefreshToken(t, h, cfg, id); got != "rt-ROTATED-FROM-CHAT" {
		t.Fatalf("BUG: rotasi refresh_token kebuang di jalur chat — DB has %q", got)
	}
	var tokEnc string
	if err := h.App.DB.QueryRow(`SELECT access_token_enc FROM accounts WHERE id=?`, id).Scan(&tokEnc); err != nil {
		t.Fatal(err)
	}
	at, err := core.DecryptCredential(cfg.Security.CredentialEncryptionKey, tokEnc)
	if err != nil {
		t.Fatal(err)
	}
	if at == expired {
		t.Fatal("access_token basi masih tersimpan di DB")
	}
	if exp := upstream.JWTExp(at); exp <= time.Now().Unix() {
		t.Fatalf("access_token gak ke-update ke yang fresh: exp=%d now=%d", exp, time.Now().Unix())
	}
}

func TestChatPathRejectsDeadRefreshToken(t *testing.T) {
	h, cfg := newTestHandler(t)
	hits := 0
	authSrv := mockOAuth(t, "", true)
	defer authSrv.Close()
	convSrv := mockConv(t, &hits)
	defer convSrv.Close()
	t.Setenv("OPENAI_AUTH_BASE", authSrv.URL)
	t.Setenv("CHATGPT_API_BASE", convSrv.URL)

	expired := jwtWithExp(time.Now().Add(-time.Hour).Unix())
	id := seedOAuthAccount(t, h, cfg, expired, "rt-DEAD")
	key := seedAPIKey(t, h)

	body := `{"model":"gpt-5","stream":false,"messages":[{"role":"user","content":"halo"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", authHeader(key))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, req)

	if rec.Code != 503 {
		t.Fatalf("want 503 got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "account_unavailable") {
		t.Fatalf("want error type account_unavailable, got %s", rec.Body.String())
	}
	if hits != 0 {
		t.Fatalf("BUG: akun mati harusnya gak di-stream-kan ke upstream, conv hits=%d", hits)
	}
	var status string
	if err := h.App.DB.QueryRow(`SELECT status FROM accounts WHERE id=?`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "invalid" {
		t.Fatalf("want status=invalid got %q", status)
	}
}

func TestChatPathKeepsWorkingWhenTokenStillValid(t *testing.T) {
	h, cfg := newTestHandler(t)
	hits := 0
	convSrv := mockConv(t, &hits)
	defer convSrv.Close()
	t.Setenv("CHATGPT_API_BASE", convSrv.URL)
	authSrv := mockOAuth(t, "SHOULD-NOT-BE-USED", false)
	defer authSrv.Close()
	t.Setenv("OPENAI_AUTH_BASE", authSrv.URL)

	valid := jwtWithExp(time.Now().Add(48 * time.Hour).Unix())
	id := seedOAuthAccount(t, h, cfg, valid, "rt-IDLE")
	key := seedAPIKey(t, h)

	body := `{"model":"gpt-5","stream":false,"messages":[{"role":"user","content":"halo"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", authHeader(key))
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, req)
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := storedRefreshToken(t, h, cfg, id); got != "rt-IDLE" {
		t.Fatalf("token belum expired gak boleh di-refresh, DB refresh=%q", got)
	}
}

func authHeader(tok string) string {
	return "Bearer" + " " + tok
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
