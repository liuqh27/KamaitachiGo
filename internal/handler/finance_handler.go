package handler

import (
	"fmt"
	"net/http"

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
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.Errorf("bind request error: %v", err)
		c.JSON(http.StatusOK, model.SnapshotResponse{
			StatusCode: 400,
			StatusMsg:  fmt.Sprintf("invalid request: %v", err),
			Data:       nil,
		})
		return
	}

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
	stats, err := h.service.GetStats()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status_code": 500,
			"status_msg":  fmt.Sprintf("error: %v", err),
			"data":        nil,
		})
		return
	}

	cacheStats := h.service.GetCacheStats()
	stats["cache"] = cacheStats

	c.JSON(http.StatusOK, gin.H{
		"status_code": 0,
		"status_msg":  "success",
		"data":        stats,
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
