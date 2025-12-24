package service

import (
	"crypto/md5"
	"fmt"
	"strings"
	"sync/atomic"

	"KamaitachiGo/internal/cache/lru"
	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/repository"
	"KamaitachiGo/pkg/json"

	"github.com/sirupsen/logrus"
)

type FinanceService struct {
	repo      *repository.SQLiteRepository
	cache     *lru.Cache
	cacheHits int64
	cacheMiss int64
}

func NewFinanceService(repo *repository.SQLiteRepository, cacheSize int64) *FinanceService {
	// 使用传入的cacheSize参数，如果太小则设置默认值
	if cacheSize < 100*1024*1024 { // 小于100MB
		cacheSize = 500 * 1024 * 1024 // 默认500MB
	}

	logrus.Infof("Initializing FinanceService with cache size: %.2f MB", float64(cacheSize)/(1024*1024))

	cache := lru.NewCache(cacheSize, func(key string, value lru.Value) {
		logrus.Debugf("Cache evicted: %s", key)
	})

	return &FinanceService{
		repo:      repo,
		cache:     cache,
		cacheHits: 0,
		cacheMiss: 0,
	}
}

// QuerySnapshot 快照查询
func (s *FinanceService) QuerySnapshot(req *model.SnapshotRequest) (*model.SnapshotResponse, error) {
	// 生成缓存key
	cacheKey := s.generateCacheKey("snapshot", req)

	// 查询缓存
	if cached, ok := s.cache.Get(cacheKey); ok {
		atomic.AddInt64(&s.cacheHits, 1)
		if cacheValue, ok := cached.(*CacheValue); ok {
			if response, ok := cacheValue.GetSnapshotResponse(); ok {
				logrus.Debugf("Cache hit: %s", cacheKey)
				return response, nil
			}
		}
	}
	atomic.AddInt64(&s.cacheMiss, 1)
	logrus.Debugf("Cache miss: %s", cacheKey)

	var records []*model.SnapshotRecord
	var err error

	// 判断是topic查询还是subjects查询
	if req.Topic != "" {
		// 全市场查询
		records, err = s.repo.QueryByTopic(req.Topic, req.Field, req.Order, req.Offset, req.Limit)
	} else if req.Subjects != "" {
		// 指定证券查询
		subjects := strings.Split(req.Subjects, ",")
		records, err = s.repo.QuerySnapshot(subjects, req.Field, req.Order, req.Offset, req.Limit)
	} else {
		return nil, fmt.Errorf("subjects or topic is required")
	}

	if err != nil {
		return &model.SnapshotResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("query error: %v", err),
			Data:       nil,
		}, nil
	}

	response := &model.SnapshotResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Data:       records,
	}

	// 写入LRU缓存（自动淘汰最久未使用的）
	cacheValue := NewCacheValue(response)
	s.cache.Add(cacheKey, cacheValue)

	return response, nil
}

// QueryPeriod 区间查询
func (s *FinanceService) QueryPeriod(req *model.PeriodRequest) (*model.PeriodResponse, error) {
	// 生成缓存key
	cacheKey := s.generateCacheKey("period", req)

	// 查询缓存
	if cached, ok := s.cache.Get(cacheKey); ok {
		atomic.AddInt64(&s.cacheHits, 1)
		if cacheValue, ok := cached.(*CacheValue); ok {
			if response, ok := cacheValue.GetPeriodResponse(); ok {
				logrus.Debugf("Cache hit: %s", cacheKey)
				return response, nil
			}
		}
	}
	atomic.AddInt64(&s.cacheMiss, 1)
	logrus.Debugf("Cache miss: %s", cacheKey)

	// 解析subjects
	if req.Subjects == "" {
		return &model.PeriodResponse{
			StatusCode: 400,
			StatusMsg:  "subjects is required",
			Data:       nil,
		}, nil
	}

	subjects := strings.Split(req.Subjects, ",")
	records, err := s.repo.QueryPeriod(subjects, req.From, req.To)

	if err != nil {
		return &model.PeriodResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("query error: %v", err),
			Data:       nil,
		}, nil
	}

	response := &model.PeriodResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Data:       records,
	}

	// 写入LRU缓存
	cacheValue := NewCacheValue(response)
	s.cache.Add(cacheKey, cacheValue)

	return response, nil
}

// WarmupCache 预热缓存
func (s *FinanceService) WarmupCache() error {
	logrus.Info("Starting cache warmup...")

	warmupCount := 0

	// 1. 预热全市场最新数据（营业收入排名）
	req1 := &model.SnapshotRequest{
		IDs:    "operating_income,parent_holder_net_profit",
		Topic:  "stock_a_listing_pool",
		Field:  "operating_income",
		Order:  -1,
		Offset: 0,
		Limit:  50,
	}
	if _, err := s.QuerySnapshot(req1); err == nil {
		warmupCount++
	}

	// 2. 预热全市场最新数据（净利润排名）
	req2 := &model.SnapshotRequest{
		IDs:    "operating_income,parent_holder_net_profit",
		Topic:  "stock_a_listing_pool",
		Field:  "parent_holder_net_profit",
		Order:  -1,
		Offset: 0,
		Limit:  50,
	}
	if _, err := s.QuerySnapshot(req2); err == nil {
		warmupCount++
	}

	// 3. 预热常用证券的快照
	commonStocks := []string{
		"33:00000009", "33:00082582", "33:01000729",
		"33:02600053", "33:02600171", "33:03131331",
	}
	for _, stock := range commonStocks {
		req := &model.SnapshotRequest{
			IDs:      "operating_income,parent_holder_net_profit",
			Subjects: stock,
			Field:    "operating_income",
			Order:    -1,
			Limit:    1,
		}
		if _, err := s.QuerySnapshot(req); err == nil {
			warmupCount++
		}
	}

	// 4. 预热常用证券的区间数据
	for _, stock := range commonStocks[:3] { // 只预热前3个
		req := &model.PeriodRequest{
			IDs:      "operating_income,parent_holder_net_profit",
			Subjects: stock,
			From:     1577836800, // 2020-01-01
			To:       1735660800, // 2025-01-01
		}
		if _, err := s.QueryPeriod(req); err == nil {
			warmupCount++
		}
	}

	logrus.Infof("Cache warmup completed: %d queries, %d entries in cache", warmupCount, s.cache.Len())
	return nil
}

// generateCacheKey 生成缓存key
func (s *FinanceService) generateCacheKey(prefix string, req interface{}) string {
	data, _ := json.Marshal(req)
	hash := md5.Sum(data)
	return fmt.Sprintf("%s:%x", prefix, hash)
}

// GetStats 获取统计信息
func (s *FinanceService) GetStats() (map[string]interface{}, error) {
	return s.repo.GetStats()
}

// GetCacheStats 获取缓存统计
func (s *FinanceService) GetCacheStats() map[string]interface{} {
	hits := atomic.LoadInt64(&s.cacheHits)
	miss := atomic.LoadInt64(&s.cacheMiss)
	total := hits + miss

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"entries":  s.cache.Len(),
		"hits":     hits,
		"misses":   miss,
		"hit_rate": fmt.Sprintf("%.2f%%", hitRate),
	}
}

// ResetCacheStats 重置缓存统计
func (s *FinanceService) ResetCacheStats() {
	atomic.StoreInt64(&s.cacheHits, 0)
	atomic.StoreInt64(&s.cacheMiss, 0)
}
