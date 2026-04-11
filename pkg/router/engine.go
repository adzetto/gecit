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
	return &Engine{cfg: cfg.Normalized()}
}

// Start reserves the lifecycle shape that will later satisfy the engine.Engine contract.
func (e *Engine) Start(ctx context.Context) error {
	_ = ctx
	if err := e.cfg.Validate(); err != nil {
		return err
	}
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

// Validate checks whether the current router-mode config is renderable.
func (e *Engine) Validate() error {
	return e.cfg.Validate()
}

// RuleSet returns the current nftables dry-run output for this engine.
func (e *Engine) RuleSet() (RuleSet, error) {
	return BuildRuleSet(e.cfg)
}
