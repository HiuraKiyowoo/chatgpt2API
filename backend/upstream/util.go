package upstream

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// uuidFrom bikin UUID v4-format deterministik dari (index, konten) — cukup
// buat message id client-side yang upstream butuhkan unik per request.
func uuidFrom(i int, content string) string {
	h := sha256Hex(fmt.Sprintf("chatgpt2api:%d:%s", i, content))
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}
