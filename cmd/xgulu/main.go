// Command xgulu 是 x-gulu 的唯一可执行入口。
// 它只做顺序编排：解析旗标 → 建日志 → 开库 → 跑迁移 → 装配 → 服务 → 优雅关闭
// （副作用顺序写在 main 里，装配在 internal/app，见 02-后端设计 §3.1）。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/xgulu/internal/app"
	"github.com/xgulu/internal/config"
	"github.com/xgulu/internal/logging"
	"github.com/xgulu/internal/migrate"
	"github.com/xgulu/internal/store"
	"github.com/xgulu/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "xgulu 启动失败：%v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := config.Parse(args)
	if err != nil {
		return err
	}

	logger, logCloser, err := logging.New(cfg)
	if err != nil {
		return err
	}

	st, err := store.Open(cfg)
	if err != nil {
		return errors.Join(err, logCloser.Close())
	}

	application, err := app.New(cfg, app.Deps{Logger: logger, LogCloser: logCloser, Store: st})
	if err != nil {
		return errors.Join(err, st.Close(), logCloser.Close())
	}

	applied, err := migrate.Run(ctx, st.DB(), migrations.FS)
	if err != nil {
		// 迁移失败即拒绝启动并给出可照做的指引，不做半迁移（02-后端设计 §2）
		return errors.Join(
			fmt.Errorf("数据库迁移失败，服务未启动（数据文件：%s）: %w", cfg.DBPath(), err),
			application.Close(),
		)
	}
	if applied > 0 {
		logger.Info("数据库迁移完成", "applied", applied, "db", cfg.DBPath())
	}

	runErr := application.Run(ctx)
	return errors.Join(runErr, application.Close())
}
