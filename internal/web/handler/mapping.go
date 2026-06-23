package handler

import (
	"encoding/csv"
	"fmt"
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

// MappingRuleResponse 点表映射响应
type MappingRuleResponse struct {
	Name           string  `json:"name"`
	SourceDevice   string  `json:"source_device"`
	SourceRegister uint16  `json:"source_register"`
	SourceType     string  `json:"source_type"`
	DataType       string  `json:"data_type"`
	RegisterCount  int     `json:"register_count"`
	TargetRegister uint16  `json:"target_register"`
	Scale          float64 `json:"scale"`
	Offset         float64 `json:"offset"`
	Unit           string  `json:"unit"`
}

// UpdateMappingRequest 更新点表请求
type UpdateMappingRequest struct {
	Rules []MappingRuleResponse `json:"rules"`
}

// Get 获取设备点表
func (h *MappingHandler) Get(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// 检查设备是否存在
	dev := h.cfgMgr.GetSerialDevice(deviceID)
	if dev == nil {
		dev2 := h.cfgMgr.GetNetworkDevice(deviceID)
		if dev2 == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不存在"})
			return
		}
	}

	// 直接获取该设备的映射规则
	deviceRules := h.cfgMgr.GetMappingRules(deviceID)
	rules := make([]MappingRuleResponse, 0, len(deviceRules))

	for _, rule := range deviceRules {
		rules = append(rules, MappingRuleResponse{
			Name:           rule.Name,
			SourceDevice:   rule.SourceDevice,
			SourceRegister: rule.SourceRegister,
			SourceType:     rule.SourceType,
			DataType:       rule.DataType,
			RegisterCount:  rule.GetRegisterCount(),
			TargetRegister: rule.TargetRegister,
			Scale:          rule.Scale,
			Offset:         rule.Offset,
			Unit:           rule.Unit,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"rules":     rules,
		},
	})
}

// Update 更新设备点表
func (h *MappingHandler) Update(c *gin.Context) {
	deviceID := c.Param("deviceId")

	var req UpdateMappingRequest
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

	// 将规则转换为配置格式
	configRules := make([]config.MappingRule, 0, len(req.Rules))
	for _, r := range req.Rules {
		configRules = append(configRules, config.MappingRule{
			Name:            r.Name,
			SourceDevice:    deviceID, // 自动填充设备ID
			SourceRegister:  r.SourceRegister,
			SourceType:      r.SourceType,
			DataType:        r.DataType,
			RegisterCount:   r.RegisterCount,
			TargetRegister:  r.TargetRegister,
			Scale:           r.Scale,
			Offset:          r.Offset,
			Unit:            r.Unit,
		})
	}

	// 直接更新该设备的映射规则
	if err := h.cfgMgr.SetMappingRules(deviceID, configRules); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新点表", "device_id", deviceID, "rules", len(req.Rules))

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// Export 导出点表
func (h *MappingHandler) Export(c *gin.Context) {
	deviceID := c.Param("deviceId")
	format := c.DefaultQuery("format", "json")

	// 获取该设备的映射规则
	deviceRules := h.cfgMgr.GetMappingRules(deviceID)
	rules := make([]MappingRuleResponse, 0, len(deviceRules))

	for _, rule := range deviceRules {
		rules = append(rules, MappingRuleResponse{
			Name:           rule.Name,
			SourceDevice:   rule.SourceDevice,
			SourceRegister: rule.SourceRegister,
			SourceType:     rule.SourceType,
			DataType:       rule.DataType,
			RegisterCount:  rule.GetRegisterCount(),
			TargetRegister: rule.TargetRegister,
			Scale:          rule.Scale,
			Offset:         rule.Offset,
			Unit:           rule.Unit,
		})
	}

	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_mapping.csv", deviceID))

		// 写入 UTF-8 BOM
		c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

		writer := csv.NewWriter(c.Writer)
		writer.Write([]string{"name", "source_device", "source_register", "source_type", "data_type", "register_count", "target_register", "scale", "offset", "unit"})
		for _, r := range rules {
			writer.Write([]string{
				r.Name,
				r.SourceDevice,
				strconv.Itoa(int(r.SourceRegister)),
				r.SourceType,
				r.DataType,
				strconv.Itoa(r.RegisterCount),
				strconv.Itoa(int(r.TargetRegister)),
				strconv.FormatFloat(r.Scale, 'f', -1, 64),
				strconv.FormatFloat(r.Offset, 'f', -1, 64),
				r.Unit,
			})
		}
		writer.Flush()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"device_id": deviceID,
			"rules":     rules,
		},
	})
}

// Import 导入点表
func (h *MappingHandler) Import(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请上传文件"})
		return
	}
	defer file.Close()

	h.log.Info("导入点表", "device_id", deviceID, "filename", header.Filename)

	// 解析 CSV
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "解析CSV失败: " + err.Error()})
		return
	}

	if len(records) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "CSV文件为空或只有表头"})
		return
	}

	// 跳过表头，解析数据行
	configRules := make([]config.MappingRule, 0, len(records)-1)
	for _, row := range records[1:] {
		if len(row) < 10 {
			continue
		}

		srcReg, _ := strconv.ParseUint(row[2], 10, 16)
		regCount, _ := strconv.Atoi(row[5])
		tgtReg, _ := strconv.ParseUint(row[6], 10, 16)
		scale, _ := strconv.ParseFloat(row[7], 64)
		offset, _ := strconv.ParseFloat(row[8], 64)

		configRules = append(configRules, config.MappingRule{
			Name:            row[0],
			SourceDevice:    row[1],
			SourceRegister:  uint16(srcReg),
			SourceType:      row[3],
			DataType:        row[4],
			RegisterCount:   regCount,
			TargetRegister:  uint16(tgtReg),
			Scale:           scale,
			Offset:          offset,
			Unit:            row[9],
		})
	}

	// 保存到配置
	if len(configRules) > 0 {
		h.cfgMgr.SetMappingRules(deviceID, configRules)
	}

	h.log.Info("导入点表成功", "device_id", deviceID, "rules", len(configRules))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "导入成功",
		"data": gin.H{
			"imported": len(configRules),
		},
	})
}
