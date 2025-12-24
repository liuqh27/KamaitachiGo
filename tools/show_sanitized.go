package main

import (
    "database/sql"
    "fmt"
    "log"

    _ "modernc.org/sqlite"
)

func main() {
    db, err := sql.Open("sqlite", "./data_sanitized/master.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    rows, err := db.Query("SELECT subject_key, stock_code, stock_name, end_date, operating_income, parent_holder_net_profit FROM finance_data LIMIT 5")
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()
    fmt.Println("subject_key | stock_code | stock_name | end_date | operating_income | parent_holder_net_profit")
    for rows.Next() {
        var subjectKey, stockCode, stockName, endDate sql.NullString
        var income, profit sql.NullFloat64
        if err := rows.Scan(&subjectKey, &stockCode, &stockName, &endDate, &income, &profit); err != nil {
            log.Fatal(err)
        }
        fmt.Printf("%s | %s | %s | %s | %.2f | %.2f\n",
            subjectKey.String, stockCode.String, stockName.String, endDate.String,
            income.Float64, profit.Float64)
    }
}
