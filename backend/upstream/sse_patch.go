package upstream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ---- parser SSE format JSON-Patch (jalur aplikasi Android /backend-api) ----
//
// Baris "data:" yang penting-penting:
//
//	"v1"                                                 -> header encoding, skip
//	{"p":"/message/content/parts/0","o":"append","v":"P"} -> delta teks
//	{"v":"ONG"}                                           -> append implisit (tanpa o)
//	{"p":"","o":"patch","v":[{...},{...}]}                -> batch patch
//	{"o":"add","v":{"message":{...}}}                     -> pesan baru (id/conv/usage)
//	{"type":"message_stream_complete",...}                -> final
//	{"detail":"..."} + status error di HTTP layer         -> tak ditangani di sini

type ssePatchEvent struct {
	Path           string          `json:"p"`
	Op             string          `json:"o"`
	Value          json.RawMessage `json:"v"`
	Type           string          `json:"type"`
	Detail         string          `json:"detail"`
	ConversationID string          `json:"conversation_id"`
	MessageID      string          `json:"message_id"`
}

type patchAddMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	ModelSlug      string `json:"model_slug"`
	Status         string `json:"status"`
	Author         struct {
		Role string `json:"role"`
	} `json:"author"`
	Content struct {
		Parts []string `json:"parts"`
	} `json:"content"`
	Metadata struct {
		FinishDetails *struct {
			Type string `json:"type"`
		} `json:"finish_details"`
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	} `json:"metadata"`
}

func rawStr(v json.RawMessage) (string, bool) {
	if len(v) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return "", false
	}
	return s, true
}

func rawArr(v json.RawMessage) []json.RawMessage {
	if len(v) == 0 || v[0] != '[' {
		return nil
	}
	var arr []json.RawMessage
	if json.Unmarshal(v, &arr) != nil {
		return nil
	}
	return arr
}

// consumeSSEPatch parse stream aplikasi Android. Mirip consumeSSE (format web)
// tapi event berbasis JSON-Patch.
func consumeSSEPatch(body io.Reader, ch chan<- Part) {
	defer close(ch)
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var convID, messID, model, finish string
	var usageIn, usageOut int
	done := false
	scrub := &entityScrubber{}

	var handle func(raw json.RawMessage)
	handle = func(raw json.RawMessage) {
		if len(raw) == 0 {
			return
		}
		// string telanjang ("v1" dsb.) -> skip
		if raw[0] == '"' {
			return
		}
		var ev ssePatchEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		switch {
		case ev.Type == "message_stream_complete":
			if ev.ConversationID != "" {
				convID = ev.ConversationID
			}
			done = true
		case ev.Type != "":
			// meta: title_generation, message_marker, server_ste_metadata,
			// conversation_detail_metadata, resume_conversation_token, ...
			if ev.Type == "stream_quota_check" && ev.Detail != "" {
				ch <- Part{Err: fmt.Errorf("upstream quota: %s", ev.Detail)}
				done = true
			}
		case ev.Op == "patch" || (ev.Op == "" && ev.Path == "" && len(rawArr(ev.Value)) > 0):
			for _, sub := range rawArr(ev.Value) {
				handle(sub)
			}
		case ev.Op == "append" || (ev.Op == "" && len(ev.Path) == 0 && ev.Value != nil && ev.Value[0] == '"'):
			// delta teks; path biasanya /message/content/parts/0, kadang implisit
			if txt, ok := rawStr(ev.Value); ok && txt != "" {
				if clean := scrub.feed(txt); clean != "" {
					ch <- Part{Text: clean}
				}
			}
		case ev.Op == "add":
			var wrap struct {
				Message *patchAddMessage `json:"message"`
			}
			if json.Unmarshal(ev.Value, &wrap) == nil && wrap.Message != nil {
				m := wrap.Message
				if m.ConversationID != "" {
					convID = m.ConversationID
				}
				if m.Status == "finished_successfully" && m.Author.Role == "assistant" {
					messID = m.ID
					model = m.ModelSlug
					if m.Metadata.FinishDetails != nil {
						finish = m.Metadata.FinishDetails.Type
					}
					if m.Metadata.Usage != nil {
						usageIn = m.Metadata.Usage.PromptTokens
						usageOut = m.Metadata.Usage.CompletionTokens
					}
				}
			}
		}
	}

	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(line[6:])
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			done = true
			break
		}
		handle(json.RawMessage(payload))
		if done {
			break
		}
	}
	if err := sc.Err(); err != nil && !done {
		ch <- Part{Err: fmt.Errorf("baca SSE patch: %w", err)}
		return
	}
	ch <- Part{Done: true, ConvID: convID, MessID: messID, Model: model,
		Finish: finish, UsageIn: usageIn, UsageOut: usageOut}
}
