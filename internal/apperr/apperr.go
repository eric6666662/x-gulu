// Package apperr 是领域错误到 HTTP 的唯一出口：错误对象自带错误码、HTTP 状态与
// 已去行话的中文文案；handler 只 return err，由 httpapi 统一映射成响应体（02-后端设计 §1.4）。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code 是机器可读的错误码（02-后端设计 §1.4 的错误码表）。
type Code string

const (
	CodeInvalidRequest       Code = "invalid_request"
	CodeConversationNotFound Code = "conversation_not_found"
	CodeStorageError         Code = "storage_error"
	CodeInternalError        Code = "internal_error"
)

// Error 是带 HTTP 语义的领域错误。
type Error struct {
	Code    Code   // 机器可读错误码
	HTTP    int    // 对应 HTTP 状态
	Message string // 去行话中文文案，前端直接展示
	Field   string // 可选：出错的请求字段
	Err     error  // 可选：底层错误，只进日志，绝不进响应体
}

// New 构造不含底层错误的领域错误。
func New(code Code, httpStatus int, message string) *Error {
	return &Error{Code: code, HTTP: httpStatus, Message: message}
}

// Wrap 在领域错误上附带底层错误（只用于日志）。
func Wrap(err error, code Code, httpStatus int, message string) *Error {
	return &Error{Code: code, HTTP: httpStatus, Message: message, Err: err}
}

// InvalidRequest 表示请求体不合规。
func InvalidRequest(field, message string) *Error {
	return &Error{
		Code:    CodeInvalidRequest,
		HTTP:    http.StatusBadRequest,
		Message: message,
		Field:   field,
	}
}

// StorageError 表示本地存储失败。
func StorageError(err error) *Error {
	return Wrap(err, CodeStorageError, http.StatusInternalServerError, "数据读写失败，请重试")
}

// Internal 是未映射错误的安全出口：对外只给 internal_error，细节留给日志。
func Internal(err error) *Error {
	return Wrap(err, CodeInternalError, http.StatusInternalServerError, "服务内部错误")
}

// As 判定 err 链上是否有 *Error。
func As(err error) (*Error, bool) {
	return errors.AsType[*Error](err)
}

// Error 形如 "storage_error: 数据读写失败，请重试: database is locked"——
// 前两段给用户与错误码，末段只出现在日志里。
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }
