package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"KamaitachiGo/internal/model"

	_ "modernc.org/sqlite"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 性能优化配置
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA cache_size=10000")
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	return &SQLiteRepository{db: db}, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// QuerySnapshot 快照查询
func (r *SQLiteRepository) QuerySnapshot(subjects []string, field string, order int, offset, limit int) ([]*model.SnapshotRecord, error) {
	if len(subjects) == 0 {
		return nil, fmt.Errorf("subjects cannot be empty")
	}

	// 构建IN子句的占位符
	placeholders := make([]string, len(subjects))
	args := make([]interface{}, len(subjects))
	for i, subject := range subjects {
		placeholders[i] = "?"
		args[i] = subject
	}

	orderClause := "DESC"
	if order > 0 {
		orderClause = "ASC"
	}

	// 使用子查询获取每个subject的最新数据
	query := fmt.Sprintf(`
		SELECT 
			f1.subject_key,
			f1.stock_name,
			f1.end_date,
			f1.operating_income,
			f1.parent_holder_net_profit,
			f1.category
		FROM finance_data f1
		INNER JOIN (
			SELECT subject_key, MAX(report_date) as max_date
			FROM finance_data
			WHERE subject_key IN (%s)
			GROUP BY subject_key
		) f2 ON f1.subject_key = f2.subject_key AND f1.report_date = f2.max_date
		ORDER BY f1.%s %s
		LIMIT ? OFFSET ?
	`, strings.Join(placeholders, ","), field, orderClause)

	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*model.SnapshotRecord
	for rows.Next() {
		record := &model.SnapshotRecord{
			Subject: &model.SubjectInfo{},
			Data:    make(map[string]interface{}),
		}

		var subjectKey, endDate, category string
		var stockName sql.NullString
		var income, profit sql.NullFloat64

		err := rows.Scan(&subjectKey, &stockName, &endDate, &income, &profit, &category)
		if err != nil {
			return nil, err
		}

		// 填充Subject信息
		record.Subject.Subject = subjectKey
		if stockName.Valid {
			record.Subject.Name = stockName.String
		} else {
			record.Subject.Name = subjectKey
		}
		record.Subject.Status = "213001"
		record.Subject.ListingDate = ""
		record.Subject.Category = category

		// 填充Data
		if income.Valid {
			record.Data["operating_income"] = income.Float64
		}
		if profit.Valid {
			record.Data["parent_holder_net_profit"] = profit.Float64
		}

		records = append(records, record)
	}

	return records, nil
}

// QueryPeriod 区间查询
func (r *SQLiteRepository) QueryPeriod(subjects []string, fromDate, toDate int64) ([]*model.PeriodRecord, error) {
	if len(subjects) == 0 {
		return nil, fmt.Errorf("subjects cannot be empty")
	}

	placeholders := make([]string, len(subjects))
	args := make([]interface{}, len(subjects)+2)
	for i, subject := range subjects {
		placeholders[i] = "?"
		args[i] = subject
	}
	args[len(subjects)] = fromDate
	args[len(subjects)+1] = toDate

	query := fmt.Sprintf(`
		SELECT 
			subject_key,
			stock_name,
			end_date,
			period,
			year,
			operating_income,
			parent_holder_net_profit
		FROM finance_data
		WHERE subject_key IN (%s)
		  AND report_date BETWEEN ? AND ?
		ORDER BY subject_key, report_date DESC
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 按subject分组
	recordMap := make(map[string]*model.PeriodRecord)

	for rows.Next() {
		var subjectKey, endDate, period, year string
		var stockName sql.NullString
		var income, profit sql.NullFloat64

		err := rows.Scan(&subjectKey, &stockName, &endDate, &period, &year, &income, &profit)
		if err != nil {
			return nil, err
		}

		// 获取或创建PeriodRecord
		record, exists := recordMap[subjectKey]
		if !exists {
			record = &model.PeriodRecord{
				Subject: &model.SubjectInfo{
					Subject:     subjectKey,
					Name:        subjectKey,
					Status:      "213001",
					ListingDate: "",
					Category:    "stock",
				},
				Data: make([]*model.PeriodDataItem, 0),
			}
			if stockName.Valid {
				record.Subject.Name = stockName.String
			}
			recordMap[subjectKey] = record
		}

		// 添加数据项
		dataItem := &model.PeriodDataItem{
			EndDate:     endDate,
			Period:      period,
			DeclareDate: "",
			Year:        year,
			Combine:     fmt.Sprintf("%s:%s_%s", subjectKey, year, period),
		}

		if income.Valid {
			dataItem.OperatingIncome = income.Float64
		}
		if profit.Valid {
			dataItem.ParentHolderNetProfit = profit.Float64
		}

		record.Data = append(record.Data, dataItem)
	}

	// 转换为切片
	records := make([]*model.PeriodRecord, 0, len(recordMap))
	for _, record := range recordMap {
		records = append(records, record)
	}

	return records, nil
}

// QueryByTopic 主题池查询（全市场）
func (r *SQLiteRepository) QueryByTopic(topic string, field string, order int, offset, limit int) ([]*model.SnapshotRecord, error) {
	orderClause := "DESC"
	if order > 0 {
		orderClause = "ASC"
	}

	query := fmt.Sprintf(`
		SELECT 
			f1.subject_key,
			f1.stock_name,
			f1.end_date,
			f1.operating_income,
			f1.parent_holder_net_profit,
			f1.category
		FROM finance_data f1
		INNER JOIN (
			SELECT subject_key, MAX(report_date) as max_date
			FROM finance_data
			WHERE topic = ?
			GROUP BY subject_key
		) f2 ON f1.subject_key = f2.subject_key AND f1.report_date = f2.max_date
		ORDER BY f1.%s %s
		LIMIT ? OFFSET ?
	`, field, orderClause)

	rows, err := r.db.Query(query, topic, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*model.SnapshotRecord
	for rows.Next() {
		record := &model.SnapshotRecord{
			Subject: &model.SubjectInfo{},
			Data:    make(map[string]interface{}),
		}

		var subjectKey, endDate, category string
		var stockName sql.NullString
		var income, profit sql.NullFloat64

		err := rows.Scan(&subjectKey, &stockName, &endDate, &income, &profit, &category)
		if err != nil {
			return nil, err
		}

		record.Subject.Subject = subjectKey
		if stockName.Valid {
			record.Subject.Name = stockName.String
		} else {
			record.Subject.Name = subjectKey
		}
		record.Subject.Status = "213001"
		record.Subject.ListingDate = ""
		record.Subject.Category = category

		if income.Valid {
			record.Data["operating_income"] = income.Float64
		}
		if profit.Valid {
			record.Data["parent_holder_net_profit"] = profit.Float64
		}

		records = append(records, record)
	}

	return records, nil
}

// GetStats 获取统计信息
func (r *SQLiteRepository) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总记录数
	var totalRecords int
	r.db.QueryRow("SELECT COUNT(*) FROM finance_data").Scan(&totalRecords)
	stats["total_records"] = totalRecords

	// 股票数量
	var stockCount int
	r.db.QueryRow("SELECT COUNT(DISTINCT stock_code) FROM finance_data").Scan(&stockCount)
	stats["stock_count"] = stockCount

	return stats, nil
}
