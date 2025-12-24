package main

import (
    "database/sql"
    "flag"
    "fmt"
    "math"
    "math/rand"
    "os"
    "time"

    _ "modernc.org/sqlite"
)

func main() {
    var (
        rows      int
        stocks    int
        dbPath    string
        startYear int
        endYear   int
        seed      int64
        noise     float64
    )
    flag.IntVar(&rows, "rows", 2000, "approximate number of rows to generate")
    flag.IntVar(&stocks, "stocks", 0, "number of distinct stock codes to generate (optional)")
    flag.StringVar(&dbPath, "db", "../data/sample.db", "output sqlite path")
    flag.IntVar(&startYear, "start-year", 2020, "start year (inclusive)")
    flag.IntVar(&endYear, "end-year", 2023, "end year (inclusive)")
    flag.Int64Var(&seed, "seed", 0, "random seed (0 = time-based)")
    flag.Float64Var(&noise, "noise", 0.0, "relative gaussian noise sigma (e.g. 0.05)")
    flag.Parse()

    if err := os.MkdirAll(getDir(dbPath), 0755); err != nil {
        fmt.Printf("create data dir failed: %v\n", err)
        return
    }

    if seed == 0 {
        seed = time.Now().UnixNano()
    }
    rand.Seed(seed)

    years := make([]int, 0, endYear-startYear+1)
    for y := startYear; y <= endYear; y++ {
        years = append(years, y)
    }

    if stocks <= 0 {
        stocks = int(math.Max(10, float64(rows)/float64(len(years))))
    }

    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        fmt.Printf("open db failed: %v\n", err)
        return
    }
    defer db.Close()

    schema := `
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
`

    if _, err := db.Exec(schema); err != nil {
        fmt.Printf("create schema failed: %v\n", err)
        return
    }

    tx, err := db.Begin()
    if err != nil {
        fmt.Printf("begin tx failed: %v\n", err)
        return
    }
    stmt, err := tx.Prepare(`INSERT INTO finance_data (stock_code, market_code, subject_key, stock_name, report_date, end_date, year, period, operating_income, parent_holder_net_profit, category, topic) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
    if err != nil {
        fmt.Printf("prepare failed: %v\n", err)
        return
    }
    defer stmt.Close()

    inserted := 0
    baseCode := 100000
    for i := 0; i < stocks; i++ {
        code := fmt.Sprintf("%06d", baseCode+i)
        market := "33"
        subject := market + ":" + code
        name := fmt.Sprintf("Sample Corp %s", code)

        // per-stock scale: choose revenue scale (log-uniform)
        scale := math.Pow(10, randFloat(7, 11))

        for _, y := range years {
            t := time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
            reportDate := t.Unix()
            endDate := fmt.Sprintf("%d-12-31", y)
            yearStr := fmt.Sprintf("%d", y)
            period := "596001"

            op := scale * (0.5 + rand.Float64()*1.5)
            profit := op * (0.03 + rand.Float64()*0.25)

            // apply optional gaussian noise to numeric fields
            if noise > 0 {
                op = op * (1 + rand.NormFloat64()*noise)
                profit = profit * (1 + rand.NormFloat64()*noise)
            }

            if _, err := stmt.Exec(code, market, subject, name, reportDate, endDate, yearStr, period, op, profit, "stock", "stock_a_listing_pool"); err != nil {
                fmt.Printf("insert failed: %v\n", err)
                tx.Rollback()
                return
            }
            inserted++
        }
    }

    if err := tx.Commit(); err != nil {
        fmt.Printf("commit failed: %v\n", err)
        return
    }

    fmt.Printf("db generated: %s (rows: %d, stocks: %d, years: %d, seed: %d)\n", dbPath, inserted, stocks, len(years), seed)
}

func randFloat(a, b float64) float64 {
    return a + rand.Float64()*(b-a)
}

func getDir(path string) string {
    // naive dir extract
    i := len(path) - 1
    for i >= 0 && path[i] != '/' && path[i] != '\\' {
        i--
    }
    if i <= 0 {
        return "."
    }
    return path[:i]
}
