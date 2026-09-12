package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

// ---- AES-GCM credential encryption (pattern credentialEncryptionKey) ----

var errEncryptedFormat = errors.New("credential value is not in encrypted format")

func deriveKey(secret string) []byte {
	return pbkdf2.Key([]byte(secret), []byte("chatgpt2api.credential.v1"), 20000, 32, sha256.New)
}

// EncryptCredential returns "enc:v1:<b64 iv>:<b64 ciphertext>".
func EncryptCredential(secret, plaintext string) (string, error) {
	if secret == "" {
		return "", errors.New("credentialEncryptionKey kosong: set dulu di config.yaml")
	}
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, iv, []byte(plaintext), nil)
	return fmt.Sprintf("enc:v1:%s:%s", base64.RawStdEncoding.EncodeToString(iv), base64.RawStdEncoding.EncodeToString(ct)), nil
}

// DecryptCredential reverses EncryptCredential. Plaintext (non "enc:v1:") input
// passes through untouched so legacy/manual rows keep working.
func DecryptCredential(secret, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", nil
	}
	if !strings.HasPrefix(v, "enc:v1:") {
		return v, nil
	}
	if secret == "" {
		return "", errors.New("credentialEncryptionKey kosong: tidak bisa dekripsi kredensial")
	}
	parts := strings.SplitN(strings.TrimPrefix(v, "enc:v1:"), ":", 2)
	if len(parts) != 2 {
		return "", errEncryptedFormat
	}
	iv, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errEncryptedFormat
	}
	ct, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errEncryptedFormat
	}
	block, err := aes.NewCipher(deriveKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	pt, err := gcm.Open(nil, iv, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// MaskCredential shows only the tail of a secret for UI display.
func MaskCredential(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}
	if len(v) <= 10 {
		return strings.Repeat("*", len(v))
	}
	return v[:6] + "..." + v[len(v)-4:]
}

// RandomHex returns n random bytes hex-encoded (for install.sh secrets it is
// duplicated in shell; this is used for API keys).
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}
