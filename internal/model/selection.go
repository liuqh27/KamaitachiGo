package model

// SelectionRequest 选股请求基础结构
type SelectionRequest struct {
	Selection SelectionCriteria `json:"selection"` // 选股策略
	IDs       string            `json:"ids"`       // 指标ID，逗号分隔
	Field     string            `json:"field"`     // 排序字段
	Order     int               `json:"order"`     // 排序方式：-1倒序，1正序
	Offset    int               `json:"offset"`    // 偏移量
	Limit     int               `json:"limit"`     // 限制条数
}

// SelectionCriteria 选股策略
type SelectionCriteria struct {
	ID      string `json:"id"`      // 选股指定指标ID，如 "concept"
	Topic   string `json:"topic"`   // 股票池主题，如 "stock_a_listing_pool"
	Subject string `json:"subject"` // 实体，如 "33:300033"
	Keyword string `json:"keyword"` // 关键词，如 "互联网金融"
}

// SelectionSnapshotRequest 选股快照请求
type SelectionSnapshotRequest struct {
	SelectionRequest
	Timestamp int64 `json:"timestamp"` // 指定日期，秒级时间戳，0为最新
}

// Validate 验证选股快照请求
func (r *SelectionSnapshotRequest) Validate() error {
	if r.Selection.ID == "" {
		return ErrInvalidParameter("selection.id is required")
	}
	if r.Selection.Topic == "" {
		return ErrInvalidParameter("selection.topic is required")
	}
	if r.IDs == "" {
		return ErrInvalidParameter("ids is required")
	}
	return nil
}

// SelectionPeriodRequest 选股区间请求
type SelectionPeriodRequest struct {
	SelectionRequest
	From int64 `json:"from"` // 起始时间，秒级时间戳
	To   int64 `json:"to"`   // 结束时间，秒级时间戳
}

// Validate 验证选股区间请求
func (r *SelectionPeriodRequest) Validate() error {
	if r.Selection.ID == "" {
		return ErrInvalidParameter("selection.id is required")
	}
	if r.Selection.Topic == "" {
		return ErrInvalidParameter("selection.topic is required")
	}
	if r.IDs == "" {
		return ErrInvalidParameter("ids is required")
	}
	if r.From <= 0 || r.To <= 0 {
		return ErrInvalidParameter("from and to is required")
	}
	if r.From > r.To {
		return ErrInvalidParameter("from must be less than to")
	}
	return nil
}

// SelectionSnapshotResponse 选股快照响应
type SelectionSnapshotResponse struct {
	StatusCode int                         `json:"status_code"` // 状态码
	StatusMsg  string                      `json:"status_msg"`  // 状态信息
	Data       []SelectionSnapshotDataItem `json:"data"`        // 数据列表
}

// SelectionSnapshotDataItem 选股快照数据项
type SelectionSnapshotDataItem struct {
	Subject *ExtendedSecuritySubject `json:"subject"` // 实体信息
	Data    map[string]interface{}   `json:"data"`    // 快照数据
}

// SelectionPeriodResponse 选股区间响应
type SelectionPeriodResponse struct {
	StatusCode int                                  `json:"status_code"` // 状态码
	StatusMsg  string                               `json:"status_msg"`  // 状态信息
	Data       map[string][]SelectionPeriodDataItem `json:"data"`        // 数据列表，按日期分组
}

// SelectionPeriodDataItem 选股区间数据项
type SelectionPeriodDataItem struct {
	Subject *ExtendedSecuritySubject `json:"subject"` // 实体信息
	Data    PeriodDataDetail         `json:"data"`    // 区间数据
}

// PeriodDataDetail 区间数据详情
type PeriodDataDetail struct {
	EndDate              string      `json:"end_date"`               // 结束日期
	Period               string      `json:"period"`                 // 期间
	DeclareDate          string      `json:"declare_date"`           // 公告日期
	Year                 string      `json:"year"`                   // 年份
	Combine              string      `json:"combine"`                // 组合键
	ParentHolderNetProfit interface{} `json:"parent_holder_net_profit,omitempty"` // 归母净利润
	OperatingIncome      interface{} `json:"operating_income,omitempty"`         // 营业收入
	CustomFields         map[string]interface{} `json:"-"` // 其他自定义字段
}

// ExtendedSecuritySubject 扩展的证券实体信息
type ExtendedSecuritySubject struct {
	Subject     string `json:"subject"`      // 实体代码
	Name        string `json:"name"`         // 实体名称
	Status      string `json:"status"`       // 状态
	ListingDate string `json:"listing_date"` // 上市日期
	Category    string `json:"category"`     // 类别
}

// NewExtendedSecuritySubject 创建扩展证券实体
func NewExtendedSecuritySubject(subject, name, status, listingDate, category string) *ExtendedSecuritySubject {
	return &ExtendedSecuritySubject{
		Subject:     subject,
		Name:        name,
		Status:      status,
		ListingDate: listingDate,
		Category:    category,
	}
}

// NewSelectionSnapshotResponse 创建选股快照响应
func NewSelectionSnapshotResponse(data []SelectionSnapshotDataItem) *SelectionSnapshotResponse {
	return &SelectionSnapshotResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Data:       data,
	}
}

// NewSelectionPeriodResponse 创建选股区间响应
func NewSelectionPeriodResponse(data map[string][]SelectionPeriodDataItem) *SelectionPeriodResponse {
	return &SelectionPeriodResponse{
		StatusCode: 0,
		StatusMsg:  "success",
		Data:       data,
	}
}

// NewErrorSelectionResponse 创建错误响应
func NewErrorSelectionResponse(code int, message string) interface{} {
	return map[string]interface{}{
		"status_code": code,
		"status_msg":  message,
		"data":        nil,
	}
}

