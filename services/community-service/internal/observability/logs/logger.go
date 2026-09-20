package logs

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:          "msg",
		LevelKey:            "level",
		TimeKey:             "time",
		NameKey:             "logger",
		CallerKey:           "caller",
		FunctionKey:         zapcore.OmitKey,
		StacktraceKey:       "stacktrace",
		SkipLineEnding:      false,
		LineEnding:          zapcore.DefaultLineEnding,
		EncodeLevel:         zapcore.CapitalColorLevelEncoder,
		EncodeTime:          zapcore.ISO8601TimeEncoder,
		EncodeDuration:      zapcore.StringDurationEncoder,
		EncodeCaller:        zapcore.ShortCallerEncoder,
		EncodeName:          zapcore.FullNameEncoder,
		NewReflectedEncoder: nil,
		ConsoleSeparator:    "\t",
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zap.InfoLevel)

	options := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Development(),
	}

	logger := zap.New(core, options...)
	return logger
}
