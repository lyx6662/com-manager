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

// Manager 配置管理器
type Manager struct {
	path string
	cfg  *Config
}

// NewManager 创建配置管理器
func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Manager{path: path, cfg: cfg}, nil
}

// Get 获取配置
func (m *Manager) Get() *Config {
	return m.cfg
}

// Save 保存配置到文件
func (m *Manager) Save() error {
	return Save(m.path, m.cfg)
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

// Save 保存配置到文件
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// === 设备管理方法 ===

// GetSerialDevice 获取串口设备
func (m *Manager) GetSerialDevice(id string) *SerialDeviceConfig {
	for i := range m.cfg.SerialDevices {
		if m.cfg.SerialDevices[i].ID == id {
			return &m.cfg.SerialDevices[i]
		}
	}
	return nil
}

// GetNetworkDevice 获取网口设备
func (m *Manager) GetNetworkDevice(id string) *NetworkDeviceConfig {
	for i := range m.cfg.NetworkDevices {
		if m.cfg.NetworkDevices[i].ID == id {
			return &m.cfg.NetworkDevices[i]
		}
	}
	return nil
}

// AddSerialDevice 添加串口设备
func (m *Manager) AddSerialDevice(dev SerialDeviceConfig) error {
	if m.GetSerialDevice(dev.ID) != nil {
		return fmt.Errorf("设备已存在: %s", dev.ID)
	}
	m.cfg.SerialDevices = append(m.cfg.SerialDevices, dev)
	return m.Save()
}

// UpdateSerialDevice 更新串口设备
func (m *Manager) UpdateSerialDevice(dev SerialDeviceConfig) error {
	for i := range m.cfg.SerialDevices {
		if m.cfg.SerialDevices[i].ID == dev.ID {
			m.cfg.SerialDevices[i] = dev
			return m.Save()
		}
	}
	return fmt.Errorf("设备不存在: %s", dev.ID)
}

// DeleteSerialDevice 删除串口设备
func (m *Manager) DeleteSerialDevice(id string) error {
	for i := range m.cfg.SerialDevices {
		if m.cfg.SerialDevices[i].ID == id {
			m.cfg.SerialDevices = append(m.cfg.SerialDevices[:i], m.cfg.SerialDevices[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("设备不存在: %s", id)
}

// AddNetworkDevice 添加网口设备
func (m *Manager) AddNetworkDevice(dev NetworkDeviceConfig) error {
	if m.GetNetworkDevice(dev.ID) != nil {
		return fmt.Errorf("设备已存在: %s", dev.ID)
	}
	m.cfg.NetworkDevices = append(m.cfg.NetworkDevices, dev)
	return m.Save()
}

// UpdateNetworkDevice 更新网口设备
func (m *Manager) UpdateNetworkDevice(dev NetworkDeviceConfig) error {
	for i := range m.cfg.NetworkDevices {
		if m.cfg.NetworkDevices[i].ID == dev.ID {
			m.cfg.NetworkDevices[i] = dev
			return m.Save()
		}
	}
	return fmt.Errorf("设备不存在: %s", dev.ID)
}

// DeleteNetworkDevice 删除网口设备
func (m *Manager) DeleteNetworkDevice(id string) error {
	for i := range m.cfg.NetworkDevices {
		if m.cfg.NetworkDevices[i].ID == id {
			m.cfg.NetworkDevices = append(m.cfg.NetworkDevices[:i], m.cfg.NetworkDevices[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("设备不存在: %s", id)
}

// === 输出配置管理方法 ===

// GetModbusTCPServer 获取Modbus TCP输出配置
func (m *Manager) GetModbusTCPServer(id string) *ModbusTCPServerConfig {
	for i := range m.cfg.Outputs.ModbusTCPServers {
		if m.cfg.Outputs.ModbusTCPServers[i].ID == id {
			return &m.cfg.Outputs.ModbusTCPServers[i]
		}
	}
	return nil
}

// AddModbusTCPServer 添加Modbus TCP输出配置
func (m *Manager) AddModbusTCPServer(srv ModbusTCPServerConfig) error {
	if m.GetModbusTCPServer(srv.ID) != nil {
		return fmt.Errorf("输出配置已存在: %s", srv.ID)
	}
	m.cfg.Outputs.ModbusTCPServers = append(m.cfg.Outputs.ModbusTCPServers, srv)
	return m.Save()
}

// UpdateModbusTCPServer 更新Modbus TCP输出配置
func (m *Manager) UpdateModbusTCPServer(srv ModbusTCPServerConfig) error {
	for i := range m.cfg.Outputs.ModbusTCPServers {
		if m.cfg.Outputs.ModbusTCPServers[i].ID == srv.ID {
			m.cfg.Outputs.ModbusTCPServers[i] = srv
			return m.Save()
		}
	}
	return fmt.Errorf("输出配置不存在: %s", srv.ID)
}

// DeleteModbusTCPServer 删除Modbus TCP输出配置
func (m *Manager) DeleteModbusTCPServer(id string) error {
	for i := range m.cfg.Outputs.ModbusTCPServers {
		if m.cfg.Outputs.ModbusTCPServers[i].ID == id {
			m.cfg.Outputs.ModbusTCPServers = append(m.cfg.Outputs.ModbusTCPServers[:i], m.cfg.Outputs.ModbusTCPServers[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("输出配置不存在: %s", id)
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
