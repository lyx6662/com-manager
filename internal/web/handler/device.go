package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// DeviceHandler 设备管理处理器
type DeviceHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewDeviceHandler 创建设备处理器
func NewDeviceHandler(cfgMgr *config.Manager, log *logger.Logger) *DeviceHandler {
	return &DeviceHandler{
		cfgMgr: cfgMgr,
		log:    log,
	}
}

// DeviceResponse 设备响应
type DeviceResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Protocol     string `json:"protocol"`
	Enabled      bool   `json:"enabled"`
	Online       bool   `json:"online"`
	Host         string `json:"host,omitempty"`
	Port         interface{} `json:"port,omitempty"`
	BaudRate     int    `json:"baud_rate,omitempty"`
	SlaveID      int    `json:"slave_id"`
	PollInterval string `json:"poll_interval"`
	Timeout      string `json:"timeout"`
	Retry        int    `json:"retry"`
}

// CreateDeviceRequest 创建设备请求
type CreateDeviceRequest struct {
	ID           string `json:"id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Type         string `json:"type" binding:"required,oneof=serial network"`
	Protocol     string `json:"protocol" binding:"required"`
	Port         string `json:"port"`
	BaudRate     int    `json:"baud_rate"`
	DataBits     int    `json:"data_bits"`
	StopBits     int    `json:"stop_bits"`
	Parity       string `json:"parity"`
	Host         string `json:"host"`
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
	for _, dev := range h.cfgMgr.Get().SerialDevices {
		devices = append(devices, DeviceResponse{
			ID:           dev.ID,
			Name:         dev.Name,
			Type:         "serial",
			Protocol:     dev.Protocol,
			Enabled:      dev.Enabled,
			Port:         dev.Port,
			BaudRate:     dev.BaudRate,
			SlaveID:      dev.SlaveID,
			PollInterval: dev.PollInterval,
			Timeout:      dev.Timeout,
			Retry:        dev.Retry,
		})
	}

	// 网口设备
	for _, dev := range h.cfgMgr.Get().NetworkDevices {
		devices = append(devices, DeviceResponse{
			ID:           dev.ID,
			Name:         dev.Name,
			Type:         "network",
			Protocol:     dev.Protocol,
			Enabled:      dev.Enabled,
			Host:         dev.Host,
			Port:         dev.Port,
			SlaveID:      dev.SlaveID,
			PollInterval: dev.PollInterval,
			Timeout:      dev.Timeout,
			Retry:        dev.Retry,
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
	if dev := h.cfgMgr.GetSerialDevice(id); dev != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": DeviceResponse{
				ID:           dev.ID,
				Name:         dev.Name,
				Type:         "serial",
				Protocol:     dev.Protocol,
				Enabled:      dev.Enabled,
				Port:         dev.Port,
				BaudRate:     dev.BaudRate,
				SlaveID:      dev.SlaveID,
				PollInterval: dev.PollInterval,
				Timeout:      dev.Timeout,
				Retry:        dev.Retry,
			},
		})
		return
	}

	// 查找网口设备
	if dev := h.cfgMgr.GetNetworkDevice(id); dev != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": DeviceResponse{
				ID:           dev.ID,
				Name:         dev.Name,
				Type:         "network",
				Protocol:     dev.Protocol,
				Enabled:      dev.Enabled,
				Host:         dev.Host,
				Port:         dev.Port,
				SlaveID:      dev.SlaveID,
				PollInterval: dev.PollInterval,
				Timeout:      dev.Timeout,
				Retry:        dev.Retry,
			},
		})
		return
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

	if req.Type == "serial" {
		dev := config.SerialDeviceConfig{
			ID:           req.ID,
			Name:         req.Name,
			Port:         req.Port,
			BaudRate:     req.BaudRate,
			DataBits:     req.DataBits,
			StopBits:     req.StopBits,
			Parity:       req.Parity,
			Protocol:     req.Protocol,
			SlaveID:      req.SlaveID,
			PollInterval: req.PollInterval,
			Timeout:      req.Timeout,
			Retry:        req.Retry,
			Enabled:      req.Enabled,
		}
		if err := h.cfgMgr.AddSerialDevice(dev); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
			})
			return
		}
	} else {
		dev := config.NetworkDeviceConfig{
			ID:           req.ID,
			Name:         req.Name,
			Host:         req.Host,
			Port:         req.NetPort,
			Protocol:     req.Protocol,
			SlaveID:      req.SlaveID,
			PollInterval: req.PollInterval,
			Timeout:      req.Timeout,
			Retry:        req.Retry,
			Enabled:      req.Enabled,
		}
		if err := h.cfgMgr.AddNetworkDevice(dev); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
			})
			return
		}
	}

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

	req.ID = id

	if req.Type == "serial" {
		dev := config.SerialDeviceConfig{
			ID:           req.ID,
			Name:         req.Name,
			Port:         req.Port,
			BaudRate:     req.BaudRate,
			DataBits:     req.DataBits,
			StopBits:     req.StopBits,
			Parity:       req.Parity,
			Protocol:     req.Protocol,
			SlaveID:      req.SlaveID,
			PollInterval: req.PollInterval,
			Timeout:      req.Timeout,
			Retry:        req.Retry,
			Enabled:      req.Enabled,
		}
		if err := h.cfgMgr.UpdateSerialDevice(dev); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	} else {
		dev := config.NetworkDeviceConfig{
			ID:           req.ID,
			Name:         req.Name,
			Host:         req.Host,
			Port:         req.NetPort,
			Protocol:     req.Protocol,
			SlaveID:      req.SlaveID,
			PollInterval: req.PollInterval,
			Timeout:      req.Timeout,
			Retry:        req.Retry,
			Enabled:      req.Enabled,
		}
		if err := h.cfgMgr.UpdateNetworkDevice(dev); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	}

	h.log.Info("更新设备", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除设备
func (h *DeviceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// 尝试删除串口设备
	if err := h.cfgMgr.DeleteSerialDevice(id); err == nil {
		h.log.Info("删除串口设备", "id", id)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
		return
	}

	// 尝试删除网口设备
	if err := h.cfgMgr.DeleteNetworkDevice(id); err == nil {
		h.log.Info("删除网口设备", "id", id)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "设备不存在",
	})
}

// GetStatus 获取设备状态
func (h *DeviceHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")

	// TODO: 查询实际设备连接状态
	_ = id

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
