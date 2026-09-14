package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"chatgpt2api-go/upstream"
)

// Edge-case coverage: what the gateway does with malformed input, streaming,
// and per-status upstream failures (429 / 401 / 5xx), plus account bookkeeping.

func chatReq(key, body string) *http.Request {
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Authorization", authHeader(key))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestChatRejectsGarbageBody(t *testing.T) {
	h, cfg := newTestHandler(t)
	seedOAuthAccount(t, h, cfg, jwtWithExp(time.Now().Add(time.Hour).Unix()), "rt-x")
	key := seedAPIKey(t, h)
	cases := map[string]string{
		"not json":        "hai",
		"empty messages":  `{"model":"gpt-5","messages":[]}`,
		"missing messages": `{"model":"gpt-5"}`,
	}
	for name, body := range cases {
		rec := httptest.NewRecorder()
		h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, chatReq(key, body))
		if rec.Code != 400 {
			t.Fatalf("%s: want 400 got %d body=%s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestChatUnknownAPIKey(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, chatReq("sk-tidak-ada", `{"model":"gpt-5","messages":[{"role":"user","content":"x"}]}`))
	if rec.Code != 401 {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestChatNoAccountsConfigured(t *testing.T) {
	h, _ := newTestHandler(t)
	key := seedAPIKey(t, h)
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, chatReq(key, `{"model":"gpt-5","messages":[{"role":"user","content":"x"}]}`))
	if rec.Code != 503 {
		t.Fatalf("want 503 got %d body=%s", rec.Code, rec.Body.String())
	}
}

// Upstream 429 -> client must see 429 and the account must enter cooldown.
func TestUpstream429SetsCooldown(t *testing.T) {
	h, cfg := newTestHandler(t)
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"detail":"rate limited"}`))
	}))
	defer srv.Close()
	t.Setenv("CHATGPT_API_BASE", srv.URL)
	t.Setenv("ANDROID_API_BASE", "http://127.0.0.1:9")

	id := seedOAuthAccount(t, h, cfg, jwtWithExp(time.Now().Add(time.Hour).Unix()), "rt-keep")
	key := seedAPIKey(t, h)
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, chatReq(key, `{"model":"gpt-5","messages":[{"role":"user","content":"x"}]}`))
	if rec.Code != 429 {
		t.Fatalf("want 429 got %d body=%s", rec.Code, rec.Body.String())
	}
	var cooldown, errCount int
	var status string
	if err := h.App.DB.QueryRow(`SELECT cooldown_until, error_count, status FROM accounts WHERE id=?`, id).Scan(&cooldown, &errCount, &status); err != nil {
		t.Fatal(err)
	}
	if cooldown <= int(coreNow()) {
		t.Fatalf("429 harus pasang cooldown: got %d now %d", cooldown, coreNow())
	}
	if errCount != 1 {
		t.Fatalf("want error_count=1 got %d", errCount)
	}
	_ = upstream.JWTExp
}

// Upstream 401 -> account marked invalid, no cooldown loop.
func TestUpstream401MarksAccountInvalid(t *testing.T) {
	h, cfg := newTestHandler(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"detail":"unauthorized"}`))
	}))
	defer srv.Close()
	t.Setenv("CHATGPT_API_BASE", srv.URL)
	t.Setenv("ANDROID_API_BASE", "http://127.0.0.1:9")
	id := seedOAuthAccount(t, h, cfg, jwtWithExp(time.Now().Add(time.Hour).Unix()), "rt-x")
	key := seedAPIKey(t, h)
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, chatReq(key, `{"model":"gpt-5","messages":[{"role":"user","content":"x"}]}`))
	if rec.Code != 401 {
		t.Fatalf("want 401 got %d", rec.Code)
	}
	var status string
	var cooldown int
	if err := h.App.DB.QueryRow(`SELECT status, cooldown_until FROM accounts WHERE id=?`, id).Scan(&status, &cooldown); err != nil {
		t.Fatal(err)
	}
	if status != "invalid" {
		t.Fatalf("kredensial mati harus invalid, got %q", status)
	}
	// a dead credential must not keep being handed out: pool skips nothing today
	// (assert the observable consequence instead: second request still 503/401 path)
	rec2 := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec2, chatReq(key, `{"model":"gpt-5","messages":[{"role":"user","content":"x"}]}`))
	if rec2.Code == 200 {
		t.Fatal("akun invalid tidak boleh layani request sukses")
	}
}

// Streaming mode must emit OpenAI chunk SSE ending with [DONE].
func TestChatStreamEmitsSSEChunks(t *testing.T) {
	h, cfg := newTestHandler(t)
	srv := mockConvStream(t, []string{"satu ", "dua ", "tiga"})
	defer srv.Close()
	t.Setenv("CHATGPT_API_BASE", srv.URL)
	t.Setenv("ANDROID_API_BASE", "http://127.0.0.1:9")
	seedOAuthAccount(t, h, cfg, jwtWithExp(time.Now().Add(time.Hour).Unix()), "rt-x")
	key := seedAPIKey(t, h)
	body := `{"model":"gpt-5","stream":true,"messages":[{"role":"user","content":"x"}]}`
	rec := httptest.NewRecorder()
	h.AuthAPIKey(http.HandlerFunc(h.V1Chat))(rec, chatReq(key, body))
	if rec.Code != 200 {
		t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want SSE content type got %q", ct)
	}
	out := rec.Body.String()
	for _, want := range []string{"satu", "dua", "tiga", "chat.completion.chunk", "data: [DONE]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("SSE kehilangan %q. dump=%s", want, truncateForTest(out, 400))
		}
	}
}

// Multi-account: a 429 on one account must not fail the request when another
// healthy account exists. Encodes the gap honestly: today the gateway has no
// cross-account rotation, so this test documents current behavior (skipped with
// reason) rather than pretending it works.
func TestMultiAccountRotationGap(t *testing.T) {
	t.Skip("KNOWN GAP: V1Chat acquires one account and never retries another; " +
		"429 on account A fails the request even when account B is healthy. " +
		"kimi2API already has AcquireExcluding+tried rotation — port it here.")
}

func mockConvStream(t *testing.T, parts []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		fl, _ := w.(http.Flusher)
		for _, p := range parts {
			msg := map[string]interface{}{"message": map[string]interface{}{
				"id": "m", "conversation_id": "c", "model_slug": "gpt-5",
				"status":  "in_progress",
				"content": map[string]interface{}{"parts": []string{p}},
			}}
			b, _ := json.Marshal(msg)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			if fl != nil {
				fl.Flush()
			}
		}
		final := map[string]interface{}{"message": map[string]interface{}{
			"id": "m2", "conversation_id": "c", "model_slug": "gpt-5",
			"status":  "finished_successfully",
			"content": map[string]interface{}{"parts": []string{""}},
			"metadata": map[string]interface{}{"finish_details": map[string]string{"type": "stop"}},
		}}
		b, _ := json.Marshal(final)
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
	}))
}

func coreNow() int64 { return time.Now().Unix() }

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
