package main

import (
    "database/sql"
    "fmt"
    "os"

    _ "modernc.org/sqlite"
)

func main() {
    dbPath := "../data/sample.db"
    fi, err := os.Stat("../data/sample.db")
    if err != nil {
        fmt.Printf("stat db failed: %v\n", err)
    } else {
        fmt.Printf("db file: %s, size(bytes): %d\n", fi.Name(), fi.Size())
    }

    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        fmt.Printf("open db failed: %v\n", err)
        return
    }
    defer db.Close()

    var cnt int
    if err := db.QueryRow("SELECT COUNT(*) FROM finance_data;").Scan(&cnt); err != nil {
        fmt.Printf("count query failed: %v\n", err)
        return
    }
    fmt.Printf("finance_data rows: %d\n", cnt)

    fmt.Println("sample rows:")
    rows, err := db.Query("SELECT stock_code, stock_name, report_date FROM finance_data LIMIT 5;")
    if err != nil {
        fmt.Printf("sample query failed: %v\n", err)
        return
    }
    defer rows.Close()
    for rows.Next() {
        var code, name string
        var report int64
        if err := rows.Scan(&code, &name, &report); err != nil {
            fmt.Printf("scan failed: %v\n", err)
            return
        }
        fmt.Printf("- %s | %s | %d\n", code, name, report)
    }
}
