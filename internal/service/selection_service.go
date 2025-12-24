package service

import (
	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/repository"
	"sort"
	"strings"
)

// SelectionService 选股服务接口
type SelectionService interface {
	SelectionSnapshot(request *model.SelectionSnapshotRequest) (*model.SelectionSnapshotResponse, error)
	SelectionPeriod(request *model.SelectionPeriodRequest) (*model.SelectionPeriodResponse, error)
}

// selectionServiceImpl 选股服务实现
type selectionServiceImpl struct {
	repository *repository.MemoryRepository
}

// NewSelectionService 创建选股服务
func NewSelectionService(repo *repository.MemoryRepository) SelectionService {
	return &selectionServiceImpl{
		repository: repo,
	}
}

// SelectionSnapshot 选股快照查询
func (s *selectionServiceImpl) SelectionSnapshot(request *model.SelectionSnapshotRequest) (*model.SelectionSnapshotResponse, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	// 解析指标ID列表
	indicatorIDs := parseIndicators(request.IDs)

	// 根据选股策略查询匹配的股票（模拟实现）
	subjects := s.findMatchingSubjects(&request.Selection)

	// 查询每个股票的快照数据
	results := make([]model.SelectionSnapshotDataItem, 0)

	for _, subject := range subjects {
		// 查询该股票的数据
		dataInfo, err := s.repository.Get("stock_data", subject.Subject)
		if err != nil {
			// 如果找不到数据，跳过
			continue
		}

		// 查找最接近指定时间的数据
		timestamp := request.Timestamp
		if timestamp == 0 {
			// 获取最新数据
			timestamp = s.getLatestTimestamp(dataInfo.Data)
		}

		// 提取指定指标的数据
		snapshotData := make(map[string]interface{})
		if values, exists := dataInfo.Data[timestamp]; exists {
			for i, id := range indicatorIDs {
				if i < len(values) {
					snapshotData[id] = values[i]
				}
			}
		}

		if len(snapshotData) > 0 {
			results = append(results, model.SelectionSnapshotDataItem{
				Subject: subject,
				Data:    snapshotData,
			})
		}
	}

	// 排序
	if request.Field != "" {
		s.sortSnapshotResults(results, request.Field, request.Order)
	}

	// 分页
	results = s.paginateSnapshotResults(results, request.Offset, request.Limit)

	return model.NewSelectionSnapshotResponse(results), nil
}

// SelectionPeriod 选股区间查询
func (s *selectionServiceImpl) SelectionPeriod(request *model.SelectionPeriodRequest) (*model.SelectionPeriodResponse, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	// 解析指标ID列表
	indicatorIDs := parseIndicators(request.IDs)

	// 根据选股策略查询匹配的股票
	subjects := s.findMatchingSubjects(&request.Selection)

	// 按日期分组的结果
	dateGroupedResults := make(map[string][]model.SelectionPeriodDataItem)

	for _, subject := range subjects {
		// 查询该股票的数据
		dataInfo, err := s.repository.Get("stock_data", subject.Subject)
		if err != nil {
			continue
		}

		// 筛选时间范围内的数据
		for timestamp, values := range dataInfo.Data {
			if timestamp >= request.From && timestamp <= request.To {
				// 格式化日期
				dateKey := formatDate(timestamp)

				// 构建数据详情
				dataDetail := model.PeriodDataDetail{
					EndDate:     dateKey,
					Period:      "596003",
					DeclareDate: formatDate(timestamp + 86400*30), // 示例：30天后公告
					Year:        formatYear(timestamp),
					Combine:     subject.Subject + ":" + formatYear(timestamp) + "_596003",
				}

				// 填充指标数据
				for i, id := range indicatorIDs {
					if i < len(values) {
						if id == "parent_holder_net_profit" {
							dataDetail.ParentHolderNetProfit = values[i]
						} else if id == "operating_income" {
							dataDetail.OperatingIncome = values[i]
						}
					}
				}

				item := model.SelectionPeriodDataItem{
					Subject: subject,
					Data:    dataDetail,
				}

				dateGroupedResults[dateKey] = append(dateGroupedResults[dateKey], item)
			}
		}
	}

	// 对每个日期分组进行排序
	if request.Field != "" {
		for date := range dateGroupedResults {
			s.sortPeriodResults(dateGroupedResults[date], request.Field, request.Order)
		}
	}

	// 对每个日期分组进行分页
	for date := range dateGroupedResults {
		dateGroupedResults[date] = s.paginatePeriodResults(dateGroupedResults[date], request.Offset, request.Limit)
	}

	return model.NewSelectionPeriodResponse(dateGroupedResults), nil
}

