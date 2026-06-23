package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// OutputFileConfig 输出配置文件结构
type OutputFileConfig struct {
	ModbusOutput    ModbusOutputConfig    `yaml:"modbus_output" json:"modbus_output"`
	IEC61850Output  IEC61850OutputConfig  `yaml:"iec61850_output" json:"iec61850_output"`
	MQTTOutput      MQTTOutputConfig      `yaml:"mqtt_output" json:"mqtt_output"`
}

// ModbusOutputConfig Modbus 输出配置
type ModbusOutputConfig struct {
	Enabled        bool                   `yaml:"enabled" json:"enabled"`
	TCPServers     []ModbusTCPServerConfig `yaml:"tcp_servers" json:"tcp_servers"`
	RTUServers     []ModbusRTUServerConfig `yaml:"rtu_servers" json:"rtu_servers"`
	GroupDevices   map[string][]string    `yaml:"group_devices" json:"group_devices"` // 分组包含的设备ID列表
	Mappings       []ModbusOutputMapping  `yaml:"mappings" json:"mappings"`
}

// ModbusOutputMapping Modbus 输出映射配置
type ModbusOutputMapping struct {
	SourceID        string  `yaml:"source_id" json:"source_id"`           // 数据点ID
	TargetRegister  uint16  `yaml:"target_register" json:"target_register"` // 目标寄存器地址
	TargetType      string  `yaml:"target_type" json:"target_type"`       // holding/coil
	Scale           float64 `yaml:"scale" json:"scale"`
	Offset          float64 `yaml:"offset" json:"offset"`
	DataType        string  `yaml:"data_type" json:"data_type"`           // 输出数据类型
	ByteOrder       string  `yaml:"byte_order" json:"byte_order"`
}

// IEC61850OutputConfig IEC 61850 输出配置
type IEC61850OutputConfig struct {
	Enabled   bool                   `yaml:"enabled" json:"enabled"`
	Port      int                    `yaml:"port" json:"port"`
	IEDName   string                 `yaml:"ied_name" json:"ied_name"`
	Mappings  []IEC61850OutputMapping `yaml:"mappings" json:"mappings"`
}

// IEC61850OutputMapping IEC 61850 输出映射配置
type IEC61850OutputMapping struct {
	SourceID     string  `yaml:"source_id" json:"source_id"`         // 数据点ID
	IEC61850Path string  `yaml:"iec61850_path" json:"iec61850_path"` // IEC 61850 路径
	TargetType   string  `yaml:"target_type" json:"target_type"`     // float32/int32/bool
	Scale        float64 `yaml:"scale" json:"scale"`
	Offset       float64 `yaml:"offset" json:"offset"`
}

// MQTTOutputConfig MQTT 输出配置
type MQTTOutputConfig struct {
	Enabled  bool            `yaml:"enabled" json:"enabled"`
	Broker   string          `yaml:"broker" json:"broker"`
	ClientID string          `yaml:"client_id" json:"client_id"`
	Topics   []MQTTTopicConfig `yaml:"topics" json:"topics"`
}

// MQTTTopicConfig MQTT 主题配置
type MQTTTopicConfig struct {
	SourceID string `yaml:"source_id" json:"source_id"` // 数据点ID
	Topic    string `yaml:"topic" json:"topic"`
	QoS      byte   `yaml:"qos" json:"qos"`
}

// OutputFileConfigManager 输出配置文件管理器
type OutputFileConfigManager struct {
	path string
	cfg  *OutputFileConfig
}

// NewOutputFileConfigManager 创建输出配置文件管理器
func NewOutputFileConfigManager(path string) *OutputFileConfigManager {
	return &OutputFileConfigManager{
		path: path,
	}
}

// Load 加载输出配置文件
func (m *OutputFileConfigManager) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建默认配置
			m.cfg = &OutputFileConfig{
				ModbusOutput: ModbusOutputConfig{
					Enabled:  true,
					Mappings: make([]ModbusOutputMapping, 0),
				},
				IEC61850Output: IEC61850OutputConfig{
					Enabled:  false,
					Mappings: make([]IEC61850OutputMapping, 0),
				},
				MQTTOutput: MQTTOutputConfig{
					Enabled: false,
					Topics:  make([]MQTTTopicConfig, 0),
				},
			}
			return m.Save()
		}
		return fmt.Errorf("读取输出配置文件失败: %w", err)
	}

	m.cfg = &OutputFileConfig{}
	if err := yaml.Unmarshal(data, m.cfg); err != nil {
		return fmt.Errorf("解析输出配置文件失败: %w", err)
	}

	return nil
}

// Save 保存输出配置文件
func (m *OutputFileConfigManager) Save() error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}

	data, err := yaml.Marshal(m.cfg)
	if err != nil {
		return fmt.Errorf("序列化输出配置失败: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("写入输出配置文件失败: %w", err)
	}

	return nil
}

// Get 获取输出配置
func (m *OutputFileConfigManager) Get() *OutputFileConfig {
	return m.cfg
}

// GetModbusOutput 获取 Modbus 输出配置
func (m *OutputFileConfigManager) GetModbusOutput() *ModbusOutputConfig {
	if m.cfg == nil {
		return nil
	}
	return &m.cfg.ModbusOutput
}

// GetIEC61850Output 获取 IEC 61850 输出配置
func (m *OutputFileConfigManager) GetIEC61850Output() *IEC61850OutputConfig {
	if m.cfg == nil {
		return nil
	}
	return &m.cfg.IEC61850Output
}

// GetMQTTOutput 获取 MQTT 输出配置
func (m *OutputFileConfigManager) GetMQTTOutput() *MQTTOutputConfig {
	if m.cfg == nil {
		return nil
	}
	return &m.cfg.MQTTOutput
}

// SetModbusOutput 设置 Modbus 输出配置
func (m *OutputFileConfigManager) SetModbusOutput(cfg ModbusOutputConfig) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}
	m.cfg.ModbusOutput = cfg
	return m.Save()
}

// SetIEC61850Output 设置 IEC 61850 输出配置
func (m *OutputFileConfigManager) SetIEC61850Output(cfg IEC61850OutputConfig) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}
	m.cfg.IEC61850Output = cfg
	return m.Save()
}

// SetMQTTOutput 设置 MQTT 输出配置
func (m *OutputFileConfigManager) SetMQTTOutput(cfg MQTTOutputConfig) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}
	m.cfg.MQTTOutput = cfg
	return m.Save()
}

// AddModbusMapping 添加 Modbus 映射
func (m *OutputFileConfigManager) AddModbusMapping(mapping ModbusOutputMapping) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}
	m.cfg.ModbusOutput.Mappings = append(m.cfg.ModbusOutput.Mappings, mapping)
	return m.Save()
}

// UpdateModbusMapping 更新 Modbus 映射
func (m *OutputFileConfigManager) UpdateModbusMapping(sourceID string, mapping ModbusOutputMapping) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}
	for i, item := range m.cfg.ModbusOutput.Mappings {
		if item.SourceID == sourceID {
			m.cfg.ModbusOutput.Mappings[i] = mapping
			return m.Save()
		}
	}
	return fmt.Errorf("映射不存在: %s", sourceID)
}

// DeleteModbusMapping 删除 Modbus 映射
func (m *OutputFileConfigManager) DeleteModbusMapping(sourceID string) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}
	for i, mapping := range m.cfg.ModbusOutput.Mappings {
		if mapping.SourceID == sourceID {
			m.cfg.ModbusOutput.Mappings = append(m.cfg.ModbusOutput.Mappings[:i], m.cfg.ModbusOutput.Mappings[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("映射不存在: %s", sourceID)
}
