package service

import (
	"KamaitachiGo/internal/model"
	"KamaitachiGo/internal/repository"
)

// DataInfoService 数据信息服务接口
type DataInfoService interface {
	Save(tableID int, data *model.DataInfo) error
	Get(tableID int, id string) (*model.DataInfo, error)
	Search(option *model.DataInfoOption) ([]*model.DataInfo, error)
	Count(option *model.DataInfoOption) int64
	Delete(tableID int, id string) error
}

// dataInfoServiceImpl 数据信息服务实现
type dataInfoServiceImpl struct {
	repository  *repository.MemoryRepository
	tableInfos  map[int]*model.TableInfo // 表信息映射
}

// NewDataInfoService 创建数据信息服务
func NewDataInfoService(repo *repository.MemoryRepository) DataInfoService {
	service := &dataInfoServiceImpl{
		repository: repo,
		tableInfos: make(map[int]*model.TableInfo),
	}
	
	// 初始化一些示例表信息
	service.initTableInfos()
	
	return service
}

// initTableInfos 初始化表信息
func (s *dataInfoServiceImpl) initTableInfos() {
	// 添加一些示例表
	s.tableInfos[1] = model.NewTableInfo(1, "stock_data", "股票数据表", 4)
	s.tableInfos[2] = model.NewTableInfo(2, "financial_data", "财务数据表", 4)
	s.tableInfos[3] = model.NewTableInfo(3, "indicator_data", "指标数据表", 8)
}

// Save 保存数据
func (s *dataInfoServiceImpl) Save(tableID int, data *model.DataInfo) error {
	tableInfo, exists := s.tableInfos[tableID]
	if !exists {
		return model.ErrNotFound("table not found: " + string(rune(tableID)))
	}
	
	return s.repository.Save(tableInfo.Name, data)
}

// Get 获取数据
func (s *dataInfoServiceImpl) Get(tableID int, id string) (*model.DataInfo, error) {
	tableInfo, exists := s.tableInfos[tableID]
	if !exists {
		return nil, model.ErrNotFound("table not found")
	}
	
	return s.repository.Get(tableInfo.Name, id)
}

// Search 搜索数据
func (s *dataInfoServiceImpl) Search(option *model.DataInfoOption) ([]*model.DataInfo, error) {
	return s.repository.Search(option)
}

// Count 统计数据
func (s *dataInfoServiceImpl) Count(option *model.DataInfoOption) int64 {
	return s.repository.Count(option)
}

// Delete 删除数据
func (s *dataInfoServiceImpl) Delete(tableID int, id string) error {
	tableInfo, exists := s.tableInfos[tableID]
	if !exists {
		return model.ErrNotFound("table not found")
	}
	
	return s.repository.Delete(tableInfo.Name, id)
}

