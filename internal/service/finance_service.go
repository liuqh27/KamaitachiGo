package service

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"KamaitachiGo/internal/cache/lru"
	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/repository"

	"github.com/sirupsen/logrus"
)

type FinanceService struct {
	repo      *repository.SQLiteRepository
	cache     *lru.Cache // 缓存 StockDataMap
	cacheHits int64
	cacheMiss int64
}

// StockDataMap 结构用于存储某个股票的结构化数据，方便按指标或时间周期查询
type StockDataMap struct {
	Snapshots map[string][]*model.SnapshotRecord // Key: indicator, Value: list of snapshot records
	Periods   map[string][]*model.PeriodRecord   // Key: "indicator_from_to", Value: list of period records
	// 还可以增加一个时间戳，用于判断数据是否新鲜
	LastUpdated time.Time
}

// Len 实现 lru.Value 接口
func (s *StockDataMap) Len() int {
	// TODO: 根据实际数据大小计算，这里只是一个示例
	size := 0
	for _, records := range s.Snapshots {
		size += len(records) * 100 // 估算每个 SnapshotRecord 约100字节
	}
	for _, records := range s.Periods {
		size += len(records) * 200 // 估算每个 PeriodRecord 约200字节
	}
	return size
}

func NewFinanceService(repo *repository.SQLiteRepository, cacheSize int64) *FinanceService {
	// 使用传入的cacheSize参数，如果太小则设置默认值
	if cacheSize < 100*1024*1024 { // 小于100MB
		cacheSize = 500 * 1024 * 1024 // 默认500MB
	}

	logrus.Infof("Initializing FinanceService with cache size: %.2f MB", float64(cacheSize)/(1024*1024))

	cache := lru.NewCache(cacheSize, func(key string, value lru.Value) {
		logrus.Debugf("Cache evicted: key %s, value of type %T", key, value)
	})

	return &FinanceService{
		repo:      repo,
		cache:     cache,
		cacheHits: 0,
		cacheMiss: 0,
	}
}

func (s *FinanceService) QuerySnapshot(req *model.SnapshotRequest) (*model.SnapshotResponse, error) {
	if req.Subjects == "" {
		return nil, fmt.Errorf("subjects is required for snapshot query")
	}

	stockID := strings.Split(req.Subjects, ",")[0] // 使用第一个subject作为 StockDataMap 的主缓存Key
	innerKey := generateSnapshotInnerKey(req)      // 生成 StockDataMap 内部的Key，代表精确的查询参数组合

	// 尝试从缓存中获取 StockDataMap (外层缓存查询)
	if cachedStockData, ok := s.cache.Get(stockID); ok {
		if stockDataMap, ok := cachedStockData.(*StockDataMap); ok {
			// 在 StockDataMap 内部查找精确的快照请求 (内层缓存查询)
			if records, found := stockDataMap.Snapshots[innerKey]; found {
				atomic.AddInt64(&s.cacheHits, 1) // 仅在内部完全命中时，才增加命中计数
				return &model.SnapshotResponse{
					StatusCode: 0,
					StatusMsg:  "success",
					Data:       records,
				}, nil
			} else {
				// 部分命中：StockDataMap存在，但请求的精确数据不在，也算作一次Miss
				atomic.AddInt64(&s.cacheMiss, 1)
				// 继续执行数据库查询，并更新 StockDataMap
			}
		}
	} else {
		atomic.AddInt64(&s.cacheMiss, 1) // 完全未命中
	}

	// 从仓库查询数据
	var records []*model.SnapshotRecord
	var err error
	// 原始请求的subjects可能包含多个，repo层需要处理
	subjects := strings.Split(req.Subjects, ",")

	if req.Topic != "" {
		// 全市场查询 (此逻辑目前不与StockDataMap精确绑定，可根据业务需求扩展)
		records, err = s.repo.QueryByTopic(req.Topic, req.Field, int(req.Order), req.Offset, req.Limit)
	} else {
		// 指定证券查询，支持多subject
		records, err = s.repo.QuerySnapshot(subjects, req.Field, int(req.Order), req.Offset, req.Limit)
	}

	if err != nil {
		return &model.SnapshotResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("query error: %v", err),
			Data:       nil,
		}, nil
	}

	// 获取或创建 StockDataMap
	var stockDataMap *StockDataMap
	if cachedStockData, ok := s.cache.Get(stockID); ok {
		// 如果已存在StockDataMap，则获取并更新
		stockDataMap = cachedStockData.(*StockDataMap)
	} else {
		// 如果不存在，则创建新的StockDataMap
		stockDataMap = &StockDataMap{
			Snapshots: make(map[string][]*model.SnapshotRecord),
			Periods:   make(map[string][]*model.PeriodRecord),
		}
	}

	// 更新 StockDataMap，并刷新外层缓存的LRU状态
	stockDataMap.Snapshots[innerKey] = records
	stockDataMap.LastUpdated = time.Now() // 记录更新时间
	s.cache.Add(stockID, stockDataMap)    // 重新添加到缓存，更新LRU顺序和可能的大小

	response := &model.SnapshotResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Data:       records,
	}

	return response, nil
}



