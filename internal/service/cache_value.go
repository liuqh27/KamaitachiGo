package service

import (
	"KamaitachiGo/internal/model"
	"KamaitachiGo/pkg/json"
)

// CacheValue 实现lru.Value接口的包装器
type CacheValue struct {
	Data interface{}
	size int
}

// NewCacheValue 创建缓存值包装器
func NewCacheValue(data interface{}) *CacheValue {
	// 计算大小（简单估算）
	jsonData, _ := json.Marshal(data)
	return &CacheValue{
		Data: data,
		size: len(jsonData),
	}
}

// Len 返回缓存值的大小
func (cv *CacheValue) Len() int {
	return cv.size
}

// GetSnapshotResponse 获取快照响应
func (cv *CacheValue) GetSnapshotResponse() (*model.SnapshotResponse, bool) {
	if resp, ok := cv.Data.(*model.SnapshotResponse); ok {
		return resp, true
	}
	return nil, false
}

// GetPeriodResponse 获取区间响应
func (cv *CacheValue) GetPeriodResponse() (*model.PeriodResponse, bool) {
	if resp, ok := cv.Data.(*model.PeriodResponse); ok {
		return resp, true
	}
	return nil, false
}
