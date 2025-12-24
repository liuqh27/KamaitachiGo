package snapshot

import (
	"KamaitachiGo/internal/cache/lru"
	"KamaitachiGo/pkg/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
)

// Manager 快照管理器
type Manager struct {
	cache        *lru.Cache
	snapshotPath string
	mu           sync.Mutex
	stopChan     chan struct{}
}

// NewManager 创建快照管理器
func NewManager(cache *lru.Cache, snapshotPath string) *Manager {
	return &Manager{
		cache:        cache,
		snapshotPath: snapshotPath,
		stopChan:     make(chan struct{}),
	}
}

// Save 保存快照
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 确保目录存在
	dir := filepath.Dir(m.snapshotPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}
	
	// 获取所有缓存数据
	entries := m.cache.GetAll()
	
	// 序列化数据
	data, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot data: %w", err)
	}
	
	// 写入临时文件
	tmpFile := m.snapshotPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}
	
	// 重命名为正式文件（原子操作）
	if err := os.Rename(tmpFile, m.snapshotPath); err != nil {
		return fmt.Errorf("failed to rename snapshot file: %w", err)
	}
	
	logrus.Infof("[Snapshot Manager] Saved snapshot successfully, entries count: %d", len(entries))
	return nil
}

// Load 加载快照
func (m *Manager) Load() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 检查文件是否存在
	if _, err := os.Stat(m.snapshotPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("snapshot file not found: %s", m.snapshotPath)
	}
	
	// 读取文件
	data, err := os.ReadFile(m.snapshotPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read snapshot file: %w", err)
	}
	
	// 反序列化数据
	var entries []*lru.Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, fmt.Errorf("failed to unmarshal snapshot data: %w", err)
	}
	
	// 恢复数据到缓存
	count := 0
	for _, entry := range entries {
		// 检查数据是否过期
		if entry.ExpireTime > 0 && time.Now().Unix() > entry.CreateAt+entry.ExpireTime {
			continue
		}
		m.cache.Add(entry.Key, entry.Value)
		count++
	}
	
	logrus.Infof("[Snapshot Manager] Loaded snapshot successfully, entries count: %d", count)
	return count, nil
}

// AutoSnapshot 自动保存快照
func (m *Manager) AutoSnapshot(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if err := m.Save(); err != nil {
					logrus.Errorf("[Snapshot Manager] Auto save snapshot failed: %v", err)
				} else {
					logrus.Info("[Snapshot Manager] Auto save snapshot completed")
				}
			case <-m.stopChan:
				logrus.Info("[Snapshot Manager] Auto snapshot stopped")
				return
			}
		}
	}()
}

// Stop 停止自动快照
func (m *Manager) Stop() {
	close(m.stopChan)
	// 最后保存一次
	if err := m.Save(); err != nil {
		logrus.Errorf("[Snapshot Manager] Final save snapshot failed: %v", err)
	}
}

// GetSnapshotInfo 获取快照信息
func (m *Manager) GetSnapshotInfo() map[string]interface{} {
	info := make(map[string]interface{})
	
	if stat, err := os.Stat(m.snapshotPath); err == nil {
		info["path"] = m.snapshotPath
		info["size"] = stat.Size()
		info["modTime"] = stat.ModTime().Format("2006-01-02 15:04:05")
	} else {
		info["error"] = err.Error()
	}
	
	info["cacheEntries"] = m.cache.Len()
	
	return info
}

