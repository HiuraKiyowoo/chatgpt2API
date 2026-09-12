package core

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
)

var osStdout = os.Stdout

func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func b64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func jsonNewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
