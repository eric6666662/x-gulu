package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/xgulu/internal/config"
	"github.com/xgulu/internal/migrate"
	"github.com/xgulu/internal/store"
	"github.com/xgulu/migrations"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// newTestApp 组装一个用临时数据目录、日志只进缓冲区的 App。
func newTestApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.LogDir = ""
	cfg.Addr = "127.0.0.1:0"

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if _, err := migrate.Run(context.Background(), st.DB(), migrations.FS); err != nil {
		_ = st.Close()
		t.Fatalf("迁移失败: %v", err)
	}

	a, err := New(cfg, Deps{Logger: logger, LogCloser: nopCloser{}, Store: st})
	if err != nil {
		_ = st.Close()
		t.Fatalf("装配失败: %v", err)
	}
	return a
}

func TestServeAnswersHealthThenShutsDownOnContextCancel(t *testing.T) {
	a := newTestApp(t)
	defer func() { _ = a.Close() }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- a.Serve(ctx, ln) }()

	url := "http://" + ln.Addr().String() + "/api/health"
	// 监听成功不等于已在服务：listener 存在但没人 accept 时连接会进内核队列并吊住读操作，
	// 所以必须给客户端设超时，否则测试会一直等下去。
	client := &http.Client{Timeout: 2 * time.Second}
	var body map[string]any
	var lastErr error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		lastErr = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("状态码 = %d，期望 200", resp.StatusCode)
		}
		break
	}
	if lastErr != nil {
		t.Fatalf("请求健康检查失败: %v", lastErr)
	}
	if body["version"] != config.Version {
		t.Errorf("version = %v，期望 %v", body["version"], config.Version)
	}

	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Errorf("ctx 取消后应优雅关闭，得到错误: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 取消后 5s 内没有关闭")
	}
}

func TestRunReportsListenFailure(t *testing.T) {
	a := newTestApp(t)
	defer func() { _ = a.Close() }()
	a.cfg.Addr = "127.0.0.1:not-a-port"

	if err := a.Run(context.Background()); err == nil {
		t.Error("监听地址非法时应返回错误，而不是静默退出")
	}
}

func TestCloseIsIdempotentAndClosesStore(t *testing.T) {
	a := newTestApp(t)

	if err := a.Close(); err != nil {
		t.Fatalf("首次 Close 返回错误: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Errorf("重复 Close 不应报错，得到: %v", err)
	}
	if err := a.deps.Store.DB().PingContext(context.Background()); err == nil {
		t.Error("Close 后连接池应已关闭")
	}
}

func TestNewFallsBackToDefaultLogger(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.LogDir = ""

	// Logger 为 nil 时不 panic，退回默认 logger（装配层不该因缺省值崩掉）
	a, err := New(cfg, Deps{Logger: nil, Store: mustStore(t, cfg)})
	if err != nil {
		t.Fatalf("New 返回错误: %v", err)
	}
	if a.deps.Logger == nil {
		t.Error("Logger 为 nil 时应回退到默认 logger")
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close 返回错误: %v", err)
	}
}

func mustStore(t *testing.T, cfg config.Config) *store.Store {
	t.Helper()
	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	return st
}
