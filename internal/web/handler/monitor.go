package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// MonitorHandler 实时监控处理器
type MonitorHandler struct {
	log *logger.Logger
}

// NewMonitorHandler 创建监控处理器
func NewMonitorHandler(log *logger.Logger) *MonitorHandler {
	return &MonitorHandler{log: log}
}

// GetRealtime 获取实时数据
func (h *MonitorHandler) GetRealtime(c *gin.Context) {
	// TODO: 返回实时数据
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"devices": []gin.H{},
		},
	})
}

// GetDeviceData 获取单设备实时数据
func (h *MonitorHandler) GetDeviceData(c *gin.Context) {
	deviceID := c.Param("id")
	_ = deviceID

	// TODO: 查询设备实时数据
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"points":    []gin.H{},
		},
	})
}

// GetDeviceStatus 获取所有设备状态
func (h *MonitorHandler) GetDeviceStatus(c *gin.Context) {
	// TODO: 查询所有设备状态
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"total":   0,
			"online":  0,
			"offline": 0,
		},
	})
}
