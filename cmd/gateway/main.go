package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lyx6662/com-manager/internal/core"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

func main() {
	// 加载配置
	cfgMgr, err := config.NewManager("./configs/gateway.yaml")
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	cfg := cfgMgr.Get()

	// 初始化日志
	log, err := logger.New(cfg.Server.LogLevel, cfg.Server.LogPath)
	if err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("通讯管理机启动中...",
		"name", cfg.Server.Name,
		"version", "1.0.0",
	)

	// 创建核心引擎
	engine, err := core.NewEngine(cfgMgr, log)
	if err != nil {
		log.Fatal("创建引擎失败", "error", err)
	}

	// 启动引擎
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		log.Fatal("启动引擎失败", "error", err)
	}

	log.Info("通讯管理机启动成功")

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("收到退出信号，正在关闭...")
	engine.Stop()
	log.Info("通讯管理机已停止")
}
