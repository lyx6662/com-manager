package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// IEC61850Handler IEC 61850 管理处理器
type IEC61850Handler struct {
	cfgMgr      *config.Manager
	log         *logger.Logger
	restartFunc func()
}

// NewIEC61850Handler 创建 IEC 61850 处理器
func NewIEC61850Handler(cfgMgr *config.Manager, log *logger.Logger, restartFunc func()) *IEC61850Handler {
	return &IEC61850Handler{cfgMgr: cfgMgr, log: log, restartFunc: restartFunc}
}

// GetConfig 获取 IEC 61850 完整配置
func (h *IEC61850Handler) GetConfig(c *gin.Context) {
	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"iec61850": gin.H{
					"enabled":         false,
					"port":            102,
					"ied_name":        "GRID_GATEWAY",
					"max_connections": 10,
				},
				"model":    gin.H{"logical_devices": []interface{}{}},
				"mappings": []interface{}{},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"iec61850": gin.H{
				"enabled":         cfg.IEC61850.Enabled,
				"port":            cfg.IEC61850.Port,
				"ied_name":        cfg.IEC61850.IEDName,
				"max_connections": cfg.IEC61850.MaxConnections,
				"icd_output":      cfg.IEC61850.ICDOutput,
			},
			"model":    cfg.Model,
			"mappings": cfg.Mappings,
		},
	})
}

