package app

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Logger returns the logger singleton
var Logger = sync.OnceValue(func() *slog.Logger {
	baseHandler := slog.NewJSONHandler(os.Stdout, nil)
	handler := &loggerHandler{handler: baseHandler}

	return slog.New(handler)
})

type loggerHandler struct {
	handler slog.Handler
}

func (h *loggerHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *loggerHandler) Handle(ctx context.Context, r slog.Record) error {
	// Convert the time to UTC and truncate microseconds
	r.Time = r.Time.UTC().Truncate(time.Second)
	return h.handler.Handle(ctx, r)
}

func (h *loggerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &loggerHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *loggerHandler) WithGroup(name string) slog.Handler {
	return &loggerHandler{handler: h.handler.WithGroup(name)}
}
