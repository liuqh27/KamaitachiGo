package main

import (
	"KamaitachiGo/pkg/config"
	"KamaitachiGo/pkg/etcd"
	"KamaitachiGo/pkg/hash"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	stdjson "encoding/json"
)

var (
	// consistentHash 用于实现请求到后端Slave节点的一致性哈希路由
	consistentHash *hash.ConsistentHash
	etcdClient     *etcd.Client
	httpClient     *http.Client
)

func main() {
	// 加载配置

	// 初始化共享的 HTTP 客户端与连接池配置，减少短连接与连接耗尽问题
	transport := &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   1000,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	cfg, err := config.LoadConfig("conf/gateway.ini")
	if err != nil {
		logrus.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.Infof("Starting Kamaitachi Gateway on port %s", cfg.Server.Port)

	// 连接etcd
	if cfg.Etcd.Endpoints == "" {
		logrus.Fatal("Etcd endpoints not configured")
	}

	etcdClient, err = etcd.NewClient([]string{cfg.Etcd.Endpoints})
	if err != nil {
		logrus.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer etcdClient.Close()

	// 初始化一致性哈希环。虚拟节点数量150，可调整。
	// consistentHash 用于将请求Key（如股票ID）映射到后端Slave节点。
	consistentHash = hash.NewConsistentHash(150, nil)

	// 发现服务节点并添加到一致性哈希环
	// 这里发现的是所有服务名为 "kamaitachi-slave" 的节点
	nodes, err := etcdClient.Discover("kamaitachi-slave")
	if err != nil {
		logrus.Errorf("Failed to discover services: %v", err)
	} else {
		consistentHash.Add(nodes...)
		logrus.Infof("Discovered %d slave nodes and added to consistent hash ring", len(nodes))
	}

	// 监听etcd中服务节点的变化
	// 当Slave节点上线/下线时，动态更新一致性哈希环
	etcdClient.WatchPrefix("/services/kamaitachi-slave/", func(eventType, key, value string) {
		if eventType == "PUT" {
			consistentHash.Add(value)
			logrus.Infof("Node added: %s, consistent hash ring updated", value)
		} else if eventType == "DELETE" {
			consistentHash.Remove(value)
			logrus.Infof("Node removed: %s, consistent hash ring updated", value)
		}
	})

	// 设置路由
	router := setupGatewayRouter()

	// 启动HTTP服务器
	go func() {
		addr := ":" + cfg.Server.Port
		logrus.Infof("Gateway listening on %s", addr)
		if err := router.Run(addr); err != nil {
			logrus.Fatalf("Failed to start gateway: %v", err)
		}
	}()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("Shutting down gateway...")
}

func setupGatewayRouter() *gin.Engine {
	r := gin.Default()

	// 代理所有请求到后端节点
	r.Any("/data/*path", proxyHandler)

	// 统一统计接口，聚合后端节点的 /kamaitachi/api/data/v1/stats
	r.GET("/kamaitachi/api/data/v1/stats", statsHandler)

	// 统一重置接口
	r.POST("/kamaitachi/api/data/v1/cache/reset", resetHandler)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		nodes := consistentHash.GetNodes()
		c.JSON(200, gin.H{
			"status": "ok",
			"nodes":  nodes,
			"count":  len(nodes),
		})
	})

	return r
}

