package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type FinanceHandler struct {
	service *service.FinanceService
}

func NewFinanceHandler(service *service.FinanceService) *FinanceHandler {
	return &FinanceHandler{
		service: service,
	}
}

// Snapshot 快照查询接口
// POST /kamaitachi/api/data/v1/snapshot/
func (h *FinanceHandler) Snapshot(c *gin.Context) {
	var req model.SnapshotRequest

	// 读取原始请求 body，便于在解析失败时打印具体内容
	raw, err := c.GetRawData()
	if err != nil {
		logrus.Errorf("read request body error: %v", err)
		c.JSON(http.StatusOK, model.SnapshotResponse{
			StatusCode: 400,
			StatusMsg:  fmt.Sprintf("invalid request: %v", err),
			Data:       nil,
		})
		return
	}
    // logrus.Debugf("Snapshot handler: Raw request body: %s", string(raw))

	// 先用 json.Unmarshal 尝试解析为结构体（使用 model.Order 的自定义反序列化）
	if err := json.Unmarshal(raw, &req); err != nil {
		logrus.Errorf("bind request error: %v, raw: %s", err, string(raw))
		c.JSON(http.StatusOK, model.SnapshotResponse{
			StatusCode: 400,
			StatusMsg:  fmt.Sprintf("invalid request: %v", err),
			Data:       nil,
		})
		return
	}

	// 把 body 放回去以防后续中间件需要（虽然这里不再使用 ShouldBindJSON）
	c.Request.Body = io.NopCloser(strings.NewReader(string(raw)))

	// 设置默认值
	if req.Field == "" {
		req.Field = "operating_income"
	}
	if req.Order == 0 {
		req.Order = -1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	logrus.Debugf("snapshot request: ids=%s, subjects=%s, topic=%s, field=%s, order=%d, offset=%d, limit=%d",
		req.IDs, req.Subjects, req.Topic, req.Field, req.Order, req.Offset, req.Limit)

	response, err := h.service.QuerySnapshot(&req)
	if err != nil {
		logrus.Errorf("query snapshot error: %v", err)
		c.JSON(http.StatusOK, model.SnapshotResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("query error: %v", err),
			Data:       nil,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Period 区间查询接口
// POST /kamaitachi/api/data/v1/period/
func (h *FinanceHandler) Period(c *gin.Context) {
	var req model.PeriodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Errorf("bind request error: %v", err)
		c.JSON(http.StatusOK, model.PeriodResponse{
			StatusCode: 400,
			StatusMsg:  fmt.Sprintf("invalid request: %v", err),
			Data:       nil,
		})
		return
	}

	logrus.Debugf("period request: ids=%s, subjects=%s, from=%d, to=%d",
		req.IDs, req.Subjects, req.From, req.To)

	response, err := h.service.QueryPeriod(&req)
	if err != nil {
		logrus.Errorf("query period error: %v", err)
		c.JSON(http.StatusOK, model.PeriodResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("query error: %v", err),
			Data:       nil,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Stats 统计信息接口
// GET /kamaitachi/api/data/v1/stats
func (h *FinanceHandler) Stats(c *gin.Context) {
	cacheStats := h.service.GetCacheStats()

	c.JSON(http.StatusOK, gin.H{
		"status_code": 0,
		"status_msg":  "success",
		"data": gin.H{
			"cache": cacheStats,
		},
	})
}

// ResetCacheStats 重置缓存统计
// POST /kamaitachi/api/data/v1/cache/reset
func (h *FinanceHandler) ResetCacheStats(c *gin.Context) {
	h.service.ResetCacheStats()
	c.JSON(http.StatusOK, gin.H{
		"status_code": 0,
		"status_msg":  "cache stats reset successfully",
	})
}
