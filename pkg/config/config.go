package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 通讯管理机主配置
type Config struct {
	Server         ServerConfig             `yaml:"server"`
	Web            WebConfig                `yaml:"web"`
	SerialDevices  []SerialDeviceConfig     `yaml:"serial_devices"`
	NetworkDevices []NetworkDeviceConfig    `yaml:"network_devices"`
	Outputs        OutputConfig             `yaml:"outputs"`
	Mappings       map[string][]MappingRule `yaml:"mappings"` // key为设备ID，value为该设备的点表规则
	OfflineBuffer  OfflineBufferConfig      `yaml:"offline_buffer"`
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
	Enabled        bool                    `yaml:"enabled" json:"enabled"`                // 是否启用 Modbus 输出
	ModbusTCPServers []ModbusTCPServerConfig `yaml:"modbus_tcp_servers" json:"modbus_tcp_servers"`
	ModbusRTUServers []ModbusRTUServerConfig `yaml:"modbus_rtu_servers" json:"modbus_rtu_servers"`
	GroupDevices     map[string][]string     `yaml:"group_devices" json:"group_devices"` // 分组包含的设备ID列表
}

// ModbusTCPServerConfig Modbus TCP输出配置
type ModbusTCPServerConfig struct {
	ID             string `yaml:"id" json:"id"`
	Name           string `yaml:"name" json:"name"`
	ListenPort     int    `yaml:"listen_port" json:"listen_port"`
	SlaveID        int    `yaml:"slave_id" json:"slave_id"`
	MaxConnections int    `yaml:"max_connections" json:"max_connections"`
}

// ModbusRTUServerConfig Modbus RTU输出配置
type ModbusRTUServerConfig struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Port     string `yaml:"port" json:"port"`
	BaudRate int    `yaml:"baud_rate" json:"baud_rate"`
	SlaveID  int    `yaml:"slave_id" json:"slave_id"`
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

// MappingRule 点表映射规则
type MappingRule struct {
	Name            string  `yaml:"name" json:"name"`                         // 数据点名称
	SourceDevice    string  `yaml:"source_device" json:"source_device"`       // 源设备ID
	SourceRegister  uint16  `yaml:"source_register" json:"source_register"`   // 源寄存器地址
	SourceType      string  `yaml:"source_type" json:"source_type"`           // 源寄存器类型: holding/input/coil
	DataType        string  `yaml:"data_type" json:"data_type"`               // 数据类型: uint16/int16/float32/bool
	RegisterCount   int     `yaml:"register_count" json:"register_count"`     // 占用寄存器数量 (float32=2, 其他=1)
	TargetRegister  uint16  `yaml:"target_register" json:"target_register"`   // 目标输出寄存器地址
	Scale           float64 `yaml:"scale" json:"scale"`                       // 缩放系数
	Offset          float64 `yaml:"offset" json:"offset"`                     // 偏移量
	Unit            string  `yaml:"unit" json:"unit"`                         // 单位 (仅用于显示)
	MaxPoints       int     `yaml:"max_points" json:"max_points"`             // 批量读取时最大数据点数 (0=不限制)
	HighLimit       float64 `yaml:"high_limit" json:"high_limit"`             // 报警上限 (0=不启用)
	LowLimit        float64 `yaml:"low_limit" json:"low_limit"`               // 报警下限 (0=不启用)
	ByteOrder       string  `yaml:"byte_order" json:"byte_order"`             // 32位浮点字节序: ABCD(大端)/BADC(字交换)/CDAB(字节交换)/DCBA(小端)，默认ABCD
}

// Manager 配置管理器
type Manager struct {
	path         string
	cfg          *Config
	iec61850Path string
	iec61850Cfg  *ModbusToIEC61850Config
}

