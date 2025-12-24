# KamaitachiGo 技术文档

## 项目概述

KamaitachiGo 是一个用 Go 语言实现的分布式时序数据缓存系统，设计目标是提供高性能、低延迟的数据查询服务。

## 核心技术

### 1. LRU 缓存算法

**实现位置**：`internal/cache/lru/lru.go`

**核心特性**：
- 基于双向链表和哈希表
- O(1) 时间复杂度的读写操作
- 支持容量限制和自动淘汰
- 支持数据过期时间
- 线程安全（读写锁）

**数据结构**：
```go
type Cache struct {
    mu        sync.RWMutex           // 读写锁
    maxBytes  int64                  // 最大容量
    usedBytes int64                  // 已使用容量
    ll        *list.List             // 双向链表
    cache     map[string]*list.Element  // 哈希表
    OnEvicted func(key string, value Value)  // 淘汰回调
}
```

**工作原理**：
1. 新数据插入链表头部
2. 访问数据时移动到头部
3. 容量不足时从尾部淘汰

### 2. 快照管理

**实现位置**：`internal/cache/snapshot/manager.go`

**核心特性**：
- 定期自动保存
- 启动时自动恢复
- JSON 格式存储
- 原子性写入（临时文件+重命名）

**保存流程**：
```
获取所有缓存数据 -> 序列化为JSON -> 写入临时文件 -> 重命名为正式文件
```

**加载流程**：
```
读取快照文件 -> 反序列化JSON -> 检查过期 -> 恢复到缓存
```

### 3. 一致性哈希

**实现位置**：`pkg/hash/consistenthash.go`

**核心特性**：
- 虚拟节点支持
- 均衡负载分布
- 节点增删最小化数据迁移
- CRC32 哈希算法

**算法原理**：
```
1. 为每个真实节点创建多个虚拟节点（replicas）
2. 将虚拟节点映射到哈希环上
3. 查找时顺时针找到第一个虚拟节点
4. 返回对应的真实节点
```

**虚拟节点数量建议**：150（平衡性能和均衡性）

### 4. 服务发现

**实现位置**：`pkg/etcd/etcd.go`

**核心特性**：
- 基于 etcd 实现
- 服务注册与心跳
- 服务发现与监听
- 租约自动续期

**注册流程**：
```
创建租约 -> 注册服务信息 -> 启动心跳保活 -> 失败时自动重连
```

**发现流程**：
```
查询前缀键 -> 获取服务列表 -> 监听变化 -> 更新本地缓存
```

## 架构设计

### 分层架构

```
┌─────────────────────────────────┐
│         HTTP Handler            │  API层
├─────────────────────────────────┤
│          Service Layer          │  业务逻辑层
├─────────────────────────────────┤
│        Repository Layer         │  数据访问层
├─────────────────────────────────┤
│          Cache Layer            │  缓存层
└─────────────────────────────────┘
```

### 请求流程

#### 主节点写入流程

```
客户端请求 -> Handler验证 -> Service处理 -> Repository保存 -> Cache缓存
```

#### 从节点查询流程

```
客户端请求 -> Handler验证 -> Service查询 -> Repository获取 -> Cache返回
```

#### 网关路由流程

```
客户端请求 -> 解析路由key -> 一致性哈希选择节点 -> 代理请求 -> 返回结果
```

## 并发模型

### Goroutine 使用

1. **HTTP 服务器**：每个请求一个 goroutine（由 Gin 框架管理）
2. **自动快照**：独立 goroutine 定期执行
3. **etcd 监听**：独立 goroutine 持续监听
4. **心跳保活**：独立 goroutine 维护租约

### 锁策略

1. **LRU 缓存**：读写锁（sync.RWMutex）
   - 读操作：共享锁
   - 写操作：独占锁

2. **一致性哈希**：读写锁
   - 节点查询：共享锁
   - 节点变更：独占锁

3. **Repository**：读写锁
   - 查询操作：共享锁
   - 保存/删除：独占锁

## 数据模型

### DataInfo - 时序数据

```go
type DataInfo struct {
    ID         string                 // 实体ID
    Data       map[int64][]interface{} // 时间戳 -> 数据数组
    CreateTime string                 // 创建时间
    UpdateTime int64                  // 更新时间
}
```

**存储格式**：
```json
{
  "id": "000001",
  "data": {
    "1672502400": [100.5, 200.3, 300.1],
    "1672588800": [101.2, 201.5, 302.3]
  },
  "createTime": "2024-01-01 00:00:00",
  "updateTime": 1672502400
}
```

