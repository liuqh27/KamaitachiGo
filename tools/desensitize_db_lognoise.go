package main

import (
    "database/sql"
    "flag"
    "fmt"
    "io"
    "log"
    "math"
    "math/rand"
    "os"
    "path/filepath"
    "strings"
    "time"

    _ "modernc.org/sqlite"
)

func copyFile(src, dst string) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()
    out, err := os.Create(dst)
    if err != nil {
        return err
    }
    defer func() { out.Sync(); out.Close() }()
    _, err = io.Copy(out, in)
    return err
}

func scanFloat(v interface{}) (float64, bool) {
    if v == nil {
        return 0, false
    }
    switch t := v.(type) {
    case int64:
        return float64(t), true
    case float64:
        return t, true
    case []byte:
        var x float64
        if _, err := fmt.Sscan(string(t), &x); err == nil {
            return x, true
        }
    }
    return 0, false
}

func isCandidateColumn(name, ctype string) bool {
    lname := strings.ToLower(name)
    if strings.Contains(lname, "income") || strings.Contains(lname, "profit") || strings.Contains(lname, "price") || strings.Contains(lname, "amount") || strings.Contains(lname, "net") || strings.Contains(lname, "eps") {
        return true
    }
    ltype := strings.ToLower(ctype)
    if strings.Contains(ltype, "real") || strings.Contains(ltype, "double") || strings.Contains(ltype, "numeric") || strings.Contains(ltype, "float") {
        return true
    }
    return false
}

// log-space perturbation: y = sign(x) * (exp( log(|x|+shift) + N(0,sigma) ) - shift)
func transformLogNoise(x, sigma, shift float64) float64 {
    sign := 1.0
    if x < 0 {
        sign = -1.0
        x = -x
    }
    xadj := x + shift
    if xadj <= 0 {
        xadj = shift
    }
    y := math.Exp(math.Log(xadj) + rand.NormFloat64()*sigma)
    return sign * (y - shift)
}

func main() {
    dataDir := flag.String("data", "./data", "source data dir")
    outDir := flag.String("out", "./data_sanitized_log", "output sanitized dir")
    sigma := flag.Float64("sigma", 0.05, "stddev of gaussian noise in log-space (e.g. 0.05 ~ 5% multiplicative)")
    shift := flag.Float64("shift", 1e-6, "small shift to avoid log(0)")
    dry := flag.Bool("dry", false, "dry run")
    flag.Parse()

    rand.Seed(time.Now().UnixNano())

    files, err := filepath.Glob(filepath.Join(*dataDir, "*.db"))
    if err != nil {
        log.Fatal(err)
    }
    if len(files) == 0 {
        log.Fatalf("no .db files found in %s", *dataDir)
    }
    if !*dry {
        if err := os.MkdirAll(*outDir, 0755); err != nil {
            log.Fatal(err)
        }
    }

    for _, f := range files {
        base := filepath.Base(f)
        outPath := filepath.Join(*outDir, base)
        fmt.Printf("Processing %s -> %s\n", f, outPath)
        if *dry {
            fmt.Println(" dry run: copy skipped")
        } else {
            if err := copyFile(f, outPath); err != nil {
                fmt.Printf(" copy failed: %v\n", err)
                continue
            }
        }

        dbPath := outPath
        db, err := sql.Open("sqlite", dbPath)
        if err != nil {
            fmt.Printf(" open sanitized db failed: %v\n", err)
            continue
        }

        trows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
        if err != nil {
            fmt.Printf(" list tables failed: %v\n", err)
            db.Close()
            continue
        }
        var tname string
        for trows.Next() {
            trows.Scan(&tname)
            fmt.Printf("  table: %s\n", tname)
            crow, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tname))
            if err != nil {
                fmt.Printf("   pragma failed: %v\n", err)
                continue
            }
            var cid int
            var cname, ctype string
            var notnull, pk int
            var dflt sql.NullString
            candidates := []string{}
            for crow.Next() {
                crow.Scan(&cid, &cname, &ctype, &notnull, &dflt, &pk)
                if isCandidateColumn(cname, ctype) {
                    candidates = append(candidates, cname)
                }
            }
            crow.Close()
            if len(candidates) == 0 {
                fmt.Println("   no candidate numeric columns")
                continue
            }
            fmt.Printf("   candidate columns: %v\n", candidates)

            selCols := strings.Join(candidates, ", ")
            query := fmt.Sprintf("SELECT rowid, %s FROM %s", selCols, tname)
            rows, err := db.Query(query)
            if err != nil {
                fmt.Printf("   select failed: %v\n", err)
                continue
            }
            cols, _ := rows.Columns()
            vals := make([]interface{}, len(cols))
            valPtrs := make([]interface{}, len(cols))
            for i := range vals { valPtrs[i] = &vals[i] }
            tx, _ := db.Begin()
            updates := 0
            for rows.Next() {
                if err := rows.Scan(valPtrs...); err != nil {
                    fmt.Printf("   scan row failed: %v\n", err)
                    break
                }
                rowid := vals[0]
                for i := 1; i < len(cols); i++ {
                    raw := vals[i]
                    if raw == nil { continue }
                    var fv float64
                    var ok bool
                    switch v := raw.(type) {
                    case int64:
                        fv = float64(v); ok = true
                    case float64:
                        fv = v; ok = true
                    case []byte:
                        var parsed float64
                        if _, err := fmt.Sscan(string(v), &parsed); err == nil { fv = parsed; ok = true }
                    default:
                        ok = false
                    }
                    if !ok { continue }
                    newv := transformLogNoise(fv, *sigma, *shift)
                    if *dry { updates++; continue }
                    colName := cols[i]
                    updStmt := fmt.Sprintf("UPDATE %s SET %s = ? WHERE rowid = ?", tname, colName)
                    if _, err := tx.Exec(updStmt, newv, rowid); err != nil {
                        fmt.Printf("   update failed: %v\n", err)
                    } else { updates++ }
                }
            }
            rows.Close()
            if !*dry {
                if err := tx.Commit(); err != nil { fmt.Printf("   commit failed: %v\n", err) }
            }
            fmt.Printf("   updated %d values in %s\n", updates, tname)
        }
        trows.Close()
        db.Close()
    }
    fmt.Println("done")
}