// proxyHandler 处理所有转发到后端Slave节点的请求。
// 它负责从请求中提取路由Key，通过一致性哈希选择目标Slave，然后将请求转发过去。
func proxyHandler(c *gin.Context) {
	// 从请求中提取路由key。
	// 首先尝试从JSON请求体中解析 'subjects' 字段。
	var requestBody map[string]interface{}
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 重新设置请求体，以便后续处理器读取

	stdjson.Unmarshal(bodyBytes, &requestBody)

	// 确定路由key：
	// 优先级1: 请求体中 'subjects' 字段的第一个元素（例如股票ID）
	// 优先级2: 客户端IP
	routeKey := ""
	if subjects, ok := requestBody["subjects"].(string); ok && subjects != "" {
		// 使用第一个subject作为路由key，保证相同subject的请求落在同一个节点
		parts := strings.Split(subjects, ",")
		if len(parts) > 0 {
			routeKey = strings.TrimSpace(parts[0])
		}
	}
	if routeKey == "" {
		// 如果subjects为空，则回退使用客户端IP作为路由key
		// 注意：这在单机压测时可能导致所有请求路由到同一节点
		routeKey = c.ClientIP()
	}

	// 通过一致性哈希选择目标Slave节点
	targetNode := consistentHash.Get(routeKey)
	if targetNode == "" {
		logrus.Errorf("No available slave nodes found for routeKey: %s", routeKey)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "no available nodes",
		})
		return
	}

	// 构建目标URL
	// 如果请求路径以 /data/ 开头，去掉该前缀再转发给后端（后端期望 /kamaitachi/...）
	forwardPath := c.Request.URL.Path
	if strings.HasPrefix(forwardPath, "/data/") {
		forwardPath = strings.TrimPrefix(forwardPath, "/data")
		if forwardPath == "" {
			forwardPath = "/"
		}
	} else if forwardPath == "/data" {
		forwardPath = "/"
	}

	targetURL := "http://" + targetNode + forwardPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	logrus.Infof("Routing request to: %s (key: %s)", targetURL, routeKey)

	// 创建代理请求
	proxyReq, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create proxy request",
		})
		return
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 发送请求
	// 使用全局的httpClient，带有连接池优化
	resp, err := httpClient.Do(proxyReq)
	if err != nil {
		logrus.Errorf("Failed to proxy request to %s: %v", targetURL, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "failed to proxy request: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 复制响应体
	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// statsHandler 从所有后端节点拉取统计并做简单聚合
func statsHandler(c *gin.Context) {
	nodes := consistentHash.GetNodes()
	
	var aggHits int64
	var aggMisses int64
	var aggEntries int64
	var perNode []interface{}

	for _, node := range nodes {
		url := "http://" + node + "/kamaitachi/api/data/v1/stats"
		resp, err := httpClient.Get(url)
		if err != nil {
			logrus.Errorf("Failed to get stats from node %s: %v", node, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logrus.Errorf("Failed to read stats body from node %s: %v", node, err)
			continue
		}

		var parsed map[string]interface{}
		if err := stdjson.Unmarshal(body, &parsed); err != nil {
			logrus.Errorf("Failed to unmarshal stats from node %s: %v", node, err)
			continue
		}

		// Log the raw body and parsed data for debugging
		logrus.Debugf("statsHandler: received from node %s: body=%s", node, string(body))

		// 尝试提取 data.cache
		if parsedData, ok := parsed["data"].(map[string]interface{}); ok {
			perNode = append(perNode, parsedData)
			if cache, ok := parsedData["cache"].(map[string]interface{}); ok {
				if e, ok := cache["entries"]; ok {
					switch v := e.(type) {
					case float64:
						aggEntries += int64(v)
					case int64:
						aggEntries += v
					}
				}
				if h, ok := cache["hits"]; ok {
					logrus.Debugf("statsHandler: parsing 'hits' field from node %s, type is %T", node, h) // Log the type of 'hits'
					switch v := h.(type) {
					case float64:
						aggHits += int64(v)
					case int64:
						aggHits += v
					}
				}
				if m, ok := cache["misses"]; ok {
					logrus.Debugf("statsHandler: parsing 'misses' field from node %s, type is %T", node, m) // Log the type of 'misses'
					switch v := m.(type) {
					case float64:
						aggMisses += int64(v)
					case int64:
						aggMisses += v
					}
				}
			}
		}
	}

	total := aggHits + aggMisses
	hitRate := "0.00%"
	if total > 0 {
		hitRate = fmt.Sprintf("%.2f%%", float64(aggHits)/float64(total)*100)
	}

	aggregate := map[string]interface{}{
		"entries":  aggEntries,
		"hits":     aggHits,
		"misses":   aggMisses,
		"hit_rate": hitRate, // Return float64 directly
	}

	c.JSON(http.StatusOK, gin.H{
		"status_code": 0,
		"status_msg":  "success",
		"data": map[string]interface{}{
			"nodes":     nodes,
			"per_node":  perNode,
			"aggregate": aggregate,
		},
	})
}

// resetHandler 向所有后端节点转发重置统计的请求
func resetHandler(c *gin.Context) {
	const maxRetries = 5
	const retryDelay = 500 * time.Millisecond

	var nodes []string
	// Retry getting nodes for a short period, as etcd discovery might be eventual consistent
	for i := 0; i < maxRetries; i++ {
		nodes = consistentHash.GetNodes()
		if len(nodes) > 0 {
			break
		}
		logrus.Warnf("resetHandler: No slave nodes found yet, retrying in %v...", retryDelay)
		time.Sleep(retryDelay)
	}

	if len(nodes) == 0 {
		logrus.Errorf("resetHandler: No active slave nodes found after %d retries.", maxRetries)
		c.JSON(http.StatusOK, gin.H{
			"status_code": 500,
			"status_msg":  "No active slave nodes found to reset.",
		})
		return
	}

	resetCount := 0
	for _, node := range nodes {
		url := "http://" + node + "/kamaitachi/api/data/v1/cache/reset"
		// 使用POST请求
		resp, err := httpClient.Post(url, "application/json", nil)
		if err != nil {
			logrus.Errorf("Failed to send reset request to node %s: %v", node, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			resetCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status_code": 0,
		"status_msg":  fmt.Sprintf("Sent reset command to %d of %d nodes.", resetCount, len(nodes)),
	})
}


