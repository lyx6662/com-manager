package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"gopkg.in/yaml.v3"
)

const (
	checkInterval    = 5 * time.Second  // 检查间隔
	heartbeatTimeout = 20 * time.Second // 心跳超时阈值
	forceKillTimeout = 1 * time.Minute  // 进程存活但无心跳的强制杀掉阈值
)

// 简化配置结构，仅读取需要的字段
type config struct {
	OfflineBuffer struct {
		DBPath string `yaml:"db_path"`
	} `yaml:"offline_buffer"`
}

// info 同时输出到控制台和日志文件
func info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	log.Println(msg)
}

func main() {
	// 优先使用可执行文件所在目录，兼容编译后部署
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("获取可执行文件路径失败: %v\n", err)
		os.Exit(1)
	}
	baseDir := filepath.Dir(exePath)

	// go run 时可执行文件在临时目录，回退到工作目录
	if strings.Contains(baseDir, "go-build") || strings.Contains(baseDir, "Temp") {
		baseDir, _ = os.Getwd()
	}

	// 设置日志
	logDir := filepath.Join(baseDir, "logs")
	os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(
		filepath.Join(logDir, "watchdog.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		fmt.Printf("打开日志文件失败: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 读取配置
	cfgPath := filepath.Join(baseDir, "configs", "gateway.yaml")
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	dbPath := cfg.OfflineBuffer.DBPath
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(baseDir, dbPath)
	}

	// 主程序路径和 PID 文件
	mainExe := filepath.Join(baseDir, "com-manager.exe")
	pidFile := filepath.Join(baseDir, "data", "com-manager.pid")

	info("========================================")
	info("  看门狗已启动")
	info("  数据库: %s", dbPath)
	info("  主程序: %s", mainExe)
	info("  PID文件: %s", pidFile)
	info("========================================")

	// 记录心跳首次丢失的时间
	var stuckSince time.Time

	for {
		heartbeatOK := checkHeartbeat(dbPath)

		if heartbeatOK {
			if !stuckSince.IsZero() {
				info("[恢复] 心跳已恢复正常")
				stuckSince = time.Time{}
			}
			time.Sleep(checkInterval)
			continue
		}

		// 心跳超时，检查进程是否还活着
		pid := readPIDFile(pidFile)
		processAlive := pid > 0 && isProcessAlive(pid)

		if !processAlive {
			info("[启动] 主程序未运行，正在启动...")
			startMainProgram(mainExe, baseDir)
			stuckSince = time.Time{}
			time.Sleep(30 * time.Second)
			time.Sleep(checkInterval)
			continue
		}

		// 进程还在但心跳没了
		if stuckSince.IsZero() {
			stuckSince = time.Now()
			info("[警告] 心跳丢失但主程序仍在运行 (PID: %d)，等待恢复...", pid)
		}

		stuckDuration := time.Since(stuckSince)
		remaining := forceKillTimeout - stuckDuration

		if remaining <= 0 {
			info("[强制] %d 秒无心跳，终止进程 %d 并重启", int(stuckDuration.Seconds()), pid)
			killProcess(pid)
			time.Sleep(2 * time.Second)
			startMainProgram(mainExe, baseDir)
			stuckSince = time.Time{}
			time.Sleep(30 * time.Second)
		} else {
			info("[等待] 心跳丢失 %d 秒，剩余 %d 秒后强制重启", int(stuckDuration.Seconds()), int(remaining.Seconds()))
		}

		time.Sleep(checkInterval)
	}
}

// loadConfig 加载配置文件
func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// readPIDFile 读取 PID 文件
func readPIDFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// isProcessAlive 检查指定 PID 的进程是否在运行
func isProcessAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	if err != nil {
		return false
	}
	output := strings.TrimSpace(string(out))
	if len(output) == 0 || strings.Contains(output, "没有运行") || strings.Contains(output, "No tasks") {
		return false
	}
	return strings.Contains(output, strconv.Itoa(pid))
}

// killProcess 强制终止指定 PID 的进程
func killProcess(pid int) {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		info("[错误] 终止进程失败: %v, %s", err, strings.TrimSpace(string(output)))
	} else {
		info("[完成] 进程 %d 已终止", pid)
	}
}

// checkHeartbeat 检查心跳是否正常
func checkHeartbeat(dbPath string) bool {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false
	}
	defer db.Close()

	var ts int64
	err = db.QueryRow("SELECT updated_at FROM heartbeat WHERE id = 1").Scan(&ts)
	if err != nil {
		return false
	}

	if ts == 0 {
		return false
	}

	elapsed := time.Since(time.UnixMilli(ts))
	return elapsed < heartbeatTimeout
}

// startMainProgram 启动主程序
func startMainProgram(exePath, workDir string) {
	cmd := exec.Command(exePath)
	cmd.Dir = workDir

	if err := cmd.Start(); err != nil {
		info("[错误] 启动主程序失败: %v", err)
		return
	}

	info("[启动] 主程序已启动, PID: %d", cmd.Process.Pid)

	go func() {
		if err := cmd.Wait(); err != nil {
			info("[退出] 主程序退出: %v", err)
		} else {
			info("[退出] 主程序正常退出")
		}
	}()
}
