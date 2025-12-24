package model

import (
	"time"
)

// DataInfo 核心数据信息结构
// 对应Java中的DataInfo类，存储实体的时序数据
type DataInfo struct {
	ID         string                 `json:"id"`          // 实体ID
	Data       map[int64][]interface{} `json:"data"`        // 时序数据，key为时间戳，value为数据数组
	CreateTime string                 `json:"createTime"`  // 创建时间
	UpdateTime int64                  `json:"updateTime"`  // 更新时间
}

// NewDataInfo 创建新的DataInfo实例
func NewDataInfo(id string) *DataInfo {
	return &DataInfo{
		ID:         id,
		Data:       make(map[int64][]interface{}),
		CreateTime: time.Now().Format("2006-01-02 15:04:05"),
		UpdateTime: time.Now().Unix(),
	}
}

// AddData 添加时序数据
func (d *DataInfo) AddData(timestamp int64, values []interface{}) {
	d.Data[timestamp] = values
	d.UpdateTime = time.Now().Unix()
}

// GetDataByTimestamp 根据时间戳获取数据
func (d *DataInfo) GetDataByTimestamp(timestamp int64) ([]interface{}, bool) {
	data, exists := d.Data[timestamp]
	return data, exists
}

// DataInfoOption 数据查询选项
type DataInfoOption struct {
	TableID    int      `json:"tableId"`    // 表ID
	SubjectIDs []string `json:"subjectIds"` // 实体ID列表
	StartTime  int64    `json:"startTime"`  // 开始时间
	EndTime    int64    `json:"endTime"`    // 结束时间
	Limit      int      `json:"limit"`      // 限制数量
	Offset     int      `json:"offset"`     // 偏移量
}

// DataSet 数据集合
type DataSet struct {
	Rows  []*DataInfo `json:"rows"`  // 数据行
	Count int64       `json:"count"` // 总数
}

