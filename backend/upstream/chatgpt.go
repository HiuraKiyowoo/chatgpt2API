package upstream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	chatBaseDefault = "https://chatgpt.com"
	defaultUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	clientVers      = "2025-01-13"
)

// Credential = kredensial satu akun (didekripsi dari DB).
type Credential struct {
	AccessToken      string
	Cookies          string // cookie string lengkap, opsional
	CFClearance      string
	UserAgent        string
	OAIClientVersion string
	ProxyURL         string
	OAIDeviceID      string // device-id asli browser (cookie oai-did), dipakai mentah
}

// OAIDidFromCookies ambil nilai cookie oai-did (bukan HttpOnly; ada di string cookie).
func OAIDidFromCookies(cookie string) string {
	for _, kv := range strings.Split(cookie, ";") {
		kv = strings.TrimSpace(kv)
		if strings.HasPrefix(kv, "oai-did=") {
			return strings.TrimSpace(strings.TrimPrefix(kv, "oai-did="))
		}
	}
	return ""
}

// Part = potongan teks stream ke consumer.
type Part struct {
	Text     string
	Done     bool
	Err      error
	ConvID   string
	MessID   string
	Model    string
	Finish   string
	UsageIn  int
	UsageOut int
}

// Client ngirim request ke chatgpt.com/backend-api/conversation.
type Client struct {
	HTTP *http.Client
}

func NewClient(proxyURL string) *Client {
	tr := &http.Transport{ForceAttemptHTTP2: true}
	return &Client{HTTP: &http.Client{Transport: tr, Timeout: 0}}
}

type convRequest struct {
	Action             string        `json:"action"`
	Messages           []convMessage `json:"messages"`
	Model              string        `json:"model"`
	TimezoneOffsetMin  int           `json:"timezone_offset_min"`
	HistoryAndTraining bool          `json:"history_and_training"`
	ConversationMode   convMode      `json:"conversation_mode"`
	ForceParagen       bool          `json:"force_paragen"`
	ForceRateLimit     bool          `json:"force_rate_limit"`
	WebsocketRequestID string        `json:"websocket_request_id"`
	SupportedEncodings []string      `json:"supported_encodings"`
	SystemHints        []string      `json:"system_hints"`
	ConversationID     *string       `json:"conversation_id,omitempty"`
	ParentMessageID    *string       `json:"parent_message_id,omitempty"`
}

type convMode struct {
	Kind string `json:"kind"`
}

type convMessage struct {
	ID       string      `json:"id"`
	Role     string      `json:"role"`
	Content  convContent `json:"content"`
	Metadata metadata    `json:"metadata"`
}

type metadata struct {
	SerializationMetadata serializationMeta `json:"serialization_metadata"`
}

type serializationMeta struct {
	CustomSymbolOffsets interface{} `json:"custom_symbol_offsets"`
}

type convContent struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
}

