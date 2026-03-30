package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

func New(env string) zerolog.Logger {
	level := zerolog.InfoLevel
	if strings.EqualFold(env, "development") {
		level = zerolog.DebugLevel
	}

	zerolog.SetGlobalLevel(level)
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
