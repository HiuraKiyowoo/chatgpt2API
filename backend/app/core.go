package app

import (
	"chatgpt2api-go/core"
	"chatgpt2api-go/upstream"
)

// Core = wadah dependensi global (config, db, log, upstream client).
type Core struct {
	Config    *core.Config
	DB        *core.DB
	Logs      *core.LogBuffer
	Upstream  *upstream.Client
	StartedAt int64
}

func New(cfg *core.Config, db *core.DB) *Core {
	return &Core{
		Config:    cfg,
		DB:        db,
		Logs:      core.NewLogBuffer(db),
		Upstream:  upstream.NewClient(""),
		StartedAt: core.Now(),
	}
}
