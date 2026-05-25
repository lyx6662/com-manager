package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 通讯管理机主配置
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Web            WebConfig            `yaml:"web"`
	SerialDevices  []SerialDeviceConfig `yaml:"serial_devices"`
	NetworkDevices []NetworkDeviceConfig `yaml:"network_devices"`
	Outputs        OutputConfig         `yaml:"outputs"`
	OfflineBuffer  OfflineBufferConfig  `yaml:"offline_buffer"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Name     string `yaml:"name"`
	LogLevel string `yaml:"log_level"`
	LogPath  string `yaml:"log_path"`
}

// WebConfig Web管理配置
type WebConfig struct {
	Enabled bool       `yaml:"enabled"`
	Port    int        `yaml:"port"`
	Host    string     `yaml:"host"`
	Auth    AuthConfig `yaml:"auth"`
	CORS    CORSConfig `yaml:"cors"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	TokenSecret  string `yaml:"token_secret"`
	TokenExpire  string `yaml:"token_expire"`
}

// CORSConfig 跨域配置
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// SerialDeviceConfig 串口设备配置
type SerialDeviceConfig struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Port         string `yaml:"port"`
	BaudRate     int    `yaml:"baud_rate"`
	DataBits     int    `yaml:"data_bits"`
	StopBits     int    `yaml:"stop_bits"`
	Parity       string `yaml:"parity"`
	Protocol     string `yaml:"protocol"`
	SlaveID      int    `yaml:"slave_id"`
	PollInterval string `yaml:"poll_interval"`
	Timeout      string `yaml:"timeout"`
	Retry        int    `yaml:"retry"`
	Enabled      bool   `yaml:"enabled"`
}

// NetworkDeviceConfig 网口设备配置
type NetworkDeviceConfig struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Protocol     string `yaml:"protocol"`
	SlaveID      int    `yaml:"slave_id"`
	PollInterval string `yaml:"poll_interval"`
	Timeout      string `yaml:"timeout"`
	Retry        int    `yaml:"retry"`
	Enabled      bool   `yaml:"enabled"`
	CIDFile      string `yaml:"cid_file"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	ModbusTCPServers []ModbusTCPServerConfig `yaml:"modbus_tcp_servers"`
	ModbusRTUServers []ModbusRTUServerConfig `yaml:"modbus_rtu_servers"`
}

// ModbusTCPServerConfig Modbus TCP输出配置
type ModbusTCPServerConfig struct {
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	ListenPort     int    `yaml:"listen_port"`
	SlaveID        int    `yaml:"slave_id"`
	MaxConnections int    `yaml:"max_connections"`
}

// ModbusRTUServerConfig Modbus RTU输出配置
type ModbusRTUServerConfig struct {
	ID       string `yaml:"id"`
	Name     string `yaml:"name"`
	Port     string `yaml:"port"`
	BaudRate int    `yaml:"baud_rate"`
	SlaveID  int    `yaml:"slave_id"`
}

// OfflineBufferConfig 断点续传配置
type OfflineBufferConfig struct {
	Enabled          bool              `yaml:"enabled"`
	DBPath           string            `yaml:"db_path"`
	RetentionDays    int               `yaml:"retention_days"`
	MaxDBSizeMB      int               `yaml:"max_db_size_mb"`
	MemoryQueueSize  int               `yaml:"memory_queue_size"`
	FlushInterval    string            `yaml:"flush_interval"`
	ReportStrategy   ReportStrategy    `yaml:"report_strategy"`
	GroupOverride    map[string]GroupOverride `yaml:"group_override"`
}

// ReportStrategy 补传策略
type ReportStrategy struct {
	BatchSize       int    `yaml:"batch_size"`
	BatchInterval   string `yaml:"batch_interval"`
	MaxRetries      int    `yaml:"max_retries"`
	PriorityMode    string `yaml:"priority_mode"`
	ConcurrentGroup int    `yaml:"concurrent_group"`
}

// GroupOverride 分组单独配置
type GroupOverride struct {
	RetentionDays int    `yaml:"retention_days"`
	PriorityMode  string `yaml:"priority_mode"`
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	setDefaults(cfg)

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.LogLevel == "" {
		cfg.Server.LogLevel = "info"
	}
	if cfg.Server.LogPath == "" {
		cfg.Server.LogPath = "./logs"
	}
	if cfg.Web.Port == 0 {
		cfg.Web.Port = 8080
	}
	if cfg.Web.Host == "" {
		cfg.Web.Host = "0.0.0.0"
	}
	if cfg.Web.Auth.TokenExpire == "" {
		cfg.Web.Auth.TokenExpire = "24h"
	}
	if cfg.OfflineBuffer.DBPath == "" {
		cfg.OfflineBuffer.DBPath = "./data/offline.db"
	}
	if cfg.OfflineBuffer.RetentionDays == 0 {
		cfg.OfflineBuffer.RetentionDays = 10
	}
	if cfg.OfflineBuffer.MemoryQueueSize == 0 {
		cfg.OfflineBuffer.MemoryQueueSize = 10000
	}
	if cfg.OfflineBuffer.FlushInterval == "" {
		cfg.OfflineBuffer.FlushInterval = "1s"
	}
	if cfg.OfflineBuffer.ReportStrategy.BatchSize == 0 {
		cfg.OfflineBuffer.ReportStrategy.BatchSize = 100
	}
	if cfg.OfflineBuffer.ReportStrategy.BatchInterval == "" {
		cfg.OfflineBuffer.ReportStrategy.BatchInterval = "500ms"
	}
	if cfg.OfflineBuffer.ReportStrategy.MaxRetries == 0 {
		cfg.OfflineBuffer.ReportStrategy.MaxRetries = 3
	}
	if cfg.OfflineBuffer.ReportStrategy.PriorityMode == "" {
		cfg.OfflineBuffer.ReportStrategy.PriorityMode = "time"
	}
}

// GetPollDuration 获取轮询间隔
func (d *SerialDeviceConfig) GetPollDuration() time.Duration {
	duration, err := time.ParseDuration(d.PollInterval)
	if err != nil {
		return 5 * time.Second
	}
	return duration
}

// GetTimeoutDuration 获取超时时间
func (d *SerialDeviceConfig) GetTimeoutDuration() time.Duration {
	duration, err := time.ParseDuration(d.Timeout)
	if err != nil {
		return 3 * time.Second
	}
	return duration
}

// GetPollDuration 获取轮询间隔
func (d *NetworkDeviceConfig) GetPollDuration() time.Duration {
	duration, err := time.ParseDuration(d.PollInterval)
	if err != nil {
		return 5 * time.Second
	}
	return duration
}

// GetTimeoutDuration 获取超时时间
func (d *NetworkDeviceConfig) GetTimeoutDuration() time.Duration {
	duration, err := time.ParseDuration(d.Timeout)
	if err != nil {
		return 3 * time.Second
	}
	return duration
}
