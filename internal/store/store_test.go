package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xgulu/internal/config"
	"github.com/xgulu/internal/migrate"
	"github.com/xgulu/migrations"
)

// openMigrated 打开一个已跑完迁移的临时数据库。
func openMigrated(t *testing.T) *Store {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.LogDir = ""

	st, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := migrate.Run(context.Background(), st.DB(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return st
}

func TestOpenCreatesPrivateDataDirAndDBFile(t *testing.T) {
	dataDir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dataDir
	cfg.LogDir = ""

	st, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	defer func() { _ = st.Close() }()

	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf("数据目录不存在: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("数据目录权限 = %o，期望 700（内含明文 API key）", perm)
	}

	dbInfo, err := os.Stat(cfg.DBPath())
	if err != nil {
		t.Fatalf("数据库文件不存在: %v", err)
	}
	if perm := dbInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("数据库文件权限 = %o，期望 600", perm)
	}
}

func TestOpenAppliesPragmasAndSingleConnection(t *testing.T) {
	st := openMigrated(t)
	ctx := context.Background()

	var journalMode string
	if err := st.DB().QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("读取 journal_mode 失败: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q，期望 wal（ADR-0004）", journalMode)
	}

	var foreignKeys int
	if err := st.DB().QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("读取 foreign_keys 失败: %v", err)
	}
	if foreignKeys != 1 {
		t.Error("foreign_keys 未开启，级联删除会静默失效")
	}

	var busyTimeout int
	if err := st.DB().QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("读取 busy_timeout 失败: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d，期望 5000", busyTimeout)
	}

	if got := st.DB().Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d，期望 1（写串行，ADR-0004）", got)
	}
}

func TestListConversationsEmptyIsEmptySlice(t *testing.T) {
	st := openMigrated(t)

	got, err := st.ListConversations(context.Background())
	if err != nil {
		t.Fatalf("ListConversations 失败: %v", err)
	}
	if got == nil {
		t.Error("空结果应为长度 0 的切片，nil 会被编码成 JSON null 破坏契约")
	}
	if len(got) != 0 {
		t.Errorf("空库应返回 0 条，得到 %d 条", len(got))
	}
}

func TestListConversationsOrdersByUpdatedAtDescWithMessageCount(t *testing.T) {
	st := openMigrated(t)
	ctx := context.Background()

	bg := func(q string, args ...any) {
		t.Helper()
		if _, err := st.DB().ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("准备数据失败: %v", err)
		}
	}
	bg(`INSERT INTO conversations(id, title, created_at, updated_at) VALUES ('cnv_old', '旧会话', 1000, 1000)`)
	bg(`INSERT INTO conversations(id, title, created_at, updated_at) VALUES ('cnv_new', '新会话', 2000, 5000)`)
	bg(`INSERT INTO messages(id, conversation_id, seq, role, content, status, created_at)
	    VALUES ('msg_1', 'cnv_new', 1, 'user', '你好', 'complete', 2000)`)
	bg(`INSERT INTO messages(id, conversation_id, seq, role, content, status, created_at)
	    VALUES ('msg_2', 'cnv_new', 2, 'assistant', '你好，有什么可以帮你？', 'complete', 2001)`)

	got, err := st.ListConversations(ctx)
	if err != nil {
		t.Fatalf("ListConversations 失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("返回 %d 条会话，期望 2 条", len(got))
	}
	if got[0].ID != "cnv_new" || got[1].ID != "cnv_old" {
		t.Errorf("排序错误：得到 %s, %s，期望按 updated_at 倒序（cnv_new, cnv_old）", got[0].ID, got[1].ID)
	}
	if got[0].MessageCount != 2 {
		t.Errorf("cnv_new 消息数 = %d，期望 2", got[0].MessageCount)
	}
	if got[1].MessageCount != 0 {
		t.Errorf("cnv_old 消息数 = %d，期望 0", got[1].MessageCount)
	}
	if got[0].Title != "新会话" || got[0].UpdatedAt != 5000 {
		t.Errorf("字段读取错误: %+v", got[0])
	}
	// 可空的关联字段：未设置时为 nil，而不是空串
	if got[0].ServiceID != nil || got[0].ModelID != nil || got[0].RoleID != nil {
		t.Errorf("未设置关联时应为 null: %+v", got[0])
	}
}

func TestOpenFailsOnUnwritableDataDir(t *testing.T) {
	parent := t.TempDir()
	blocked := filepath.Join(parent, "blocked")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatalf("准备只读目录失败: %v", err)
	}

	cfg := config.Default()
	cfg.DataDir = filepath.Join(blocked, "xgulu")
	cfg.LogDir = ""

	if _, err := Open(cfg); err == nil {
		t.Error("数据目录不可创建时应返回错误而不是静默继续")
	}
}
