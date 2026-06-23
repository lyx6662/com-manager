package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"

	"github.com/lyx6662/com-manager/internal/core"
	"github.com/lyx6662/com-manager/internal/web"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

//go:embed all:web/frontend
var frontendFS embed.FS

func main() {
	// 启用内存访问错误转为 panic（用于捕获 C 库崩溃）
	debug.SetPanicOnFault(true)
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			fmt.Fprintf(os.Stderr, "\n========== 程序崩溃 ==========\n")
			fmt.Fprintf(os.Stderr, "错误: %v\n", r)
			fmt.Fprintf(os.Stderr, "堆栈:\n%s\n", stack)
			fmt.Fprintf(os.Stderr, "================================\n")
			// 同时写入日志文件
			os.MkdirAll("./logs", 0755)
			f, err := os.OpenFile("./logs/crash.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				fmt.Fprintf(f, "\n[%s] ========== 程序崩溃 ==========\n", fmt.Sprintf("%v", r))
				fmt.Fprintf(f, "错误: %v\n", r)
				fmt.Fprintf(f, "堆栈:\n%s\n", stack)
				fmt.Fprintf(f, "================================\n")
				f.Close()
			}
			os.Exit(1)
		}
	}()

	// 写入 PID 文件（供看门狗检测进程是否存在）
	os.MkdirAll("./data", 0755)
	pidFile := "./data/com-manager.pid"
	writePIDFile(pidFile)
	defer os.Remove(pidFile)

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

	// 设置内嵌前端资源
	web.FrontendFS = frontendFS

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

// writePIDFile 将当前进程 PID 写入文件
func writePIDFile(path string) {
	pid := os.Getpid()
	os.WriteFile(path, []byte(strconv.Itoa(pid)), 0644)
}
