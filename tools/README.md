"""markdown
# tools 目录说明

本目录包含项目运行、测试与数据生成相关的工具源码（Go）。

## 工具
- `benchmark.go`：压测库（工具依赖）。
- `benchmark_scenarios.go`：压测场景代码（请用 `go build` 编译，而不要提交二进制）。
- `generate_sample_db.go`：生成轻量示例 SQLite 数据库（`data/sample.db`）。
- `check_sample_db.go`：检查示例 DB 的脚本，用于快速验证样例数据。
- `compare_db_values.go`：对比原始与处理后数据库中的数值列（用于验证脱敏/扰动效果）。
- `desensitize_db_lognoise.go`：对数空间加噪脚本（仅保留源码，真实数据请在私有环境处理）。
- `check_db.go`, `debug_sql.go`, `show_sanitized.go`：调试与检查辅助脚本（保留源码作为参考）。
- `test_duckdb.go`, `test_sqlite.go`：实验性测试程序（保留为实验示例）。



## 如何构建（示例）

构建压测工具：

```powershell
cd tools
go build -o ../bin/benchmark_scenarios benchmark_scenarios.go benchmark.go
```

生成示例数据库：

```powershell
go run generate_sample_db.go
# 结果: data/sample.db
```

运行对比工具示例：

```powershell
go run compare_db_values.go -orig "./data/finance_test.db" -san "./data_sanitized_log/finance_test.db" -limit 10
```
