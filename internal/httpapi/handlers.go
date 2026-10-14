package httpapi

import (
	"net/http"

	"github.com/xgulu/internal/apperr"
	"github.com/xgulu/internal/config"
)

// conversationDTO 是会话在 HTTP 契约里的形状（02-后端设计 §1.1）。
// 与存储结构分开：契约变更只动这里，存储结构跟着数据库走。
type conversationDTO struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	ServiceID    *string `json:"serviceId"`
	ModelID      *string `json:"modelId"`
	RoleID       *string `json:"roleId"`
	MessageCount int     `json:"messageCount"`
	CreatedAt    int64   `json:"createdAt"`
	UpdatedAt    int64   `json:"updatedAt"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": config.Version,
	})
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	conversations, err := s.conversations.ListConversations(r.Context())
	if err != nil {
		s.respond(w, r, apperr.StorageError(err))
		return
	}

	// 非 nil 切片：空库也要编码成 []，不能是 null。
	dtos := make([]conversationDTO, 0, len(conversations))
	for _, c := range conversations {
		dtos = append(dtos, conversationDTO{
			ID:           c.ID,
			Title:        c.Title,
			ServiceID:    c.ServiceID,
			ModelID:      c.ModelID,
			RoleID:       c.RoleID,
			MessageCount: c.MessageCount,
			CreatedAt:    c.CreatedAt,
			UpdatedAt:    c.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, dtos)
}

func (s *Server) handleUnknownAPI(w http.ResponseWriter, r *http.Request) {
	s.respond(w, r, apperr.New(apperr.CodeInternalError, http.StatusNotFound, "接口不存在"))
}

func (s *Server) handleMethodNotAllowed(allowed string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allowed)
		s.respond(w, r, apperr.New(apperr.CodeInvalidRequest, http.StatusMethodNotAllowed,
			"该接口不接受 "+r.Method+" 请求"))
	}
}
