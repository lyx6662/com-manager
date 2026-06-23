package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// CollectorProvider 采集器数据提供者接口
type CollectorProvider interface {
	GetDeviceStatus(deviceID string) interface{}
	GetAllDeviceStatus() map[string]interface{}
}

// DeviceHandler 设备管理处理器
type DeviceHandler struct {
	cfgMgr    *config.Manager
	log       *logger.Logger
	collector CollectorProvider
}

// NewDeviceHandler 创建设备处理器
func NewDeviceHandler(cfgMgr *config.Manager, log *logger.Logger, collector interface{}) *DeviceHandler {
	var cp CollectorProvider
	if c, ok := collector.(CollectorProvider); ok {
		cp = c
	}
	return &DeviceHandler{
		cfgMgr:    cfgMgr,
		log:       log,
		collector: cp,
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
	DataBits     int    `json:"data_bits,omitempty"`
	StopBits     int    `json:"stop_bits,omitempty"`
	Parity       string `json:"parity,omitempty"`
	SlaveID      int    `json:"slave_id"`
	PollInterval string `json:"poll_interval"`
	Timeout      string `json:"timeout"`
	Retry        int    `json:"retry"`
	LastPoll     interface{} `json:"last_poll"`
	ErrorCount   int    `json:"error_count"`
	LastError    string `json:"last_error,omitempty"`
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

// deviceStatusInfo 设备状态信息
type deviceStatusInfo struct {
	online     bool
	lastPoll   interface{}
	errorCount int
	lastError  string
}

// List 获取设备列表
func (h *DeviceHandler) List(c *gin.Context) {
	devices := make([]DeviceResponse, 0)

	// 获取采集器中的设备状态
	deviceStatuses := make(map[string]deviceStatusInfo)
	if h.collector != nil {
		statuses := h.collector.GetAllDeviceStatus()
		for id, s := range statuses {
			info := deviceStatusInfo{}
			if status, ok := s.(interface{ IsOnline() bool }); ok {
				info.online = status.IsOnline()
			}
			// 尝试获取更多状态字段
			if status, ok := s.(interface {
				IsOnline() bool
				GetLastPoll() interface{}
				GetErrorCount() int
				GetLastError() string
			}); ok {
				info.lastPoll = status.GetLastPoll()
				info.errorCount = status.GetErrorCount()
				info.lastError = status.GetLastError()
			}
			deviceStatuses[id] = info
		}
	}

	// 串口设备
	for _, dev := range h.cfgMgr.Get().SerialDevices {
		status := deviceStatuses[dev.ID]
		devices = append(devices, DeviceResponse{
			ID:           dev.ID,
			Name:         dev.Name,
			Type:         "serial",
			Protocol:     dev.Protocol,
			Enabled:      dev.Enabled,
			Online:       status.online,
			Port:         dev.Port,
			BaudRate:     dev.BaudRate,
			DataBits:     dev.DataBits,
			StopBits:     dev.StopBits,
			Parity:       dev.Parity,
			SlaveID:      dev.SlaveID,
			PollInterval: dev.PollInterval,
			Timeout:      dev.Timeout,
			Retry:        dev.Retry,
			LastPoll:     status.lastPoll,
			ErrorCount:   status.errorCount,
			LastError:    status.lastError,
		})
	}

	// 网口设备
	for _, dev := range h.cfgMgr.Get().NetworkDevices {
		status := deviceStatuses[dev.ID]
		devices = append(devices, DeviceResponse{
			ID:           dev.ID,
			Name:         dev.Name,
			Type:         "network",
			Protocol:     dev.Protocol,
			Enabled:      dev.Enabled,
			Online:       status.online,
			Host:         dev.Host,
			Port:         dev.Port,
			SlaveID:      dev.SlaveID,
			PollInterval: dev.PollInterval,
			Timeout:      dev.Timeout,
			Retry:        dev.Retry,
			LastPoll:     status.lastPoll,
			ErrorCount:   status.errorCount,
			LastError:    status.lastError,
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
				DataBits:     dev.DataBits,
				StopBits:     dev.StopBits,
				Parity:       dev.Parity,
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

	// 从采集器获取实时状态
	if h.collector != nil {
		status := h.collector.GetDeviceStatus(id)
		if status != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": status,
			})
			return
		}
	}

	// 设备未被采集器管理，返回默认状态
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":           id,
			"online":       false,
			"last_poll":    nil,
			"error_count":  0,
			"last_error":   "设备未纳入采集",
		},
	})
}
