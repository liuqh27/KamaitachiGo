package main

import (
    "database/sql"
    "flag"
    "fmt"
    "log"
    "strings"

    _ "modernc.org/sqlite"
)

func main() {
    dbPath := flag.String("db", "./data/master.db", "path to sqlite db")
    limit := flag.Int("limit", 10, "sample rows per table")
    flag.Parse()

    db, err := sql.Open("sqlite", *dbPath)
    if err != nil {
        log.Fatalf("open db failed: %v", err)
    }
    defer db.Close()

    fmt.Printf("\n=== Dumping DB: %s ===\n", *dbPath)

    // tables
    rows, err := db.Query("SELECT name, type FROM sqlite_master WHERE type IN ('table','view') ORDER BY name")
    if err != nil {
        log.Fatalf("query sqlite_master failed: %v", err)
    }
    defer rows.Close()

    var tables []string
    for rows.Next() {
        var name, typ string
        rows.Scan(&name, &typ)
        if strings.HasPrefix(name, "sqlite_") {
            continue
        }
        tables = append(tables, name)
    }

    if len(tables) == 0 {
        fmt.Println("(no user tables found)")
        return
    }

    for _, t := range tables {
        fmt.Printf("\n--- Table: %s ---\n", t)
        // columns
        crow, err := db.Query("PRAGMA table_info(" + t + ")")
        if err != nil {
            fmt.Printf("PRAGMA failed for %s: %v\n", t, err)
            continue
        }
        cols := []string{}
        for crow.Next() {
            var cid int
            var name, ctype string
            var notnull, pk int
            var dflt sql.NullString
            crow.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
            cols = append(cols, fmt.Sprintf("%s(%s)", name, ctype))
        }
        crow.Close()
        fmt.Printf("Columns: %s\n", strings.Join(cols, ", "))

        // count
        var cnt int
        if err := db.QueryRow("SELECT COUNT(*) FROM "+t).Scan(&cnt); err != nil {
            fmt.Printf("COUNT failed for %s: %v\n", t, err)
            continue
        }
        fmt.Printf("Total rows: %d\n", cnt)

        // sample rows
        q := fmt.Sprintf("SELECT * FROM %s LIMIT %d", t, *limit)
        srows, err := db.Query(q)
        if err != nil {
            fmt.Printf("sample query failed for %s: %v\n", t, err)
            continue
        }
        colsNames, _ := srows.Columns()
        values := make([]interface{}, len(colsNames))
        ptrs := make([]interface{}, len(colsNames))
        for i := range values {
            ptrs[i] = &values[i]
        }
        fmt.Printf("Sample (up to %d rows):\n", *limit)
        rowCount := 0
        for srows.Next() {
            if err := srows.Scan(ptrs...); err != nil {
                fmt.Printf("scan error: %v\n", err)
                break
            }
            rowCount++
            parts := make([]string, len(colsNames))
            for i, v := range values {
                if v == nil {
                    parts[i] = "NULL"
                } else {
                    switch vv := v.(type) {
                    case int64:
                        parts[i] = fmt.Sprintf("%d", vv)
                    case float64:
                        parts[i] = fmt.Sprintf("%f", vv)
                    case []byte:
                        parts[i] = string(vv)
                    case string:
                        parts[i] = vv
                    default:
                        parts[i] = fmt.Sprintf("%v", vv)
                    }
                }
            }
            if len(parts) > 3 {
                fmt.Printf("  %s | %s | %s | ...\n", parts[0], parts[1], parts[2])
            } else {
                fmt.Printf("  %s\n", strings.Join(parts, " | "))
            }
        }
        srows.Close()
        if rowCount == 0 {
            fmt.Println("  (no sample rows)")
        }
    }
}
