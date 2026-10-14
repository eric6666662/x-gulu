package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestErrorCarriesCodeStatusAndMessage(t *testing.T) {
	err := New(CodeConversationNotFound, http.StatusNotFound, "会话不存在")

	if err.Code != CodeConversationNotFound {
		t.Errorf("Code = %q", err.Code)
	}
	if err.HTTP != http.StatusNotFound {
		t.Errorf("HTTP = %d，期望 404", err.HTTP)
	}
	if err.Message != "会话不存在" {
		t.Errorf("Message = %q", err.Message)
	}
	if !strings.Contains(err.Error(), "conversation_not_found") {
		t.Errorf("Error() = %q，应包含错误码便于日志排查", err.Error())
	}
}

func TestWrapKeepsCauseReachableForLogsOnly(t *testing.T) {
	cause := errors.New("database is locked")
	err := Wrap(cause, CodeStorageError, http.StatusInternalServerError, "保存失败，请重试")

	if !errors.Is(err, cause) {
		t.Error("Wrap 后应能通过 errors.Is 找到底层错误")
	}
	if !strings.Contains(err.Error(), "database is locked") {
		t.Error("底层错误应出现在 Error() 里以便日志定位")
	}

	var target *Error
	if !errors.As(err, &target) || target.Code != CodeStorageError {
		t.Error("errors.As 应能取出 *Error")
	}
}

func TestAsFindsErrorInChain(t *testing.T) {
	inner := InvalidRequest("content", "消息内容不能为空")
	wrapped := fmt.Errorf("处理消息: %w", inner)

	got, ok := As(wrapped)
	if !ok {
		t.Fatal("As 应能在包装链中找到 *Error")
	}
	if got.Field != "content" {
		t.Errorf("Field = %q，期望 content", got.Field)
	}

	if _, ok := As(errors.New("普通错误")); ok {
		t.Error("普通错误不应被当成 *Error")
	}
}

func TestInternalHidesCauseText(t *testing.T) {
	cause := errors.New("panic: runtime error: index out of range")
	err := Internal(cause)

	if err.Code != CodeInternalError || err.HTTP != http.StatusInternalServerError {
		t.Errorf("Internal 应映射为 500 internal_error，得到 %d %s", err.HTTP, err.Code)
	}
	if strings.Contains(err.Message, "index out of range") {
		t.Errorf("对外文案不得泄漏内部细节: %q", err.Message)
	}
	if !errors.Is(err, cause) {
		t.Error("底层错误仍应可达（日志用）")
	}
}

func TestStorageErrorIsFiveHundred(t *testing.T) {
	err := StorageError(errors.New("disk full"))
	if err.HTTP != http.StatusInternalServerError || err.Code != CodeStorageError {
		t.Errorf("StorageError = %d/%s，期望 500/storage_error", err.HTTP, err.Code)
	}
}
