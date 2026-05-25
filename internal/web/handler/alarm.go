package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// AlarmHandler 报警管理处理器
type AlarmHandler struct {
	log *logger.Logger
}

// NewAlarmHandler 创建报警处理器
func NewAlarmHandler(log *logger.Logger) *AlarmHandler {
	return &AlarmHandler{log: log}
}

// AlarmResponse 报警响应
type AlarmResponse struct {
	ID         int64  `json:"id"`
	DeviceID   string `json:"device_id"`
	PointName  string `json:"point_name"`
	Level      string `json:"level"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
	Acked      bool   `json:"acked"`
}

// List 获取报警列表
func (h *AlarmHandler) List(c *gin.Context) {
	// TODO: 查询报警记录
	alarms := make([]AlarmResponse, 0)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    alarms,
	})
}

// Ack 确认报警
func (h *AlarmHandler) Ack(c *gin.Context) {
	id := c.Param("id")
	_ = id

	// TODO: 标记报警已确认
	h.log.Info("确认报警", "id", id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "确认成功",
	})
}

// GetStats 获取报警统计
func (h *AlarmHandler) GetStats(c *gin.Context) {
	// TODO: 统计报警
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"total":    0,
			"unacked":  0,
			"critical": 0,
			"warning":  0,
		},
	})
}
