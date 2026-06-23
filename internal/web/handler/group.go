package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// GroupHandler 分组管理处理器
type GroupHandler struct {
	cfgMgr *config.Manager
	log    *logger.Logger
}

// NewGroupHandler 创建分组处理器
func NewGroupHandler(cfgMgr *config.Manager, log *logger.Logger) *GroupHandler {
	return &GroupHandler{cfgMgr: cfgMgr, log: log}
}

// GroupResponse 分组响应
type GroupResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Devices        []string `json:"devices"`
	Port           interface{} `json:"port"`
	BaudRate       int      `json:"baud_rate,omitempty"`
	SlaveID        int      `json:"slave_id"`
	MaxConnections int      `json:"max_connections,omitempty"`
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	ID             string   `json:"id" binding:"required"`
	Name           string   `json:"name" binding:"required"`
	Type           string   `json:"type"`
	Devices        []string `json:"devices"`
	Port           interface{} `json:"port"`
	BaudRate       int      `json:"baud_rate"`
	SlaveID        int      `json:"slave_id"`
	MaxConnections int      `json:"max_connections"`
}

// List 获取分组列表
func (h *GroupHandler) List(c *gin.Context) {
	cfg := h.cfgMgr.Get()

	// 从输出配置中提取分组信息
	groups := make([]GroupResponse, 0)

	// TCP 输出分组
	for _, srv := range cfg.Outputs.ModbusTCPServers {
		groups = append(groups, GroupResponse{
			ID:             srv.ID,
			Name:           srv.Name,
			Type:           "tcp",
			Devices:        h.cfgMgr.GetGroupDevices(srv.ID),
			Port:           srv.ListenPort,
			SlaveID:        srv.SlaveID,
			MaxConnections: srv.MaxConnections,
		})
	}

	// RTU 输出分组
	for _, srv := range cfg.Outputs.ModbusRTUServers {
		groups = append(groups, GroupResponse{
			ID:       srv.ID,
			Name:     srv.Name,
			Type:     "rtu",
			Devices:  h.cfgMgr.GetGroupDevices(srv.ID),
			Port:     srv.Port,
			BaudRate: srv.BaudRate,
			SlaveID:  srv.SlaveID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    groups,
	})
}

// Get 获取单个分组
func (h *GroupHandler) Get(c *gin.Context) {
	id := c.Param("id")

	// 查找 TCP 分组
	if srv := h.cfgMgr.GetModbusTCPServer(id); srv != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": GroupResponse{
				ID:             srv.ID,
				Name:           srv.Name,
				Type:           "tcp",
				Devices:        h.cfgMgr.GetGroupDevices(srv.ID),
				Port:           srv.ListenPort,
				SlaveID:        srv.SlaveID,
				MaxConnections: srv.MaxConnections,
			},
		})
		return
	}

	// 查找 RTU 分组
	if srv := h.cfgMgr.GetModbusRTUServer(id); srv != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": GroupResponse{
				ID:       srv.ID,
				Name:     srv.Name,
				Type:     "rtu",
				Devices:  h.cfgMgr.GetGroupDevices(srv.ID),
				Port:     srv.Port,
				BaudRate: srv.BaudRate,
				SlaveID:  srv.SlaveID,
			},
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "分组不存在",
	})
}

// Create 创建分组
func (h *GroupHandler) Create(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 默认为 TCP 类型
	groupType := req.Type
	if groupType == "" {
		groupType = "tcp"
	}

	if groupType == "tcp" {
		// 解析端口
		var port int
		switch v := req.Port.(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		}

		srv := config.ModbusTCPServerConfig{
			ID:             req.ID,
			Name:           req.Name,
			ListenPort:     port,
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
	} else {
		// RTU 类型
		port, _ := req.Port.(string)
		if port == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "串口不能为空",
			})
			return
		}

		srv := config.ModbusRTUServerConfig{
			ID:       req.ID,
			Name:     req.Name,
			Port:     port,
			BaudRate: req.BaudRate,
			SlaveID:  req.SlaveID,
		}
		if srv.BaudRate == 0 {
			srv.BaudRate = 9600
		}

		if err := h.cfgMgr.AddModbusRTUServer(srv); err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": err.Error(),
			})
			return
		}
	}

	h.log.Info("创建分组", "id", req.ID, "name", req.Name, "type", groupType)

	// 保存分组包含的设备列表
	if req.Devices != nil {
		if err := h.cfgMgr.SetGroupDevices(req.ID, req.Devices); err != nil {
			h.log.Error("保存分组设备列表失败", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
	})
}

// Update 更新分组
func (h *GroupHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 默认为 TCP 类型
	groupType := req.Type
	if groupType == "" {
		groupType = "tcp"
	}

	if groupType == "tcp" {
		// 解析端口
		var port int
		switch v := req.Port.(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		}

		srv := config.ModbusTCPServerConfig{
			ID:             id,
			Name:           req.Name,
			ListenPort:     port,
			SlaveID:        req.SlaveID,
			MaxConnections: req.MaxConnections,
		}
		if srv.MaxConnections == 0 {
			srv.MaxConnections = 10
		}

		if err := h.cfgMgr.UpdateModbusTCPServer(srv); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	} else {
		// RTU 类型
		port, _ := req.Port.(string)
		if port == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "串口不能为空",
			})
			return
		}

		srv := config.ModbusRTUServerConfig{
			ID:       id,
			Name:     req.Name,
			Port:     port,
			BaudRate: req.BaudRate,
			SlaveID:  req.SlaveID,
		}
		if srv.BaudRate == 0 {
			srv.BaudRate = 9600
		}

		if err := h.cfgMgr.UpdateModbusRTUServer(srv); err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
	}

	h.log.Info("更新分组", "id", id, "type", groupType)

	// 保存分组包含的设备列表
	if req.Devices != nil {
		if err := h.cfgMgr.SetGroupDevices(id, req.Devices); err != nil {
			h.log.Error("保存分组设备列表失败", "error", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除分组
func (h *GroupHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// 尝试从 TCP 列表删除
	if err := h.cfgMgr.DeleteModbusTCPServer(id); err == nil {
		h.log.Info("删除TCP分组", "id", id)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
		return
	}

	// 尝试从 RTU 列表删除
	if err := h.cfgMgr.DeleteModbusRTUServer(id); err == nil {
		h.log.Info("删除RTU分组", "id", id)
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": "分组不存在",
	})
}
