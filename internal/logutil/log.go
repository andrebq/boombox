package logutil

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	key byte
)

var (
	loggerKey = key(1)
)

func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func GetOrDefault(ctx context.Context) zerolog.Logger {
	v := ctx.Value(loggerKey)
	if v == nil {
		return log.Logger
	}
	return v.(zerolog.Logger)
}
