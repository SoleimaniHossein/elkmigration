package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var Log *zap.Logger

// ANSI escape codes for colors
var levelColors = map[zapcore.Level]string{
	zapcore.DebugLevel:  "\033[36m", // Cyan
	zapcore.InfoLevel:   "\033[32m", // Green
	zapcore.WarnLevel:   "\033[33m", // Yellow
	zapcore.ErrorLevel:  "\033[31m", // Red
	zapcore.DPanicLevel: "\033[35m", // Magenta
	zapcore.PanicLevel:  "\033[35m", // Magenta
	zapcore.FatalLevel:  "\033[31m", // Red
}

const resetColor = "\033[0m"

// colorizeLevel returns the log level string wrapped in color codes
func colorizeLevel(level zapcore.Level) string {
	color, ok := levelColors[level]
	if !ok {
		color = resetColor
	}
	return color + level.CapitalString() + resetColor
}

// InitLogger initializes a Zap logger with both console and file output
func InitLogger(logFilePath string) {
	// Customize the encoder config for the console
	consoleEncoderConfig := zap.NewProductionEncoderConfig()
	consoleEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // Format timestamps
	consoleEncoderConfig.EncodeLevel = zapcore.LevelEncoder(func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(colorizeLevel(level))
	})

	// Encoder config for the file (no color)
	fileEncoderConfig := zap.NewProductionEncoderConfig()
	fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// File logging core
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic("Failed to open log file: " + err.Error())
	}
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(fileEncoderConfig), // JSON format for the file
		zapcore.AddSync(logFile),
		zapcore.DebugLevel, // Minimum log level
	)

	// Console logging core
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(consoleEncoderConfig),
		zapcore.Lock(os.Stdout),
		zapcore.DebugLevel,
	)

	// Combine cores (console + file)
	core := zapcore.NewTee(consoleCore, fileCore)

	// Initialize logger with combined core
	Log = zap.New(core, zap.AddCaller())
}

// Info logs an informational message
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}