// findMatchingSubjects 根据选股策略查找匹配的股票（模拟实现）
func (s *selectionServiceImpl) findMatchingSubjects(criteria *model.SelectionCriteria) []*model.ExtendedSecuritySubject {
	// 这里是模拟实现，实际应该从数据库或其他数据源查询
	// 根据 topic、subject、keyword 等条件筛选股票

	// 返回模拟的股票列表
	subjects := []*model.ExtendedSecuritySubject{
		model.NewExtendedSecuritySubject("17:600606", "绿地控股", "213001", "1992-03-27", "stock"),
		model.NewExtendedSecuritySubject("33:000703", "恒逸石化", "213001", "1997-03-28", "stock"),
		model.NewExtendedSecuritySubject("33:002416", "爱施德", "213001", "2010-05-28", "stock"),
		model.NewExtendedSecuritySubject("17:601233", "桐昆股份", "213001", "2011-05-18", "stock"),
		model.NewExtendedSecuritySubject("33:300226", "上海钢联", "213001", "2011-06-08", "stock"),
	}

	return subjects
}

// getLatestTimestamp 获取最新的时间戳
func (s *selectionServiceImpl) getLatestTimestamp(data map[int64][]interface{}) int64 {
	var latest int64 = 0
	for timestamp := range data {
		if timestamp > latest {
			latest = timestamp
		}
	}
	return latest
}

// sortSnapshotResults 对快照结果排序
func (s *selectionServiceImpl) sortSnapshotResults(results []model.SelectionSnapshotDataItem, field string, order int) {
	sort.Slice(results, func(i, j int) bool {
		vi := s.getNumericValue(results[i].Data[field])
		vj := s.getNumericValue(results[j].Data[field])

		if order == -1 {
			return vi > vj // 倒序
		}
		return vi < vj // 正序
	})
}

// sortPeriodResults 对区间结果排序
func (s *selectionServiceImpl) sortPeriodResults(results []model.SelectionPeriodDataItem, field string, order int) {
	sort.Slice(results, func(i, j int) bool {
		var vi, vj float64

		if field == "parent_holder_net_profit" {
			vi = s.getNumericValue(results[i].Data.ParentHolderNetProfit)
			vj = s.getNumericValue(results[j].Data.ParentHolderNetProfit)
		} else if field == "operating_income" {
			vi = s.getNumericValue(results[i].Data.OperatingIncome)
			vj = s.getNumericValue(results[j].Data.OperatingIncome)
		}

		if order == -1 {
			return vi > vj // 倒序
		}
		return vi < vj // 正序
	})
}

// paginateSnapshotResults 对快照结果分页
func (s *selectionServiceImpl) paginateSnapshotResults(results []model.SelectionSnapshotDataItem, offset, limit int) []model.SelectionSnapshotDataItem {
	if limit <= 0 {
		limit = 10 // 默认10条
	}

	start := offset
	end := offset + limit

	if start >= len(results) {
		return []model.SelectionSnapshotDataItem{}
	}

	if end > len(results) {
		end = len(results)
	}

	return results[start:end]
}

// paginatePeriodResults 对区间结果分页
func (s *selectionServiceImpl) paginatePeriodResults(results []model.SelectionPeriodDataItem, offset, limit int) []model.SelectionPeriodDataItem {
	if limit <= 0 {
		limit = 10
	}

	start := offset
	end := offset + limit

	if start >= len(results) {
		return []model.SelectionPeriodDataItem{}
	}

	if end > len(results) {
		end = len(results)
	}

	return results[start:end]
}

// getNumericValue 获取数值
func (s *selectionServiceImpl) getNumericValue(val interface{}) float64 {
	if val == nil {
		return 0
	}

	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

// parseIndicators 解析指标ID列表
func parseIndicators(ids string) []string {
	if ids == "" {
		return []string{}
	}

	parts := strings.Split(ids, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// formatDate 格式化日期（时间戳转日期字符串）
func formatDate(timestamp int64) string {
	// 简化实现，实际应该使用 time 包
	return "2024-03-31" // 示例
}

// formatYear 格式化年份
func formatYear(timestamp int64) string {
	return "2024" // 示例
}