// UpdateServerConfig 更新 IEC 61850 服务器配置
func (h *IEC61850Handler) UpdateServerConfig(c *gin.Context) {
	var req struct {
		Enabled        *bool  `json:"enabled"`
		Port           int    `json:"port"`
		IEDName        string `json:"ied_name"`
		MaxConnections int    `json:"max_connections"`
		ICDOutput      string `json:"icd_output"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		// 创建默认配置
		cfg = &config.ModbusToIEC61850Config{
			IEC61850: config.IEC61850Config{
				Port:           102,
				IEDName:        "GRID_GATEWAY",
				MaxConnections: 10,
			},
		}
	}

	if req.Enabled != nil {
		cfg.IEC61850.Enabled = *req.Enabled
	}
	if req.Port > 0 {
		cfg.IEC61850.Port = req.Port
	}
	if req.IEDName != "" {
		cfg.IEC61850.IEDName = req.IEDName
	}
	if req.MaxConnections > 0 {
		cfg.IEC61850.MaxConnections = req.MaxConnections
	}
	if req.ICDOutput != "" {
		cfg.IEC61850.ICDOutput = req.ICDOutput
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新 IEC 61850 服务器配置")
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// Restart 重启 IEC 61850 服务
func (h *IEC61850Handler) Restart(c *gin.Context) {
	if h.restartFunc != nil {
		h.restartFunc()
		h.log.Info("重启 IEC 61850 服务")
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "重启指令已发送"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "重启功能不可用"})
	}
}

// GetModel 获取数据模型
func (h *IEC61850Handler) GetModel(c *gin.Context) {
	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{"logical_devices": []interface{}{}},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": cfg.Model,
	})
}

// UpdateModel 更新数据模型
func (h *IEC61850Handler) UpdateModel(c *gin.Context) {
	var req config.IEC61850ModelConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		cfg = &config.ModbusToIEC61850Config{}
	}

	cfg.Model = req

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新 IEC 61850 数据模型")
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// AddLogicalDevice 新增逻辑设备
func (h *IEC61850Handler) AddLogicalDevice(c *gin.Context) {
	var req config.LogicalDeviceConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "逻辑设备名称不能为空"})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		cfg = &config.ModbusToIEC61850Config{}
	}

	// 检查是否已存在
	for _, ld := range cfg.Model.LogicalDevices {
		if ld.Name == req.Name {
			c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "逻辑设备已存在: " + req.Name})
			return
		}
	}

	cfg.Model.LogicalDevices = append(cfg.Model.LogicalDevices, req)

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("新增逻辑设备", "name", req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功"})
}

// UpdateLogicalDevice 更新逻辑设备
func (h *IEC61850Handler) UpdateLogicalDevice(c *gin.Context) {
	name := c.Param("name")

	var req config.LogicalDeviceConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == name {
			// 如果名称改变了，需要更新映射规则中的路径
			if name != req.Name {
				oldPrefix := name + "/"
				newPrefix := req.Name + "/"
				for j, rule := range cfg.Mappings {
					if strings.HasPrefix(rule.IEC61850Path, oldPrefix) {
						cfg.Mappings[j].IEC61850Path = newPrefix + rule.IEC61850Path[len(oldPrefix):]
					}
				}
			}
			cfg.Model.LogicalDevices[i] = req
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑设备不存在: " + name})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新逻辑设备", "name", name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteLogicalDevice 删除逻辑设备
func (h *IEC61850Handler) DeleteLogicalDevice(c *gin.Context) {
	name := c.Param("name")

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == name {
			cfg.Model.LogicalDevices = append(cfg.Model.LogicalDevices[:i], cfg.Model.LogicalDevices[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑设备不存在: " + name})
		return
	}

	// 删除引用该逻辑设备的映射规则
	devicePrefix := name + "/"
	var validMappings []config.IEC61850MappingRule
	for _, rule := range cfg.Mappings {
		if !strings.HasPrefix(rule.IEC61850Path, devicePrefix) {
			validMappings = append(validMappings, rule)
		}
	}
	cfg.Mappings = validMappings

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("删除逻辑设备", "name", name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// AddLogicalNode 新增逻辑节点
func (h *IEC61850Handler) AddLogicalNode(c *gin.Context) {
	deviceName := c.Param("name")

	var req config.LogicalNodeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "逻辑节点名称不能为空"})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			// 检查节点是否已存在
			for _, ln := range ld.LogicalNodes {
				if ln.Name == req.Name {
					c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "逻辑节点已存在: " + req.Name})
					return
				}
			}
			cfg.Model.LogicalDevices[i].LogicalNodes = append(cfg.Model.LogicalDevices[i].LogicalNodes, req)
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑设备不存在: " + deviceName})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("新增逻辑节点", "device", deviceName, "node", req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功"})
}

// CopyLogicalNode 复制逻辑节点
func (h *IEC61850Handler) CopyLogicalNode(c *gin.Context) {
	deviceName := c.Param("name")
	sourceNodeName := c.Param("node")

	var req struct {
		TargetNames []string `json:"target_names" binding:"required"` // 目标节点名称列表
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if len(req.TargetNames) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "目标节点名称不能为空"})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	// 查找源节点
	var sourceNode *config.LogicalNodeConfig
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == sourceNodeName {
					sourceNode = &cfg.Model.LogicalDevices[i].LogicalNodes[j]
					break
				}
			}
			break
		}
	}

	if sourceNode == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "源逻辑节点不存在: " + sourceNodeName})
		return
	}

	// 查找逻辑设备索引
	deviceIdx := -1
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			deviceIdx = i
			break
		}
	}

	if deviceIdx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑设备不存在: " + deviceName})
		return
	}

	// 复制节点
	created := 0
	for _, targetName := range req.TargetNames {
		if targetName == "" {
			continue
		}
		// 检查是否已存在
		exists := false
		for _, ln := range cfg.Model.LogicalDevices[deviceIdx].LogicalNodes {
			if ln.Name == targetName {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		// 深拷贝源节点
		newNode := deepCopyLogicalNode(*sourceNode, targetName)
		cfg.Model.LogicalDevices[deviceIdx].LogicalNodes = append(
			cfg.Model.LogicalDevices[deviceIdx].LogicalNodes, newNode)
		created++
	}

	if created == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "所有目标节点名称已存在"})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("复制逻辑节点", "device", deviceName, "source", sourceNodeName, "created", created)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": fmt.Sprintf("成功复制 %d 个节点", created),
		"data": gin.H{"created": created},
	})
}

// deepCopyLogicalNode 深拷贝逻辑节点
func deepCopyLogicalNode(source config.LogicalNodeConfig, newName string) config.LogicalNodeConfig {
	newNode := config.LogicalNodeConfig{
		Name:        newName,
		DataObjects: make([]config.DataObjectConfig, len(source.DataObjects)),
	}
	for i, do := range source.DataObjects {
		newNode.DataObjects[i] = deepCopyDataObject(do)
	}
	return newNode
}

// deepCopyDataObject 深拷贝数据对象
func deepCopyDataObject(source config.DataObjectConfig) config.DataObjectConfig {
	newDO := config.DataObjectConfig{
		Name:           source.Name,
		DataAttributes: make([]config.DataAttributeConfig, len(source.DataAttributes)),
	}
	for i, da := range source.DataAttributes {
		newDO.DataAttributes[i] = deepCopyDataAttribute(da)
	}
	return newDO
}

// deepCopyDataAttribute 深拷贝数据属性
func deepCopyDataAttribute(source config.DataAttributeConfig) config.DataAttributeConfig {
	newDA := config.DataAttributeConfig{
		Name:     source.Name,
		Type:     source.Type,
		FC:       source.FC,
		Triggers: source.Triggers,
	}
	if source.Children != nil {
		newDA.Children = make([]config.DataAttributeConfig, len(source.Children))
		for i, child := range source.Children {
			newDA.Children[i] = deepCopyDataAttribute(child)
		}
	}
	return newDA
}

// UpdateLogicalNode 更新逻辑节点
func (h *IEC61850Handler) UpdateLogicalNode(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")

	var req config.LogicalNodeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					cfg.Model.LogicalDevices[i].LogicalNodes[j] = req
					found = true
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑节点不存在"})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新逻辑节点", "device", deviceName, "node", nodeName)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteLogicalNode 删除逻辑节点
func (h *IEC61850Handler) DeleteLogicalNode(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					cfg.Model.LogicalDevices[i].LogicalNodes = append(
						cfg.Model.LogicalDevices[i].LogicalNodes[:j],
						cfg.Model.LogicalDevices[i].LogicalNodes[j+1:]...,
					)
					found = true
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑节点不存在"})
		return
	}

	// 删除引用该节点的映射规则
	nodePrefix := deviceName + "/" + nodeName + "."
	var validMappings []config.IEC61850MappingRule
	for _, rule := range cfg.Mappings {
		if !strings.HasPrefix(rule.IEC61850Path, nodePrefix) {
			validMappings = append(validMappings, rule)
		}
	}
	cfg.Mappings = validMappings

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("删除逻辑节点", "device", deviceName, "node", nodeName)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// AddDataObject 新增数据对象
func (h *IEC61850Handler) AddDataObject(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")

	var req config.DataObjectConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "数据对象名称不能为空"})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects = append(cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects, req)
					found = true
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "逻辑节点不存在"})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("新增数据对象", "device", deviceName, "node", nodeName, "object", req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功"})
}

// UpdateDataObject 更新数据对象
func (h *IEC61850Handler) UpdateDataObject(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")
	objectName := c.Param("object")

	var req config.DataObjectConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					for k, do := range ln.DataObjects {
						if do.Name == objectName {
							cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k] = req
							found = true
							break
						}
					}
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "数据对象不存在"})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新数据对象", "device", deviceName, "node", nodeName, "object", objectName)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteDataObject 删除数据对象
func (h *IEC61850Handler) DeleteDataObject(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")
	objectName := c.Param("object")

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					for k, do := range ln.DataObjects {
						if do.Name == objectName {
							cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects = append(
								cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[:k],
								cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k+1:]...,
							)
							found = true
							break
						}
					}
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "数据对象不存在"})
		return
	}

	// 删除引用该数据对象的映射规则
	objectPrefix := deviceName + "/" + nodeName + "." + objectName + "."
	var validMappings []config.IEC61850MappingRule
	for _, rule := range cfg.Mappings {
		if !strings.HasPrefix(rule.IEC61850Path, objectPrefix) {
			validMappings = append(validMappings, rule)
		}
	}
	cfg.Mappings = validMappings

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("删除数据对象", "device", deviceName, "node", nodeName, "object", objectName)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// AddDataAttribute 新增数据属性
func (h *IEC61850Handler) AddDataAttribute(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")
	objectName := c.Param("object")

	var req struct {
		config.DataAttributeConfig
		ParentPath string `json:"parent_path"` // 父属性路径，如 "mag" 表示添加为 mag 的子属性
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "数据属性名称不能为空"})
		return
	}

	// 如果是容器节点（类型为空），确保 Children 是空数组而不是 nil
	if req.Type == "" && req.Children == nil {
		req.Children = []config.DataAttributeConfig{}
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					for k, do := range ln.DataObjects {
						if do.Name == objectName {
							if req.ParentPath == "" {
								// 添加到顶层
								cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes = append(
									cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes, req.DataAttributeConfig)
							} else {
								// 添加到指定父属性的 children 中
								parentParts := strings.Split(req.ParentPath, ".")
								if !addChildAttribute(&cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes, parentParts, 0, req.DataAttributeConfig) {
									c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "父属性不存在: " + req.ParentPath})
									return
								}
							}
							found = true
							break
						}
					}
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "数据对象不存在"})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("新增数据属性", "device", deviceName, "node", nodeName, "object", objectName, "attr", req.Name, "parent", req.ParentPath)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功"})
}

// addChildAttribute 递归添加子属性到指定父属性
func addChildAttribute(attrs *[]config.DataAttributeConfig, parts []string, depth int, newAttr config.DataAttributeConfig) bool {
	for i, attr := range *attrs {
		if attr.Name == parts[depth] {
			if depth == len(parts)-1 {
				// 找到父属性，添加子属性
				(*attrs)[i].Children = append((*attrs)[i].Children, newAttr)
				return true
			}
			// 继续递归查找
			if (*attrs)[i].Children != nil {
				return addChildAttribute(&(*attrs)[i].Children, parts, depth+1, newAttr)
			}
			return false
		}
	}
	return false
}

// UpdateDataAttribute 更新数据属性
func (h *IEC61850Handler) UpdateDataAttribute(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")
	objectName := c.Param("object")
	attrPath := strings.TrimPrefix(c.Param("attr"), "/") // 去掉前导斜杠，支持嵌套路径如 "mag.f"

	var req config.DataAttributeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	// 解析路径
	attrParts := strings.Split(attrPath, ".")

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					for k, do := range ln.DataObjects {
						if do.Name == objectName {
							if updateNestedAttribute(cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes, attrParts, 0, &req) {
								found = true
							}
							break
						}
					}
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "数据属性不存在"})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新数据属性", "device", deviceName, "node", nodeName, "object", objectName, "attr", attrPath)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// updateNestedAttribute 递归更新嵌套属性
func updateNestedAttribute(attrs []config.DataAttributeConfig, parts []string, depth int, newVal *config.DataAttributeConfig) bool {
	for i, attr := range attrs {
		if attr.Name == parts[depth] {
			if depth == len(parts)-1 {
				// 找到目标属性，更新它
				attrs[i] = *newVal
				return true
			}
			// 继续递归查找
			if attr.Children != nil {
				return updateNestedAttribute(attr.Children, parts, depth+1, newVal)
			}
			return false
		}
	}
	return false
}

// DeleteDataAttribute 删除数据属性
func (h *IEC61850Handler) DeleteDataAttribute(c *gin.Context) {
	deviceName := c.Param("name")
	nodeName := c.Param("node")
	objectName := c.Param("object")
	attrPath := strings.TrimPrefix(c.Param("attr"), "/") // 去掉前导斜杠，支持嵌套路径如 "mag.f"

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "IEC 61850 配置不存在"})
		return
	}

	// 解析路径
	attrParts := strings.Split(attrPath, ".")

	found := false
	for i, ld := range cfg.Model.LogicalDevices {
		if ld.Name == deviceName {
			for j, ln := range ld.LogicalNodes {
				if ln.Name == nodeName {
					for k, do := range ln.DataObjects {
						if do.Name == objectName {
							newAttrs, ok := deleteNestedAttribute(cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes, attrParts, 0)
							if ok {
								cfg.Model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes = newAttrs
								found = true
							}
							break
						}
					}
					break
				}
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "数据属性不存在"})
		return
	}

	// 删除引用该数据属性的映射规则
	// 精确匹配叶子属性（如 "mag.f"）或前缀匹配容器属性（如 "mag" → 匹配 "mag.f"、"mag.i" 等）
	attrPrefix := deviceName + "/" + nodeName + "." + objectName + "." + attrPath
	var validMappings []config.IEC61850MappingRule
	for _, rule := range cfg.Mappings {
		if rule.IEC61850Path == attrPrefix || strings.HasPrefix(rule.IEC61850Path, attrPrefix+".") {
			continue // 删除匹配的映射
		}
		validMappings = append(validMappings, rule)
	}
	cfg.Mappings = validMappings

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("删除数据属性", "device", deviceName, "node", nodeName, "object", objectName, "attr", attrPath)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// deleteNestedAttribute 递归删除嵌套属性
func deleteNestedAttribute(attrs []config.DataAttributeConfig, parts []string, depth int) ([]config.DataAttributeConfig, bool) {
	for i, attr := range attrs {
		if attr.Name == parts[depth] {
			if depth == len(parts)-1 {
				// 找到目标属性，删除它
				return append(attrs[:i], attrs[i+1:]...), true
			}
			// 继续递归查找
			if attr.Children != nil {
				newChildren, ok := deleteNestedAttribute(attr.Children, parts, depth+1)
				if ok {
					attrs[i].Children = newChildren
					return attrs, true
				}
			}
			return attrs, false
		}
	}
	return attrs, false
}

// GetMappings 获取所有映射规则
func (h *IEC61850Handler) GetMappings(c *gin.Context) {
	deviceFilter := c.Query("device")

	mappings := h.cfgMgr.GetIEC61850Mappings()
	if deviceFilter != "" {
		filtered := make([]config.IEC61850MappingRule, 0)
		for _, m := range mappings {
			if m.SourceDevice == deviceFilter {
				filtered = append(filtered, m)
			}
		}
		mappings = filtered
	}

	if mappings == nil {
		mappings = make([]config.IEC61850MappingRule, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": mappings,
	})
}

// AddMapping 新增映射规则
func (h *IEC61850Handler) AddMapping(c *gin.Context) {
	var req config.IEC61850MappingRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		cfg = &config.ModbusToIEC61850Config{}
	}

	if req.Scale == 0 {
		req.Scale = 1.0
	}

	cfg.Mappings = append(cfg.Mappings, req)

	// 校验映射路径
	if err := h.cfgMgr.ValidateIEC61850Mappings(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("新增 IEC 61850 映射规则", "source", req.SourceDevice, "path", req.IEC61850Path)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功"})
}

// UpdateMapping 更新映射规则
func (h *IEC61850Handler) UpdateMapping(c *gin.Context) {
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的索引"})
		return
	}

	var req config.IEC61850MappingRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil || index < 0 || index >= len(cfg.Mappings) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "映射规则不存在"})
		return
	}

	if req.Scale == 0 {
		req.Scale = 1.0
	}

	cfg.Mappings[index] = req

	// 校验映射路径
	if err := h.cfgMgr.ValidateIEC61850Mappings(cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("更新 IEC 61850 映射规则", "index", index)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteMapping 删除映射规则
func (h *IEC61850Handler) DeleteMapping(c *gin.Context) {
	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的索引"})
		return
	}

	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil || index < 0 || index >= len(cfg.Mappings) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "映射规则不存在"})
		return
	}

	cfg.Mappings = append(cfg.Mappings[:index], cfg.Mappings[index+1:]...)

	if err := h.cfgMgr.SetIEC61850Config(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("删除 IEC 61850 映射规则", "index", index)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// GetStatus 获取 IEC 61850 运行状态
func (h *IEC61850Handler) GetStatus(c *gin.Context) {
	cfg := h.cfgMgr.GetIEC61850Config()
	enabled := cfg != nil && cfg.IEC61850.Enabled

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"enabled":          enabled,
			"port":             cfg.IEC61850.Port,
			"ied_name":         cfg.IEC61850.IEDName,
			"max_connections":  cfg.IEC61850.MaxConnections,
			"active_connections": 0, // TODO: 从 IEC 61850 管理器获取实际连接数
			"mapping_count":    len(cfg.Mappings),
		},
	})
}

// GetPaths 获取可用的 IEC 61850 路径列表
func (h *IEC61850Handler) GetPaths(c *gin.Context) {
	cfg := h.cfgMgr.GetIEC61850Config()
	if cfg == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []string{}})
		return
	}

	paths := collectModelPaths(&cfg.Model)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": paths})
}

// collectModelPaths 收集所有模型路径
func collectModelPaths(model *config.IEC61850ModelConfig) []string {
	var paths []string
	for _, ld := range model.LogicalDevices {
		for _, ln := range ld.LogicalNodes {
			for _, do := range ln.DataObjects {
				collectDAPrefixPaths(ld.Name, ln.Name, do.Name, do.DataAttributes, &paths)
			}
		}
	}
	return paths
}

// collectDAPrefixPaths 递归收集数据属性路径
func collectDAPrefixPaths(ldName, lnName, doPath string, attrs []config.DataAttributeConfig, paths *[]string) {
	for _, da := range attrs {
		fullPath := fmt.Sprintf("%s/%s.%s.%s", ldName, lnName, doPath, da.Name)
		if len(da.Children) > 0 {
			collectDAPrefixPaths(ldName, lnName, doPath+"."+da.Name, da.Children, paths)
		} else {
			*paths = append(*paths, fullPath)
		}
	}
}
