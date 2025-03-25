package log

import (
	"context"
	"os"
	"strings"
	"time"

	"vpsub/pkg/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxLoggerKey struct{}

type Logger struct {
	*zap.Logger
}

func (l *Logger) WithValue(ctx context.Context, fields ...zapcore.Field) context.Context {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.Request.Context()
		c.Request = c.Request.WithContext(context.WithValue(ctx, ctxLoggerKey{}, l.WithContext(ctx).With(fields...)))
		return c
	}

	return context.WithValue(ctx, ctxLoggerKey{}, l.WithContext(ctx).With(fields...))
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.Request.Context()
	}

	ctxLogger, ok := ctx.Value(ctxLoggerKey{}).(*zap.Logger)
	if ok {
		return &Logger{ctxLogger}
	}

	return l
}

func NewLog(conf config.LogConfig) *Logger {
	return newZap(conf)
}

func newZap(conf config.LogConfig) *Logger {
	// 日志级别
	lv := strings.ToLower(conf.Level)

	var level zapcore.Level
	switch lv {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	default:
		level = zap.InfoLevel
	}

	encoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "function",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.DateTime),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder,
	})

	core := zapcore.NewCore(
		encoder,                    // 编码器配置
		zapcore.AddSync(os.Stdout), // 打印到控制台和文件
		level,                      // 日志级别
	)

	return &Logger{zap.New(core, zap.AddCaller())}
}
