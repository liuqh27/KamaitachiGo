package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
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
	requests   = flag.Int("requests", 5000, "Total number of requests")
	concurrent = flag.Int("concurrent", 30, "Number of concurrent workers")
	nodeCount  = flag.Int("nodes", 3, "Number of nodes to use from default list (ignored if target provided)")
	scenario   = flag.Int("scenario", 1, "Test scenario (1-4)")
	target     = flag.String("target", "", "Override target nodes (comma-separated, e.g., 'http://localhost:9000/data')")

	v2Rate = flag.Float64("v2Rate", 0.0, "Fraction of requests to set use_v2_read=true (0-1)")
	repeat     = flag.Int("repeat", 0, "Number of unique requests to generate and repeat (0 for fully random)")

	errorCount = make(map[string]int64)
	errorMutex sync.Mutex
)

func recordError(errMsg string) {
	errorMutex.Lock()
	errorCount[errMsg]++
	errorMutex.Unlock()
}

var stockCodes = []string{
	"33:00000009", "33:00000010", "33:00000011", "33:00000012",
	"33:00000013", "33:00000014", "33:00000015", "33:00000016",
}

var indicators = []string{
	"operating_income", "parent_holder_net_profit", "total_operating_cost",
	"net_profit", "operating_profit", "total_profit",
}

func getRequestBody(scenarioID int) []byte {
	switch scenarioID {
	case 1:
		// 场景1：多实体、多指标取区间数据（股票PK）- 使用后端兼容字段 ids/subjects
		ids := []string{
			indicators[rand.Intn(len(indicators))],
			indicators[rand.Intn(len(indicators))],
			indicators[rand.Intn(len(indicators))],
		}
		subjects := []string{stockCodes[rand.Intn(len(stockCodes))], stockCodes[rand.Intn(len(stockCodes))]}
		return []byte(fmt.Sprintf(`{
			"ids": "%s",
			"subjects": "%s",
			"field": "report_date",
			"order": -1,
			"limit": 20,
			"offset": 0
		}`, strings.Join(ids, ","), strings.Join(subjects, ",")))

	case 2:
		// 场景2：单实体、多指标取最近几条数据 - 使用 ids/subjects
		ids := []string{indicators[rand.Intn(len(indicators))], indicators[rand.Intn(len(indicators))]}
		subject := stockCodes[rand.Intn(len(stockCodes))]
		return []byte(fmt.Sprintf(`{
			"ids": "%s",
			"subjects": "%s",
			"field": "operating_income",
			"order": -1,
			"limit": 8,
			"offset": 0
		}`, strings.Join(ids, ","), subject))

	case 3:
		// 场景3：多实体大量请求，构造 subjects 为逗号分隔字符串（后端期待）
		var subjects []string
		numSubjects := 100 + rand.Intn(200)
		for i := 0; i < numSubjects; i++ {
			subjects = append(subjects, stockCodes[rand.Intn(len(stockCodes))])
		}
		// 限制到最多50用于请求体
		if len(subjects) > 50 {
			subjects = subjects[:50]
		}
		ids := []string{indicators[rand.Intn(len(indicators))], indicators[rand.Intn(len(indicators))], indicators[rand.Intn(len(indicators))], indicators[rand.Intn(len(indicators))]}
		return []byte(fmt.Sprintf(`{
			"ids": "%s",
			"subjects": "%s",
			"field": "operating_income",
			"order": -1,
			"limit": 20,
			"offset": 0
		}`, strings.Join(ids, ","), strings.Join(subjects, ",")))

	case 4:
		// 场景4：多实体指定日期 - 使用 ids/subjects，并带时间戳字段可选
		ids := []string{indicators[rand.Intn(len(indicators))], indicators[rand.Intn(len(indicators))], indicators[rand.Intn(len(indicators))]}
		subs := []string{stockCodes[rand.Intn(len(stockCodes))], stockCodes[rand.Intn(len(stockCodes))], stockCodes[rand.Intn(len(stockCodes))]}
		return []byte(fmt.Sprintf(`{
			"ids": "%s",
			"subjects": "%s",
			"field": "operating_income",
			"order": -1,
			"limit": 10,
			"offset": 0,
			"timestamp": 1696032000
		}`, strings.Join(ids, ","), strings.Join(subs, ",")))
	}

	return []byte(`{}`)
}

