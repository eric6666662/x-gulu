package migrate

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"

	"github.com/xgulu/migrations"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var found string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name,
	).Scan(&found)
	switch {
	case err == sql.ErrNoRows:
		return false
	case err != nil:
		t.Fatalf("查询表 %s 失败: %v", name, err)
	}
	return true
}

func TestRunAppliesEmbeddedMigrationsThenIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	applied, err := Run(ctx, db, migrations.FS)
	if err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if applied != 1 {
		t.Errorf("首次应用 %d 个迁移，期望 1", applied)
	}

	for _, table := range []string{"services", "roles", "conversations", "messages", "settings"} {
		if !tableExists(t, db, table) {
			t.Errorf("迁移后缺少表 %s", table)
		}
	}

	// 设置表初始就有默认键（context_limit 默认 20）
	var limit string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'context_limit'`).Scan(&limit); err != nil {
		t.Fatalf("读取默认 context_limit 失败: %v", err)
	}
	if limit != "20" {
		t.Errorf("context_limit 默认值 = %q，期望 20", limit)
	}

	// 幂等：重复启动不重复执行
	applied, err = Run(ctx, db, migrations.FS)
	if err != nil {
		t.Fatalf("二次迁移失败: %v", err)
	}
	if applied != 0 {
		t.Errorf("二次迁移应用了 %d 个，期望 0", applied)
	}
}

func TestRunAppliesOnlyPendingFiles(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	fsys := fstest.MapFS{
		"0001_a.sql": {Data: []byte("CREATE TABLE t1(id TEXT);")},
	}
	if _, err := Run(ctx, db, fsys); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}

	fsys["0002_b.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE t2(id TEXT);")}
	applied, err := Run(ctx, db, fsys)
	if err != nil {
		t.Fatalf("增量迁移失败: %v", err)
	}
	if applied != 1 {
		t.Errorf("增量应用 %d 个迁移，期望 1", applied)
	}
	if !tableExists(t, db, "t2") {
		t.Error("增量迁移后缺少表 t2")
	}
}

func TestRunLeavesNoHalfMigrationOnFailure(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	fsys := fstest.MapFS{
		"0001_a.sql":   {Data: []byte("CREATE TABLE t1(id TEXT);")},
		"0002_bad.sql": {Data: []byte("CREATE TABLE t2(id TEXT); 这不是 SQL;")},
	}

	applied, err := Run(ctx, db, fsys)
	if err == nil {
		t.Fatal("坏迁移应返回错误")
	}
	if applied != 1 {
		t.Errorf("失败前应用了 %d 个迁移，期望 1", applied)
	}
	if tableExists(t, db, "t2") {
		t.Error("失败的迁移不应留下半成品表 t2")
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("读取 user_version 失败: %v", err)
	}
	if version != 1 {
		t.Errorf("user_version = %d，期望停在 1（不因失败而前进）", version)
	}
}

func TestRunRejectsBadFileNames(t *testing.T) {
	ctx := context.Background()

	t.Run("序号重复", func(t *testing.T) {
		fsys := fstest.MapFS{
			"0001_a.sql": {Data: []byte("CREATE TABLE t1(id TEXT);")},
			"0001_b.sql": {Data: []byte("CREATE TABLE t2(id TEXT);")},
		}
		if _, err := Run(ctx, openTestDB(t), fsys); err == nil {
			t.Error("序号重复应返回错误")
		}
	})

	t.Run("缺少下划线", func(t *testing.T) {
		fsys := fstest.MapFS{"init.sql": {Data: []byte("SELECT 1;")}}
		if _, err := Run(ctx, openTestDB(t), fsys); err == nil {
			t.Error("文件名不合规应返回错误")
		}
	})

	t.Run("序号非数字", func(t *testing.T) {
		fsys := fstest.MapFS{"abc_init.sql": {Data: []byte("SELECT 1;")}}
		if _, err := Run(ctx, openTestDB(t), fsys); err == nil {
			t.Error("序号非数字应返回错误")
		}
	})
}
