package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"chatgpt2api-go/upstream"
)

func main() {
	raw, _ := os.ReadFile("/tmp/ck_ready.txt")
	ck := strings.TrimSpace(string(raw))
	ua := "ChatGPT/1.2026.181 (Android 16; Neo/1.0; build 2222222)"
	// mint JWT
	req, _ := http.NewRequest("GET", "https://chatgpt.com/api/auth/session", nil)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("OAI-Package-Name", "com.openai.chatgpt")
	req.Header.Set("OAI-Client-Type", "android")
	req.Header.Set("Cookie", ck)
	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		fmt.Println("session ERR", err)
		return
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	var d struct {
		AccessToken string `json:"accessToken"`
	}
	json.Unmarshal(b, &d)
	fmt.Println("JWT didapat:", len(d.AccessToken), "char")
	// coba chat pakai token itu (jalur android, sama kayak gateway)
	did := ""
	for _, kv := range strings.Split(ck, ";") {
		kv = strings.TrimSpace(kv)
		if strings.HasPrefix(kv, "oai-did=") {
			did = strings.TrimPrefix(kv, "oai-did=")
		}
	}
	cli := upstream.NewClient("")
	ch, err := cli.StreamAndroid(upstream.Credential{AccessToken: d.AccessToken, Cookies: ck, OAIDeviceID: did},
		"gpt-5", []upstream.ChatTurn{{Role: "user", Content: "Balas hanya: SEHAT"}}, "", upstream.ChatFeatures{})
	if err != nil {
		fmt.Println("CHAT GAGAL:", err)
		return
	}
	var got string
	t := time.After(90 * time.Second)
loop:
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				break loop
			}
			if p.Err != nil {
				fmt.Println("PART ERR:", p.Err)
				return
			}
			if p.Done {
				break loop
			}
			got += p.Text
		case <-t:
			break loop
		}
	}
	fmt.Printf("REPLY: %q\n", got[:min(120, len(got))])
	if strings.TrimSpace(got) != "" {
		fmt.Println("=> AKUN SEHAT, cookie ini layak dipakai")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
