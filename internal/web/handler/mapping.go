package handler

import (
	"encoding/csv"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// MappingHandler 点表管理处理器
type MappingHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewMappingHandler 创建点表处理器
func NewMappingHandler(cfgMgr *config.Manager, log *logger.Logger) *MappingHandler {
	return &MappingHandler{cfgMgr: cfgMgr, log: log}
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

	// 检查设备是否存在
	dev := h.cfgMgr.GetSerialDevice(deviceID)
	if dev == nil {
		dev2 := h.cfgMgr.GetNetworkDevice(deviceID)
		if dev2 == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "设备不存在",
			})
			return
		}
	}

	// TODO: 从独立的点表文件读取
	points := make([]PointEntryResponse, 0)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"points":    points,
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

	// TODO: 保存到独立的点表文件
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

	points := make([]PointEntryResponse, 0)

	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename="+deviceID+"_mapping.csv")

		writer := csv.NewWriter(c.Writer)
		writer.Write([]string{"name", "description", "register", "data_type", "scale", "offset", "unit"})
		for _, p := range points {
			writer.Write([]string{
				p.Name,
				p.Description,
				strconv.Itoa(p.Register),
				p.DataType,
				strconv.FormatFloat(p.Scale, 'f', -1, 64),
				strconv.FormatFloat(p.Offset, 'f', -1, 64),
				p.Unit,
			})
		}
		writer.Flush()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"points":    points,
		},
	})
}

// Import 导入点表
func (h *MappingHandler) Import(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// TODO: 解析上传的CSV/JSON文件
	h.log.Info("导入点表", "device_id", deviceID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "导入功能开发中",
	})
}
