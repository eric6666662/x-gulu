// Package store 负责 SQLite 持久化：连接与 pragma、迁移执行入口、按实体的仓储方法。
// 不含业务规则（那在 internal/chat），也不 import 任何业务包（依赖方向的单向保证）。
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"

	_ "modernc.org/sqlite" // 纯 Go 驱动（ADR-0004）

	"github.com/xgulu/internal/config"
)

const (
	driverName    = "sqlite"
	busyTimeoutMS = 5000
)

// Conversation 是会话的存储形态（02-后端设计 §1.1 的 Conversation 实体）。
type Conversation struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	ServiceID    *string `json:"serviceId"`
	ModelID      *string `json:"modelId"`
	RoleID       *string `json:"roleId"`
	MessageCount int     `json:"messageCount"`
	CreatedAt    int64   `json:"createdAt"`
	UpdatedAt    int64   `json:"updatedAt"`
}

// Store 持有连接池，是全部仓储方法的接收者。
type Store struct {
	db *sql.DB
}

// Open 打开（必要时创建）数据库：数据目录 0700、数据库文件 0600（02-后端设计 §0）。
func Open(cfg config.Config) (*Store, error) {
	if err := ensurePrivateDir(cfg.DataDir); err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn(cfg.DBPath()))
	if err != nil {
		return nil, fmt.Errorf("打开数据库 %s: %w", cfg.DBPath(), err)
	}
	// 写串行，实现最简单；本机负载下无瓶颈（ADR-0004）。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// 连一次以确认可写，并让 modernc 真正建出文件（它是懒创建的）。
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接数据库 %s: %w", cfg.DBPath(), err)
	}
	if err := hardenDBFiles(cfg.DBPath()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// DB 暴露底层连接池，供迁移与需要事务的调用方使用。
func (s *Store) DB() *sql.DB { return s.db }

// Close 关闭连接池。
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("关闭数据库: %w", err)
	}
	return nil
}

// ListConversations 返回全部会话，按 updated_at 倒序；空结果为长度 0 的切片（不是 nil）。
func (s *Store) ListConversations(ctx context.Context) ([]Conversation, error) {
	const query = `
SELECT c.id, c.title, c.service_id, c.model_id, c.role_id,
       (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id),
       c.created_at, c.updated_at
FROM conversations c
ORDER BY c.updated_at DESC, c.id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询会话列表: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// 非 nil 切片：nil 会被编码成 JSON 的 null，破坏 [] 的契约。
	conversations := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(
			&c.ID, &c.Title, &c.ServiceID, &c.ModelID, &c.RoleID,
			&c.MessageCount, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("读取会话行: %w", err)
		}
		conversations = append(conversations, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历会话列表: %w", err)
	}
	return conversations, nil
}

// dsn 把 pragma 写进连接串而不是执行一次 PRAGMA：连接池重建连接时同样生效。
func dsn(path string) string {
	params := url.Values{}
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	params.Add("_pragma", "foreign_keys(1)") // ON DELETE CASCADE / SET NULL 依赖它
	return "file:" + (&url.URL{Path: path}).EscapedPath() + "?" + params.Encode()
}

// ensurePrivateDir 建目录并确保权限为 0700——数据目录里放着明文 API key。
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建数据目录 %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("设置数据目录权限 %s: %w", dir, err)
	}
	return nil
}

// hardenDBFiles 把数据库及其 WAL 旁路文件收紧到 0600（驱动建文件时用的是 umask 默认值）。
func hardenDBFiles(dbPath string) error {
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Chmod(path, 0o600); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // 尚未产生的旁路文件
			}
			return fmt.Errorf("设置数据库文件权限 %s: %w", path, err)
		}
	}
	return nil
}
