package api

import (
	"sync"

	"chatgpt2api-go/app"
	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// Account = baris akun dari DB (kredensial masih terenkripsi).
type Account struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	CredType      string `json:"credentialType"` // access_token | session_token
	Status        string `json:"status"`         // valid | invalid | cooldown | unknown
	Preview       string `json:"preview"`
	ProxyURL      string `json:"proxyUrl,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	Email         string `json:"email,omitempty"`
	CooldownUntil int64  `json:"cooldownUntil,omitempty"`
	RequestCount  int64  `json:"requestCount"`
	SuccessCount  int64  `json:"successCount"`
	ErrorCount    int64  `json:"errorCount"`
	LastUsedAt    int64  `json:"lastUsedAt,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
}

// Pool = rotasi akun round-robin + cooldown.
type Pool struct {
	mu  sync.Mutex
	rr  int
	app *app.Core
}

func NewPool(c *app.Core) *Pool {
	return &Pool{app: c}
}

// Acquire ambil akun valid dengan in-flight terkecil; skip yang cooldown.
func (p *Pool) Acquire() (id string, cred upstream.Credential, err error) {
	rows, err := p.app.DB.Query(`SELECT id, access_token_enc, cookies_enc, cf_clearance,
		user_agent, oai_client_version, in_flight, cooldown_until FROM accounts ORDER BY created_at ASC`)
	if err != nil {
		return "", cred, err
	}
	defer rows.Close()

	type cand struct {
		id       string
		inFlight int
		tokEnc   string
		ckEnc    string
		cf       string
		ua       string
		ver      string
	}
	var cands []cand
	now := core.Now()
	for rows.Next() {
		var c cand
		var inFlight, cooldown int64
		if err := rows.Scan(&c.id, &c.tokEnc, &c.ckEnc, &c.cf, &c.ua, &c.ver, &inFlight, &cooldown); err != nil {
			continue
		}
		if cooldown > now {
			continue
		}
		c.inFlight = int(inFlight)
		cands = append(cands, c)
	}
	if len(cands) == 0 {
		return "", cred, errNoAccount
	}
	p.mu.Lock()
	p.rr = (p.rr + 1) % len(cands)
	off := p.rr
	p.mu.Unlock()
	best := cands[0]
	for i := 0; i < len(cands); i++ {
		c := cands[(off+i)%len(cands)]
		if c.inFlight < best.inFlight {
			best = c
		}
	}
	key := p.app.Config.Security.CredentialEncryptionKey
	tok, err := core.DecryptCredential(key, best.tokEnc)
	if err != nil {
		return "", cred, err
	}
	ck, _ := core.DecryptCredential(key, best.ckEnc)
	cred = upstream.Credential{
		AccessToken:      tok,
		Cookies:          ck,
		CFClearance:      best.cf,
		UserAgent:        best.ua,
		OAIClientVersion: best.ver,
		OAIDeviceID:      upstream.OAIDidFromCookies(ck),
	}
	return best.id, cred, nil
}

// ReportUpdate catat hasil pemakaian akun.
func (p *Pool) ReportUpdate(id string, ok bool, errMsg string, cooldownSec int64) {
	now := core.Now()
	if ok {
		p.app.DB.Exec(`UPDATE accounts SET in_flight = MAX(in_flight-1,0), request_count = request_count+1,
			success_count = success_count+1, last_used_at = ?, status = 'valid', last_error = '', updated_at = ? WHERE id = ?`,
			now, now, id)
		return
	}
	cd := int64(0)
	if cooldownSec > 0 {
		cd = now + cooldownSec
	}
	p.app.DB.Exec(`UPDATE accounts SET in_flight = MAX(in_flight-1,0), request_count = request_count+1,
		error_count = error_count+1, last_used_at = ?, status = CASE WHEN ? > 0 THEN 'cooldown' ELSE 'invalid' END,
		last_error = ?, cooldown_until = ?, updated_at = ? WHERE id = ?`, now, cd, core.Truncate(errMsg, 300), cd, now, id)
}

// AcquireInc naikkan in-flight saat mulai request.
func (p *Pool) AcquireInc(id string) {
	p.app.DB.Exec(`UPDATE accounts SET in_flight = in_flight+1 WHERE id = ?`, id)
}

var errNoAccount = &APIError{Status: 503, Code: "no_account", Message: "belum ada akun valid — tambahkan accessToken di dashboard (menu Accounts)"}

// APIError = error standar balikan ke client.
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string { return e.Message }
