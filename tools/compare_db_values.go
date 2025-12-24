package main

import (
    "database/sql"
    "flag"
    "fmt"
    "log"
    "math"
    "strings"

    _ "modernc.org/sqlite"
)

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

func collectCandidates(db *sql.DB, table string) ([]string, error) {
    rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var cid int
    var name, ctype string
    var notnull, pk int
    var dflt sql.NullString
    cand := []string{}
    for rows.Next() {
        if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
            return nil, err
        }
        if isCandidateColumn(name, ctype) {
            cand = append(cand, name)
        }
    }
    return cand, nil
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

func main() {
    orig := flag.String("orig", "./data/finance_test.db", "original db path")
    san := flag.String("san", "./data_sanitized/finance_test.db", "sanitized db path")
    table := flag.String("table", "finance_data", "table to compare")
    limit := flag.Int("limit", 10, "sample rows to show")
    flag.Parse()

    dorig, err := sql.Open("sqlite", *orig)
    if err != nil {
        log.Fatal(err)
    }
    defer dorig.Close()
    dsan, err := sql.Open("sqlite", *san)
    if err != nil {
        log.Fatal(err)
    }
    defer dsan.Close()

    cand, err := collectCandidates(dorig, *table)
    if err != nil {
        log.Fatalf("collectCandidates failed: %v", err)
    }
    if len(cand) == 0 {
        log.Fatalf("no candidate numeric columns found in %s", *table)
    }
    fmt.Printf("candidate columns: %v\n", cand)

    cols := strings.Join(cand, ", ")
    q := fmt.Sprintf("SELECT rowid, %s FROM %s LIMIT %d", cols, *table, *limit)
    ro, err := dorig.Query(q)
    if err != nil {
        log.Fatalf("orig query failed: %v", err)
    }
    defer ro.Close()

    type rec struct{ rowid int64; vals []float64; valid []bool }
    origRecs := map[int64]rec{}
    for ro.Next() {
        var rowid int64
        vals := make([]interface{}, len(cand))
        ptrs := make([]interface{}, len(cand))
        for i := range vals { ptrs[i] = &vals[i] }
        if err := ro.Scan(append([]interface{}{&rowid}, ptrs...)...); err != nil {
            log.Fatal(err)
        }
        numeric := make([]float64, len(cand))
        valid := make([]bool, len(cand))
        for i := range vals {
            v, ok := scanFloat(vals[i])
            numeric[i] = v
            valid[i] = ok
        }
        origRecs[rowid] = rec{rowid: rowid, vals: numeric, valid: valid}
    }

    fmt.Println("\nRowID | column | orig -> san | abs_diff | rel_diff%")
    for id, r := range origRecs {
        q2 := fmt.Sprintf("SELECT %s FROM %s WHERE rowid = %d", cols, *table, id)
        rs, err := dsan.Query(q2)
        if err != nil {
            fmt.Printf("san query failed for row %d: %v\n", id, err)
            continue
        }
        if rs.Next() {
            vals := make([]interface{}, len(cand))
            ptrs := make([]interface{}, len(cand))
            for i := range vals { ptrs[i] = &vals[i] }
            if err := rs.Scan(ptrs...); err != nil {
                fmt.Printf("scan san row %d failed: %v\n", id, err)
                rs.Close()
                continue
            }
            for i := range vals {
                sanv, ok := scanFloat(vals[i])
                if !r.valid[i] && !ok { continue }
                origv := r.vals[i]
                absd := math.Abs(sanv-origv)
                reld := 0.0
                if origv != 0 { reld = absd / math.Abs(origv) * 100.0 }
                fmt.Printf("%d | %s | %.6g -> %.6g | %.6g | %.4f%%\n", id, cand[i], origv, sanv, absd, reld)
            }
        }
        rs.Close()
    }
}
