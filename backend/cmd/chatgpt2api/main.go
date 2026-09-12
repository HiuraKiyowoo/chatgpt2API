package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"chatgpt2api-go/api"
	"chatgpt2api-go/app"
	"chatgpt2api-go/core"
)

func main() {
	flag.String("config", core.DefaultConfigPath(), "path config.yaml")
	flag.Parse()

	cfg, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	// path relatif dianter relatif ke direktori binary
	if !filepath.IsAbs(cfg.Database.Path) {
		cfg.Database.Path = filepath.Join(".", cfg.Database.Path)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir data:", err)
		os.Exit(1)
	}
	db, err := core.OpenDB(cfg.Database.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		os.Exit(1)
	}
	a := app.New(cfg, db)
	h := api.NewHandler(a)

	mux := http.NewServeMux()

	// ---- publik ----
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "version": core.Version})
	})
	mux.HandleFunc("/api/admin/login", h.AdminLogin)

	// ---- API kompatibel OpenAI (butuh API key sk-...) ----
	mux.HandleFunc("/v1/models", h.Models) // listing terbuka
	mux.Handle("/v1/chat/completions", h.AuthAPIKey(http.HandlerFunc(h.V1Chat)))

	// ---- admin (butuh token) ----
	mux.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.AuthAdmin(h.AccountsList)(w, r)
		case http.MethodPost:
			h.AuthAdmin(h.AccountsAdd)(w, r)
		default:
			jsonErrOut(w, 405, "method tidak didukung")
		}
	})
	mux.HandleFunc("/api/accounts/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/check") {
			h.AuthAdmin(h.AccountCheck)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/auto-login") {
			h.AuthAdmin(h.AutoLogin)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/oauth-harvest") {
			h.AuthAdmin(h.OAuthHarvest)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/oauth-refresh") {
			h.AuthAdmin(h.OAuthRefresh)(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/import-tokens") {
			h.AuthAdmin(h.OAuthImport)(w, r)
			return
		}
		h.AuthAdmin(h.AccountsDelete)(w, r)
	})
	mux.Handle("/api/keys", h.AuthAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.KeysList(w, r)
		case http.MethodPost:
			h.KeysAdd(w, r)
		default:
			jsonErrOut(w, 405, "method tidak didukung")
		}
	})))
	mux.HandleFunc("/api/keys/", h.AuthAdmin(h.KeysDelete))
	mux.Handle("/api/logs", h.AuthAdmin(http.HandlerFunc(h.LogsList)))
	mux.Handle("/api/system", h.AuthAdmin(http.HandlerFunc(h.SystemInfo)))

	// ---- frontend statis ----
	frontendPath := cfg.Server.FrontendPath
	if !filepath.IsAbs(frontendPath) {
		frontendPath = filepath.Join(".", frontendPath)
	}
	if st, err := os.Stat(frontendPath); err == nil && st.IsDir() {
		fs := http.FileServer(http.Dir(frontendPath))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			p := filepath.Join(frontendPath, filepath.Clean(r.URL.Path))
			if _, err := os.Stat(p); err != nil {
				// SPA fallback
				http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("chatgpt2API backend v" + core.Version + " — frontend belum di-build (frontend/dist kosong)"))
		})
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	a.Logs.Add("info", "system", "chatgpt2API v"+core.Version+" jalan di "+addr, "")
	fmt.Println("chatgpt2API v" + core.Version + " -> http://0.0.0.0" + addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
}

func jsonErrOut(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
