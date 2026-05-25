package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// DeviceHandler 设备管理处理器
type DeviceHandler struct {
	cfg *config.Config
	log *logger.Logger
}

// NewDeviceHandler 创建设备处理器
func NewDeviceHandler(cfg *config.Config, log *logger.Logger) *DeviceHandler {
	return &DeviceHandler{
		cfg: cfg,
		log: log,
	}
}

// DeviceResponse 设备响应
type DeviceResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`     // serial / network
	Protocol string `json:"protocol"`
	Enabled  bool   `json:"enabled"`
	Online   bool   `json:"online"`
}

// CreateDeviceRequest 创建设备请求
type CreateDeviceRequest struct {
	ID           string `json:"id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Type         string `json:"type" binding:"required,oneof=serial network"`
	Protocol     string `json:"protocol" binding:"required"`
	Port         string `json:"port"`          // 串口
	BaudRate     int    `json:"baud_rate"`
	DataBits     int    `json:"data_bits"`
	StopBits     int    `json:"stop_bits"`
	Parity       string `json:"parity"`
	Host         string `json:"host"`          // 网口
	NetPort      int    `json:"net_port"`
	SlaveID      int    `json:"slave_id"`
	PollInterval string `json:"poll_interval"`
	Timeout      string `json:"timeout"`
	Retry        int    `json:"retry"`
	Enabled      bool   `json:"enabled"`
}

// List 获取设备列表
func (h *DeviceHandler) List(c *gin.Context) {
	devices := make([]DeviceResponse, 0)

	// 串口设备
	for _, dev := range h.cfg.SerialDevices {
		devices = append(devices, DeviceResponse{
			ID:       dev.ID,
			Name:     dev.Name,
			Type:     "serial",
			Protocol: dev.Protocol,
			Enabled:  dev.Enabled,
			Online:   false, // TODO: 查询实际状态
		})
	}

	// 网口设备
	for _, dev := range h.cfg.NetworkDevices {
		devices = append(devices, DeviceResponse{
			ID:       dev.ID,
			Name:     dev.Name,
			Type:     "network",
			Protocol: dev.Protocol,
			Enabled:  dev.Enabled,
			Online:   false, // TODO: 查询实际状态
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    devices,
	})
}

// Get 获取单个设备
func (h *DeviceHandler) Get(c *gin.Context) {
	id := c.Param("id")

	// 查找串口设备
	for _, dev := range h.cfg.SerialDevices {
		if dev.ID == id {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": DeviceResponse{
					ID:       dev.ID,
					Name:     dev.Name,
					Type:     "serial",
					Protocol: dev.Protocol,
					Enabled:  dev.Enabled,
				},
			})
			return
		}
	}

	// 查找网口设备
	for _, dev := range h.cfg.NetworkDevices {
		if dev.ID == id {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": DeviceResponse{
					ID:       dev.ID,
					Name:     dev.Name,
					Type:     "network",
					Protocol: dev.Protocol,
					Enabled:  dev.Enabled,
				},
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "设备不存在",
	})
}

// Create 创建设备
func (h *DeviceHandler) Create(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// TODO: 保存到配置文件
	h.log.Info("创建设备", "id", req.ID, "name", req.Name, "type", req.Type)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
	})
}

// Update 更新设备
func (h *DeviceHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// TODO: 更新配置文件
	h.log.Info("更新设备", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除设备
func (h *DeviceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// TODO: 从配置文件删除
	h.log.Info("删除设备", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// GetStatus 获取设备状态
func (h *DeviceHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")

	// TODO: 查询实际设备状态
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":           id,
			"online":       false,
			"last_poll":    nil,
			"error_count":  0,
			"last_error":   "",
		},
	})
}
