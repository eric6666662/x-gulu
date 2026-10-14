// Package migrate 按序号执行内嵌的 SQL 迁移，用 PRAGMA user_version 记录已应用到第几号。
// 自研约 20 行，不引迁移库（02-后端设计 §2）。
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

const versionPragma = "user_version"

// Run 应用 fsys 中所有序号大于当前 user_version 的迁移，返回本次应用的条数。
// 每个迁移文件在**独立事务**内执行、与 user_version 同步提交：
// 任一文件失败即整体回滚该文件，已成功的文件保持已应用状态（不做半迁移，见 02-后端设计 §2）。
func Run(ctx context.Context, db *sql.DB, fsys fs.FS) (int, error) {
	files, err := collect(fsys)
	if err != nil {
		return 0, err
	}

	current, err := userVersion(ctx, db)
	if err != nil {
		return 0, err
	}

	applied := 0
	for _, f := range files {
		if f.version <= current {
			continue
		}
		body, err := fs.ReadFile(fsys, f.name)
		if err != nil {
			return applied, fmt.Errorf("读取迁移 %s: %w", f.name, err)
		}
		if err := applyOne(ctx, db, f, string(body)); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

type migrationFile struct {
	version int
	name    string
}

// collect 按序号收集 NNNN_*.sql，序号必须唯一。
func collect(fsys fs.FS) ([]migrationFile, error) {
	entries, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("扫描迁移目录: %w", err)
	}

	seen := map[int]string{}
	files := make([]migrationFile, 0, len(entries))
	for _, name := range entries {
		prefix, _, found := strings.Cut(name, "_")
		if !found {
			return nil, fmt.Errorf("迁移文件名 %q 不符合 NNNN_描述.sql", name)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("迁移文件名 %q 的序号不是数字: %w", name, err)
		}
		if other, dup := seen[version]; dup {
			return nil, fmt.Errorf("迁移序号 %d 重复：%s 与 %s", version, other, name)
		}
		seen[version] = name
		files = append(files, migrationFile{version: version, name: name})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

func applyOne(ctx context.Context, db *sql.DB, f migrationFile, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启迁移 %s 的事务: %w", f.name, err)
	}
	defer func() { _ = tx.Rollback() }() // 已提交时是 no-op

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("执行迁移 %s: %w", f.name, err)
	}
	// PRAGMA 不接受占位符，序号由文件名解析而来，非外部输入。
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA %s = %d", versionPragma, f.version)); err != nil {
		return fmt.Errorf("更新 %s 到 %d: %w", versionPragma, f.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交迁移 %s: %w", f.name, err)
	}
	return nil
}

func userVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA "+versionPragma).Scan(&version); err != nil {
		return 0, errors.Join(errors.New("读取数据库 schema 版本失败"), err)
	}
	return version, nil
}
