package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// OutputHandler 输出配置处理器
type OutputHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewOutputHandler 创建输出处理器
func NewOutputHandler(cfgMgr *config.Manager, log *logger.Logger) *OutputHandler {
	return &OutputHandler{cfgMgr: cfgMgr, log: log}
}

// CreateOutputRequest 创建输出请求
type CreateOutputRequest struct {
	ID             string `json:"id" binding:"required"`
	Name           string `json:"name" binding:"required"`
	Protocol       string `json:"protocol" binding:"required"`
	ListenPort     int    `json:"listen_port"`
	Port           string `json:"port"`
	BaudRate       int    `json:"baud_rate"`
	SlaveID        int    `json:"slave_id"`
	MaxConnections int    `json:"max_connections"`
}

// List 获取输出配置列表
func (h *OutputHandler) List(c *gin.Context) {
	cfg := h.cfgMgr.Get()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled":            cfg.Outputs.Enabled,
			"modbus_tcp_servers": cfg.Outputs.ModbusTCPServers,
			"modbus_rtu_servers": cfg.Outputs.ModbusRTUServers,
		},
	})
}

// Get 获取单个输出配置
func (h *OutputHandler) Get(c *gin.Context) {
	id := c.Param("id")

	// 查找 TCP 输出
	if srv := h.cfgMgr.GetModbusTCPServer(id); srv != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"type": "tcp",
				"config": srv,
			},
		})
		return
	}

	// 查找 RTU 输出
	if srv := h.cfgMgr.GetModbusRTUServer(id); srv != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"type": "rtu",
				"config": srv,
			},
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "输出配置不存在",
	})
}

// Create 创建输出配置
func (h *OutputHandler) Create(c *gin.Context) {
	var req CreateOutputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 根据协议类型处理
	protocol := req.Protocol
	if protocol == "" {
		protocol = "tcp" // 默认 TCP
	}

	if protocol == "tcp" || protocol == "modbus-tcp" {
		srv := config.ModbusTCPServerConfig{
			ID:             req.ID,
			Name:           req.Name,
			ListenPort:     req.ListenPort,
			SlaveID:        req.SlaveID,
			MaxConnections: req.MaxConnections,
		}
		if srv.MaxConnections == 0 {
			srv.MaxConnections = 10
		}
		if srv.SlaveID == 0 {
			srv.SlaveID = 1
		}
		if err := h.cfgMgr.AddModbusTCPServer(srv); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
			})
			return
		}
	} else if protocol == "rtu" || protocol == "modbus-rtu" {
		srv := config.ModbusRTUServerConfig{
			ID:       req.ID,
			Name:     req.Name,
			Port:     req.Port,
			BaudRate: req.BaudRate,
			SlaveID:  req.SlaveID,
		}
		if srv.BaudRate == 0 {
			srv.BaudRate = 9600
		}
		if srv.SlaveID == 0 {
			srv.SlaveID = 1
		}
		if err := h.cfgMgr.AddModbusRTUServer(srv); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
			})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的协议类型: " + protocol,
		})
		return
	}

	h.log.Info("创建输出配置", "id", req.ID, "protocol", protocol)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"note":    "新创建的输出配置将在重启后生效",
	})
}

// Update 更新输出配置
func (h *OutputHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req CreateOutputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	req.ID = id

	// 根据协议类型处理
	protocol := req.Protocol
	if protocol == "" {
		protocol = "tcp" // 默认 TCP
	}

	if protocol == "tcp" || protocol == "modbus-tcp" {
		srv := config.ModbusTCPServerConfig{
			ID:             req.ID,
			Name:           req.Name,
			ListenPort:     req.ListenPort,
			SlaveID:        req.SlaveID,
			MaxConnections: req.MaxConnections,
		}
		if err := h.cfgMgr.UpdateModbusTCPServer(srv); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	} else if protocol == "rtu" || protocol == "modbus-rtu" {
		srv := config.ModbusRTUServerConfig{
			ID:       req.ID,
			Name:     req.Name,
			Port:     req.Port,
			BaudRate: req.BaudRate,
			SlaveID:  req.SlaveID,
		}
		if err := h.cfgMgr.UpdateModbusRTUServer(srv); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的协议类型: " + protocol,
		})
		return
	}

	h.log.Info("更新输出配置", "id", id, "protocol", protocol)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除输出配置
func (h *OutputHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// 尝试从TCP列表删除
	if err := h.cfgMgr.DeleteModbusTCPServer(id); err == nil {
		h.log.Info("删除TCP输出配置", "id", id)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
		return
	}

	// 尝试从RTU列表删除
	if err := h.cfgMgr.DeleteModbusRTUServer(id); err == nil {
		h.log.Info("删除RTU输出配置", "id", id)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "输出配置不存在",
	})
}

// BatchUpdateRequest 批量更新输出请求
type BatchUpdateRequest struct {
	ModbusTCPServers []config.ModbusTCPServerConfig `json:"modbus_tcp_servers"`
	ModbusRTUServers []config.ModbusRTUServerConfig `json:"modbus_rtu_servers"`
}

// ToggleEnabled 切换 Modbus 输出启用状态
func (h *OutputHandler) ToggleEnabled(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	cfg := h.cfgMgr.Get()
	cfg.Outputs.Enabled = req.Enabled
	if err := h.cfgMgr.Save(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败: " + err.Error()})
		return
	}

	h.log.Info("切换 Modbus 输出状态", "enabled", req.Enabled)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// BatchUpdate 批量更新所有输出配置
func (h *OutputHandler) BatchUpdate(c *gin.Context) {
	var req BatchUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 设置默认值
	for i := range req.ModbusTCPServers {
		if req.ModbusTCPServers[i].MaxConnections == 0 {
			req.ModbusTCPServers[i].MaxConnections = 10
		}
		if req.ModbusTCPServers[i].SlaveID == 0 {
			req.ModbusTCPServers[i].SlaveID = 1
		}
	}
	for i := range req.ModbusRTUServers {
		if req.ModbusRTUServers[i].SlaveID == 0 {
			req.ModbusRTUServers[i].SlaveID = 1
		}
	}

	if err := h.cfgMgr.SetOutputs(req.ModbusTCPServers, req.ModbusRTUServers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "保存失败: " + err.Error(),
		})
		return
	}

	h.log.Info("批量更新输出配置", "tcp", len(req.ModbusTCPServers), "rtu", len(req.ModbusRTUServers))

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "保存成功",
	})
}
