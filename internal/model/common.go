package model

import "fmt"

// SecuritySubject 证券实体信息
type SecuritySubject struct {
	Subject string `json:"subject"` // 实体代码
	Name    string `json:"name"`    // 实体名称
	Market  string `json:"market"`  // 市场
	Type    string `json:"type"`    // 类型
}

// NewSecuritySubject 创建证券实体
func NewSecuritySubject(subject, name, market, subjectType string) *SecuritySubject {
	return &SecuritySubject{
		Subject: subject,
		Name:    name,
		Market:  market,
		Type:    subjectType,
	}
}

// SubjectsGroup 实体分组
type SubjectsGroup struct {
	Dic map[string]*SecuritySubject // 实体字典
}

// NewSubjectsGroup 创建实体分组
func NewSubjectsGroup() *SubjectsGroup {
	return &SubjectsGroup{
		Dic: make(map[string]*SecuritySubject),
	}
}

// Add 添加实体
func (g *SubjectsGroup) Add(subject *SecuritySubject) {
	if subject != nil && subject.Subject != "" {
		g.Dic[subject.Subject] = subject
	}
}

// Get 获取实体
func (g *SubjectsGroup) Get(subject string) (*SecuritySubject, bool) {
	s, ok := g.Dic[subject]
	return s, ok
}

// DispatcherQuery 分发查询
type DispatcherQuery struct {
	Partition int                `json:"partition"` // 分区号
	Subjects  []*SecuritySubject `json:"subjects"`  // 实体列表
}

// NewDispatcherQuery 创建分发查询
func NewDispatcherQuery(partition int) *DispatcherQuery {
	return &DispatcherQuery{
		Partition: partition,
		Subjects:  make([]*SecuritySubject, 0),
	}
}

// AddSubject 添加实体
func (d *DispatcherQuery) AddSubject(subject *SecuritySubject) {
	d.Subjects = append(d.Subjects, subject)
}

// TableInfo 表信息
type TableInfo struct {
	ID          int    `json:"id"`          // 表ID
	Name        string `json:"name"`        // 表名
	Description string `json:"description"` // 描述
	Partition   int    `json:"partition"`   // 分区数
}

// NewTableInfo 创建表信息
func NewTableInfo(id int, name, description string, partition int) *TableInfo {
	return &TableInfo{
		ID:          id,
		Name:        name,
		Description: description,
		Partition:   partition,
	}
}

// KamaitachiError 自定义错误类型
type KamaitachiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *KamaitachiError) Error() string {
	return fmt.Sprintf("code: %d, message: %s", e.Code, e.Message)
}

// 预定义错误
var (
	ErrInvalidParameter = func(msg string) error {
		return &KamaitachiError{Code: 400, Message: msg}
	}
	ErrNotFound = func(msg string) error {
		return &KamaitachiError{Code: 404, Message: msg}
	}
	ErrInternalError = func(msg string) error {
		return &KamaitachiError{Code: 500, Message: msg}
	}
)

