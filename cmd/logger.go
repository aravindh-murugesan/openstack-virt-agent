package cmd

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Automatically extract contextual attributes if present in ctx
	if host, ok := ctx.Value("libvirt_host").(string); ok && host != "" {
		r.AddAttrs(slog.String("libvirt_host", host))
	}
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	if globalReqID, ok := ctx.Value("global_request_id").(string); ok && globalReqID != "" {
		r.AddAttrs(slog.String("global_request_id", globalReqID))
	}
	if handler, ok := ctx.Value("controller_handler").(string); ok && handler != "" {
		r.AddAttrs(slog.String("controller_handler", handler))
	}
	return h.Handler.Handle(ctx, r)
}

func setupLogger() {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(Config.LogLevel)); err != nil {
		lvl = slog.LevelInfo
	}

	var baseHandler slog.Handler
	if Config.Environment == "production" {
		baseHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	} else {
		baseHandler = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      lvl,
			TimeFormat: time.RFC3339,
		})
	}

	// Wrap with ContextHandler so all slog.*Context(ctx, ...) automatically include context attributes
	slog.SetDefault(slog.New(&ContextHandler{Handler: baseHandler}))
}
