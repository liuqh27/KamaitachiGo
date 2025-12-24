package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"KamaitachiGo/pkg/json"

	_ "modernc.org/sqlite"
)

// FinanceRecord 财报记录
type FinanceRecord struct {
	StockCode             string
	MarketCode            string
	SubjectKey            string
	StockName             string
	ReportDate            int64
	EndDate               string
	Year                  string
	Period                string
	OperatingIncome       float64
	ParentHolderNetProfit float64
	Category              string
	Topic                 string
}

var (
	dbPath     = flag.String("db", "./data/finance.db", "SQLite数据库文件路径")
	sqlDir     = flag.String("dir", "../f10sql", "SQL文件目录")
	batchSize  = flag.Int("batch", 1000, "批量插入大小")
	maxRecords = flag.Int("max", 0, "最大导入记录数（0=全部）")
)

func main() {
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   财报数据导入工具 - SQLite版本      ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// 1. 连接SQLite
	fmt.Printf("📂 数据库: %s\n", *dbPath)
	db, err := initDatabase(*dbPath)
	if err != nil {
		log.Fatal("数据库初始化失败:", err)
	}
	defer db.Close()

	// 2. 查找SQL文件
	fmt.Printf("📁 扫描目录: %s\n", *sqlDir)
	sqlFiles, err := findSQLFiles(*sqlDir)
	if err != nil {
		log.Fatal("查找SQL文件失败:", err)
	}

	if len(sqlFiles) == 0 {
		log.Fatal("未找到SQL文件")
	}

	fmt.Printf("✅ 找到 %d 个SQL文件\n\n", len(sqlFiles))

	// 3. 导入数据
	totalRecords := 0
	startTime := time.Now()

	for i, sqlFile := range sqlFiles {
		fileName := filepath.Base(sqlFile)
		fmt.Printf("[%d/%d] 处理: %s\n", i+1, len(sqlFiles), fileName)

		count, err := importSQLFile(db, sqlFile, *batchSize, *maxRecords-totalRecords)
		if err != nil {
			log.Printf("  ⚠️  警告: %v\n", err)
			continue
		}

		totalRecords += count
		fmt.Printf("  ✅ 导入 %d 条记录\n", count)

		if *maxRecords > 0 && totalRecords >= *maxRecords {
			fmt.Printf("\n⚠️  已达到最大记录数限制: %d\n", *maxRecords)
			break
		}
	}

	elapsed := time.Since(startTime)

	// 4. 显示统计
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║           导入完成统计               ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Printf("✅ 总记录数: %d\n", totalRecords)
	fmt.Printf("⏱️  总耗时: %s\n", elapsed)
	fmt.Printf("🚀 速度: %.0f 条/秒\n", float64(totalRecords)/elapsed.Seconds())
	fmt.Println()

	// 5. 验证数据
	verifyData(db)
}

// initDatabase 初始化数据库
func initDatabase(dbPath string) (*sql.DB, error) {
	// 创建目录
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// 连接数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// 创建表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS finance_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			stock_code TEXT NOT NULL,
			market_code TEXT NOT NULL,
			subject_key TEXT NOT NULL,
			stock_name TEXT,
			report_date INTEGER NOT NULL,
			end_date TEXT,
			year TEXT,
			period TEXT,
			operating_income REAL,
			parent_holder_net_profit REAL,
			category TEXT DEFAULT 'stock',
			topic TEXT DEFAULT 'stock_a_listing_pool'
		);

		CREATE INDEX IF NOT EXISTS idx_subject_date ON finance_data(subject_key, report_date DESC);
		CREATE INDEX IF NOT EXISTS idx_topic ON finance_data(topic);
		CREATE INDEX IF NOT EXISTS idx_stock_code ON finance_data(stock_code);
	`)
	if err != nil {
		return nil, err
	}

	// 性能优化
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA cache_size=10000")
	db.SetMaxOpenConns(1)

	fmt.Println("✅ 数据库初始化成功")
	return db, nil
}

// findSQLFiles 查找SQL文件
func findSQLFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".sql") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// importSQLFile 导入SQL文件
func importSQLFile(db *sql.DB, sqlFile string, batchSize int, maxRecords int) (int, error) {
	file, err := os.Open(sqlFile)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// 准备插入语句
	stmt, err := db.Prepare(`
		INSERT INTO finance_data 
		(stock_code, market_code, subject_key, stock_name, report_date, 
		 end_date, year, period, operating_income, parent_holder_net_profit, category, topic)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	// 正则表达式匹配INSERT语句
	// VALUES ('stock_code', '{json}', 'update_time', version)
	insertPattern := regexp.MustCompile(`VALUES\s*\('([^']+)',\s*'(\{[^}]+\}[^']*)',\s*'[^']*',\s*\d+\)`)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 10*1024*1024)  // 10MB buffer
	scanner.Buffer(buf, 100*1024*1024) // 最大100MB

	totalCount := 0
	batchCount := 0
	tx, _ := db.Begin()

	for scanner.Scan() {
		line := scanner.Text()

		// 查找INSERT语句
		matches := insertPattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		stockCode := matches[1]
		jsonData := matches[2]

		// 解析JSON时间序列数据
		records, err := parseTimeSeriesData(stockCode, jsonData)
		if err != nil {
			log.Printf("  ⚠️  解析失败 [%s]: %v\n", stockCode, err)
			continue
		}

		// 批量插入
		for _, record := range records {
			_, err := tx.Stmt(stmt).Exec(
				record.StockCode,
				record.MarketCode,
				record.SubjectKey,
				record.StockName,
				record.ReportDate,
				record.EndDate,
				record.Year,
				record.Period,
				record.OperatingIncome,
				record.ParentHolderNetProfit,
				record.Category,
				record.Topic,
			)
			if err != nil {
				log.Printf("  ⚠️  插入失败: %v\n", err)
				continue
			}

			totalCount++
			batchCount++

			// 批量提交
			if batchCount >= batchSize {
				if err := tx.Commit(); err != nil {
					return totalCount, err
				}
				tx, _ = db.Begin()
				batchCount = 0
			}

			// 检查最大记录数
			if maxRecords > 0 && totalCount >= maxRecords {
				tx.Commit()
				return totalCount, nil
			}
		}
	}

	// 提交剩余的
	if err := tx.Commit(); err != nil {
		return totalCount, err
	}

	if err := scanner.Err(); err != nil {
		return totalCount, err
	}

	return totalCount, nil
}

