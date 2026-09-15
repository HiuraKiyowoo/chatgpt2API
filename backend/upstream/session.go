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
// Return (tokenBaru, cookieSessionTerupdate, err). PENTING: NextAuth MEMUTAR
// __Secure-next-auth.session-token di Set-Cookie setiap kali /api/auth/session
// dipanggil. Caller WAJIB menyimpan return ke-2 — memakai cookie lama terus
// = sidik jari 'session curian' (terbukti: token pool di-revoke server setelah
// beberapa panggilan dengan cookie basi). Kalau gagal, token lama tetap dipakai.
func RefreshAccessToken(httpCli *http.Client, cred Credential) (string, string, error) {
	if strings.TrimSpace(cred.Cookies) == "" {
		return "", "", fmt.Errorf("tidak ada cookie session buat refresh")
	}
	cookies := cred.Cookies
	if cred.CFClearance != "" {
		cookies = strings.TrimSpace(cookies + "; cf_clearance=" + cred.CFClearance)
	}
	// Cloudflare menyaring /api/auth/session berdasar identitas klien:
	// UA web (Chrome) -> 403 HTML challenge, UA aplikasi ChatGPT -> 200 + accessToken.
	// Terbukti 2026-09: 200 untuk 'ChatGPT/1.2026.181 (Android 16; ...)' maupun
	// UA Android seluler biasa. Coba kandidat berurutan; berhenti saat dapat token.
	uaWeb := cred.UserAgent
	if uaWeb == "" {
		uaWeb = defaultUA
	}
	cands := []struct {
		ua    string
		phone bool
	}{
		{"ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)", true},
		{uaWeb, false},
	}
	var lastErr error
	for _, c := range cands {
		tok, rot, err := refreshOnce(httpCli, cookies, c.ua, c.phone)
		if err == nil {
			return tok, rot, nil
		}
		lastErr = err
	}
	return "", "", lastErr
}

func refreshOnce(httpCli *http.Client, cookies, ua string, androidIdent bool) (string, string, error) {
	req, err := http.NewRequest("GET", sessionURL(), nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	if androidIdent {
		req.Header.Set("OAI-Package-Name", "com.openai.chatgpt")
		req.Header.Set("OAI-Client-Type", "android")
	}
	req.Header.Set("Cookie", cookies)

	resp, err := httpCli.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("refresh: HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	var out struct {
		AccessToken string `json:"accessToken"`
		Expires     string `json:"expires"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", "", fmt.Errorf("refresh: parse JSON: %w", err)
	}
	if out.Error != "" || out.AccessToken == "" {
		return "", "", fmt.Errorf("refresh: sesi mati (%s)", out.Error)
	}
	return out.AccessToken, mergeRotatedSession(cookies, resp.Cookies()), nil
}

// mergeRotatedSession ganti semua pasangan *session-token* di string cookie lama
// dengan versi rotasi dari Set-Cookie response (chunk .0/.1 disimpan apa adanya —
// server cuma ngenalin bentuk chunk terpisah). Cookie non-session tidak disentuh.
func mergeRotatedSession(old string, sc []*http.Cookie) string {
	var rot []*http.Cookie
	for _, c := range sc {
		if strings.Contains(c.Name, "session-token") && c.Value != "" && !c.Expires.Before(time.Now().Add(-time.Hour)) {
			rot = append(rot, c)
		}
	}
	if len(rot) == 0 {
		return ""
	}
	kept := []string{}
	for _, kv := range strings.Split(old, ";") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		name := kv
		if i := strings.Index(kv, "="); i > 0 {
			name = kv[:i]
		}
		if strings.Contains(name, "session-token") {
			continue
		}
		kept = append(kept, kv)
	}
	for _, c := range rot {
		kept = append(kept, c.Name+"="+c.Value)
	}
	return strings.Join(kept, "; ")
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
