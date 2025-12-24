# Kamaitachi Finance API 使用说明

## 快速开始

### 1. 导入数据

首先需要导入SQL数据到SQLite数据库：

```powershell
# 导入数据（需要几分钟）
go run cmd/import/import_sql.go
```

导入完成后会在 `data/finance_test.db` 生成数据库文件。

### 2. 启动服务器

```powershell
# 使用默认配置启动（端口8080）
.\start_server.ps1

# 指定端口启动
.\start_server.ps1 -Port 9000

# 开启调试模式
.\start_server.ps1 -Debug

# 指定数据库文件
.\start_server.ps1 -DbPath "D:\data\finance.db"
```

### 3. 测试API

启动服务器后，在另一个PowerShell窗口运行：

```powershell
.\test_api.ps1
```

## API 接口

### 1. 快照查询接口

**接口**: `POST /kamaitachi/api/data/v1/snapshot`

**功能**: 查询指定证券或全市场的最新快照数据

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ids | string | 是 | 指标ID列表，逗号分隔，如 "operating_income,parent_holder_net_profit" |
| subjects | string | 条件 | 证券列表，逗号分隔，如 "33:000001,33:000002"（与topic二选一） |
| topic | string | 条件 | 主题池，如 "stock_a_listing_pool"（与subjects二选一） |
| method | string | 否 | 方法，如 "market:code" |
| field | string | 是 | 排序字段，如 "operating_income" |
| order | int | 是 | 排序方式：-1=降序，1=升序 |
| offset | int | 否 | 分页偏移，默认0 |
| limit | int | 否 | 返回数量，默认10 |
| timestamp | int64 | 否 | 时间戳，0表示最新 |

#### 请求示例1：指定证券查询

```json
{
    "ids": "operating_income,parent_holder_net_profit",
    "subjects": "33:000001,33:000002,33:000003",
    "field": "operating_income",
    "order": -1,
    "offset": 0,
    "limit": 5,
    "timestamp": 0
}
```

#### 请求示例2：全市场查询

```json
{
    "ids": "operating_income,parent_holder_net_profit",
    "topic": "stock_a_listing_pool",
    "field": "operating_income",
    "order": -1,
    "offset": 0,
    "limit": 100,
    "timestamp": 0
}
```

#### 响应示例

```json
{
    "status_code": 0,
    "status_msg": "success",
    "data": [
        {
            "subject": {
                "subject": "33:000001",
                "name": "平安银行",
                "status": "213001",
                "listing_date": "",
                "category": "stock"
            },
            "data": {
                "operating_income": 125678900000.50,
                "parent_holder_net_profit": 35678900000.25
            }
        }
    ]
}
```

### 2. 区间查询接口

**接口**: `POST /kamaitachi/api/data/v1/period`

**功能**: 查询指定证券在时间区间内的数据

#### 请求参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ids | string | 是 | 指标ID列表 |
| subjects | string | 是 | 证券列表 |
| method | string | 否 | 方法 |
| from | int64 | 是 | 开始时间戳（秒） |
| to | int64 | 是 | 结束时间戳（秒） |

#### 请求示例

```json
{
    "ids": "operating_income,parent_holder_net_profit",
    "subjects": "33:000001,33:000002",
    "from": 1609459200,
    "to": 1640995200
}
```

#### 响应示例

```json
{
    "status_code": 0,
    "status_msg": "success",
    "data": [
        {
            "subject": {
                "subject": "33:000001",
                "name": "平安银行",
                "status": "213001",
                "listing_date": "",
                "category": "stock"
            },
            "data": [
                {
                    "end_date": "2020-12-31",
                    "period": "Q4",
                    "declare_date": "",
                    "year": "2020",
                    "parent_holder_net_profit": 35678900000.25,
                    "operating_income": 125678900000.50,
                    "combine": "33:000001:2020_Q4"
                }
            ]
        }
    ]
}
```

### 3. 统计信息接口

**接口**: `GET /kamaitachi/api/data/v1/stats`

**功能**: 获取系统统计信息

#### 响应示例

```json
{
    "status_code": 0,
    "status_msg": "success",
    "data": {
        "total_records": 50000,
        "stock_count": 5000,
        "cache": {
            "entries": 234,
            "max": 10000
        }
    }
}
```

### 4. 健康检查接口

**接口**: `GET /health`

**功能**: 检查服务器是否正常运行

#### 响应示例

```json
{
    "status": "ok"
}
```

### 5. 监控接口

#### 熔断器状态

**接口**: `GET /monitor/circuitbreaker`

#### 系统状态

**接口**: `GET /monitor/status`

## 性能特性

### 1. 限流保护

- 令牌桶算法限流
- 默认：1000 QPS
- 超出限制返回 429 状态码

### 2. 熔断保护

- 三状态熔断器（CLOSED/OPEN/HALF_OPEN）
- 错误率阈值：50%
- 熔断时长：10秒
- 半开状态成功次数：5次

### 3. 缓存机制

- 内存缓存（Map + RWMutex）
- 缓存大小：10000条
- MD5 key生成
- 自动缓存命中率优化

### 4. 数据库优化

- SQLite WAL模式
- 连接池（最大10连接）
- 索引优化
- 批量查询

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 400 | 请求参数错误 |
| 429 | 请求过于频繁（限流） |
| 500 | 服务器内部错误 |
| 503 | 服务熔断（暂时不可用） |

## PowerShell 测试示例

```powershell
# 快照查询
$body = @{
    ids = "operating_income,parent_holder_net_profit"
    subjects = "33:000001"
    field = "operating_income"
    order = -1
    limit = 10
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/kamaitachi/api/data/v1/snapshot" `
    -Method POST `
    -Body $body `
    -ContentType "application/json"

# 区间查询
$body = @{
    ids = "operating_income,parent_holder_net_profit"
    subjects = "33:000001"
    from = 1609459200
    to = 1640995200
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/kamaitachi/api/data/v1/period" `
    -Method POST `
    -Body $body `
    -ContentType "application/json"
```

## 命令行参数

服务器支持以下命令行参数：

```
-db string
    SQLite数据库文件路径（默认：./data/finance_test.db）
    
-port int
    HTTP服务器端口（默认：8080）
    
-cache int
    LRU缓存大小（字节）（默认：2GB）
    
-debug
    启用调试日志
```

示例：

```powershell
.\cmd\server\server.exe -db ./data/finance.db -port 9000 -debug
```

## 注意事项

1. 首次使用前必须导入数据
2. 数据库文件大约几百MB
3. 建议使用SSD存储数据库文件
4. 生产环境建议关闭debug模式
5. 根据实际需求调整限流和熔断参数

