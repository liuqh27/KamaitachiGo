package main

import (
	"KamaitachiGo/pkg/config"
	"KamaitachiGo/pkg/etcd"
	"KamaitachiGo/pkg/hash"
	"KamaitachiGo/pkg/json"
	"bytes"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	consistentHash *hash.ConsistentHash
	etcdClient     *etcd.Client
)

func main() {
	// 加载配置
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

	// 初始化一致性哈希
	consistentHash = hash.NewConsistentHash(150, nil)

	// 发现服务节点
	nodes, err := etcdClient.Discover("kamaitachi-slave")
	if err != nil {
		logrus.Errorf("Failed to discover services: %v", err)
	} else {
		consistentHash.Add(nodes...)
		logrus.Infof("Discovered %d slave nodes", len(nodes))
	}

	// 监听服务变化
	etcdClient.WatchPrefix("/services/kamaitachi-slave/", func(eventType, key, value string) {
		if eventType == "PUT" {
			consistentHash.Add(value)
			logrus.Infof("Added node: %s", value)
		} else if eventType == "DELETE" {
			consistentHash.Remove(value)
			logrus.Infof("Removed node: %s", value)
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

func proxyHandler(c *gin.Context) {
	// 从请求中提取路由key（使用subjects参数或其他标识）
	var requestBody map[string]interface{}
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	json.Unmarshal(bodyBytes, &requestBody)

	// 确定路由key
	routeKey := ""
	if subjects, ok := requestBody["subjects"].(string); ok && subjects != "" {
		// 使用第一个subject作为路由key
		parts := strings.Split(subjects, ",")
		if len(parts) > 0 {
			routeKey = strings.TrimSpace(parts[0])
		}
	}
	if routeKey == "" {
		routeKey = c.ClientIP()
	}

	// 通过一致性哈希选择节点
	targetNode := consistentHash.Get(routeKey)
	if targetNode == "" {
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
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(proxyReq)
	if err != nil {
		logrus.Errorf("Failed to proxy request: %v", err)
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