func getScenarioName(id int) string {
	switch id {
	case 1:
		return "Scenario 1: Multi-Entity, Multi-Indicator, Snapshot Query (Target QPS: 300)"
	case 2:
		return "Scenario 2: Single-Entity, Multi-Indicator, Snapshot Query (Target QPS: 500)"
	case 3:
		return "Scenario 3: Multi-Entity, Snapshot Query with Pagination (Target QPS: 10000+)"
	case 4:
		return "Scenario 4: Multi-Entity, Snapshot Query with Timestamp (Target QPS: 300)"
	}
	return "Unknown"
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	// 计算 activeNodes：优先使用 -target，如果未提供则使用默认列表的前 N 个
	var activeNodes []string
	if strings.TrimSpace(*target) != "" {
		// 从命令行解析逗号分隔的节点地址
		parts := strings.Split(*target, ",")
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s != "" {
				activeNodes = append(activeNodes, s)
			}
		}
		if len(activeNodes) == 0 {
			fmt.Println("nodesAddr provided but no valid addresses parsed")
			return
		}
	} else {
		if *nodeCount > len(nodes) {
			*nodeCount = len(nodes)
		}
		activeNodes = nodes[:*nodeCount]
	}

	fmt.Println("===========================================")
	fmt.Println("Go Benchmark Tool - Multi-Scenario")
	fmt.Println("===========================================")
	fmt.Printf("Scenario: %s\n", getScenarioName(*scenario))
	fmt.Printf("Nodes: %d\n", *nodeCount)
	fmt.Printf("Requests: %d\n", *requests)
	fmt.Printf("Concurrent: %d\n", *concurrent)
	if *repeat > 0 {
		fmt.Printf("Repeat requests: %d unique requests will be generated and repeated.\n", *repeat)
	}
	fmt.Println()

	fmt.Println("Nodes:")
	for _, node := range activeNodes {
		if checkHealth(node) {
			fmt.Printf("  %s - OK\n", node)
		} else {
			fmt.Printf("  %s - FAILED\n", node)
			return
		}
	}

	// Generate request pool if repeat is enabled
	var requestPool [][]byte
	if *repeat > 0 {
		fmt.Printf("Generating %d unique requests for the pool...\n", *repeat)
		requestPool = make([][]byte, *repeat)
		for i := 0; i < *repeat; i++ {
			requestPool[i] = getRequestBody(*scenario)
		}
		fmt.Println("Request pool generated.")
	}

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *concurrent * *nodeCount,
			MaxIdleConnsPerHost: *concurrent,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
		Timeout: 30 * time.Second,
	}

	stats := &Stats{
		latencies: make([]int64, 0, *requests),
	}

	var completed int64
	progressInterval := int64(*requests / 15)
	if progressInterval == 0 {
		progressInterval = 1
	}

	fmt.Println("\nTesting...")
	startTime := time.Now()

	var wg sync.WaitGroup
	requestChan := make(chan int, *requests)

	go func() {
		for i := 0; i < *requests; i++ {
			requestChan <- i
		}
		close(requestChan)
	}()

	for i := 0; i < *concurrent; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for reqID := range requestChan {
				node := activeNodes[reqID%len(activeNodes)]
				url := node + "/kamaitachi/api/data/v1/snapshot"

				var requestBody []byte
				if *repeat > 0 {
					requestBody = requestPool[reqID%len(requestPool)]
				} else {
					requestBody = getRequestBody(*scenario)
				}
				
				reqStart := time.Now()
				success := executeRequest(client, url, requestBody)
				latency := time.Since(reqStart).Milliseconds()

				atomic.AddInt64(&stats.total, 1)
				if success {
					atomic.AddInt64(&stats.success, 1)
					stats.AddLatency(latency)
				} else {
					atomic.AddInt64(&stats.failed, 1)
				}

				current := atomic.AddInt64(&completed, 1)
				if current%progressInterval == 0 {
					fmt.Printf("Progress: %d/%d\n", current, *requests)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Println("\n===========================================")
	fmt.Println("Results")
	fmt.Println("===========================================")
	fmt.Printf("Scenario: %s\n", getScenarioName(*scenario))
	fmt.Printf("Total: %d\n", stats.total)
	fmt.Printf("Success: %d\n", stats.success)
	fmt.Printf("Failed: %d\n", stats.failed)
	fmt.Printf("Duration: %.2f sec\n\n", duration.Seconds())

	qps := float64(stats.success) / duration.Seconds()
	fmt.Printf("QPS: %.2f\n", qps)

	var targetQPS int
	switch *scenario {
	case 1, 4:
		targetQPS = 300
	case 2:
		targetQPS = 500
	case 3:
		targetQPS = 10000
	}

	if qps >= float64(targetQPS) {
		fmt.Printf("Target QPS: %d - PASSED (%.1fx)\n\n", targetQPS, qps/float64(targetQPS))
	} else {
		fmt.Printf("Target QPS: %d - NOT YET (%.1f%%)\n\n", targetQPS, qps/float64(targetQPS)*100)
	}

	if stats.success > 0 {
		fmt.Println("Latency (ms):")
		fmt.Printf("  Avg: %d\n", stats.GetAvg())
		fmt.Printf("  P50: %d\n", stats.GetPercentile(0.50))
		fmt.Printf("  P95: %d\n", stats.GetPercentile(0.95))
		fmt.Printf("  P99: %d\n", stats.GetPercentile(0.99))
	}

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

	// 尝试多个可能的 health 路径：
	// 1) 直接 node + "/health"（当 node 为 base URL 时）
	// 2) node 本身如果包含 path（如 /data），尝试使用 node + "/health"
	// 3) 基础主机地址 (scheme://host) + "/health"
	candidates := []string{}
	trimmed := strings.TrimRight(node, "/")
	candidates = append(candidates, trimmed+"/health")

	if u, err := url.Parse(node); err == nil {
		base := u.Scheme + "://" + u.Host
		if base != trimmed {
			candidates = append(candidates, base+"/health")
		}
	}

	for _, healthURL := range candidates {
		resp, err := client.Get(healthURL)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return true
		}
	}

	return false
}

func executeRequest(client *http.Client, url string, requestBody []byte) bool {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		recordError("read_body: " + err.Error())
		return false
	}

	if resp.StatusCode != 200 {
		// 安全地取最多前100字节作为错误摘要，避免短响应导致切片越界
		snippet := string(body)
		if len(body) > 100 {
			snippet = string(body[:100])
		}
		recordError(fmt.Sprintf("status_%d: %s", resp.StatusCode, snippet))
		return false
	}

	return true
}
