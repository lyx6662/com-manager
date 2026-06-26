package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// DataPoolProvider 数据池提供者接口
type DataPoolProvider interface {
	GetAllDataAsMap() []map[string]interface{}
	GetDeviceDataAsMap(deviceID string) []map[string]interface{}
	GetDataPointCount() int
	GetSubscriberCount() int
}

// DataPoolHandler 数据池处理器
type DataPoolHandler struct {
	log  *logger.Logger
	pool DataPoolProvider
}

// NewDataPoolHandler 创建数据池处理器
func NewDataPoolHandler(log *logger.Logger, pool DataPoolProvider) *DataPoolHandler {
	return &DataPoolHandler{
		log:  log,
		pool: pool,
	}
}

// GetAll 获取所有数据池数据
func (h *DataPoolHandler) GetAll(c *gin.Context) {
	if h.pool == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    nil,
		})
		return
	}

	data := h.pool.GetAllDataAsMap()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"points":           data,
			"total":            len(data),
			"subscriber_count": h.pool.GetSubscriberCount(),
		},
	})
}

// GetByDevice 获取指定设备的数据
func (h *DataPoolHandler) GetByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备ID不能为空",
		})
		return
	}

	if h.pool == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    nil,
		})
		return
	}

	data := h.pool.GetDeviceDataAsMap(deviceID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device_id": deviceID,
			"points":    data,
			"total":     len(data),
		},
	})
}

// GetStats 获取数据池统计信息
func (h *DataPoolHandler) GetStats(c *gin.Context) {
	if h.pool == nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"total_points":     0,
				"subscriber_count": 0,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total_points":     h.pool.GetDataPointCount(),
			"subscriber_count": h.pool.GetSubscriberCount(),
		},
	})
}
