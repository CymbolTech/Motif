package runtime

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	return ctx, stop
}
