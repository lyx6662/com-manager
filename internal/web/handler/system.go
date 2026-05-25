package handler

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// SystemHandler 系统管理处理器
type SystemHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewSystemHandler 创建系统处理器
func NewSystemHandler(cfgMgr *config.Manager, log *logger.Logger) *SystemHandler {
	return &SystemHandler{cfgMgr: cfgMgr, log: log}
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
		Name     string `json:"name"`
		LogLevel string `json:"log_level"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	cfg := h.cfgMgr.Get()
	if req.Name != "" {
		cfg.Server.Name = req.Name
	}
	if req.LogLevel != "" {
		cfg.Server.LogLevel = req.LogLevel
	}

	if err := h.cfgMgr.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存配置失败: " + err.Error(),
		})
		return
	}

	h.log.Info("更新系统配置")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// ReloadConfig 热重载配置
func (h *SystemHandler) ReloadConfig(c *gin.Context) {
	h.log.Info("热重载配置")

	// 重新加载配置
	if err := h.cfgMgr.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "重载失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "重载成功",
	})
}

// GetLogs 获取日志
func (h *SystemHandler) GetLogs(c *gin.Context) {
	// TODO: 读取最近日志文件
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []string{"日志功能开发中"},
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

// Restore 恢复配置
func (h *SystemHandler) Restore(c *gin.Context) {
	h.log.Info("恢复配置")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "恢复功能开发中",
	})
}
