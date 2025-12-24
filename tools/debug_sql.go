package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
)

func main() {
	fmt.Println("=== SQL数据格式调试工具 ===\n")

	sqlFile := "../f10sql/finance_quarter_stock_benefit.sql"

	file, err := os.Open(sqlFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	insertPattern := regexp.MustCompile(`VALUES\s*\('([^']+)',\s*'(\{[^}]+\}[^']*)',\s*'[^']*',\s*\d+\)`)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 10*1024*1024)
	scanner.Buffer(buf, 100*1024*1024)

	count := 0
	for scanner.Scan() {
		line := scanner.Text()

		matches := insertPattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		stockCode := matches[1]
		jsonData := matches[2]

		count++
		fmt.Printf("=== 第 %d 条数据 ===\n", count)
		fmt.Printf("股票代码: %s\n", stockCode)
		fmt.Printf("JSON长度: %d 字符\n", len(jsonData))
		fmt.Printf("JSON前500字符:\n%s\n\n", jsonData[:min(500, len(jsonData))])

		if count >= 3 {
			break
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