### SnapshotData - 快照数据

用于返回某个时间点的数据切片。

### PeriodData - 区间数据

用于返回一段时间范围内的数据序列。

## 性能优化

### 1. 内存优化

- 使用 `sync.Pool` 复用对象（预留扩展）
- 避免不必要的数据拷贝
- 使用指针传递大对象

### 2. 网络优化

- HTTP Keep-Alive
- 请求超时控制
- 连接池复用

### 3. 缓存优化

- LRU 算法保证热数据常驻
- 读写分离锁
- 批量操作支持（预留）

### 4. 序列化优化

- JSON 序列化（平衡性能和可读性）
- 可扩展为 Protobuf（更高性能）

## 扩展点

### 1. 数据持久化

可以扩展支持 MySQL、PostgreSQL 等数据库：

```go
// internal/repository/mysql_repository.go
type MySQLRepository struct {
    db *gorm.DB
}

func (r *MySQLRepository) Save(tableName string, data *model.DataInfo) error {
    // 实现 MySQL 存储逻辑
}
```

### 2. 缓存策略

可以实现多种缓存策略：

- LFU（最不经常使用）
- FIFO（先进先出）
- 自适应缓存

### 3. 压缩支持

对大数据进行压缩：

```go
import "compress/gzip"

func compressData(data []byte) []byte {
    // 实现压缩逻辑
}
```

### 4. 监控指标

集成 Prometheus：

```go
import "github.com/prometheus/client_golang/prometheus"

var cacheHitCounter = prometheus.NewCounter(...)
```

## 配置调优

### 缓存容量

根据实际数据量和内存大小调整：

```ini
# 1GB
max_bytes = 1073741824

# 2GB
max_bytes = 2147483648

# 4GB
max_bytes = 4294967296
```

### 快照间隔

根据数据重要性和写入频率调整：

```ini
# 高频写入，重要数据：5分钟
snapshot_interval = 5

# 低频写入，一般数据：30分钟
snapshot_interval = 30
```

### 虚拟节点数

一致性哈希虚拟节点数量：

```go
// 节点数量少（< 10）：建议 150
consistentHash = hash.NewConsistentHash(150, nil)

// 节点数量多（> 100）：建议 50
consistentHash = hash.NewConsistentHash(50, nil)
```

## 安全建议

### 1. 访问控制

建议添加 API 认证：

```go
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        // 验证 token
        c.Next()
    }
}
```

### 2. 数据加密

敏感数据加密存储：

```go
import "crypto/aes"

func encryptData(data []byte, key []byte) []byte {
    // 实现加密逻辑
}
```

### 3. 限流保护

防止恶意请求：

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(100, 200) // 100 QPS，突发 200
```

## 故障处理

### 节点故障

- etcd 心跳超时自动摘除
- 网关自动重路由
- 数据可从其他节点恢复

### 快照损坏

- 保留多个历史快照
- 实现快照校验机制
- 备份到远程存储

### 内存溢出

- 设置合理的缓存容量
- 实现内存告警
- 自动清理过期数据

## 测试建议

### 单元测试

```go
func TestCacheAdd(t *testing.T) {
    cache := lru.NewCache(1024, nil)
    cache.Add("key1", common.NewByteView([]byte("value1")))
    // 断言
}
```

### 压力测试

```bash
# 使用 Apache Bench
ab -n 10000 -c 100 http://localhost:8080/data/v1/snapshot

# 使用 wrk
wrk -t4 -c100 -d30s http://localhost:8080/health
```

### 集成测试

```go
func TestDistributedQuery(t *testing.T) {
    // 启动多个节点
    // 发送请求
    // 验证结果
}
```

## 部署建议

### Docker 部署

可以创建 Dockerfile：

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN go build -o kamaitachi cmd/master/main.go

FROM alpine:latest
COPY --from=builder /app/kamaitachi /usr/local/bin/
CMD ["kamaitachi"]
```

### Kubernetes 部署

可以创建 deployment.yaml：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kamaitachi-slave
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: slave
        image: kamaitachi:latest
```

## 总结

KamaitachiGo 是一个高性能、可扩展的分布式缓存系统，适合用于：

- 时序数据查询
- 金融数据缓存
- 实时数据分析
- 高并发读场景

通过合理的架构设计和性能优化，可以达到较高的 QPS 和较低的延迟。