type eventWrapper struct {
	Message *convEventMessage `json:"message"`
	Error   *convError        `json:"error"`
}
type convEventMessage struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversation_id"`
	ModelSlug      string      `json:"model_slug"`
	Status         string      `json:"status"`
	EndTurn        interface{} `json:"end_turn"`
	Content        struct {
		Parts []json.RawMessage `json:"parts"`
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
type convError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Stream kirim conversation request, balikin channel part streaming.
// ChatBase = asal endpoint backend-api. Default resmi; override lewat
// CHATGPT_API_BASE khusus test (mock upstream), bukan untuk produksi.
func ChatBase() string {
	if v := strings.TrimSpace(os.Getenv("CHATGPT_API_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return chatBaseDefault
}

func (c *Client) Stream(cred Credential, model string, history []ChatTurn, parentID string) (<-chan Part, error) {
	req := buildConvRequest(model, history, parentID)
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", ChatBase()+"/backend-api/conversation", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	ua := cred.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	ver := cred.OAIClientVersion
	if ver == "" {
		ver = clientVers
	}
	httpReq.Header.Set("User-Agent", ua)
	httpReq.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("oai-language", "en-US")
	httpReq.Header.Set("oai-client-version", ver)
	httpReq.Header.Set("oai-device-id", deviceID(cred))
	httpReq.Header.Set("origin", "https://chatgpt.com")
	httpReq.Header.Set("referer", "https://chatgpt.com/")
	cookies := cred.Cookies
	if cred.CFClearance != "" {
		cookies = strings.TrimSpace(cookies + "; cf_clearance=" + cred.CFClearance)
	}
	if cookies != "" {
		httpReq.Header.Set("Cookie", cookies)
	}

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request upstream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, &UpstreamError{Status: resp.StatusCode, Body: string(b)}
	}

	ch := make(chan Part, 16)
	go func() {
		defer resp.Body.Close()
		consumeSSE(resp.Body, ch)
	}()
	return ch, nil
}

// UpstreamError bawa status HTTP + potongan body upstream.
type UpstreamError struct {
	Status int
	Body   string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("upstream %d: %s", e.Status, truncate(e.Body, 300))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func deviceID(cred Credential) string {
	// Device-id asli browser (oai-did) lebih dipercaya Cloudflare daripada hash.
	if cred.OAIDeviceID != "" {
		return cred.OAIDeviceID
	}
	// fallback stabil per akun: hash dari token.
	if cred.AccessToken == "" {
		return ""
	}
	h := sha256Hex(cred.AccessToken)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func buildConvRequest(model string, history []ChatTurn, parentID string) convRequest {
	msgs := make([]convMessage, 0, len(history))
	for i, t := range history {
		m := convMessage{
			ID:   uuidFrom(i, t.Content),
			Role: t.Role,
			Content: convContent{
				ContentType: "text",
				Parts:       []string{t.Content},
			},
		}
		msgs = append(msgs, m)
	}
	var convID *string
	var pid *string
	if parentID != "" && parentID != "root" {
		pid = &parentID
	}
	return convRequest{
		Action:             "next",
		Messages:           msgs,
		Model:              model,
		TimezoneOffsetMin:  420,
		HistoryAndTraining: false,
		ConversationMode:   convMode{Kind: "primary_assistant"},
		ForceParagen:       false,
		ForceRateLimit:     false,
		SupportedEncodings: []string{"v1"},
		SystemHints:        []string{},
		ConversationID:     convID,
		ParentMessageID:    pid,
	}
}

// consumeSSE parse SSE stream dari upstream jadi channel Part.
// body diasumsikan tetap dibuka sampai selesai (ditutup lewat sseCloser).
func consumeSSE(body io.Reader, ch chan<- Part) {
	defer close(ch)
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	var lastConvID, lastMessID, model, finish string
	var usageIn, usageOut int
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if payload == "[DONE]" {
			break
		}
		var ev eventWrapper
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if ev.Error != nil {
			ch <- Part{Err: fmt.Errorf("upstream: %s (%s)", ev.Error.Message, ev.Error.Code)}
			return
		}
		if ev.Message == nil {
			continue
		}
		m := ev.Message
		lastConvID = m.ConversationID
		lastMessID = m.ID
		model = m.ModelSlug
		if m.Metadata.FinishDetails != nil {
			finish = m.Metadata.FinishDetails.Type
		}
		if m.Metadata.Usage != nil {
			usageIn = m.Metadata.Usage.PromptTokens
			usageOut = m.Metadata.Usage.CompletionTokens
		}
		for _, raw := range m.Content.Parts {
			var txt string
			if err := json.Unmarshal(raw, &txt); err == nil {
				if txt != "" {
					ch <- Part{Text: txt}
				}
			}
		}
		if m.Status == "finished_successfully" {
			ch <- Part{Done: true, ConvID: lastConvID, MessID: lastMessID, Model: model, Finish: finish, UsageIn: usageIn, UsageOut: usageOut}
			return
		}
	}
	// stream berakhir tanpa marker final: tetap kirim Done dengan apa pun yang terkumpul
	if err := sc.Err(); err != nil {
		ch <- Part{Err: fmt.Errorf("baca SSE: %w", err)}
		return
	}
	ch <- Part{Done: true, ConvID: lastConvID, MessID: lastMessID, Model: model, Finish: finish, UsageIn: usageIn, UsageOut: usageOut}
}

// ChatTurn = percakapan normalisasi (role user/assistant/system).
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// HealthCheck test kredensial ke endpoint ringan /models.
func (c *Client) HealthCheck(cred Credential) (bool, string, error) {
	req, err := http.NewRequest("GET", ChatBase()+"/backend-api/models?history_and_training=false", nil)
	if err != nil {
		return false, "", err
	}
	ua := cred.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	if cred.Cookies != "" || cred.CFClearance != "" {
		cookies := cred.Cookies
		if cred.CFClearance != "" {
			cookies = strings.TrimSpace(cookies + "; cf_clearance=" + cred.CFClearance)
		}
		req.Header.Set("Cookie", cookies)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode == 200 {
		return true, "ok", nil
	}
	return false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(b), 200)), nil
}

var _ = time.Now
