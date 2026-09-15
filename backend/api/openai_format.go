package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// ---- permintaan OpenAI ----

type oaMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type oaRequest struct {
	Model     string      `json:"model"`
	Messages  []oaMessage `json:"messages"`
	Stream    bool        `json:"stream"`
	MaxTokens int         `json:"max_tokens"`
}

func oaText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []map[string]interface{}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if t, ok := p["text"].(string); ok {
				b.WriteString(t)
			}
		}
		return b.String()
	}
	return ""
}

// toChatTurns ubah pesan OpenAI jadi ChatTurn — hanya user/assistant/system;
// tool/function messages dilewati (upstream web cuma paham teks).
func toChatTurns(msgs []oaMessage) []upstream.ChatTurn {
	turns := make([]upstream.ChatTurn, 0, len(msgs))
	for _, m := range msgs {
		role := m.Role
		if role != "user" && role != "assistant" && role != "system" {
			continue
		}
		if role == "system" {
			role = "user"
		}
		turns = append(turns, upstream.ChatTurn{Role: role, Content: oaText(m.Content)})
	}
	// upstream butuh pesan terakhir dari user
	if len(turns) > 0 && turns[len(turns)-1].Role != "user" {
		turns = append(turns, upstream.ChatTurn{Role: "user", Content: "Lanjutkan."})
	}
	return turns
}

// ---- SSE writer (format OpenAI) ----

type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	model   string
	id      string
	created int64
}

func newSSE(w http.ResponseWriter, model string) *sseWriter {
	fl, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	sw := &sseWriter{w: w, flusher: fl, model: model, created: time.Now().Unix()}
	core.RandomHex(8)
	sw.id = "chatcmpl-" + core.RandomHex(12)
	return sw
}

func (s *sseWriter) send(chunk map[string]interface{}) {
	b, _ := json.Marshal(chunk)
	s.w.Write([]byte("data: "))
	s.w.Write(b)
	s.w.Write([]byte("\n\n"))
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

func (s *sseWriter) delta(text string) {
	s.send(map[string]interface{}{
		"id": s.id, "object": "chat.completion.chunk", "created": s.created, "model": s.model,
		"choices": []map[string]interface{}{
			{"index": 0, "delta": map[string]string{"content": text}, "finish_reason": nil},
		},
	})
}

func (s *sseWriter) finish(reason string) {
	s.send(map[string]interface{}{
		"id": s.id, "object": "chat.completion.chunk", "created": s.created, "model": s.model,
		"choices": []map[string]interface{}{
			{"index": 0, "delta": map[string]string{}, "finish_reason": reason},
		},
	})
	s.w.Write([]byte("data: [DONE]\n\n"))
	if s.flusher != nil {
		s.flusher.Flush()
	}
}

// completeResp balikan non-stream format OpenAI.
func completeResp(w http.ResponseWriter, model, text, convID string, uIn, uOut int) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": "chatcmpl-" + core.RandomHex(12), "object": "chat.completion", "created": time.Now().Unix(), "model": model,
		"choices": []map[string]interface{}{
			{"index": 0, "message": map[string]string{"role": "assistant", "content": text}, "finish_reason": "stop"},
		},
		"usage":              map[string]int{"prompt_tokens": uIn, "completion_tokens": uOut, "total_tokens": uIn + uOut},
		"system_fingerprint": convID,
	})
}

func apiErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]string{"message": msg, "type": code, "code": code}})
}
