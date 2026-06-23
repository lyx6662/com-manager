package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/internal/storage/alarm"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// AlarmHandler 报警管理处理器
type AlarmHandler struct {
	log   *logger.Logger
	store *alarm.Store
}

// NewAlarmHandler 创建报警处理器
func NewAlarmHandler(log *logger.Logger, store *alarm.Store) *AlarmHandler {
	return &AlarmHandler{log: log, store: store}
}

// List 获取报警列表
func (h *AlarmHandler) List(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []alarm.Record{}})
		return
	}

	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	unackedOnly := c.Query("unacked") == "true"

	records, err := h.store.List(limit, unackedOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    records,
	})
}

// Ack 确认报警
func (h *AlarmHandler) Ack(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "报警服务未初始化"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的报警ID"})
		return
	}

	user := c.Query("user")
	if user == "" {
		user = "admin"
	}

	if err := h.store.Ack(id, user); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "确认成功",
	})
}

// GetStats 获取报警统计
func (h *AlarmHandler) GetStats(c *gin.Context) {
	if h.store == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": &alarm.Stats{}})
		return
	}

	stats, err := h.store.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": stats,
	})
}