// NewManager 创建配置管理器
func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	m := &Manager{path: path, cfg: cfg}

	// 尝试加载 IEC 61850 配置文件
	iec61850Path := "./configs/modbus_to_61850.yaml"
	iec61850Cfg, err := LoadModbusToIEC61850(iec61850Path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[WARN] IEC 61850 配置文件不存在: %s，将自动生成默认配置\n", iec61850Path)
			if genErr := GenerateDefaultIEC61850Config(iec61850Path); genErr != nil {
				fmt.Fprintf(os.Stderr, "[ERROR] 自动生成 IEC 61850 默认配置失败: %v\n", genErr)
			} else {
				// 重新加载生成的默认配置
				iec61850Cfg, _ = LoadModbusToIEC61850(iec61850Path)
				m.iec61850Path = iec61850Path
				m.iec61850Cfg = iec61850Cfg
			}
		} else {
			fmt.Fprintf(os.Stderr, "[WARN] 加载 IEC 61850 配置失败: %v\n", err)
		}
	} else {
		m.iec61850Path = iec61850Path
		m.iec61850Cfg = iec61850Cfg
	}

	return m, nil
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

// GetModbusRTUServer 获取Modbus RTU输出配置
func (m *Manager) GetModbusRTUServer(id string) *ModbusRTUServerConfig {
	for i := range m.cfg.Outputs.ModbusRTUServers {
		if m.cfg.Outputs.ModbusRTUServers[i].ID == id {
			return &m.cfg.Outputs.ModbusRTUServers[i]
		}
	}
	return nil
}

// AddModbusRTUServer 添加Modbus RTU输出配置
func (m *Manager) AddModbusRTUServer(srv ModbusRTUServerConfig) error {
	if m.GetModbusRTUServer(srv.ID) != nil {
		return fmt.Errorf("输出配置已存在: %s", srv.ID)
	}
	m.cfg.Outputs.ModbusRTUServers = append(m.cfg.Outputs.ModbusRTUServers, srv)
	return m.Save()
}

// UpdateModbusRTUServer 更新Modbus RTU输出配置
func (m *Manager) UpdateModbusRTUServer(srv ModbusRTUServerConfig) error {
	for i := range m.cfg.Outputs.ModbusRTUServers {
		if m.cfg.Outputs.ModbusRTUServers[i].ID == srv.ID {
			m.cfg.Outputs.ModbusRTUServers[i] = srv
			return m.Save()
		}
	}
	return fmt.Errorf("输出配置不存在: %s", srv.ID)
}

