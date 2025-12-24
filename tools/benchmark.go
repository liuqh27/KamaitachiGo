package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	total     int64
	success   int64
	failed    int64
	latencies []int64
	mu        sync.Mutex
}

func (s *Stats) AddLatency(latency int64) {
	s.mu.Lock()
	s.latencies = append(s.latencies, latency)
	s.mu.Unlock()
}

func (s *Stats) GetPercentile(p float64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.latencies) == 0 {
		return 0
	}

	sort.Slice(s.latencies, func(i, j int) bool {
		return s.latencies[i] < s.latencies[j]
	})

	index := int(float64(len(s.latencies)) * p)
	if index >= len(s.latencies) {
		index = len(s.latencies) - 1
	}
	return s.latencies[index]
}

func (s *Stats) GetAvg() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.latencies) == 0 {
		return 0
	}

	var sum int64
	for _, lat := range s.latencies {
		sum += lat
	}
	return sum / int64(len(s.latencies))
}

var (
	nodes      = []string{"http://localhost:8080", "http://localhost:8081", "http://localhost:8082"}
	requests   = flag.Int("requests", 10000, "Total number of requests")
	concurrent = flag.Int("concurrent", 100, "Number of concurrent workers")
	nodeCount  = flag.Int("nodes", 3, "Number of nodes (1-3)")
)

var requestBody = []byte(`{
	"subject_keys": ["33:00000009"],
	"indicators": ["operating_income", "parent_holder_net_profit"],
	"start_date": "2020-01-01",
	"end_date": "2023-12-31",
	"order_by": "report_date",
	"order": "desc",
	"limit": 10,
	"offset": 0
}`)

func main() {
	flag.Parse()

	// 限制节点数量
	if *nodeCount > len(nodes) {
		*nodeCount = len(nodes)
	}
	activeNodes := nodes[:*nodeCount]

	fmt.Println("===========================================")
	fmt.Println("Go Benchmark Tool")
	fmt.Println("===========================================")
	fmt.Printf("Nodes: %d\n", *nodeCount)
	fmt.Printf("Requests: %d\n", *requests)
	fmt.Printf("Concurrent: %d\n\n", *concurrent)

	// 检查节点健康状态
	fmt.Println("Nodes:")
	for _, node := range activeNodes {
		if checkHealth(node) {
			fmt.Printf("  %s - OK\n", node)
		} else {
			fmt.Printf("  %s - FAILED\n", node)
			return
		}
	}

	// 创建HTTP客户端（连接复用）
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *concurrent * *nodeCount,
			MaxIdleConnsPerHost: *concurrent,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
		Timeout: 30 * time.Second,
	}

	// 统计
	stats := &Stats{
		latencies: make([]int64, 0, *requests),
	}

	// 进度显示
	var completed int64
	progressInterval := int64(*requests / 15)
	if progressInterval == 0 {
		progressInterval = 1
	}

	fmt.Println("\nTesting...")
	startTime := time.Now()

	// 并发测试
	var wg sync.WaitGroup
	requestChan := make(chan int, *requests)

	// 生产者：生成请求
	go func() {
		for i := 0; i < *requests; i++ {
			requestChan <- i
		}
		close(requestChan)
	}()

	// 消费者：执行请求
	for i := 0; i < *concurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for reqID := range requestChan {
				// 轮询选择节点
				node := activeNodes[reqID%len(activeNodes)]
				url := node + "/kamaitachi/api/data/v1/snapshot"

				// 执行请求
				reqStart := time.Now()
				success := executeRequest(client, url)
				latency := time.Since(reqStart).Milliseconds()

				// 统计
				atomic.AddInt64(&stats.total, 1)
				if success {
					atomic.AddInt64(&stats.success, 1)
					stats.AddLatency(latency)
				} else {
					atomic.AddInt64(&stats.failed, 1)
				}

				// 进度显示
				current := atomic.AddInt64(&completed, 1)
				if current%progressInterval == 0 {
					fmt.Printf("Progress: %d/%d\n", current, *requests)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// 输出结果
	fmt.Println("\n===========================================")
	fmt.Println("Results")
	fmt.Println("===========================================")
	fmt.Printf("Total: %d\n", stats.total)
	fmt.Printf("Success: %d\n", stats.success)
	fmt.Printf("Failed: %d\n", stats.failed)
	fmt.Printf("Duration: %.2f sec\n\n", duration.Seconds())

	qps := float64(stats.success) / duration.Seconds()
	fmt.Printf("QPS: %.2f\n\n", qps)

	if stats.success > 0 {
		fmt.Println("Latency (ms):")
		fmt.Printf("  Avg: %d\n", stats.GetAvg())
		fmt.Printf("  P50: %d\n", stats.GetPercentile(0.50))
		fmt.Printf("  P95: %d\n", stats.GetPercentile(0.95))
		fmt.Printf("  P99: %d\n", stats.GetPercentile(0.99))
	}

	// 显示错误统计
	if stats.failed > 0 {
		fmt.Println("\nError Summary:")
		errorMutex.Lock()
		for errMsg, count := range errorCount {
			fmt.Printf("  [%d] %s\n", count, errMsg)
		}
		errorMutex.Unlock()
	}

	fmt.Println("\n===========================================")
}

func checkHealth(node string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(node + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

var (
	errorCount = make(map[string]int64)
	errorMutex sync.Mutex
)

func recordError(errMsg string) {
	errorMutex.Lock()
	errorCount[errMsg]++
	errorMutex.Unlock()
}

func executeRequest(client *http.Client, url string) bool {
	req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
	if err != nil {
		recordError("create_request: " + err.Error())
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		recordError("do_request: " + err.Error())
		return false
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		recordError("read_body: " + err.Error())
		return false
	}

	if resp.StatusCode != 200 {
		recordError(fmt.Sprintf("status_%d: %s", resp.StatusCode, string(body)))
		return false
	}

	return true
}
