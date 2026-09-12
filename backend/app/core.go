package app

import (
	"chatgpt2api-go/core"
	"chatgpt2api-go/solver"
	"chatgpt2api-go/upstream"
)

// Core = wadah dependensi global (config, db, log, upstream client).
type Core struct {
	Config    *core.Config
	DB        *core.DB
	Logs      *core.LogBuffer
	Upstream  *upstream.Client
	Solver    *solver.Client
	StartedAt int64
}

func New(cfg *core.Config, db *core.DB) *Core {
	return &Core{
		Config:    cfg,
		DB:        db,
		Logs:      core.NewLogBuffer(db),
		Upstream:  upstream.NewClient(""),
		Solver:    solver.NewClient(cfg.Solver.URL),
		StartedAt: core.Now(),
	}
}
