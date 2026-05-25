package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// MappingHandler 点表管理处理器
type MappingHandler struct {
	log *logger.Logger
}

// NewMappingHandler 创建点表处理器
func NewMappingHandler(log *logger.Logger) *MappingHandler {
	return &MappingHandler{log: log}
}

// PointEntryResponse 点表条目响应
type PointEntryResponse struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Register    int     `json:"register"`
	DataType    string  `json:"data_type"`
	Scale       float64 `json:"scale"`
	Offset      float64 `json:"offset"`
	Unit        string  `json:"unit"`
}

// UpdateMappingRequest 更新点表请求
type UpdateMappingRequest struct {
	Points []PointEntryResponse `json:"points"`
}

// Get 获取设备点表
func (h *MappingHandler) Get(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// TODO: 从配置读取点表
	_ = deviceID

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"points":    []PointEntryResponse{},
		},
	})
}

// Update 更新设备点表
func (h *MappingHandler) Update(c *gin.Context) {
	deviceID := c.Param("deviceId")

	var req UpdateMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	h.log.Info("更新点表", "device_id", deviceID, "points", len(req.Points))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Export 导出点表
func (h *MappingHandler) Export(c *gin.Context) {
	deviceID := c.Param("deviceId")
	format := c.DefaultQuery("format", "json")

	// TODO: 导出点表
	_ = deviceID
	_ = format

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []PointEntryResponse{},
	})
}

// Import 导入点表
func (h *MappingHandler) Import(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// TODO: 导入点表
	_ = deviceID

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "导入成功",
	})
}
