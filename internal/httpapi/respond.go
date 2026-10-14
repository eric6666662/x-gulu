package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/xgulu/internal/apperr"
	"github.com/xgulu/internal/logging"
)

// errorBody 是统一的错误响应体：{"error":{"code","message","field"?}}（02-后端设计 §1.4）。
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// respond 是错误到 HTTP 的唯一出口：handler 只 return/传递 err，由这里决定状态码与文案。
// 未映射的错误一律 500 internal_error，且响应体不含底层错误文本（细节只进日志）。
func (s *Server) respond(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := apperr.As(err)
	if !ok {
		appErr = apperr.Internal(err)
	}

	logger := logging.FromContext(r.Context())
	if appErr.HTTP >= http.StatusInternalServerError {
		logger.ErrorContext(r.Context(), "请求失败",
			"err_code", string(appErr.Code), "err", appErr.Error())
	} else {
		logger.WarnContext(r.Context(), "请求被拒绝",
			"err_code", string(appErr.Code), "err", appErr.Error())
	}

	// 让访问日志那一行也能带上错误码（同一个 recorder 对象）。
	if rec, ok := w.(*statusRecorder); ok {
		rec.errCode = string(appErr.Code)
	}

	writeJSON(w, appErr.HTTP, errorBody{Error: errorDetail{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Field:   appErr.Field,
	}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// 响应已开始写出，只能记录——这里没有可回退的状态码。
		slog.Default().Error("写出 JSON 响应失败", "err", err)
	}
}
