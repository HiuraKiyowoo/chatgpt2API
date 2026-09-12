package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"chatgpt2api-go/app"
	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// Handler = dependensi handler HTTP.
type Handler struct {
	App  *app.Core
	Pool *Pool
}

func NewHandler(a *app.Core) *Handler {
	return &Handler{App: a, Pool: NewPool(a)}
}

// Models daftar alias model (semua route ke upstream web yang sama).
func (h *Handler) Models(w http.ResponseWriter, r *http.Request) {
	models := make([]map[string]interface{}, 0, len(core.ModelAliases))
	for _, m := range core.ModelAliases {
		models = append(models, map[string]interface{}{
			"id": m, "object": "model", "owned_by": "chatgpt2api",
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"object": "list", "data": models})
}

// V1Chat endpoint /v1/chat/completions (stream + non-stream).
func (h *Handler) V1Chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apiErr(w, 405, "method_not_allowed", "pakai POST")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		apiErr(w, 400, "bad_request", "baca body gagal")
		return
	}
	var req oaRequest
	if err := json.Unmarshal(body, &req); err != nil {
		apiErr(w, 400, "bad_request", "JSON invalid: "+err.Error())
		return
	}
	if len(req.Messages) == 0 {
		apiErr(w, 400, "bad_request", "messages kosong")
		return
	}
	if !aliasValid(req.Model) {
		req.Model = "gpt-5"
	}
	turns := toChatTurns(req.Messages)

	accID, cred, err := h.Pool.Acquire()
	if err != nil {
		h.writeUpstreamErr(w, err)
		return
	}
	h.Pool.AcquireInc(accID)

	// refresh token kalau expired/hampir (60 detik sebelum)
	if upstream.TokenExpiredAtauHampir(cred.AccessToken, time.Now().Unix()) {
		key := h.App.Config.Security.CredentialEncryptionKey
		var refreshTok string
		// tipe oauth_refresh: credential_enc = refresh_token
		var credEnc string
		if err := h.App.DB.QueryRow(`SELECT credential_enc FROM accounts WHERE id = ?`, accID).Scan(&credEnc); err == nil {
			if v, err2 := core.DecryptCredential(key, credEnc); err2 == nil && v != "" && !strings.HasPrefix(v, "eyJ") {
				refreshTok = v
			}
		}
		if refreshTok != "" {
			// jalur cepat: HTTP murni, tanpa browser
			if toks, errR := upstream.RefreshTokens(refreshTok); errR == nil && toks.AccessToken != "" {
				cred.AccessToken = toks.AccessToken
				tokEnc, _ := core.EncryptCredential(key, toks.AccessToken)
				h.App.DB.Exec(`UPDATE accounts SET access_token_enc = ?, status='valid', updated_at = ? WHERE id = ?`,
					tokEnc, core.Now(), accID)
			}
		} else if cred.Cookies != "" {
			// fallback: tukar cookie session jadi JWT
			if fresh, errRefresh := upstream.RefreshAccessToken(h.App.Upstream.HTTP, cred); errRefresh == nil && fresh != "" {
				cred.AccessToken = fresh
				tokEnc, _ := core.EncryptCredential(key, fresh)
				h.App.DB.Exec(`UPDATE accounts SET access_token_enc = ?, updated_at = ? WHERE id = ?`, tokEnc, core.Now(), accID)
			}
		}
	}

	parts, err := h.App.Upstream.Stream(cred, modelSlug(req.Model), turns, "root")
	if err != nil {
		h.Pool.ReportUpdate(accID, false, err.Error(), cooldownFor(err))
		h.writeUpstreamErr(w, err)
		return
	}

	if !req.Stream {
		var sb strings.Builder
		var done bool
		for p := range parts {
			if p.Err != nil {
				h.Pool.ReportUpdate(accID, false, p.Err.Error(), cooldownFor(p.Err))
				h.writeUpstreamErr(w, p.Err)
				return
			}
			if p.Done {
				done = true
				break
			}
			sb.WriteString(p.Text)
		}
		if !done {
			h.Pool.ReportUpdate(accID, true, "", 0)
		} else {
			h.Pool.ReportUpdate(accID, true, "", 0)
		}
		completeResp(w, req.Model, sb.String(), "", 0, 0)
		h.App.Logs.Add("info", "api", "chat ok ("+req.Model+")", core.Truncate(sb.String(), 120))
		return
	}

	sw := newSSE(w, req.Model)
	var sb strings.Builder
	for p := range parts {
		if p.Err != nil {
			h.Pool.ReportUpdate(accID, false, p.Err.Error(), cooldownFor(p.Err))
			if sb.Len() == 0 {
				apiErr(w, 502, "upstream_error", p.Err.Error())
			} else {
				sw.finish("stop")
			}
			return
		}
		if p.Done {
			sw.finish("stop")
			h.Pool.ReportUpdate(accID, true, "", 0)
			h.App.Logs.Add("info", "api", "chat stream ok ("+req.Model+")", core.Truncate(sb.String(), 120))
			return
		}
		sb.WriteString(p.Text)
		sw.delta(p.Text)
	}
	sw.finish("stop")
	h.Pool.ReportUpdate(accID, true, "", 0)
}

func (h *Handler) writeUpstreamErr(w http.ResponseWriter, err error) {
	if ue, ok := err.(*upstream.UpstreamError); ok {
		apiErr(w, mapStatus(ue.Status), "upstream_error", ue.Error())
		return
	}
	if ae, ok := err.(*APIError); ok {
		apiErr(w, ae.Status, ae.Code, ae.Message)
		return
	}
	apiErr(w, 502, "upstream_error", err.Error())
}

func mapStatus(s int) int {
	switch {
	case s == 401 || s == 403:
		return 401 // kredensial invalid
	case s == 429:
		return 429
	default:
		return 502
	}
}

func cooldownFor(err error) int64 {
	if ue, ok := err.(*upstream.UpstreamError); ok {
		if ue.Status == 429 {
			return 300
		}
		if ue.Status == 401 || ue.Status == 403 {
			return 0
		}
		return 60
	}
	return 60
}

func aliasValid(m string) bool {
	for _, a := range core.ModelAliases {
		if m == a {
			return true
		}
	}
	return false
}

// modelSlug: upstream web pakai slug "gpt-5" dsb — sementara 1:1 dengan alias.
func modelSlug(alias string) string {
	return alias
}
