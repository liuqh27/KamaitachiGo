package handler

import (
	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// SelectionHandler 选股处理器
type SelectionHandler struct {
	selectionService service.SelectionService
}

// NewSelectionHandler 创建选股处理器
func NewSelectionHandler(selectionService service.SelectionService) *SelectionHandler {
	return &SelectionHandler{
		selectionService: selectionService,
	}
}

// SelectionSnapshot 选股快照查询
// POST /kamaitachi/api/selection/v1/snapshot
func (h *SelectionHandler) SelectionSnapshot(c *gin.Context) {
	var request model.SelectionSnapshotRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logrus.Errorf("Invalid request: %v", err)
		c.JSON(http.StatusOK, model.NewErrorSelectionResponse(-1, "param error"))
		return
	}

	// 验证请求
	if err := request.Validate(); err != nil {
		logrus.Errorf("Validation failed: %v", err)
		c.JSON(http.StatusOK, model.NewErrorSelectionResponse(-1, err.Error()))
		return
	}

	// 查询选股快照数据
	response, err := h.selectionService.SelectionSnapshot(&request)
	if err != nil {
		logrus.Errorf("Failed to query selection snapshot: %v", err)
		c.JSON(http.StatusOK, model.NewErrorSelectionResponse(500, "server error"))
		return
	}

	c.JSON(http.StatusOK, response)
}

// SelectionPeriod 选股区间查询
// POST /kamaitachi/api/selection/v1/period
func (h *SelectionHandler) SelectionPeriod(c *gin.Context) {
	var request model.SelectionPeriodRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logrus.Errorf("Invalid request: %v", err)
		c.JSON(http.StatusOK, model.NewErrorSelectionResponse(-1, "param error"))
		return
	}

	// 验证请求
	if err := request.Validate(); err != nil {
		logrus.Errorf("Validation failed: %v", err)
		c.JSON(http.StatusOK, model.NewErrorSelectionResponse(-1, err.Error()))
		return
	}

	// 查询选股区间数据
	response, err := h.selectionService.SelectionPeriod(&request)
	if err != nil {
		logrus.Errorf("Failed to query selection period: %v", err)
		c.JSON(http.StatusOK, model.NewErrorSelectionResponse(500, "server error"))
		return
	}

	c.JSON(http.StatusOK, response)
}