// generateSnapshotInnerKey 为 SnapshotRequest 生成 StockDataMap 内部的 Key
func generateSnapshotInnerKey(req *model.SnapshotRequest) string {
	// 对IDs和Subjects进行排序，确保不同顺序的相同内容能生成相同的Key
	ids := strings.Split(req.IDs, ",")
	subjects := strings.Split(req.Subjects, ",")
	sort.Strings(ids)
	sort.Strings(subjects)
	sortedIDs := strings.Join(ids, ",")
	sortedSubjects := strings.Join(subjects, "_")

	return fmt.Sprintf("snap_%s_%s_%d_%d_%d_%s", sortedIDs, req.Field, int(req.Order), req.Offset, req.Limit, sortedSubjects)
}

// QueryPeriod 区间查询
func (s *FinanceService) QueryPeriod(req *model.PeriodRequest) (*model.PeriodResponse, error) {
	if req.Subjects == "" {
		return nil, fmt.Errorf("subjects is required for period query")
	}

	stockID := strings.Split(req.Subjects, ",")[0] // 使用第一个subject作为缓存的stockID
	innerKey := generatePeriodInnerKey(req)

	// 尝试从缓存中获取 StockDataMap
	if cachedStockData, ok := s.cache.Get(stockID); ok {
		if stockDataMap, ok := cachedStockData.(*StockDataMap); ok {
			// 在 StockDataMap 中查找精确的区间请求
			if records, found := stockDataMap.Periods[innerKey]; found {
				atomic.AddInt64(&s.cacheHits, 1) // 仅在内部完全命中时，才增加命中计数
				logrus.Debugf("Cache hit (inner): %s - %s", stockID, innerKey)
				return &model.PeriodResponse{
					StatusCode: 0,
					StatusMsg:  "success",
					Data:       records,
				}, nil
			} else {
				// 部分命中：StockDataMap存在，但请求的精确数据不在，也算作一次Miss
				atomic.AddInt64(&s.cacheMiss, 1)
				logrus.Debugf("Cache partial hit: %s - %s. Fetching missing data.", stockID, innerKey)
				// 继续执行数据库查询，并更新 StockDataMap
			}
		}
	} else {
		atomic.AddInt64(&s.cacheMiss, 1) // 完全未命中
		logrus.Debugf("Cache miss (full): %s. Fetching from repo.", stockID)
	}

	// 从仓库查询数据
	var records []*model.PeriodRecord
	var err error
	subjects := strings.Split(req.Subjects, ",") // 原始请求的subjects

	records, err = s.repo.QueryPeriod(subjects, req.From, req.To)

	if err != nil {
		return &model.PeriodResponse{
			StatusCode: 500,
			StatusMsg:  fmt.Sprintf("query error: %v", err),
			Data:       nil,
		}, nil
	}

	// 获取或创建 StockDataMap
	var stockDataMap *StockDataMap
	if cachedStockData, ok := s.cache.Get(stockID); ok {
		stockDataMap = cachedStockData.(*StockDataMap)
	} else {
		stockDataMap = &StockDataMap{
			Snapshots: make(map[string][]*model.SnapshotRecord),
			Periods:   make(map[string][]*model.PeriodRecord),
		}
	}

	// 更新 StockDataMap
	stockDataMap.Periods[innerKey] = records
	stockDataMap.LastUpdated = time.Now()
	s.cache.Add(stockID, stockDataMap) // 重新添加到缓存，更新LRU

	response := &model.PeriodResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Data:       records,
	}

	return response, nil
}

// generatePeriodInnerKey 为 PeriodRequest 生成 StockDataMap 内部的 Key
func generatePeriodInnerKey(req *model.PeriodRequest) string {
	// 对IDs和Subjects进行排序，确保不同顺序的相同内容能生成相同的Key
	ids := strings.Split(req.IDs, ",")
	subjects := strings.Split(req.Subjects, ",")
	sort.Strings(ids)
	sort.Strings(subjects)
	sortedIDs := strings.Join(ids, ",")
	sortedSubjects := strings.Join(subjects, "_")

	return fmt.Sprintf("period_%s_%d_%d_%s", sortedIDs, req.From, req.To, sortedSubjects)
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

// GetCacheStats 获取缓存统计
func (s *FinanceService) GetCacheStats() map[string]interface{} {
	hits := atomic.LoadInt64(&s.cacheHits)
	miss := atomic.LoadInt64(&s.cacheMiss)
	total := hits + miss

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	// logrus.Debugf("GetCacheStats: hits=%d, miss=%d, total=%d, hitRate=%.2f", hits, miss, total, hitRate)

	return map[string]interface{}{
		"entries":  s.cache.Len(),
		"hits":     hits,
		"misses":   miss,
		"hit_rate": hitRate, // Return float64 directly
	}
}

// ResetCacheStats 重置缓存统计
func (s *FinanceService) ResetCacheStats() {
	atomic.StoreInt64(&s.cacheHits, 0)
	atomic.StoreInt64(&s.cacheMiss, 0)
}
