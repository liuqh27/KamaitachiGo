package repository

import (
	"KamaitachiGo/internal/cache/lru"
	"KamaitachiGo/internal/model"
	"KamaitachiGo/pkg/common"
	"KamaitachiGo/pkg/json"
	"sync"
)

// MemoryRepository 内存存储仓库
type MemoryRepository struct {
	cache *lru.Cache
	mu    sync.RWMutex
}

// NewMemoryRepository 创建内存仓库
func NewMemoryRepository(cache *lru.Cache) *MemoryRepository {
	return &MemoryRepository{
		cache: cache,
	}
}

// Save 保存数据信息
func (r *MemoryRepository) Save(tableName string, data *model.DataInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// 序列化数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		return model.ErrInternalError("failed to marshal data: " + err.Error())
	}
	
	// 构建缓存key
	key := tableName + ":" + data.ID
	
	// 保存到缓存
	r.cache.Add(key, common.NewByteView(jsonData))
	
	return nil
}

// Get 获取数据信息
func (r *MemoryRepository) Get(tableName, id string) (*model.DataInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	key := tableName + ":" + id
	
	value, ok := r.cache.Get(key)
	if !ok {
		return nil, model.ErrNotFound("data not found: " + key)
	}
	
	// 反序列化数据
	var data model.DataInfo
	if err := json.Unmarshal(value.(common.ByteView).ByteSlice(), &data); err != nil {
		return nil, model.ErrInternalError("failed to unmarshal data: " + err.Error())
	}
	
	return &data, nil
}

// Search 搜索数据
func (r *MemoryRepository) Search(option *model.DataInfoOption) ([]*model.DataInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make([]*model.DataInfo, 0)
	
	// 遍历缓存，查找匹配的数据
	entries := r.cache.GetAll()
	for _, entry := range entries {
		var data model.DataInfo
		if err := json.Unmarshal(entry.Value.(common.ByteView).ByteSlice(), &data); err != nil {
			continue
		}
		
		// 简单的过滤逻辑
		if len(option.SubjectIDs) > 0 {
			matched := false
			for _, id := range option.SubjectIDs {
				if data.ID == id {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		
		result = append(result, &data)
		
		// 应用分页
		if option.Limit > 0 && len(result) >= option.Limit {
			break
		}
	}
	
	return result, nil
}

// Count 统计数据数量
func (r *MemoryRepository) Count(option *model.DataInfoOption) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if len(option.SubjectIDs) == 0 {
		return int64(r.cache.Len())
	}
	
	// 统计匹配的数据
	count := int64(0)
	entries := r.cache.GetAll()
	for _, entry := range entries {
		var data model.DataInfo
		if err := json.Unmarshal(entry.Value.(common.ByteView).ByteSlice(), &data); err != nil {
			continue
		}
		
		for _, id := range option.SubjectIDs {
			if data.ID == id {
				count++
				break
			}
		}
	}
	
	return count
}

// Delete 删除数据
func (r *MemoryRepository) Delete(tableName, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	key := tableName + ":" + id
	r.cache.Remove(key)
	
	return nil
}