// DeleteModbusRTUServer 删除Modbus RTU输出配置
func (m *Manager) DeleteModbusRTUServer(id string) error {
	for i := range m.cfg.Outputs.ModbusRTUServers {
		if m.cfg.Outputs.ModbusRTUServers[i].ID == id {
			m.cfg.Outputs.ModbusRTUServers = append(m.cfg.Outputs.ModbusRTUServers[:i], m.cfg.Outputs.ModbusRTUServers[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("输出配置不存在: %s", id)
}

// SetOutputs 批量设置所有输出配置
func (m *Manager) SetOutputs(tcpServers []ModbusTCPServerConfig, rtuServers []ModbusRTUServerConfig) error {
	m.cfg.Outputs.ModbusTCPServers = tcpServers
	m.cfg.Outputs.ModbusRTUServers = rtuServers
	return m.Save()
}

// GetGroupDevices 获取分组包含的设备列表
func (m *Manager) GetGroupDevices(groupID string) []string {
	if m.cfg.Outputs.GroupDevices == nil {
		return nil
	}
	return m.cfg.Outputs.GroupDevices[groupID]
}

// SetGroupDevices 设置分组包含的设备列表
func (m *Manager) SetGroupDevices(groupID string, deviceIDs []string) error {
	if m.cfg.Outputs.GroupDevices == nil {
		m.cfg.Outputs.GroupDevices = make(map[string][]string)
	}
	m.cfg.Outputs.GroupDevices[groupID] = deviceIDs
	return m.Save()
}

func setDefaults(cfg *Config) {
	// 串口设备默认值
	for i := range cfg.SerialDevices {
		if cfg.SerialDevices[i].DataBits == 0 {
			cfg.SerialDevices[i].DataBits = 8
		}
		if cfg.SerialDevices[i].StopBits == 0 {
			cfg.SerialDevices[i].StopBits = 1
		}
		if cfg.SerialDevices[i].Parity == "" {
			cfg.SerialDevices[i].Parity = "none"
		}
		if cfg.SerialDevices[i].Retry <= 0 {
			cfg.SerialDevices[i].Retry = 3
		}
	}

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
		cfg.OfflineBuffer.FlushInterval = "10m"
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

// GetRegisterCount 获取映射规则占用的寄存器数量
func (r *MappingRule) GetRegisterCount() int {
	if r.RegisterCount > 0 {
		return r.RegisterCount
	}
	switch r.DataType {
	case "float32", "int32", "uint32":
		return 2
	default:
		return 1
	}
}

// GetMappingRules 获取指定设备的映射规则
func (m *Manager) GetMappingRules(deviceID string) []MappingRule {
	if m.cfg.Mappings == nil {
		return nil
	}
	return m.cfg.Mappings[deviceID]
}

// SetMappingRules 设置指定设备的映射规则
func (m *Manager) SetMappingRules(deviceID string, rules []MappingRule) error {
	if m.cfg.Mappings == nil {
		m.cfg.Mappings = make(map[string][]MappingRule)
	}
	m.cfg.Mappings[deviceID] = rules
	return m.Save()
}

// GetAllMappings 获取所有映射配置
func (m *Manager) GetAllMappings() map[string][]MappingRule {
	return m.cfg.Mappings
}

// GetDeviceGroupID 获取设备所属的分组ID
func (m *Manager) GetDeviceGroupID(deviceID string) string {
	for groupID, devices := range m.cfg.Outputs.GroupDevices {
		for _, devID := range devices {
			if devID == deviceID {
				return groupID
			}
		}
	}
	return ""
}

// GetDeviceByID 根据ID查找设备 (串口或网口)
func (m *Manager) GetDeviceByID(id string) interface{} {
	if dev := m.GetSerialDevice(id); dev != nil {
		return dev
	}
	return m.GetNetworkDevice(id)
}

// === IEC 61850 配置管理方法 ===

// GetIEC61850Config 获取 IEC 61850 配置
func (m *Manager) GetIEC61850Config() *ModbusToIEC61850Config {
	return m.iec61850Cfg
}

// IsIEC61850Enabled 检查 IEC 61850 功能是否启用
func (m *Manager) IsIEC61850Enabled() bool {
	return m.iec61850Cfg != nil && m.iec61850Cfg.IEC61850.Enabled
}

// SetIEC61850Config 设置 IEC 61850 配置
func (m *Manager) SetIEC61850Config(cfg *ModbusToIEC61850Config) error {
	m.iec61850Cfg = cfg
	return m.SaveIEC61850()
}

// ValidateIEC61850Mappings 校验 IEC 61850 映射路径是否在模型中存在
func (m *Manager) ValidateIEC61850Mappings(cfg *ModbusToIEC61850Config) error {
	return validateModelPaths(cfg)
}

// SaveIEC61850 保存 IEC 61850 配置到文件
func (m *Manager) SaveIEC61850() error {
	if m.iec61850Path == "" {
		m.iec61850Path = "./configs/modbus_to_61850.yaml"
	}
	return SaveModbusToIEC61850(m.iec61850Path, m.iec61850Cfg)
}

// GetIEC61850Mappings 获取 IEC 61850 映射规则
func (m *Manager) GetIEC61850Mappings() []IEC61850MappingRule {
	if m.iec61850Cfg == nil {
		return nil
	}
	return m.iec61850Cfg.Mappings
}

// SetIEC61850Mappings 设置 IEC 61850 映射规则
func (m *Manager) SetIEC61850Mappings(rules []IEC61850MappingRule) error {
	if m.iec61850Cfg == nil {
		return fmt.Errorf("IEC 61850 配置未加载")
	}
	m.iec61850Cfg.Mappings = rules
	return m.SaveIEC61850()
}
