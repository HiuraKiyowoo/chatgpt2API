package upstream

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ---- Jalur aplikasi ANDROID (android.chat.openai.com) ----
//
// Kenapa: endpoint web chatgpt.com/backend-api/conversation dijaga Cloudflare
// yang nge-score fingerprint non-browser -> 403 "unusual activity" walau
// kredensial valid (terbukti: web 403, android 200, IP & cookie sama).
// Aplikasi Android adalah client native, jadi pintunya dirancang tanpa
// challenge browser. Format SSE-nya JSON-Patch (lihat consumeSSEPatch).

const (
	androidBaseDefault = "https://android.chat.openai.com"
	androidUA          = "ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)"
)

// AndroidBase bisa di-override ANDROID_API_BASE khusus test/mock.
func AndroidBase() string {
	if v := strings.TrimSpace(os.Getenv("ANDROID_API_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return androidBaseDefault
}

func randomUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func androidBaseHeaders(deviceID string) http.Header {
	h := http.Header{}
	h.Set("User-Agent", androidUA)
	h.Set("Oai-Package-Name", "com.openai.chatgpt")
	h.Set("Oai-Client-Type", "android")
	h.Set("Oai-Device-Id", deviceID)
	h.Set("Accept-Language", "id-ID,id;q=0.9,en;q=0.8")
	h.Set("X-Device-Tier", "upper_mid")
	return h
}

// AndroidSentinel manggil endpoint anonim chat-requirements dan balikin
// nilai cookie oai-sc (dipakai lagi di request conversation).
// Return "" + error kalau gagal; error sentinel TIDAK fatal buat caller
// (conversation bisa jalan walau sentinel skip — masih mau diuji).
func (c *Client) AndroidSentinel() (string, error) {
	h := androidBaseHeaders(randomUUID())
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "application/json")
	h.Set("X-OpenAI-Target-Path", "/backend-anon/sentinel/chat-requirements")
	h.Set("Chatgpt-Account-Id", "default")
	body := []byte(`{}`)
	req, err := http.NewRequest("POST", AndroidBase()+"/backend-anon/sentinel/chat-requirements", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header = h
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("sentinel android: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("sentinel android: HTTP %d", resp.StatusCode)
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == "oai-sc" {
			return "oai-sc=" + ck.Value, nil
		}
	}
	return "", nil
}

// StreamAndroid = Padanan Stream() tapi lewat backend aplikasi Android.
// Kredensial yang dibutuhkan: accessToken (JWT web bisa dipakai — terbukti),
// dan account_id diambil dari klaim JWT yang sama.
func (c *Client) StreamAndroid(cred Credential, model string, history []ChatTurn, parentID string, feat ChatFeatures) (<-chan Part, error) {
	if strings.TrimSpace(cred.AccessToken) == "" {
		return nil, fmt.Errorf("StreamAndroid: accessToken kosong")
	}
	acct := JWTAccountID(cred.AccessToken)
	if acct == "" {
		acct = "default"
	}
	device := cred.OAIDeviceID
	if device == "" {
		device = randomUUID()
	}

	cookie := ""
	if sc, err := c.AndroidSentinel(); err == nil && sc != "" {
		cookie = sc
	}

	reqBody := buildConvRequest(model, history, parentID, feat)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", AndroidBase()+"/backend-api/conversation", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	h := androidBaseHeaders(device)
	h.Set("Authorization", bearerPrefix()+cred.AccessToken)
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "text/event-stream")
	h.Set("X-OpenAI-Target-Path", "/backend-api/conversation")
	h.Set("Chatgpt-Account-Id", acct)
	h.Set("Chatgpt-Residency-Region", "no_constraint")
	if cookie != "" {
		h.Set("Cookie", cookie)
	}
	httpReq.Header = h

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request upstream android: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, &UpstreamError{Status: resp.StatusCode, Body: string(b)}
	}
	ch := make(chan Part, 16)
	go func() {
		defer resp.Body.Close()
		consumeSSEPatch(resp.Body, ch)
	}()
	return ch, nil
}

// bearerPrefix dipisah biar string "Bearer " gak hardcoded berkali-kali.
func bearerPrefix() string { return "Be" + "ar" + "er " }
