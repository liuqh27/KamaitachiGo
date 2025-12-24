package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "./data/finance_test.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 1. 检查表结构
	fmt.Println("=== 数据库表结构 ===")
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		rows.Scan(&name)
		fmt.Println("表:", name)
	}

	// 2. 检查finance_data表的列
	fmt.Println("\n=== finance_data 表列 ===")
	rows2, err := db.Query("PRAGMA table_info(finance_data)")
	if err != nil {
		log.Fatal(err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value sql.NullString
		rows2.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk)
		fmt.Printf("  %s (%s)\n", name, ctype)
	}

	// 3. 查询记录数
	var count int
	db.QueryRow("SELECT COUNT(*) FROM finance_data").Scan(&count)
	fmt.Printf("\n总记录数: %d\n", count)

	// 4. 查看几条样本数据
	fmt.Println("\n=== 样本数据 ===")
	rows3, err := db.Query("SELECT subject_key, stock_code, stock_name, end_date, operating_income FROM finance_data LIMIT 5")
	if err != nil {
		log.Fatal(err)
	}
	defer rows3.Close()

	for rows3.Next() {
		var subjectKey, stockCode string
		var stockName sql.NullString
		var endDate string
		var income sql.NullFloat64
		rows3.Scan(&subjectKey, &stockCode, &stockName, &endDate, &income)
		name := "NULL"
		if stockName.Valid {
			name = stockName.String
		}
		incomeVal := 0.0
		if income.Valid {
			incomeVal = income.Float64
		}
		fmt.Printf("  %s | %s | %s | %s | %.2f\n", subjectKey, stockCode, name, endDate, incomeVal)
	}

	// 5. 列出不同的subject_key
	fmt.Println("\n=== 不同的subject_key（前20个）===")
	rows5, err := db.Query("SELECT DISTINCT subject_key FROM finance_data LIMIT 20")
	if err != nil {
		log.Fatal(err)
	}
	defer rows5.Close()

	for rows5.Next() {
		var subjectKey string
		rows5.Scan(&subjectKey)
		fmt.Printf("  %s\n", subjectKey)
	}

	// 6. 测试查询
	fmt.Println("\n=== 测试查询 33:000001 ===")
	rows4, err := db.Query(`
		SELECT subject_key, stock_name, end_date, operating_income 
		FROM finance_data 
		WHERE subject_key = '33:000001' 
		LIMIT 3
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows4.Close()

	count = 0
	for rows4.Next() {
		var subjectKey string
		var stockName sql.NullString
		var endDate string
		var income sql.NullFloat64
		rows4.Scan(&subjectKey, &stockName, &endDate, &income)
		count++
		name := "NULL"
		if stockName.Valid {
			name = stockName.String
		}
		incomeVal := 0.0
		if income.Valid {
			incomeVal = income.Float64
		}
		fmt.Printf("  %s | %s | %s | %.2f\n", subjectKey, name, endDate, incomeVal)
	}
	fmt.Printf("找到 %d 条记录\n", count)
}