// parseTimeSeriesData 解析时间序列JSON数据
func parseTimeSeriesData(stockCode string, jsonStr string) ([]*FinanceRecord, error) {
	// SQL中的JSON格式不标准：{1234567:[...], ...}
	// 需要将数字key加上引号：{"1234567":[...], ...}
	fixedJSON := fixJSONKeys(jsonStr)

	// 解析JSON: {timestamp: [field1, field2, ...], ...}
	var timeSeriesData map[string][]interface{}

	if err := json.Unmarshal([]byte(fixedJSON), &timeSeriesData); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v", err)
	}

	var records []*FinanceRecord

	for timestampStr, values := range timeSeriesData {
		// 解析时间戳
		var timestamp int64
		fmt.Sscanf(timestampStr, "%d", &timestamp)

		// 提取字段（根据实际SQL数据结构）
		if len(values) < 10 {
			continue
		}

		record := &FinanceRecord{
			StockCode:  stockCode,
			ReportDate: timestamp,
			Category:   "stock",
			Topic:      "stock_a_listing_pool",
		}

		// 提取字段值
		if v, ok := values[1].(string); ok {
			record.EndDate = v
		}
		if v, ok := values[2].(string); ok {
			record.Year = v
		}
		if v, ok := values[3].(string); ok {
			record.Period = v
		}
		if v, ok := values[4].(float64); ok {
			record.OperatingIncome = v
		}
		// parent_holder_net_profit 的索引需要根据实际数据确定
		// 暂时使用索引 5-15 之间尝试
		for i := 5; i < len(values) && i < 15; i++ {
			if v, ok := values[i].(float64); ok && v > 0 && record.ParentHolderNetProfit == 0 {
				record.ParentHolderNetProfit = v
				break
			}
		}

		// 推断市场代码（33=深圳, 17=上海, 等）
		record.MarketCode = inferMarketCode(stockCode)
		record.SubjectKey = fmt.Sprintf("%s:%s", record.MarketCode, stockCode)

		records = append(records, record)
	}

	return records, nil
}

// fixJSONKeys 修复JSON格式
func fixJSONKeys(jsonStr string) string {
	// 1. 将转义的引号替换为正常引号：\" → "
	fixed := strings.ReplaceAll(jsonStr, `\"`, `"`)

	// 2. 将数字key加引号：{1234567:[...]} → {"1234567":[...]}
	re := regexp.MustCompile(`(\{|,)(\d+):`)
	fixed = re.ReplaceAllString(fixed, `$1"$2":`)

	return fixed
}

// inferMarketCode 推断市场代码
func inferMarketCode(stockCode string) string {
	if strings.HasPrefix(stockCode, "00") || strings.HasPrefix(stockCode, "30") {
		return "33" // 深圳
	} else if strings.HasPrefix(stockCode, "60") {
		return "17" // 上海
	}
	return "33" // 默认
}

// verifyData 验证数据
func verifyData(db *sql.DB) {
	fmt.Println("🔍 验证数据...")

	// 统计记录数
	var count int
	db.QueryRow("SELECT COUNT(*) FROM finance_data").Scan(&count)
	fmt.Printf("  📊 总记录数: %d\n", count)

	// 统计股票数
	var stockCount int
	db.QueryRow("SELECT COUNT(DISTINCT stock_code) FROM finance_data").Scan(&stockCount)
	fmt.Printf("  📈 股票数量: %d\n", stockCount)

	// 显示样本数据
	rows, err := db.Query(`
		SELECT subject_key, stock_name, end_date, operating_income, parent_holder_net_profit
		FROM finance_data
		ORDER BY operating_income DESC
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		fmt.Println("\n  📊 营业收入TOP5:")
		for rows.Next() {
			var subjectKey, stockName, endDate string
			var income, profit float64
			rows.Scan(&subjectKey, &stockName, &endDate, &income, &profit)
			if stockName == "" {
				stockName = subjectKey
			}
			fmt.Printf("    %s - %.2f亿元\n", stockName, income/100000000)
		}
	}

	fmt.Println()
}
