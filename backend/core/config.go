package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const Version = "1.0.0"

// ModelAlias maps a public model name to the single ChatGPT-web upstream.
// Semua alias di-route ke upstream yang sama (chat mode default).
var ModelAliases = []string{"gpt-5", "gpt-5-mini", "gpt-4o", "gpt-4o-mini", "o4-mini"}

type SolverConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

type Config struct {
	Server struct {
		Port         int    `yaml:"port"`
		FrontendPath string `yaml:"frontendPath"`
	} `yaml:"server"`
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	Security struct {
		JWTSecret             string `yaml:"jwtSecret"`
		CredentialEncryptionKey string `yaml:"credentialEncryptionKey"`
	} `yaml:"security"`
	Admin struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
	Solver SolverConfig `yaml:"solver"`
}

func DefaultConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("CONFIG_PATH")); v != "" {
		return v
	}
	return "config.yaml"
}

// LoadConfig reads config.yaml (env CONFIG_PATH overrides), applying safe defaults.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	path := DefaultConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("baca config %s: %w", path, err)
		}
		// No config file: fall back to env-only mode (docker).
		applyEnvOverrides(cfg)
		return cfg, nil
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8800
	}
	if cfg.Server.FrontendPath == "" {
		cfg.Server.FrontendPath = "./frontend/dist"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/chatgpt2api.db"
	}
	if cfg.Solver.URL == "" {
		cfg.Solver.URL = "http://127.0.0.1:7900"
	}
	if cfg.Admin.Username == "" {
		cfg.Admin.Username = "admin"
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("PORT"); v != "" {
		if n := parseIntEnv(v); n > 0 {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Security.JWTSecret = v
	}
	if v := os.Getenv("CREDENTIAL_ENCRYPTION_KEY"); v != "" {
		cfg.Security.CredentialEncryptionKey = v
	}
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		cfg.Admin.Password = v
	}
	if v := os.Getenv("SOLVER_URL"); v != "" {
		cfg.Solver.URL = v
	}
	if v := strings.TrimSpace(os.Getenv("SOLVER_ENABLED")); v != "" {
		cfg.Solver.Enabled = v == "true" || v == "1" || v == "yes"
	}
}

func parseIntEnv(v string) int {
	n := 0
	for _, c := range strings.TrimSpace(v) {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ResolvePath makes relative paths absolute against the binary working dir.
func ResolvePath(base string, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
