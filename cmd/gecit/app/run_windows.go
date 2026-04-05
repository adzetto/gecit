package app

import (
	"fmt"

	"github.com/boratanrikulu/gecit/pkg/engine"
	"github.com/sirupsen/logrus"
)

func newPlatformEngine(cfg engine.Config, logger *logrus.Logger) (engine.Engine, error) {
	// TODO: Phase 3 — implement WinDivert engine
	return nil, fmt.Errorf("Windows WinDivert engine not yet implemented (Phase 3)")
}
