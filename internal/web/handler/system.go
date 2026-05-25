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
	cfg *config.Config
	log *logger.Logger
}

// NewSystemHandler 创建系统处理器
func NewSystemHandler(cfg *config.Config, log *logger.Logger) *SystemHandler {
	return &SystemHandler{cfg: cfg, log: log}
}

// GetInfo 获取系统信息
func (h *SystemHandler) GetInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"version":         "1.0.0",
			"name":            h.cfg.Server.Name,
			"go_version":      runtime.Version(),
			"goroutines":      runtime.NumGoroutine(),
			"cpu_count":       runtime.NumCPU(),
			"memory_alloc":    memStats.Alloc / 1024 / 1024,
			"memory_sys":      memStats.Sys / 1024 / 1024,
			"gc_count":        memStats.NumGC,
		},
	})
}

// GetConfig 获取系统配置
func (h *SystemHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": h.cfg,
	})
}

// UpdateConfig 更新系统配置
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	// TODO: 更新配置并保存
	h.log.Info("更新系统配置")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// ReloadConfig 热重载配置
func (h *SystemHandler) ReloadConfig(c *gin.Context) {
	h.log.Info("热重载配置")

	// TODO: 通知引擎重载配置

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "重载成功",
	})
}

// GetLogs 获取日志
func (h *SystemHandler) GetLogs(c *gin.Context) {
	// TODO: 读取最近日志
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []string{},
	})
}

// Backup 备份配置
func (h *SystemHandler) Backup(c *gin.Context) {
	// TODO: 打包配置文件
	h.log.Info("备份配置")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "备份成功",
		"data": gin.H{
			"file": "backup.zip",
		},
	})
}

// Restore 恢复配置
func (h *SystemHandler) Restore(c *gin.Context) {
	// TODO: 恢复配置
	h.log.Info("恢复配置")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "恢复成功",
	})
}
