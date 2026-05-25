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
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Devices  []string `json:"devices"`
	Port     int      `json:"port"`
	SlaveID  int      `json:"slave_id"`
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	ID      string   `json:"id" binding:"required"`
	Name    string   `json:"name" binding:"required"`
	Devices []string `json:"devices"`
	Port    int      `json:"port"`
	SlaveID int      `json:"slave_id"`
}

// List 获取分组列表
func (h *GroupHandler) List(c *gin.Context) {
	cfg := h.cfgMgr.Get()

	// 从输出配置中提取分组信息
	groups := make([]GroupResponse, 0)
	for _, srv := range cfg.Outputs.ModbusTCPServers {
		groups = append(groups, GroupResponse{
			ID:      srv.ID,
			Name:    srv.Name,
			Port:    srv.ListenPort,
			SlaveID: srv.SlaveID,
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

	if srv := h.cfgMgr.GetModbusTCPServer(id); srv != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": GroupResponse{
				ID:      srv.ID,
				Name:    srv.Name,
				Port:    srv.ListenPort,
				SlaveID: srv.SlaveID,
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

	srv := config.ModbusTCPServerConfig{
		ID:             req.ID,
		Name:           req.Name,
		ListenPort:     req.Port,
		SlaveID:        req.SlaveID,
		MaxConnections: 10,
	}

	if err := h.cfgMgr.AddModbusTCPServer(srv); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": err.Error(),
		})
		return
	}

	h.log.Info("创建分组", "id", req.ID, "name", req.Name)

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

	srv := config.ModbusTCPServerConfig{
		ID:             id,
		Name:           req.Name,
		ListenPort:     req.Port,
		SlaveID:        req.SlaveID,
		MaxConnections: 10,
	}

	if err := h.cfgMgr.UpdateModbusTCPServer(srv); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	h.log.Info("更新分组", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除分组
func (h *GroupHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.cfgMgr.DeleteModbusTCPServer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	h.log.Info("删除分组", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}
