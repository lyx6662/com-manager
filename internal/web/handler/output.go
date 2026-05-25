package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// OutputHandler 输出配置处理器
type OutputHandler struct {
	cfg *config.Config
	log *logger.Logger
}

// NewOutputHandler 创建输出处理器
func NewOutputHandler(cfg *config.Config, log *logger.Logger) *OutputHandler {
	return &OutputHandler{cfg: cfg, log: log}
}

// List 获取输出配置列表
func (h *OutputHandler) List(c *gin.Context) {
	outputs := gin.H{
		"modbus_tcp_servers": h.cfg.Outputs.ModbusTCPServers,
		"modbus_rtu_servers": h.cfg.Outputs.ModbusRTUServers,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    outputs,
	})
}

// Get 获取单个输出配置
func (h *OutputHandler) Get(c *gin.Context) {
	id := c.Param("id")

	// TODO: 查找配置
	_ = id

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": nil,
	})
}

// Create 创建输出配置
func (h *OutputHandler) Create(c *gin.Context) {
	// TODO: 创建输出配置
	h.log.Info("创建输出配置")

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
	})
}

// Update 更新输出配置
func (h *OutputHandler) Update(c *gin.Context) {
	id := c.Param("id")
	_ = id

	// TODO: 更新输出配置
	h.log.Info("更新输出配置", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除输出配置
func (h *OutputHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	_ = id

	// TODO: 删除输出配置
	h.log.Info("删除输出配置", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}
