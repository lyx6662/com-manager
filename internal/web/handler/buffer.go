package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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

// GetStatus 获取所有分组补传状态
func (h *BufferHandler) GetStatus(c *gin.Context) {
	if h.buf == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"groups": []gin.H{}}})
		return
	}

	// 从查询参数获取分组ID列表 (可选)
	groupIDs := c.QueryArray("group_id")

	groups := make([]gin.H, 0)

	if len(groupIDs) > 0 {
		// 查询指定分组
		for _, groupID := range groupIDs {
			status := h.getGroupStatus(groupID)
			groups = append(groups, status)
		}
	} else {
		// 获取所有分组
		allGroupIDs, err := h.buf.GetAllGroupIDs()
		if err != nil {
			h.log.Error("获取分组列表失败", "error", err)
		}
		for _, groupID := range allGroupIDs {
			status := h.getGroupStatus(groupID)
			groups = append(groups, status)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"groups": groups,
		},
	})
}

// GetGroupStatus 获取指定组补传状态
func (h *BufferHandler) GetGroupStatus(c *gin.Context) {
	groupID := c.Param("groupId")

	if h.buf == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "缓冲服务未初始化"})
		return
	}

	status := h.getGroupStatus(groupID)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": status,
	})
}

// getGroupStatus 获取单个分组状态
func (h *BufferHandler) getGroupStatus(groupID string) gin.H {
	count, _ := h.buf.CountUntransmitted(groupID)
	earliest, _ := h.buf.EarliestUntransmitted(groupID)
	total, _ := h.buf.Size()

	result := gin.H{
		"id":            groupID,
		"name":          groupID,
		"group_id":      groupID,
		"pending":       count,
		"total_records": total,
	}

	if !earliest.IsZero() {
		result["earliest"] = earliest
	}

	return result
}

// Retry 手动触发重传
func (h *BufferHandler) Retry(c *gin.Context) {
	groupID := c.Param("groupId")

	if h.buf == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "缓冲服务未初始化"})
		return
	}

	// 查询待补传数量
	count, err := h.buf.CountUntransmitted(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败: " + err.Error(),
		})
		return
	}

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "没有待补传的数据",
		})
		return
	}

	// 加载待补传数据
	records, err := h.buf.LoadUntransmitted(groupID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "加载数据失败: " + err.Error(),
		})
		return
	}

	h.log.Info("手动触发补传",
		"group_id", groupID,
		"count", len(records),
	)

	// 注意: 实际的数据重传需要引擎配合
	// 这里只是返回待补传数据的信息，实际重传由引擎的 handleMasterConnected 处理
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "补传请求已提交",
		"data": gin.H{
			"group_id":   groupID,
			"pending":    count,
			"loaded":     len(records),
		},
	})
}

// MarkAllTransmitted 将指定分组所有未补传数据标记为已补传
func (h *BufferHandler) MarkAllTransmitted(c *gin.Context) {
	groupID := c.Param("groupId")

	if h.buf == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "缓冲服务未初始化"})
		return
	}

	affected, err := h.buf.MarkAllTransmitted(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "操作失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": fmt.Sprintf("已标记 %d 条记录为已补传", affected),
		"data":    gin.H{"affected": affected},
	})
}

// PurgeTransmitted 清除所有已补传数据
func (h *BufferHandler) PurgeTransmitted(c *gin.Context) {
	if h.buf == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "缓冲服务未初始化"})
		return
	}

	affected, err := h.buf.PurgeTransmitted()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "操作失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": fmt.Sprintf("已清除 %d 条已补传数据", affected),
		"data":    gin.H{"affected": affected},
	})
}

// GetRecords 分页查询待补传数据记录
func (h *BufferHandler) GetRecords(c *gin.Context) {
	groupID := c.Param("groupId")

	if h.buf == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "缓冲服务未初始化"})
		return
	}

	page := 1
	pageSize := 50
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 200 {
			pageSize = v
		}
	}

	records, total, err := h.buf.LoadPendingRecords(groupID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"records":   records,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetHeartbeat 获取心跳状态
func (h *BufferHandler) GetHeartbeat(c *gin.Context) {
	if h.buf == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"alive":        false,
				"last_heartbeat": 0,
				"elapsed_sec":  -1,
			},
		})
		return
	}

	ts, err := h.buf.ReadHeartbeat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "读取心跳失败: " + err.Error()})
		return
	}

	now := time.Now().UnixMilli()
	elapsed := (now - ts) / 1000 // 秒

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"alive":          elapsed < 20,
			"last_heartbeat": ts,
			"elapsed_sec":    elapsed,
		},
	})
}
