// Package httpapi 是传输层：路由、中间件、DTO 编解码、错误码映射、静态资源。
// 不写业务判断——业务在 internal/chat，本包只把 HTTP 与领域对象互相翻译（02-后端设计 §3）。
package httpapi

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/xgulu/internal/config"
	"github.com/xgulu/internal/store"
)

// ConversationLister 是 httpapi 对会话存储的最小需求（接口定义在使用方，ADR-0010 §3）。
type ConversationLister interface {
	ListConversations(ctx context.Context) ([]store.Conversation, error)
}

// Server 持有路由依赖；本身可测，不隐式使用全局状态。
type Server struct {
	cfg           config.Config
	conversations ConversationLister
	static        fs.FS // 前端构建产物（web/dist）；nil 表示未注入
	logger        *slog.Logger
}

// New 装配 HTTP 层。static 为前端 dist 的文件系统，可为 nil。
func New(cfg config.Config, conversations ConversationLister, static fs.FS, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{cfg: cfg, conversations: conversations, static: static, logger: logger}
}

// Handler 返回挂好中间件的路由。
// 执行顺序（由外到内）：recover → requestid → accesslog → 路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	s.route(mux, "GET /api/health", s.handleHealth)
	s.route(mux, "GET /api/conversations", s.handleListConversations)

	// /api/* 兜底：未知路径不给前端回退吞掉，返回 JSON 404（契约见 02-后端设计 §1.4）。
	mux.Handle("/api/", http.HandlerFunc(s.handleUnknownAPI))
	mux.Handle("/", s.staticHandler())

	var handler http.Handler = mux
	handler = accessLogMiddleware(s.logger)(handler)
	handler = requestIDMiddleware(s.logger)(handler)
	return recoverMiddleware(s.logger)(handler)
}

// route 注册一个方法化路由，并给同路径的其他方法挂 405（而不是让它落到 /api/ 兜底 404）。
func (s *Server) route(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	mux.HandleFunc(pattern, h)
	if method, path, found := strings.Cut(pattern, " "); found {
		mux.HandleFunc(path, s.handleMethodNotAllowed(method))
	}
}
