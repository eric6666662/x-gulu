package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xgulu/internal/config"
	"github.com/xgulu/internal/store"
)

type fakeLister struct {
	conversations []store.Conversation
	err           error
}

func (f fakeLister) ListConversations(context.Context) ([]store.Conversation, error) {
	return f.conversations, f.err
}

type testServer struct {
	handler http.Handler
	logs    *bytes.Buffer
}

func newTestServer(t *testing.T, lister ConversationLister, static fs.FS) testServer {
	t.Helper()
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	cfg := config.Default()

	srv := New(cfg, lister, static, logger)
	return testServer{handler: srv.Handler(), logs: logs}
}

func (ts testServer) do(t *testing.T, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON: %v（原文 %q）", err, rec.Body.String())
	}
	return out
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	body := decodeJSON(t, rec)
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("响应缺少 error 对象: %v", body)
	}
	code, _ := errObj["code"].(string)
	return code
}

func TestHealthReturnsOKAndVersion(t *testing.T) {
	ts := newTestServer(t, fakeLister{}, nil)

	rec := ts.do(t, http.MethodGet, "/api/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", rec.Code)
	}
	body := decodeJSON(t, rec)
	if body["ok"] != true {
		t.Errorf("ok = %v，期望 true", body["ok"])
	}
	if body["version"] != config.Version {
		t.Errorf("version = %v，期望 %v", body["version"], config.Version)
	}
}

