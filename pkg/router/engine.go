package router

import (
	"context"
	"errors"
)

// ErrNotImplemented makes the scaffold explicit until router mode is wired up.
var ErrNotImplemented = errors.New("router mode scaffold is not implemented yet")

// Engine is a placeholder for a future NFQUEUE-backed router-wide DPI bypass mode.
type Engine struct {
	cfg Config
}

// New returns a placeholder engine so future CLI wiring can share one constructor.
func New(cfg Config) *Engine {
	return &Engine{cfg: cfg}
}

// Start reserves the lifecycle shape that will later satisfy the engine.Engine contract.
func (e *Engine) Start(ctx context.Context) error {
	_ = ctx
	return ErrNotImplemented
}

// Stop is currently a no-op because the scaffold installs no runtime state yet.
func (e *Engine) Stop() error {
	return nil
}

// Mode returns the intended name for the future router-wide backend.
func (e *Engine) Mode() string {
	return "router-nfqueue"
}

// Config exposes the stored scaffold configuration for tests and future wiring.
func (e *Engine) Config() Config {
	return e.cfg
}
