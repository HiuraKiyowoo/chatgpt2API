package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---- activity log buffer (admin UI) ----

type LogEntry struct {
	ID      int64  `json:"id"`
	TS      int64  `json:"ts"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type LogBuffer struct {
	mu      sync.Mutex
	entries []LogEntry
	nextID  int64
	db      *DB
}

func NewLogBuffer(db *DB) *LogBuffer {
	return &LogBuffer{db: db}
}

func (l *LogBuffer) Add(level, source, message, detail string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextID++
	e := LogEntry{ID: l.nextID, TS: Now(), Level: level, Source: source, Message: message, Detail: detail}
	l.entries = append(l.entries, e)
	if len(l.entries) > 500 {
		l.entries = l.entries[len(l.entries)-500:]
	}
	if l.db != nil {
		_, _ = l.db.Exec(`INSERT INTO logs (ts, level, source, message, detail) VALUES (?,?,?,?,?)`,
			e.TS, e.Level, e.Source, e.Message, e.Detail)
	}
}

func (l *LogBuffer) List(limit int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}
	out := make([]LogEntry, len(l.entries)-limit)
	_ = out
	res := make([]LogEntry, limit)
	copy(res, l.entries[len(l.entries)-limit:])
	// newest first
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}
	return res
}

// ---- standard library logger with simple levels ----

var levelOrder = map[string]int{"DEBUG": 0, "INFO": 1, "WARN": 2, "ERROR": 3}

type Logger struct {
	min   int
	mu    sync.Mutex
	buf   *LogBuffer
	plain *log.Logger
}

func NewLogger(level string, buf *LogBuffer) *Logger {
	min, ok := levelOrder[strings.ToUpper(level)]
	if !ok {
		min = 1
	}
	return &Logger{min: min, buf: buf, plain: log.New(osStdout, "", log.LstdFlags)}
}

func (lg *Logger) log(level, source, format string, args ...any) {
	if levelOrder[level] < lg.min {
		return
	}
	msg := fmt.Sprintf(format, args...)
	lg.plain.Printf("[%s] [%s] %s", level, source, msg)
	if lg.buf != nil {
		lg.buf.Add(strings.ToLower(level), source, msg, "")
	}
}

func (lg *Logger) Debug(source, format string, args ...any) { lg.log("DEBUG", source, format, args...) }
func (lg *Logger) Info(source, format string, args ...any)  { lg.log("INFO", source, format, args...) }
func (lg *Logger) Warn(source, format string, args ...any)  { lg.log("WARN", source, format, args...) }
func (lg *Logger) Error(source, format string, args ...any) { lg.log("ERROR", source, format, args...) }

// ---- JWT (HS256, manual, no external dep) ----

func HS256Sign(secret string, claims map[string]any, ttl time.Duration) (string, error) {
	header := base64URL([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body := fmt.Sprintf(`{"exp":%d}`, time.Now().Add(ttl).Unix())
	for k, v := range claims {
		if k == "exp" {
			continue
		}
		switch vv := v.(type) {
		case string:
			body = body[:len(body)-1] + fmt.Sprintf(`,"%s":"%s"}`, k, vv)
		case float64:
			body = body[:len(body)-1] + fmt.Sprintf(`,"%s":%d}`, k, int64(vv))
		}
	}
	payload := base64URL([]byte(body))
	signing := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	sig := base64URL(mac.Sum(nil))
	return signing + "." + sig, nil
}

func HS256Verify(secret, token string) (bool, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, fmt.Errorf("format token tidak valid")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64URL(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return false, fmt.Errorf("signature token tidak cocok")
	}
	payload, err := b64Decode(parts[1])
	if err != nil {
		return false, err
	}
	if strings.Contains(string(payload), `"exp":`) {
		// crude but sufficient: extract exp integer
		idx := strings.Index(string(payload), `"exp":`)
		rest := string(payload)[idx+7:]
		end := strings.IndexAny(rest, ",}")
		if end > 0 {
			var exp int64
			if _, err := fmt.Sscanf(rest[:end], "%d", &exp); err == nil && exp > 0 && Now() > exp {
				return false, fmt.Errorf("token kedaluwarsa")
			}
		}
	}
	return true, nil
}

// ---- HTTP helpers ----

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := jsonNewEncoder(w)
	_ = enc.Encode(v)
}

func HTTPError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]any{"error": map[string]any{"message": msg, "type": "gateway_error", "code": status}})
}

// ClientIP extracts the remote address without proxy trust.
func ClientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

var _ = hex.EncodeToString
