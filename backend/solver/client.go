package solver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client = pemanggil sidecar solver.cjs (HTTP JSON).
type Client struct {
	URL  string
	HTTP *http.Client
}

func NewClient(url string) *Client {
	return &Client{URL: url, HTTP: &http.Client{Timeout: 300 * time.Second}}
}

// LoginResult = balikan /login sidecar.
type LoginResult struct {
	OK           bool     `json:"ok"`
	Stage        string   `json:"stage"`
	AccessToken  string   `json:"accessToken"`
	SessionToken string   `json:"sessionToken"`
	CfClearance  string   `json:"cf_clearance"`
	Cookies      string   `json:"cookies"`
	UserAgent    string   `json:"userAgent"`
	Error        string   `json:"error"`
	TimeSec      string   `json:"timeSec"`
}

// SolveResult = balikan /solve sidecar.
type SolveResult struct {
	OK          bool   `json:"ok"`
	CfClearance string `json:"cf_clearance"`
	Cookies     string `json:"cookies"`
	UserAgent   string `json:"userAgent"`
	JSONBody    string `json:"jsonBody"`
	Error       string `json:"error"`
	TimeSec     string `json:"timeSec"`
}

// HarvestOAuthResult = balikan /oauth-harvest sidecar (jalur A).
type HarvestOAuthResult struct {
	OK         bool   `json:"ok"`
	Code       string `json:"code"`
	State      string `json:"state"`
	Verifier   string `json:"verifier"`
	FinalURL   string `json:"finalUrl"`
	SawConsent bool   `json:"sawConsent"`
	Error      string `json:"error"`
	TimeSec    string `json:"timeSec"`
}

func (c *Client) post(path string, payload map[string]interface{}, out interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", c.URL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("solver sidecar gak reachable (%s): %w — nyalakan: CHROME_PATH=... node solver/server.cjs 7900", c.URL, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return fmt.Errorf("solver HTTP %d: %s", resp.StatusCode, string(b[:min(200, len(b))]))
	}
	return json.Unmarshal(b, out)
}

// Login auto-login email/pw via browser sidecar.
func (c *Client) Login(email, password string, timeoutSec int) (*LoginResult, error) {
	if timeoutSec <= 0 {
		timeoutSec = 150
	}
	var r LoginResult
	if err := c.post("/login", map[string]interface{}{"email": email, "password": password, "timeoutSec": timeoutSec}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// SolveChallenge lewatin Cloudflare challenge, ambil cf_clearance+cookies.
func (c *Client) SolveChallenge(url string, timeoutSec int) (*SolveResult, error) {
	if timeoutSec <= 0 {
		timeoutSec = 90
	}
	var r SolveResult
	if err := c.post("/solve", map[string]interface{}{"url": url, "timeoutSec": timeoutSec}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// HarvestOAuthCode jalanin sidecar buat panen authorization code (jalur A).
// cookiesRaw: array JSON Cookie-Editor atau cookie-string Google.
func (c *Client) HarvestOAuthCode(cookiesRaw, authURL string, timeoutSec int) (*HarvestOAuthResult, error) {
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	var r HarvestOAuthResult
	if err := c.post("/oauth-harvest", map[string]interface{}{
		"cookies": cookiesRaw, "authUrl": authURL, "timeoutSec": timeoutSec,
	}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func min(a, b int) int { if a < b { return a }; return b }
