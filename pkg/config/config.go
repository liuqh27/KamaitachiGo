package config

import (
	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
)

// Config 应用配置
type Config struct {
	Server   ServerConfig   `ini:"server"`
	Cache    CacheConfig    `ini:"cache"`
	Etcd     EtcdConfig     `ini:"etcd"`
	Database DatabaseConfig `ini:"database"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port        string `ini:"port"`         // 服务端口
	Mode        string `ini:"mode"`         // 运行模式: master/slave/gateway
	ServiceName string `ini:"service_name"` // 服务名称
	ServiceAddr string `ini:"service_addr"` // 服务地址
}

// CacheConfig 缓存配置
type CacheConfig struct {
	MaxBytes     int64  `ini:"max_bytes"`     // 最大缓存字节数
	SnapshotPath string `ini:"snapshot_path"` // 快照文件路径
	SnapshotInterval int `ini:"snapshot_interval"` // 快照间隔（分钟）
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	Endpoints  string `ini:"endpoints"`   // etcd地址列表，逗号分隔
	Prefix     string `ini:"prefix"`      // 键前缀
	TTL        int64  `ini:"ttl"`         // 租约TTL（秒）
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `ini:"host"`
	Port     int    `ini:"port"`
	Username string `ini:"username"`
	Password string `ini:"password"`
	Database string `ini:"database"`
	MaxIdle  int    `ini:"max_idle"`  // 最大空闲连接数
	MaxOpen  int    `ini:"max_open"`  // 最大打开连接数
}

// LoadConfig 加载配置文件
func LoadConfig(filePath string) (*Config, error) {
	cfg := &Config{}
	
	err := ini.MapTo(cfg, filePath)
	if err != nil {
		logrus.Errorf("Failed to load config file: %v", err)
		return nil, err
	}
	
	logrus.Infof("Config loaded successfully from: %s", filePath)
	return cfg, nil
}

// GetDSN 获取数据库连接字符串
func (d *DatabaseConfig) GetDSN() string {
	return d.Username + ":" + d.Password + "@tcp(" + d.Host + ")/" + d.Database + "?charset=utf8mb4&parseTime=True&loc=Local"
}

