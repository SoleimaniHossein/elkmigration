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

func InitLogger() {
	// Customize the encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // Format timestamps
	encoderConfig.EncodeLevel = zapcore.LevelEncoder(func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(colorizeLevel(level))
	})

	// Create a new core with the custom encoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.Lock(os.Stdout), // Output to standard out
		zapcore.DebugLevel,      // Minimum log level to display
	)

	// Initialize logger with the core
	Log = zap.New(core)
}

func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}
