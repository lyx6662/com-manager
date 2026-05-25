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
			"modbus_tcp_servers": cfg.Outputs.ModbusTCPServers,
			"modbus_rtu_servers": cfg.Outputs.ModbusRTUServers,
		},
	})
}

// Get 获取单个输出配置
func (h *OutputHandler) Get(c *gin.Context) {
	id := c.Param("id")

	if srv := h.cfgMgr.GetModbusTCPServer(id); srv != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": srv,
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

	if req.Protocol == "modbus-tcp" {
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
		if err := h.cfgMgr.AddModbusTCPServer(srv); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
			})
			return
		}
	}

	h.log.Info("创建输出配置", "id", req.ID, "protocol", req.Protocol)

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

	if req.Protocol == "modbus-tcp" {
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
	}

	h.log.Info("更新输出配置", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除输出配置
func (h *OutputHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.cfgMgr.DeleteModbusTCPServer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	h.log.Info("删除输出配置", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}
