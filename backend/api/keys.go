package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"chatgpt2api-go/core"
)

// KeysList daftar API key (preview doang, hash gak dibalikin).
func (h *Handler) KeysList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.App.DB.Query(`SELECT id, name, key_preview, enabled, created_at, last_used_at FROM api_keys ORDER BY created_at ASC`)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type keyRow struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		KeyPreview string `json:"keyPreview"`
		Enabled    bool   `json:"enabled"`
		CreatedAt  int64  `json:"createdAt"`
		LastUsedAt int64  `json:"lastUsedAt,omitempty"`
	}
	list := make([]keyRow, 0)
	for rows.Next() {
		var k keyRow
		var enabled int
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPreview, &enabled, &k.CreatedAt, &k.LastUsedAt); err != nil {
			continue
		}
		k.Enabled = enabled == 1
		list = append(list, k)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"keys": list})
}

// KeysAdd bikin API key baru; balikin plaintext SEKALI.
func (h *Handler) KeysAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	key := core.NewAPIKey()
	id := "key_" + core.RandomHex(8)
	preview := core.MaskCredential(key)
	if _, err := h.App.DB.Exec(`INSERT INTO api_keys (id, name, key_hash, key_preview, enabled, created_at)
		VALUES (?,?,?,?,1,?)`, id, req.Name, core.HashAPIKey(key), preview, core.Now()); err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	h.App.Logs.Add("info", "admin", "API key dibuat: "+req.Name, preview)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "key": key})
}

// KeysDelete hapus API key.
func (h *Handler) KeysDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/keys/")
	if id == "" {
		jsonErr(w, 400, "id kosong")
		return
	}
	res, err := h.App.DB.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonErr(w, 404, "key tidak ditemukan")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// LogsList activity log untuk dashboard.
func (h *Handler) LogsList(w http.ResponseWriter, r *http.Request) {
	limit := 100
	list := h.App.Logs.List(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"logs": list})
}

// SystemInfo info dasar buat dashboard.
func (h *Handler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	var accTotal, accValid, keyTotal int
	h.App.DB.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&accTotal)
	h.App.DB.QueryRow(`SELECT COUNT(*) FROM accounts WHERE status='valid'`).Scan(&accValid)
	h.App.DB.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE enabled=1`).Scan(&keyTotal)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":   core.Version,
		"startedAt": h.App.StartedAt,
		"uptimeSec": core.Now() - h.App.StartedAt,
		"accounts":  map[string]int{"total": accTotal, "valid": accValid},
		"apiKeys":   keyTotal,
		"models":    core.ModelAliases,
		"modelVariants": func() []string {
			// varian fitur (suffix) buat UI Playground/Dashboard
			var v []string
			for _, m := range core.ModelAliases {
				v = append(v, m+"-web", m+"-thinking", m+"-research")
			}
			return v
		}(),
		"solver": map[string]interface{}{"enabled": h.App.Config.Solver.Enabled, "url": h.App.Config.Solver.URL},
	})
}
