package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xgulu/internal/config"
)

func TestNewWithoutLogDirLandsNoFile(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.LogDir = ""

	logger, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New 返回错误: %v", err)
	}
	if logger == nil {
		t.Fatal("New 应返回可用的 logger")
	}
	logger.Info("只应进 stderr")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close 返回错误: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取临时目录失败: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("LogDir 为空时不应落盘，却有 %d 个文件", len(entries))
	}
}

func TestNewWritesJSONToFileWithPrivatePermissions(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "logs")
	cfg := config.Default()
	cfg.LogDir = logDir

	logger, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New 返回错误: %v", err)
	}
	logger.Info("服务已启动", "addr", "127.0.0.1:8737")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close 返回错误: %v", err)
	}

	info, err := os.Stat(logDir)
	if err != nil {
		t.Fatalf("日志目录不存在: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("日志目录权限 = %o，期望 700", perm)
	}

	logPath := filepath.Join(logDir, "xgulu.log")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("日志文件不存在: %v", err)
	}
	if fileInfo, err := os.Stat(logPath); err != nil {
		t.Fatalf("stat 日志文件失败: %v", err)
	} else if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("日志文件权限 = %o，期望 600", perm)
	}

	line := strings.TrimSpace(string(raw))
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("日志不是 JSON 行: %v（原文 %q）", err, line)
	}
	if got["msg"] != "服务已启动" {
		t.Errorf("msg = %v，期望 服务已启动", got["msg"])
	}
	if got["addr"] != "127.0.0.1:8737" {
		t.Errorf("addr = %v，期望 127.0.0.1:8737", got["addr"])
	}
}

func TestNewHonoursLogLevel(t *testing.T) {
	cfg := config.Default()
	cfg.LogDir = t.TempDir()
	cfg.LogLevel = "warn"

	logger, closer, err := New(cfg)
	if err != nil {
		t.Fatalf("New 返回错误: %v", err)
	}
	logger.Info("不该出现")
	logger.Warn("该出现")
	if err := closer.Close(); err != nil {
		t.Fatalf("Close 返回错误: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cfg.LogDir, "xgulu.log"))
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}
	if bytes.Contains(raw, []byte("不该出现")) {
		t.Error("级别为 warn 时 Info 不应落盘")
	}
	if !bytes.Contains(raw, []byte("该出现")) {
		t.Error("级别为 warn 时 Warn 应落盘")
	}
}

func TestLoggerRoundTripThroughContext(t *testing.T) {
	if got := FromContext(context.Background()); got != slog.Default() {
		t.Error("ctx 中没有 logger 时应回退 slog.Default()")
	}

	custom := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := WithLogger(context.Background(), custom)
	if got := FromContext(ctx); got != custom {
		t.Error("WithLogger 注入的 logger 应能从 ctx 取回")
	}

	ctx = WithRequestID(ctx, "req-1")
	if got := RequestIDFromContext(ctx); got != "req-1" {
		t.Errorf("RequestIDFromContext = %q，期望 req-1", got)
	}
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Errorf("无请求 id 时应返回空串，得到 %q", got)
	}
}
