package rootcmd

import (
	"context"
	"sync"

	"github.com/bombsimon/logrusr/v2"

	"kong-portal-controller/internal/diagnostics"
	"kong-portal-controller/internal/manager"
	"kong-portal-controller/internal/util"
)

const (
	// DiagnosticConfigBufferDepth is the size of the channel buffer for receiving diagnostic
	// config dumps from the proxy sync loop. The chosen size is essentially arbitrary: we don't
	// expect that the receive end will get backlogged (it only assigns the value to a local
	// variable) but do want a small amount of leeway to account for goroutine scheduling, so it
	// is not zero.
	DiagnosticConfigBufferDepth = 3
)

func StartDiagnosticsServer(ctx context.Context, port int, c *manager.Config) (diagnostics.Server, error) {
	customizedLogger, err := util.MakeLogger(c.LogLevel, c.LogFormat)
	if err != nil {
		return diagnostics.Server{}, err
	}
	logger := logrusr.New(customizedLogger)

	if !c.EnableProfiling {
		logger.Info("Diagnostics server disabled")
		return diagnostics.Server{}, nil
	}

	s := diagnostics.Server{
		Logger:           logger,
		ProfilingEnabled: c.EnableProfiling,
		ConfigLock:       &sync.RWMutex{},
	}

	go func() {
		if err := s.Listen(ctx, port); err != nil {
			logger.Error(err, "unable to start diagnostics server")
		}
	}()
	return s, nil
}
