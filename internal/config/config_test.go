package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Addr != "127.0.0.1:8737" {
		t.Errorf("Addr 默认值 = %q，期望 %q（需求 #14：默认只监听本机）", cfg.Addr, "127.0.0.1:8737")
	}
	if !strings.HasSuffix(cfg.DataDir, ".xgulu") {
		t.Errorf("DataDir 默认值 = %q，期望以 .xgulu 结尾", cfg.DataDir)
	}
	if cfg.LogDir != filepath.Join(cfg.DataDir, "logs") {
		t.Errorf("LogDir 默认值 = %q，期望 %q", cfg.LogDir, filepath.Join(cfg.DataDir, "logs"))
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat 默认值 = %q，期望 json", cfg.LogFormat)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel 默认值 = %q，期望 info", cfg.LogLevel)
	}
}

func TestParseOverrides(t *testing.T) {
	cfg, err := Parse([]string{"--addr=0.0.0.0:9000", "--data-dir=/tmp/xgulu-test"})
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}

	if cfg.Addr != "0.0.0.0:9000" {
		t.Errorf("Addr = %q，期望被旗标覆盖", cfg.Addr)
	}
	if cfg.DataDir != "/tmp/xgulu-test" {
		t.Errorf("DataDir = %q，期望 /tmp/xgulu-test", cfg.DataDir)
	}
	// LogDir 未显式给出时随 DataDir 迁移（02-后端设计 §0）
	if cfg.LogDir != filepath.Join("/tmp/xgulu-test", "logs") {
		t.Errorf("LogDir = %q，期望随 DataDir 迁移", cfg.LogDir)
	}
}

func TestParseLogDirEmptyMeansNoFile(t *testing.T) {
	cfg, err := Parse([]string{"--log-dir="})
	if err != nil {
		t.Fatalf("Parse 返回错误: %v", err)
	}
	if cfg.LogDir != "" {
		t.Errorf("LogDir = %q，期望空串表示不落盘", cfg.LogDir)
	}
}

func TestParseRejectsUnknownFlag(t *testing.T) {
	if _, err := Parse([]string{"--nope"}); err == nil {
		t.Error("未知旗标应返回错误")
	}
}

func TestParseRejectsInvalidLogFormatAndLevel(t *testing.T) {
	if _, err := Parse([]string{"--log-format=xml"}); err == nil {
		t.Error("非法 --log-format 应返回错误")
	}
	if _, err := Parse([]string{"--log-level=verbose"}); err == nil {
		t.Error("非法 --log-level 应返回错误")
	}
}
