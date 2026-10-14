package httpapi

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

const indexFile = "index.html"

// staticHandler 提供内嵌的前端产物：命中文件就原样返回，
// 其余路径（前端路由 /c/:id、/settings）回退 index.html（02-后端设计 §0）。
func (s *Server) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = indexFile
		}
		if name != indexFile && fs.ValidPath(name) && serveAsset(w, s.static, name) {
			return
		}
		s.serveIndex(w)
	})
}

func serveAsset(w http.ResponseWriter, fsys fs.FS, name string) bool {
	if fsys == nil {
		return false
	}
	data, err := fs.ReadFile(fsys, name) // fs.FS 自身拒绝非法路径（含 ..）
	if err != nil {
		return false
	}
	w.Header().Set("Content-Type", contentTypeFor(name))
	_, _ = w.Write(data)
	return true
}

func (s *Server) serveIndex(w http.ResponseWriter) {
	if serveAsset(w, s.static, indexFile) {
		return
	}
	// 前端尚未构建：给出能直接照做的提示，而不是一片空白。
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w,
		"前端资源未构建。请在 web/ 目录执行 pnpm install && pnpm build 后重新启动；使用发布版二进制则无需此步。\n")
}

func contentTypeFor(name string) string {
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
