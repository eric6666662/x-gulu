package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/xgulu/internal/apperr"
	"github.com/xgulu/internal/logging"
)

// recoverMiddleware 兜住 handler 的 panic：记录堆栈，回 500 JSON，且不泄漏 panic 内容。
func recoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				logger.ErrorContext(r.Context(), "处理请求时 panic",
					"err", recovered, "method", r.Method, "path", r.URL.Path)

				if rec, ok := w.(*statusRecorder); ok {
					rec.errCode = string(apperr.CodeInternalError)
				}
				writeJSON(w, http.StatusInternalServerError, errorBody{Error: errorDetail{
					Code:    string(apperr.CodeInternalError),
					Message: apperr.Internal(nil).Message,
				}})
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// requestIDMiddleware 生成（或沿用调用方传入的）req_id，回写响应头，
// 并把请求域 logger（带 req_id 字段）注入 ctx —— 这是 ctx 作为日志主通道的入口（ADR-0008）。
func requestIDMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = newRequestID()
			}
			w.Header().Set("X-Request-Id", id)

			ctx := logging.WithRequestID(r.Context(), id)
			ctx = logging.WithLogger(ctx, logger.With("req_id", id))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// accessLogMiddleware 每个请求记一行：req_id / method / path / status / duration_ms / bytes / err_code。
func accessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", rec.bytes,
			}
			if id := logging.RequestIDFromContext(r.Context()); id != "" {
				attrs = append(attrs, "req_id", id)
			}
			if rec.errCode != "" {
				attrs = append(attrs, "err_code", rec.errCode)
			}
			logger.InfoContext(r.Context(), "请求完成", attrs...)
		})
	}
}

// statusRecorder 记录状态码与写出的字节数；Unwrap 让 http.NewResponseController
// 仍能拿到底层 ResponseWriter（SSE 的 Flush 依赖它，见 02-后端设计 §3.2）。
type statusRecorder struct {
	http.ResponseWriter
	status  int
	bytes   int
	errCode string
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// newRequestID 用 crypto/rand 生成 16 字节十六进制 id（零依赖，ADR-0008）。
func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand 失败意味着系统熵源异常；退化成时间戳仍能给出可关联的 id
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
