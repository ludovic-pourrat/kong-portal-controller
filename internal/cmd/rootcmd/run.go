package rootcmd

import (
	"context"
	"fmt"

	"kong-portal-controller/internal/manager"
)

func Run(ctx context.Context, c *manager.Config) error {
	_, err := StartDiagnosticsServer(ctx, manager.DiagnosticsPort, c)
	if err != nil {
		return fmt.Errorf("failed to start diagnostics server: %w", err)
	}
	return manager.Run(ctx, c)
}
