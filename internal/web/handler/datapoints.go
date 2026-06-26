package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// DataPointsHandler 采集点表管理处理器
type DataPointsHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewDataPointsHandler 创建采集点表处理器
func NewDataPointsHandler(cfgMgr *config.Manager, log *logger.Logger) *DataPointsHandler {
	return &DataPointsHandler{cfgMgr: cfgMgr, log: log}
}

// DataPointRequest 采集点请求
type DataPointRequest struct {
	Name           string `json:"name"`
	SourceDevice   string `json:"source_device"`
	SourceRegister uint16 `json:"source_register"`
	SourceType     string `json:"source_type"`
	DataType       string `json:"data_type"`
	RegisterCount  int    `json:"register_count"`
	ByteOrder      string `json:"byte_order"`
}

// GetByDevice 获取指定设备的采集点表
func (h *DataPointsHandler) GetByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	points := h.cfgMgr.GetDataPoints(deviceID)
	if points == nil {
		points = []config.DataPointDef{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"points":    points,
		},
	})
}

// UpdateByDevice 更新指定设备的采集点表
func (h *DataPointsHandler) UpdateByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	var req []DataPointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 检查设备是否存在
	dev := h.cfgMgr.GetSerialDevice(deviceID)
	if dev == nil {
		dev2 := h.cfgMgr.GetNetworkDevice(deviceID)
		if dev2 == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不存在"})
			return
		}
	}

	// 转换为配置格式
	points := make([]config.DataPointDef, 0, len(req))
	for _, r := range req {
		points = append(points, config.DataPointDef{
			Name:           r.Name,
			SourceDevice:   deviceID,
			SourceRegister: r.SourceRegister,
			SourceType:     r.SourceType,
			DataType:       r.DataType,
			RegisterCount:  r.RegisterCount,
			ByteOrder:      r.ByteOrder,
		})
	}

	if err := h.cfgMgr.SetDataPoints(deviceID, points); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新采集点表", "device_id", deviceID, "points", len(points))

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteByDevice 删除指定设备的采集点表
func (h *DataPointsHandler) DeleteByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	if err := h.cfgMgr.DeleteDataPoints(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}

	h.log.Info("删除采集点表", "device_id", deviceID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// OutputMappingsHandler 输出映射管理处理器
type OutputMappingsHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewOutputMappingsHandler 创建输出映射处理器
func NewOutputMappingsHandler(cfgMgr *config.Manager, log *logger.Logger) *OutputMappingsHandler {
	return &OutputMappingsHandler{cfgMgr: cfgMgr, log: log}
}

// OutputMappingRequest 输出映射请求
type OutputMappingRequest struct {
	Name           string  `json:"name"`
	TargetRegister uint16  `json:"target_register"`
	Scale          float64 `json:"scale"`
	Offset         float64 `json:"offset"`
	Unit           string  `json:"unit"`
	MaxPoints      int     `json:"max_points"`
	HighLimit      float64 `json:"high_limit"`
	LowLimit       float64 `json:"low_limit"`
}

// GetAll 获取所有输出映射
func (h *OutputMappingsHandler) GetAll(c *gin.Context) {
	mappings := h.cfgMgr.GetOutputMappings()
	if mappings == nil {
		mappings = make(map[string][]config.OutputMappingDef)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": mappings,
	})
}

// GetByDevice 获取指定设备的输出映射
func (h *OutputMappingsHandler) GetByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	mappings := h.cfgMgr.GetOutputMappingsByDevice(deviceID)
	if mappings == nil {
		mappings = []config.OutputMappingDef{}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"mappings":  mappings,
		},
	})
}

// UpdateByDevice 更新指定设备的输出映射
func (h *OutputMappingsHandler) UpdateByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	var req []OutputMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 转换为配置格式
	mappings := make([]config.OutputMappingDef, 0, len(req))
	for _, r := range req {
		mappings = append(mappings, config.OutputMappingDef{
			Name:           r.Name,
			TargetRegister: r.TargetRegister,
			Scale:          r.Scale,
			Offset:         r.Offset,
			Unit:           r.Unit,
			MaxPoints:      r.MaxPoints,
			HighLimit:      r.HighLimit,
			LowLimit:       r.LowLimit,
		})
	}

	if err := h.cfgMgr.SetOutputMappingsByDevice(deviceID, mappings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新输出映射", "device_id", deviceID, "mappings", len(mappings))

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteByDevice 删除指定设备的输出映射
func (h *OutputMappingsHandler) DeleteByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	if err := h.cfgMgr.DeleteOutputMappingsByDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}

	h.log.Info("删除输出映射", "device_id", deviceID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}
