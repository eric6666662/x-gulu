// Package config 是后端配置的唯一原始值来源：默认值、旗标解析、目录定位。
// 其他包不自行读旗标或环境变量，一律以 config.Config 结构体为单位注入（见 02-后端设计 §3.1）。
package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Version 为构建版本号，健康检查返回给前端；发布时用
// -ldflags "-X github.com/xgulu/internal/config.Version=x.y.z" 覆盖。
var Version = "dev"

const (
	defaultAddr      = "127.0.0.1:8737" // 需求功能 #14：默认只监听本机
	defaultLogFormat = "json"
	defaultLogLevel  = "info"
)

// Config 是一次进程运行的全部配置。
type Config struct {
	Addr      string // 监听地址
	DataDir   string // 数据目录（数据库与日志的父目录）
	LogDir    string // 日志目录；空串表示不落盘，只输出到 stderr
	LogFormat string // 日志格式：json | text
	LogLevel  string // 日志级别：debug | info | warn | error
}

// DBPath 返回 SQLite 数据库文件路径。
func (c Config) DBPath() string { return filepath.Join(c.DataDir, "xgulu.db") }

// Default 返回全默认配置。
func Default() Config {
	dataDir := filepath.Join(homeDir(), ".xgulu")
	return Config{
		Addr:      defaultAddr,
		DataDir:   dataDir,
		LogDir:    filepath.Join(dataDir, "logs"),
		LogFormat: defaultLogFormat,
		LogLevel:  defaultLogLevel,
	}
}

// Parse 在默认值之上解析命令行旗标。--log-dir 显式给出时照用（`--log-dir=` 即不落盘），
// 未给出时随 --data-dir 迁移。
func Parse(args []string) (Config, error) {
	cfg := Default()

	fs := flag.NewFlagSet("xgulu", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // 用法与错误由 main 负责呈现，避免重复输出
	addr := fs.String("addr", cfg.Addr, "监听地址")
	dataDir := fs.String("data-dir", cfg.DataDir, "数据目录")
	logDir := fs.String("log-dir", "", "日志目录；留空则不落盘（默认 <data-dir>/logs）")
	logFormat := fs.String("log-format", cfg.LogFormat, "日志格式：json | text")
	logLevel := fs.String("log-level", cfg.LogLevel, "日志级别：debug | info | warn | error")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("多余的参数: %s", strings.Join(fs.Args(), " "))
	}

	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })

	cfg.Addr = *addr
	cfg.DataDir = *dataDir
	cfg.LogFormat = *logFormat
	cfg.LogLevel = *logLevel
	if given["log-dir"] {
		cfg.LogDir = *logDir
	} else {
		cfg.LogDir = filepath.Join(cfg.DataDir, "logs")
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("--data-dir 不能为空")
	}
	if c.Addr == "" {
		return fmt.Errorf("--addr 不能为空")
	}
	switch c.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("--log-format 只能是 json 或 text，收到 %q", c.LogFormat)
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("--log-level 只能是 debug / info / warn / error，收到 %q", c.LogLevel)
	}
	return nil
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "." // 定位不到家目录时退回当前目录，至少不产生绝对路径写出
	}
	return home
}
