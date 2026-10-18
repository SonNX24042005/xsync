package webui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Run serves the web interface using options selected in the TUI.
func Run(port int, openBrowser bool) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid web port: %d", port)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return Serve(ctx, port, openBrowser)
}
