package lru

import (
	"container/list"
	"sync"
	"time"
)

// Cache LRU缓存结构
type Cache struct {
	mu        sync.RWMutex
	maxBytes  int64
	usedBytes int64
	ll        *list.List
	cache     map[string]*list.Element
	OnEvicted func(key string, value Value)
}

// Entry 缓存条目
type Entry struct {
	Key        string
	Value      Value
	CreateAt   int64
	ExpireTime int64 // 过期时间（秒）
}

// Value 缓存值接口
type Value interface {
	Len() int
}

// NewCache 创建LRU缓存
func NewCache(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

// Get 获取缓存值
func (c *Cache) Get(key string) (Value, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if ele, ok := c.cache[key]; ok {
		entry := ele.Value.(*Entry)
		// 检查是否过期
		if entry.ExpireTime > 0 && time.Now().Unix() > entry.CreateAt+entry.ExpireTime {
			return nil, false
		}
		c.ll.MoveToFront(ele)
		return entry.Value, true
	}
	return nil, false
}

// Add 添加缓存值
func (c *Cache) Add(key string, value Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if ele, ok := c.cache[key]; ok {
		// 更新现有条目
		c.ll.MoveToFront(ele)
		entry := ele.Value.(*Entry)
		oldSize := int64(len(entry.Key)) + int64(entry.Value.Len())
		entry.Value = value
		entry.CreateAt = time.Now().Unix()
		newSize := int64(len(key)) + int64(value.Len())
		c.usedBytes += newSize - oldSize
	} else {
		// 添加新条目
		entry := &Entry{
			Key:        key,
			Value:      value,
			CreateAt:   time.Now().Unix(),
			ExpireTime: 3600, // 默认1小时过期
		}
		ele := c.ll.PushFront(entry)
		c.cache[key] = ele
		c.usedBytes += int64(len(key)) + int64(value.Len())
	}
	
	// 清理过期数据
	c.removeExpired()
	
	// 如果超过最大容量，移除最旧的数据
	for c.maxBytes > 0 && c.usedBytes > c.maxBytes {
		c.removeOldest()
	}
}

// Remove 移除指定缓存
func (c *Cache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if ele, ok := c.cache[key]; ok {
		c.removeElement(ele)
	}
}

// removeOldest 移除最旧的缓存
func (c *Cache) removeOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

// removeElement 移除指定元素
func (c *Cache) removeElement(ele *list.Element) {
	c.ll.Remove(ele)
	entry := ele.Value.(*Entry)
	delete(c.cache, entry.Key)
	c.usedBytes -= int64(len(entry.Key)) + int64(entry.Value.Len())
	
	if c.OnEvicted != nil {
		c.OnEvicted(entry.Key, entry.Value)
	}
}

// removeExpired 移除过期数据
func (c *Cache) removeExpired() {
	now := time.Now().Unix()
	for ele := c.ll.Back(); ele != nil; {
		entry := ele.Value.(*Entry)
		if entry.ExpireTime > 0 && now > entry.CreateAt+entry.ExpireTime {
			prev := ele.Prev()
			c.removeElement(ele)
			ele = prev
		} else {
			break
		}
	}
}

// Len 返回缓存条目数量
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ll.Len()
}

// Clear 清空缓存
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.ll.Init()
	c.cache = make(map[string]*list.Element)
	c.usedBytes = 0
}

// GetAll 获取所有缓存条目（只读视图）
func (c *Cache) GetAll() []*Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entries := make([]*Entry, 0, c.ll.Len())
	for ele := c.ll.Front(); ele != nil; ele = ele.Next() {
		entry := ele.Value.(*Entry)
		entries = append(entries, &Entry{
			Key:        entry.Key,
			Value:      entry.Value,
			CreateAt:   entry.CreateAt,
			ExpireTime: entry.ExpireTime,
		})
	}
	return entries
}

