package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitZLogger InitLogger initializes the Zerolog logger with a time format.
func InitZLogger() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
}

// ZInfo Info logs an informational message.
func ZInfo(msg string) {
	log.Info().Msg(msg)
}

// ZWarn logs a warning message.
func ZWarn(msg string) {
	log.Warn().Msg(msg)
}

// ZError logs an error message.
func ZError(msg string, err error) {
	log.Error().Err(err).Msg(msg)
}

// ZFatal logs a fatal message and exits the application.
func ZFatal(msg string, err error) {
	log.Fatal().Err(err).Msg(msg)
}
