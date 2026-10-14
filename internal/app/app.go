// Package app 是唯一的装配层：把长生命周期对象按依赖顺序接起来，并管理进程生命周期
// （Run / Serve / Close）。不引任何 DI 工具或框架（ADR-0010）。
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/xgulu/internal/config"
	"github.com/xgulu/internal/httpapi"
	"github.com/xgulu/internal/store"
	"github.com/xgulu/web"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Deps 是 main 已经构造好的长生命周期依赖。
// 装配只接对象、不按顺序做副作用：建 logger、开库、跑迁移的先后由 main 掌握
// （02-后端设计 §3.1 的"启动顺序写在 main 里"）。
type Deps struct {
	Logger    *slog.Logger
	LogCloser io.Closer // 关闭轮转日志文件；可为 nil
	Store     *store.Store
}

// App 是一次进程运行的全部资源。
type App struct {
	cfg    config.Config
	deps   Deps
	server *http.Server
}

// New 装配依赖并构造 HTTP 服务（不监听）。
func New(cfg config.Config, deps Deps) (*App, error) {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Store == nil {
		return nil, errors.New("装配 App 需要已打开的 store")
	}

	static, err := web.Dist()
	if err != nil {
		return nil, fmt.Errorf("加载内嵌前端资源: %w", err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(cfg, deps.Store, static, deps.Logger).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		WriteTimeout:      0, // SSE 必须为 0，靠 per-request context 控超时（02-后端设计 §3.2）
	}
	return &App{cfg: cfg, deps: deps, server: server}, nil
}

// Run 监听 cfg.Addr 并服务，直到 ctx 取消后优雅关闭。
func (a *App) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", a.cfg.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", a.cfg.Addr, err)
	}
	return a.Serve(ctx, listener)
}

// Serve 在既有 listener 上服务（测试用 127.0.0.1:0 拿随机端口）。
// ctx 取消 → 停止接收新连接并给在途请求 10s 收尾。
func (a *App) Serve(ctx context.Context, listener net.Listener) error {
	a.deps.Logger.InfoContext(ctx, "服务已启动",
		"addr", listener.Addr().String(), "data_dir", a.cfg.DataDir, "log_dir", a.cfg.LogDir)

	serveErr := make(chan error, 1)
	go func() { serveErr <- a.server.Serve(listener) }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("优雅关闭超时: %w", err)
		}
		<-serveErr // Serve 已返回 ErrServerClosed
		a.deps.Logger.Info("服务已关闭")
		return nil
	}
}

// Close 逆序释放：先 server 再 store，最后关闭日志文件。重复调用安全。
func (a *App) Close() error {
	var errs []error
	if err := a.server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errs = append(errs, fmt.Errorf("关闭 HTTP 服务: %w", err))
	}
	if a.deps.Store != nil {
		if err := a.deps.Store.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.deps.LogCloser != nil {
		if err := a.deps.LogCloser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("关闭日志文件: %w", err))
		}
	}
	return errors.Join(errs...)
}
