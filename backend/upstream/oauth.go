package upstream

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ---- OAuth konstanta (pake client_id resmi Codex CLI) ----

const (
	OAuthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	OAuthRedirectURI = "http://localhost:1455/auth/callback"
	OAuthScope       = "openid profile email offline_access"

	authBaseDefault = "https://auth.openai.com"
)

// authBase = endpoint OAuth. Default resmi; OPENAI_AUTH_BASE cuma buat test/mock.
func authBase() string {
	if v := strings.TrimSpace(os.Getenv("OPENAI_AUTH_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return authBaseDefault
}

// OAuthTokenURL tetap diekspos untuk kompatibilitas; pakai authBase().
func OAuthTokenURL() string { return authBase() + "/oauth/token" }

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// PKCEVerifier bikin code_verifier acak.
func PKCEVerifier() string {
	b := make([]byte, 48)
	_, _ = rand.Read(b)
	return b64url(b)
}

// PKCEChallenge S256 dari verifier.
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return b64url(sum[:])
}

// RandomState bikin nilai state acak.
func RandomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "cgt2api-" + b64url(b)
}

// BuildAuthURL susun URL authorize dengan PKCE.
func BuildAuthURL(challenge, state string) string {
	p := url.Values{
		"client_id":             {OAuthClientID},
		"response_type":         {"code"},
		"redirect_uri":          {OAuthRedirectURI},
		"scope":                 {OAuthScope},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"prompt":                {"consent"},
	}
	return "https://auth.openai.com/oauth/authorize?" + p.Encode()
}

// ---- token exchange (HTTP murni, tanpa browser) ----

// OAuthTokens = hasil POST /oauth/token.
type OAuthTokens struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	IDToken          string `json:"id_token"`
	ExpiresIn        int    `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// ExchangeCode tukar authorization code jadi token.
func ExchangeCode(code, verifier string) (*OAuthTokens, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {OAuthRedirectURI},
		"client_id":     {OAuthClientID},
		"code_verifier": {verifier},
	}
	return postToken(form)
}

// RefreshTokens tukar refresh_token jadi access_token baru (jalur runtime).
func RefreshTokens(refreshToken string) (*OAuthTokens, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {OAuthClientID},
		"scope":         {OAuthScope},
	}
	return postToken(form)
}

func postToken(form url.Values) (*OAuthTokens, error) {
	req, err := http.NewRequest("POST", OAuthTokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	cli := &http.Client{Timeout: 45 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth token: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	var t OAuthTokens
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("oauth token: parse JSON (HTTP %d): %s", resp.StatusCode, trunc(string(b), 200))
	}
	if t.Error != "" || (t.AccessToken == "" && t.RefreshToken == "") {
		msg := t.Error
		if t.ErrorDescription != "" {
			msg = t.Error + ": " + t.ErrorDescription
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, trunc(string(b), 200))
		}
		return nil, fmt.Errorf("oauth token gagal: %s", msg)
	}
	return &t, nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
