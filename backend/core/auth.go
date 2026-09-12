package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ---- admin session token (HMAC, tanpa dependensi JWT) ----

func adminMAC(secret, payload string) []byte {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// AdminToken buat token "<user>:<expiry>:<hex>" — kedaluwarsa 7 hari.
func AdminToken(secret, username string) string {
	if secret == "" {
		secret = "insecure-dev-secret"
	}
	exp := time.Now().Add(7 * 24 * time.Hour).Unix()
	payload := fmt.Sprintf("%s:%d", username, exp)
	return payload + ":" + hex.EncodeToString(adminMAC(secret, payload))
}

// VerifyAdminToken cek format + MAC + kedaluwarsa.
func VerifyAdminToken(secret, token string) bool {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + ":" + parts[1]
	if secret == "" {
		secret = "insecure-dev-secret"
	}
	want := hex.EncodeToString(adminMAC(secret, payload))
	if hmac.Equal([]byte(want), []byte(parts[2])) == false {
		return false
	}
	var exp int64
	if _, err := fmt.Sscanf(parts[1], "%d", &exp); err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

// ---- API key ----

// HashAPIKey sha256 hex dari key mentah (disimpan di api_keys.key_hash).
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// NewAPIKey generate "sk-..." baru.
func NewAPIKey() string {
	return "sk-" + RandomHex(24)
}
