package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// MonitorCollector 监控用采集器接口
type MonitorCollector interface {
	GetDeviceStatus(deviceID string) interface{}
	GetAllDeviceStatus() map[string]interface{}
}

// MonitorRouter 监控用路由器接口
type MonitorRouter interface {
	GetDeviceStatus() map[string]map[string]interface{}
}

// MonitorHandler 实时监控处理器
type MonitorHandler struct {
	log       *logger.Logger
	collector MonitorCollector
	router    MonitorRouter
}

// NewMonitorHandler 创建监控处理器
func NewMonitorHandler(log *logger.Logger, collector interface{}, router interface{}) *MonitorHandler {
	h := &MonitorHandler{log: log}

	if c, ok := collector.(MonitorCollector); ok {
		h.collector = c
	}
	if r, ok := router.(MonitorRouter); ok {
		h.router = r
	}

	return h
}

// GetRealtime 获取实时数据
func (h *MonitorHandler) GetRealtime(c *gin.Context) {
	devices := make([]gin.H, 0)

	// 获取设备状态
	if h.collector != nil {
		statuses := h.collector.GetAllDeviceStatus()
		for id, status := range statuses {
			deviceData := gin.H{
				"device_id": id,
				"status":    status,
			}

			// 获取该设备的实时数据
			if h.router != nil {
				cache := h.router.GetDeviceStatus()
				if points, ok := cache[id]; ok {
					deviceData["points"] = points
				}
			}

			devices = append(devices, deviceData)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"devices": devices,
		},
	})
}

// GetDeviceData 获取单设备实时数据
func (h *MonitorHandler) GetDeviceData(c *gin.Context) {
	deviceID := c.Param("id")

	result := gin.H{
		"device_id": deviceID,
	}

	// 获取设备状态
	if h.collector != nil {
		status := h.collector.GetDeviceStatus(deviceID)
		result["status"] = status
	}

	// 获取设备数据
	if h.router != nil {
		cache := h.router.GetDeviceStatus()
		if points, ok := cache[deviceID]; ok {
			result["points"] = points
		} else {
			result["points"] = []gin.H{}
		}
	} else {
		result["points"] = []gin.H{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": result,
	})
}

// GetDeviceStatus 获取所有设备状态
func (h *MonitorHandler) GetDeviceStatus(c *gin.Context) {
	total := 0
	online := 0
	offline := 0

	if h.collector != nil {
		statuses := h.collector.GetAllDeviceStatus()
		total = len(statuses)
		for _, status := range statuses {
			if s, ok := status.(interface{ IsOnline() bool }); ok {
				if s.IsOnline() {
					online++
				} else {
					offline++
				}
			} else {
				offline++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"total":   total,
			"online":  online,
			"offline": offline,
		},
	})
}
