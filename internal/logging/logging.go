// Package logging 构造全站唯一的 slog.Logger，并负责请求域 logger 在 context 上的传递。
// 输出端为 io.MultiWriter(stderr, lumberjack 轮转文件)（ADR-0008）：
// 终端能立刻看见，历史留存在 <log-dir>/xgulu.log。--log-dir 为空则不落盘。
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/xgulu/internal/config"
)

// 轮转参数（ADR-0008：按大小/时间轮转，不做按级别分流）。
const (
	logFileName = "xgulu.log"
	maxSizeMB   = 10
	maxAgeDays  = 30
	maxBackups  = 10
)

// New 按配置构造 logger。第二个返回值负责关闭轮转文件（不落盘时为 no-op），
// 由 App.Close() 逆序释放时调用。
func New(cfg config.Config) (*slog.Logger, io.Closer, error) {
	sink := io.Writer(os.Stderr)
	closer := io.Closer(nopCloser{})

	if cfg.LogDir != "" {
		// 自己建目录以拿到 0700：交给 lumberjack 建会是 0755（与数据库同级的隐私边界）。
		if err := os.MkdirAll(cfg.LogDir, 0o700); err != nil {
			return nil, nil, fmt.Errorf("创建日志目录 %s: %w", cfg.LogDir, err)
		}
		rotator := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.LogDir, logFileName),
			MaxSize:    maxSizeMB,
			MaxAge:     maxAgeDays,
			MaxBackups: maxBackups,
			Compress:   true,
			LocalTime:  true,
		}
		// 同一个 handler + MultiWriter：slog 写每条记录时串行化，两个 sink 不会交错。
		sink = io.MultiWriter(os.Stderr, rotator)
		closer = rotator
	}

	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}
	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(sink, opts)
	} else {
		handler = slog.NewJSONHandler(sink, opts)
	}
	return slog.New(handler), closer, nil
}

// FromContext 取出请求域 logger；ctx 中没有时回退 slog.Default()。
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// WithLogger 把 logger 注入 ctx（中间件在请求入口调用一次）。
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// WithRequestID 把请求 id 注入 ctx。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext 取出请求 id，没有则返回空串。
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

type (
	loggerKey    struct{}
	requestIDKey struct{}
)

func parseLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
