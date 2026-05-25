package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// GroupHandler 分组管理处理器
type GroupHandler struct {
	log *logger.Logger
}

// NewGroupHandler 创建分组处理器
func NewGroupHandler(log *logger.Logger) *GroupHandler {
	return &GroupHandler{log: log}
}

// GroupResponse 分组响应
type GroupResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Devices []string `json:"devices"`
	Port    int    `json:"port"`
}

// CreateGroupRequest 创建分组请求
type CreateGroupRequest struct {
	ID      string   `json:"id" binding:"required"`
	Name    string   `json:"name" binding:"required"`
	Devices []string `json:"devices"`
	Port    int      `json:"port"`
}

// List 获取分组列表
func (h *GroupHandler) List(c *gin.Context) {
	// TODO: 从配置读取分组
	groups := make([]GroupResponse, 0)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    groups,
	})
}

// Get 获取单个分组
func (h *GroupHandler) Get(c *gin.Context) {
	id := c.Param("id")
	_ = id

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": nil,
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

	h.log.Info("创建分组", "id", req.ID, "name", req.Name)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
	})
}

// Update 更新分组
func (h *GroupHandler) Update(c *gin.Context) {
	id := c.Param("id")
	_ = id

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// Delete 删除分组
func (h *GroupHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	_ = id

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}
