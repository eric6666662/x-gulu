package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// 等迁移真正落库（user_version = 1）再取消 ctx，避免与启动过程抢时序。
func waitForSchemaVersion(t *testing.T, dbPath string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if currentSchemaVersion(dbPath) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("5s 内数据库 schema 版本没有到达 %d（当前 %d）", want, currentSchemaVersion(dbPath))
}

func currentSchemaVersion(dbPath string) int {
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		return -1
	}
	defer func() { _ = db.Close() }()

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return -1
	}
	return version
}

// run 应完成"解析旗标 → 建日志 → 开库 → 跑迁移 → 装配 → 服务"整条编排，并在 ctx 取消后干净收尾。
func TestRunDoesFullOrchestrationThenStops(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "xgulu.db")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"--data-dir", dataDir,
			"--addr", "127.0.0.1:0", // 随机端口，不占用开发者机器的 8737
			"--log-dir=", // 不落盘，测试不留垃圾
		})
	}()

	waitForSchemaVersion(t, dbPath, 1)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ctx 取消后 run 应干净返回，得到: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ctx 取消后 10s 内没有退出")
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	if err := run(context.Background(), []string{"--nope"}); err == nil {
		t.Error("未知旗标应返回错误，而不是静默按默认值启动")
	}
}

func TestRunRejectsInvalidLogLevel(t *testing.T) {
	dataDir := t.TempDir()
	if err := run(context.Background(), []string{"--data-dir", dataDir, "--log-level", "verbose"}); err == nil {
		t.Error("非法日志级别应返回错误")
	}
}
