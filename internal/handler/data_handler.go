package handler

import (
	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/service"
	"KamaitachiGo/pkg/common"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// DataHandler 数据处理器
type DataHandler struct {
	dataService service.DataInfoService
}

// NewDataHandler 创建数据处理器
func NewDataHandler(dataService service.DataInfoService) *DataHandler {
	return &DataHandler{
		dataService: dataService,
	}
}

// Search 搜索数据
// POST /data/v1/search
func (h *DataHandler) Search(c *gin.Context) {
	var option model.DataInfoOption
	if err := c.ShouldBindJSON(&option); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(400, "invalid request: "+err.Error()))
		return
	}

	rows, err := h.dataService.Search(&option)
	if err != nil {
		logrus.Errorf("Failed to search data: %v", err)
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(500, err.Error()))
		return
	}

	count := h.dataService.Count(&option)

	result := model.DataSet{
		Rows:  rows,
		Count: count,
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(result))
}

// Save 保存数据
// POST /data/v1/save?id=1
func (h *DataHandler) Save(c *gin.Context) {
	// 获取表ID
	tableIDStr := c.Query("id")
	tableID, err := strconv.Atoi(tableIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(400, "invalid table id"))
		return
	}

	var data model.DataInfo
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(400, "invalid request: "+err.Error()))
		return
	}

	// 验证数据
	if data.ID == "" || len(data.Data) == 0 {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(400, "id or data is empty"))
		return
	}

	// 保存数据
	if err := h.dataService.Save(tableID, &data); err != nil {
		logrus.Errorf("Failed to save data: %v", err)
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(true))
}

// Get 获取数据
// GET /data/v1/get/:id?tableId=1
func (h *DataHandler) Get(c *gin.Context) {
	id := c.Param("id")
	tableIDStr := c.Query("tableId")
	tableID, err := strconv.Atoi(tableIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(400, "invalid table id"))
		return
	}

	data, err := h.dataService.Get(tableID, id)
	if err != nil {
		logrus.Errorf("Failed to get data: %v", err)
		c.JSON(http.StatusNotFound, common.NewErrorResponse(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(data))
}

// Delete 删除数据
// DELETE /data/v1/delete/:id?tableId=1
func (h *DataHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	tableIDStr := c.Query("tableId")
	tableID, err := strconv.Atoi(tableIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.NewErrorResponse(400, "invalid table id"))
		return
	}

	if err := h.dataService.Delete(tableID, id); err != nil {
		logrus.Errorf("Failed to delete data: %v", err)
		c.JSON(http.StatusInternalServerError, common.NewErrorResponse(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, common.NewSuccessResponse(true))
}