func TestEmptyConversationListIsJSONArray(t *testing.T) {
	ts := newTestServer(t, fakeLister{conversations: []store.Conversation{}}, nil)

	rec := ts.do(t, http.MethodGet, "/api/conversations", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("空列表响应体 = %q，期望 []（不能是 null）", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q，期望 application/json", ct)
	}
}

func TestConversationListMatchesContractShape(t *testing.T) {
	serviceID := "svc_01J"
	ts := newTestServer(t, fakeLister{conversations: []store.Conversation{{
		ID: "cnv_01J", Title: "帮我写周报", ServiceID: &serviceID, ModelID: nil, RoleID: nil,
		MessageCount: 12, CreatedAt: 1760000000000, UpdatedAt: 1760000100000,
	}}}, nil)

	rec := ts.do(t, http.MethodGet, "/api/conversations", nil)

	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("响应不是数组: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("返回 %d 条，期望 1 条", len(list))
	}
	got := list[0]
	for _, key := range []string{"id", "title", "serviceId", "modelId", "roleId", "messageCount", "createdAt", "updatedAt"} {
		if _, ok := got[key]; !ok {
			t.Errorf("缺少字段 %s（契约见 02-后端设计 §1.1）", key)
		}
	}
	if got["messageCount"] != float64(12) {
		t.Errorf("messageCount = %v，期望 12", got["messageCount"])
	}
	if got["updatedAt"] != float64(1760000100000) {
		t.Errorf("updatedAt = %v，时间戳应为 Unix 毫秒数字", got["updatedAt"])
	}
	if got["modelId"] != nil {
		t.Errorf("modelId = %v，未挂模型时应为 null", got["modelId"])
	}
}

func TestStorageFailureBecomes500WithoutLeakingCause(t *testing.T) {
	ts := newTestServer(t, fakeLister{err: errors.New("database is locked")}, nil)

	rec := ts.do(t, http.MethodGet, "/api/conversations", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d，期望 500", rec.Code)
	}
	if code := errorCode(t, rec); code != "storage_error" {
		t.Errorf("错误码 = %q，期望 storage_error", code)
	}
	if strings.Contains(rec.Body.String(), "database is locked") {
		t.Error("响应体不得包含底层错误细节")
	}
	// 细节要留在日志里，否则线上无从排查
	if !strings.Contains(ts.logs.String(), "database is locked") {
		t.Error("底层错误应写入日志")
	}
	if !strings.Contains(ts.logs.String(), "storage_error") {
		t.Error("日志应带错误码")
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	// 即便有前端资源，/api/* 也不该被 SPA 回退吞掉
	static := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	ts := newTestServer(t, fakeLister{}, static)

	rec := ts.do(t, http.MethodGet, "/api/nope", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d，期望 404", rec.Code)
	}
	if code := errorCode(t, rec); code != "internal_error" {
		t.Errorf("错误码 = %q，期望 internal_error", code)
	}
}

func TestWrongMethodIsRejected(t *testing.T) {
	ts := newTestServer(t, fakeLister{}, nil)

	rec := ts.do(t, http.MethodPost, "/api/conversations", nil)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("状态码 = %d，期望 405", rec.Code)
	}
}

func TestStaticServesIndexAndFallsBackForSPARoutes(t *testing.T) {
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	ts := newTestServer(t, fakeLister{}, static)

	t.Run("根路径给 index.html", func(t *testing.T) {
		rec := ts.do(t, http.MethodGet, "/", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") {
			t.Errorf("GET / = %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("静态资源按原样返回", func(t *testing.T) {
		rec := ts.do(t, http.MethodGet, "/assets/app.js", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "console.log") {
			t.Errorf("GET /assets/app.js = %d %q", rec.Code, rec.Body.String())
		}
	})

	t.Run("前端路由回退 index.html", func(t *testing.T) {
		rec := ts.do(t, http.MethodGet, "/c/cnv_01J", nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html>app</html>") {
			t.Errorf("GET /c/cnv_01J = %d %q，期望回退 index.html", rec.Code, rec.Body.String())
		}
	})
}

func TestMissingFrontendBuildExplainsHowToFix(t *testing.T) {
	ts := newTestServer(t, fakeLister{}, fstest.MapFS{})

	rec := ts.do(t, http.MethodGet, "/", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("状态码 = %d，期望 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "pnpm") {
		t.Errorf("提示应告诉开发者怎么构建前端，得到 %q", rec.Body.String())
	}
}

func TestRequestIDIsGeneratedEchoedAndLogged(t *testing.T) {
	ts := newTestServer(t, fakeLister{}, nil)

	rec := ts.do(t, http.MethodGet, "/api/health", nil)

	got := rec.Header().Get("X-Request-Id")
	if len(got) != 32 {
		t.Errorf("X-Request-Id = %q，期望 32 位十六进制（crypto/rand）", got)
	}
	if !strings.Contains(ts.logs.String(), got) {
		t.Error("访问日志里应带上同一个 req_id，否则串不起一次请求")
	}
}

func TestInboundRequestIDIsHonoured(t *testing.T) {
	ts := newTestServer(t, fakeLister{}, nil)

	rec := ts.do(t, http.MethodGet, "/api/health", map[string]string{"X-Request-Id": "from-caller"})

	if got := rec.Header().Get("X-Request-Id"); got != "from-caller" {
		t.Errorf("X-Request-Id = %q，期望沿用调用方传入的值", got)
	}
}

func TestAccessLogCarriesContractFields(t *testing.T) {
	ts := newTestServer(t, fakeLister{}, nil)

	ts.do(t, http.MethodGet, "/api/health", nil)

	line := strings.TrimSpace(ts.logs.String())
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("访问日志不是 JSON 行: %v（原文 %q）", err, line)
	}
	if entry["method"] != "GET" {
		t.Errorf("method = %v", entry["method"])
	}
	if entry["path"] != "/api/health" {
		t.Errorf("path = %v", entry["path"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("status = %v，期望 200 数字", entry["status"])
	}
	for _, key := range []string{"req_id", "duration_ms", "bytes"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("访问日志缺字段 %s（02-后端设计 §4）", key)
		}
	}
}

func TestPanicIsRecoveredIntoFiveHundred(t *testing.T) {
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})

	handler := recoverMiddleware(logger)(boom)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("状态码 = %d，期望 500", rec.Code)
	}
	if code := errorCode(t, rec); code != "internal_error" {
		t.Errorf("错误码 = %q，期望 internal_error", code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("panic 内容不得出现在响应体")
	}
	if !strings.Contains(logs.String(), "boom") {
		t.Error("panic 内容应记入日志")
	}
}
