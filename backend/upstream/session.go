package upstream

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// JWTPayload datar kecil yang kita butuh dari accessToken.
type JWTPayload struct {
	Exp       int64  `json:"exp"`
	Sub       string `json:"sub"`
	AccountID string `json:"chatgpt_account_id"`
}

// JWTAccountID baca klaim chatgpt_account_id dari accessToken (dipakai header
// ChatGPT-Account-Id jalur android). "" kalau gak ada.
func JWTAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var p JWTPayload
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	return p.AccountID
}

// JWTExp ambil klaim exp dari accessToken tanpa verifikasi (kita cuma
// perlu tahu kapan kedaluwarsa, bukan memvalidasi tanda tangan upstream).
func JWTExp(token string) int64 {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}
	var p JWTPayload
	if json.Unmarshal(raw, &p) != nil {
		return 0
	}
	return p.Exp
}

// sessionURL ikut override CHATGPT_API_BASE (lihat ChatBase).
func sessionURL() string { return ChatBase() + "/api/auth/session" }

// RefreshAccessToken tukar cookie session jadi accessToken JWT baru.
// Return (tokenBaru, pesanError). Kalau gagal, token lama tetap dipakai caller.
func RefreshAccessToken(httpCli *http.Client, cred Credential) (string, error) {
	if strings.TrimSpace(cred.Cookies) == "" {
		return "", fmt.Errorf("tidak ada cookie session buat refresh")
	}
	req, err := http.NewRequest("GET", sessionURL(), nil)
	if err != nil {
		return "", err
	}
	ua := cred.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	cookies := cred.Cookies
	if cred.CFClearance != "" {
		cookies = strings.TrimSpace(cookies + "; cf_clearance=" + cred.CFClearance)
	}
	req.Header.Set("Cookie", cookies)

	resp, err := httpCli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("refresh: HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	var out struct {
		AccessToken string `json:"accessToken"`
		Expires     string `json:"expires"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("refresh: parse JSON: %w", err)
	}
	if out.Error != "" || out.AccessToken == "" {
		return "", fmt.Errorf("refresh: sesi mati (%s)", out.Error)
	}
	return out.AccessToken, nil
}

// TokenExpiredAtauHampir: true kalau exp < now + margin (60 detik) atau gak terbaca.
func TokenExpiredAtauHampir(token string, now int64) bool {
	exp := JWTExp(token)
	if exp == 0 {
		return true
	}
	return now+60 >= exp
}

var _ = time.Now

// JWTEmailFromToken baca klaim email (https://api.openai.com/profile.email)
// dari accessToken NextAuth ChatGPT. "" kalau gak ada/gagal.
func JWTEmailFromToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var p struct {
		Email   string `json:"email"`
		Profile struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"https://api.openai.com/profile"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return ""
	}
	if p.Profile.Email != "" {
		return p.Profile.Email
	}
	return p.Email
}
