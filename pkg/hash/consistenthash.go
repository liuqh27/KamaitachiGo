package hash

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// Hash 哈希函数类型
type Hash func(data []byte) uint32

// ConsistentHash 一致性哈希结构
type ConsistentHash struct {
	hash     Hash
	replicas int               // 虚拟节点倍数
	keys     []int             // 哈希环
	hashMap  map[int]string    // 虚拟节点到真实节点的映射
	mu       sync.RWMutex
}

// NewConsistentHash 创建一致性哈希
func NewConsistentHash(replicas int, fn Hash) *ConsistentHash {
	ch := &ConsistentHash{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if ch.hash == nil {
		ch.hash = crc32.ChecksumIEEE
	}
	return ch
}

// Add 添加节点
func (ch *ConsistentHash) Add(keys ...string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	for _, key := range keys {
		// 为每个真实节点创建多个虚拟节点
		for i := 0; i < ch.replicas; i++ {
			hash := int(ch.hash([]byte(strconv.Itoa(i) + key)))
			ch.keys = append(ch.keys, hash)
			ch.hashMap[hash] = key
		}
	}
	sort.Ints(ch.keys)
}

// Remove 移除节点
func (ch *ConsistentHash) Remove(key string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	
	for i := 0; i < ch.replicas; i++ {
		hash := int(ch.hash([]byte(strconv.Itoa(i) + key)))
		idx := sort.SearchInts(ch.keys, hash)
		if idx < len(ch.keys) && ch.keys[idx] == hash {
			ch.keys = append(ch.keys[:idx], ch.keys[idx+1:]...)
		}
		delete(ch.hashMap, hash)
	}
}

// Get 获取key对应的节点
func (ch *ConsistentHash) Get(key string) string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	if len(ch.keys) == 0 {
		return ""
	}
	
	hash := int(ch.hash([]byte(key)))
	// 二分查找第一个大于等于hash的虚拟节点
	idx := sort.Search(len(ch.keys), func(i int) bool {
		return ch.keys[i] >= hash
	})
	
	// 如果超出范围，使用第一个节点（形成环）
	return ch.hashMap[ch.keys[idx%len(ch.keys)]]
}

// GetNodes 获取所有节点
func (ch *ConsistentHash) GetNodes() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	
	nodes := make(map[string]bool)
	for _, node := range ch.hashMap {
		nodes[node] = true
	}
	
	result := make([]string, 0, len(nodes))
	for node := range nodes {
		result = append(result, node)
	}
	return result
}

