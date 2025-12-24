package etcd

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Client etcd客户端
type Client struct {
	cli       *clientv3.Client
	endpoints []string
	timeout   time.Duration
}

// NewClient 创建etcd客户端
func NewClient(endpoints []string) (*Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return &Client{
		cli:       cli,
		endpoints: endpoints,
		timeout:   5 * time.Second,
	}, nil
}

// Put 设置键值
func (c *Client) Put(key, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.cli.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("failed to put key %s: %w", key, err)
	}
	return nil
}

// Get 获取键值
func (c *Client) Get(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.cli.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key %s not found", key)
	}

	return string(resp.Kvs[0].Value), nil
}

// GetWithPrefix 获取指定前缀的所有键值
func (c *Client) GetWithPrefix(prefix string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resp, err := c.cli.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get keys with prefix %s: %w", prefix, err)
	}

	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}

	return result, nil
}

// Delete 删除键
func (c *Client) Delete(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	_, err := c.cli.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}
	return nil
}

// Watch 监听键的变化
func (c *Client) Watch(key string, callback func(string, string)) {
	go func() {
		watchChan := c.cli.Watch(context.Background(), key)
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				callback(string(event.Kv.Key), string(event.Kv.Value))
			}
		}
	}()
}

// WatchPrefix 监听指定前缀键的变化
func (c *Client) WatchPrefix(prefix string, callback func(eventType string, key string, value string)) {
	go func() {
		watchChan := c.cli.Watch(context.Background(), prefix, clientv3.WithPrefix())
		for watchResp := range watchChan {
			for _, event := range watchResp.Events {
				eventType := "PUT"
				if event.Type == clientv3.EventTypeDelete {
					eventType = "DELETE"
				}
				callback(eventType, string(event.Kv.Key), string(event.Kv.Value))
				logrus.Infof("[Etcd] Watch event: %s, key: %s, value: %s", 
					eventType, string(event.Kv.Key), string(event.Kv.Value))
			}
		}
	}()
}

// Register 注册服务
func (c *Client) Register(serviceName, serviceAddr string, ttl int64) error {
	// 创建租约
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	lease, err := c.cli.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to create lease: %w", err)
	}

	// 注册服务
	key := fmt.Sprintf("/services/%s/%s", serviceName, serviceAddr)
	_, err = c.cli.Put(context.Background(), key, serviceAddr, clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 保持心跳
	ch, err := c.cli.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		return fmt.Errorf("failed to keep alive: %w", err)
	}

	// 消费心跳响应
	go func() {
		for range ch {
			// 心跳响应
		}
	}()

	logrus.Infof("[Etcd] Service registered: %s -> %s", key, serviceAddr)
	return nil
}

// Discover 发现服务
func (c *Client) Discover(serviceName string) ([]string, error) {
	prefix := fmt.Sprintf("/services/%s/", serviceName)
	kvs, err := c.GetWithPrefix(prefix)
	if err != nil {
		return nil, err
	}

	services := make([]string, 0, len(kvs))
	for _, addr := range kvs {
		services = append(services, addr)
	}

	return services, nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	return c.cli.Close()
}

