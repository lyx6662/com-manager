package handler

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"go.bug.st/serial"
)

// SystemHandler 系统管理处理器
type SystemHandler struct {
	cfgMgr     *config.Manager
	log        *logger.Logger
	restartFunc func()
}

// NewSystemHandler 创建系统处理器
func NewSystemHandler(cfgMgr *config.Manager, log *logger.Logger, restartFunc func()) *SystemHandler {
	return &SystemHandler{cfgMgr: cfgMgr, log: log, restartFunc: restartFunc}
}

// GetInfo 获取系统信息
func (h *SystemHandler) GetInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	cfg := h.cfgMgr.Get()

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"version":         "1.0.0",
			"name":            cfg.Server.Name,
			"go_version":      runtime.Version(),
			"goroutines":      runtime.NumGoroutine(),
			"cpu_count":       runtime.NumCPU(),
			"memory_alloc":    memStats.Alloc / 1024 / 1024,
			"memory_sys":      memStats.Sys / 1024 / 1024,
			"gc_count":        memStats.NumGC,
			"serial_devices":  len(cfg.SerialDevices),
			"network_devices": len(cfg.NetworkDevices),
			"tcp_outputs":     len(cfg.Outputs.ModbusTCPServers),
		},
	})
}

// GetConfig 获取系统配置
func (h *SystemHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": h.cfgMgr.Get(),
	})
}

// UpdateConfig 更新系统配置
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		LogLevel    string `json:"log_level"`
		Password    string `json:"password"`
		OldPassword string `json:"old_password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	cfg := h.cfgMgr.Get()

	// 处理密码修改
	if req.Password != "" {
		// 验证旧密码
		if req.OldPassword == "" || req.OldPassword != cfg.Web.Auth.Password {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "当前密码错误"})
			return
		}
		// 验证新密码长度
		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "新密码长度不能少于6位"})
			return
		}
		cfg.Web.Auth.Password = req.Password
		h.log.Info("密码已更新")
	}

	if req.Name != "" {
		cfg.Server.Name = req.Name
	}
	if req.LogLevel != "" {
		cfg.Server.LogLevel = req.LogLevel
	}

	if err := h.cfgMgr.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存配置失败: " + err.Error()})
		return
	}

	h.log.Info("更新系统配置")

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// ReloadConfig 热重载配置
func (h *SystemHandler) ReloadConfig(c *gin.Context) {
	h.log.Info("热重载配置请求")

	// 重新加载配置文件
	newCfg, err := config.Load("./configs/gateway.yaml")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载配置失败: " + err.Error()})
		return
	}

	// 更新配置管理器中的配置
	oldCfg := h.cfgMgr.Get()
	*oldCfg = *newCfg

	h.log.Info("配置热重载成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "重载成功，部分配置需要重启才能生效",
	})
}

// GetLogs 获取最近日志
func (h *SystemHandler) GetLogs(c *gin.Context) {
	logPath := "./logs"
	lines := 100

	if l := c.Query("lines"); l != "" {
		if v, err := parseInt(l); err == nil && v > 0 {
			lines = v
		}
	}

	// 查找最新的日志文件
	logFiles, err := filepath.Glob(filepath.Join(logPath, "*.log"))
	if err != nil || len(logFiles) == 0 {
		// 尝试读取默认日志
		logFiles, _ = filepath.Glob(filepath.Join(logPath, "*"))
	}

	if len(logFiles) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"logs":  []string{"暂无日志文件"},
				"path":  logPath,
				"count": 0,
			},
		})
		return
	}

	// 按修改时间排序，取最新的
	sort.Strings(logFiles)
	latestLog := logFiles[len(logFiles)-1]

	// 读取最后N行
	logLines, err := readLastLines(latestLog, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "读取日志失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"file":  latestLog,
			"lines": logLines,
			"count": len(logLines),
		},
	})
}

// Backup 备份配置
func (h *SystemHandler) Backup(c *gin.Context) {
	h.log.Info("备份配置")

	cfg := h.cfgMgr.Get()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "备份成功",
		"data":    cfg,
	})
}

// GetSerialPorts 获取系统串口列表
func (h *SystemHandler) GetSerialPorts(c *gin.Context) {
	ports, err := serial.GetPortsList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取串口列表失败: " + err.Error(),
		})
		return
	}

	// 获取已占用的串口
	cfg := h.cfgMgr.Get()
	usedPorts := make(map[string]bool)
	for _, dev := range cfg.SerialDevices {
		usedPorts[dev.Port] = true
	}

	// 构建串口列表
	type PortInfo struct {
		Name   string `json:"name"`
		Used   bool   `json:"used"`
		UsedBy string `json:"used_by,omitempty"`
	}

	portList := make([]PortInfo, 0, len(ports))
	for _, port := range ports {
		info := PortInfo{
			Name: port,
			Used: usedPorts[port],
		}
		// 查找使用该串口的设备
		if usedPorts[port] {
			for _, dev := range cfg.SerialDevices {
				if dev.Port == port {
					info.UsedBy = dev.Name
					break
				}
			}
		}
		portList = append(portList, info)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": portList,
	})
}

// Restore 恢复配置
func (h *SystemHandler) Restore(c *gin.Context) {
	var req config.Config
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 备份当前配置
	oldCfg := h.cfgMgr.Get()

	// 恢复配置
	*oldCfg = req

	// 保存到文件
	if err := h.cfgMgr.Save(); err != nil {
		// 回滚
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存配置失败: " + err.Error()})
		return
	}

	h.log.Info("配置恢复成功")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "恢复成功，建议重启程序使配置完全生效",
	})
}

// Restart 重启程序
func (h *SystemHandler) Restart(c *gin.Context) {
	h.log.Info("收到重启请求")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "程序即将重启，看门狗将在20秒后自动拉起",
	})

	// 延迟后执行重启（先让 HTTP 响应发送完成）
	if h.restartFunc != nil {
		go func() {
			time.Sleep(500 * time.Millisecond)
			h.restartFunc()
		}()
	}
}

// readLastLines 读取文件最后N行
func readLastLines(filename string, n int) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 返回最后N行
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines, nil
}

// parseInt 简单的字符串转整数
func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// 未使用的函数，但保留以备后用
var _ = strings.Contains
