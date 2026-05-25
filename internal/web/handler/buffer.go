package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/internal/storage/buffer"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// BufferHandler 断点续传处理器
type BufferHandler struct {
	buf *buffer.OfflineBuffer
	log *logger.Logger
}

// NewBufferHandler 创建缓冲处理器
func NewBufferHandler(buf *buffer.OfflineBuffer, log *logger.Logger) *BufferHandler {
	return &BufferHandler{buf: buf, log: log}
}

// GetStatus 获取补传状态
func (h *BufferHandler) GetStatus(c *gin.Context) {
	// TODO: 查询各组补传状态
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"groups": []gin.H{},
		},
	})
}

// GetGroupStatus 获取指定组补传状态
func (h *BufferHandler) GetGroupStatus(c *gin.Context) {
	groupID := c.Param("groupId")

	count, err := h.buf.CountUntransmitted(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败: " + err.Error(),
		})
		return
	}

	earliest, _ := h.buf.EarliestUntransmitted(groupID)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"group_id":    groupID,
			"pending":     count,
			"earliest":    earliest,
		},
	})
}

// Retry 手动触发重传
func (h *BufferHandler) Retry(c *gin.Context) {
	groupID := c.Param("groupId")
	_ = groupID

	// TODO: 触发补传
	h.log.Info("手动触发补传", "group_id", groupID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "补传已触发",
	})
}
