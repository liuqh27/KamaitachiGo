package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/marcboeker/go-duckdb"
)

func main() {
	fmt.Println("=== DuckDB测试程序 ===")
	fmt.Println()

	// 1. 连接DuckDB（会自动创建文件）
	fmt.Println("1. 连接DuckDB...")
	db, err := sql.Open("duckdb", "./test.duckdb")
	if err != nil {
		log.Fatal("连接失败:", err)
	}
	defer db.Close()
	fmt.Println("✅ 连接成功！")
	fmt.Println()

	// 2. 创建测试表
	fmt.Println("2. 创建测试表...")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_stocks (
			id INTEGER PRIMARY KEY,
			stock_code VARCHAR(10),
			stock_name VARCHAR(50),
			operating_income DOUBLE,
			net_profit DOUBLE
		)
	`)
	if err != nil {
		log.Fatal("创建表失败:", err)
	}
	fmt.Println("✅ 表创建成功！")
	fmt.Println()

	// 3. 插入测试数据
	fmt.Println("3. 插入测试数据...")
	testData := []struct {
		id     int
		code   string
		name   string
		income float64
		profit float64
	}{
		{1, "000001", "平安银行", 69385000000, 24870000000},
		{2, "300033", "同花顺", 1779405283.66, 501859087.71},
		{3, "600028", "中国石化", 735356000000, 13264000000},
	}

	stmt, err := db.Prepare("INSERT INTO test_stocks VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal("准备插入语句失败:", err)
	}
	defer stmt.Close()

	for _, data := range testData {
		_, err = stmt.Exec(data.id, data.code, data.name, data.income, data.profit)
		if err != nil {
			log.Fatal("插入数据失败:", err)
		}
		fmt.Printf("  ✅ 插入: %s %s\n", data.code, data.name)
	}
	fmt.Println()

	// 4. 查询数据
	fmt.Println("4. 查询数据...")
	rows, err := db.Query(`
		SELECT stock_code, stock_name, operating_income, net_profit 
		FROM test_stocks 
		ORDER BY operating_income DESC
	`)
	if err != nil {
		log.Fatal("查询失败:", err)
	}
	defer rows.Close()

	fmt.Println("\n📊 查询结果（按营业收入排序）:")
	fmt.Println("-----------------------------------------------------------")
	fmt.Printf("%-10s %-15s %20s %20s\n", "代码", "名称", "营业收入", "净利润")
	fmt.Println("-----------------------------------------------------------")

	for rows.Next() {
		var code, name string
		var income, profit float64
		if err := rows.Scan(&code, &name, &income, &profit); err != nil {
			log.Fatal("读取数据失败:", err)
		}
		fmt.Printf("%-10s %-15s %20.2f %20.2f\n", code, name, income, profit)
	}
	fmt.Println("-----------------------------------------------------------")
	fmt.Println()

	// 5. 统计查询
	fmt.Println("5. 统计查询...")
	var totalIncome, avgProfit float64
	err = db.QueryRow(`
		SELECT 
			SUM(operating_income) as total_income,
			AVG(net_profit) as avg_profit
		FROM test_stocks
	`).Scan(&totalIncome, &avgProfit)
	if err != nil {
		log.Fatal("统计查询失败:", err)
	}

	fmt.Printf("  总营业收入: %.2f 亿元\n", totalIncome/100000000)
	fmt.Printf("  平均净利润: %.2f 亿元\n", avgProfit/100000000)
	fmt.Println()

	fmt.Println("=== 测试完成！DuckDB工作正常 ===")
	fmt.Println()
	fmt.Println("💡 说明:")
	fmt.Println("  - 已创建数据库文件: test.duckdb")
	fmt.Println("  - 不需要管理员权限")
	fmt.Println("  - 可以在VS Code的DuckDB扩展中打开该文件查看")
	fmt.Println("  - 准备好处理赛事方的SQL数据了！")
}
