package model

// SnapshotRequest 快照查询请求
type SnapshotRequest struct {
	IDs       string `json:"ids"`       // "operating_income,parent_holder_net_profit"
	Subjects  string `json:"subjects"`  // "33:300033,33:000001"
	Topic     string `json:"topic"`     // "stock_a_listing_pool"
	Method    string `json:"method"`    // "market:code"
	Field     string `json:"field"`     // "operating_income"
	Order     int    `json:"order"`     // -1=降序, 1=升序
	Offset    int    `json:"offset"`    // 分页偏移
	Limit     int    `json:"limit"`     // 返回数量
	Timestamp int64  `json:"timestamp"` // 0=最新
}

// SnapshotResponse 快照查询响应
type SnapshotResponse struct {
	StatusCode int               `json:"status_code"`
	StatusMsg  string            `json:"status_msg"`
	Data       []*SnapshotRecord `json:"data"`
}

// SnapshotRecord 快照记录
type SnapshotRecord struct {
	Subject *SubjectInfo           `json:"subject"`
	Data    map[string]interface{} `json:"data"`
}

// PeriodRequest 区间查询请求
type PeriodRequest struct {
	IDs      string `json:"ids"`      // "operating_income,parent_holder_net_profit"
	Subjects string `json:"subjects"` // "33:000001,33:3000033"
	Method   string `json:"method"`   // "market:code"
	From     int64  `json:"from"`     // 开始时间戳
	To       int64  `json:"to"`       // 结束时间戳
}

// PeriodResponse 区间查询响应
type PeriodResponse struct {
	StatusCode int             `json:"status_code"`
	StatusMsg  string          `json:"status_msg"`
	Data       []*PeriodRecord `json:"data"`
}

// PeriodRecord 区间记录
type PeriodRecord struct {
	Subject *SubjectInfo      `json:"subject"`
	Data    []*PeriodDataItem `json:"data"`
}

// PeriodDataItem 区间数据项
type PeriodDataItem struct {
	EndDate               string  `json:"end_date"`
	Period                string  `json:"period"`
	DeclareDate           string  `json:"declare_date"`
	Year                  string  `json:"year"`
	ParentHolderNetProfit float64 `json:"parent_holder_net_profit"`
	OperatingIncome       float64 `json:"operating_income"`
	Combine               string  `json:"combine"`
}

// SubjectInfo 证券信息
type SubjectInfo struct {
	Subject     string `json:"subject"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	ListingDate string `json:"listing_date"`
	Category    string `json:"category"`
}
