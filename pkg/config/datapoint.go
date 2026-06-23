package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// UnifiedDataPointConfig 统一数据点配置文件结构
type UnifiedDataPointConfig struct {
	DataPoints []UnifiedDataPoint `yaml:"data_points" json:"data_points"`
}

// UnifiedDataPoint 统一数据点定义
type UnifiedDataPoint struct {
	// 基本信息
	ID       string `yaml:"id" json:"id"`               // 唯一标识，如 "device1.temperature"
	DeviceID string `yaml:"device_id" json:"device_id"` // 所属设备ID
	Name     string `yaml:"name" json:"name"`           // 数据点名称

	// 采集配置
	RegisterType string `yaml:"register_type" json:"register_type"` // holding/coil/input
	RegisterAddr uint16 `yaml:"register_addr" json:"register_addr"` // 寄存器地址
	DataType     string `yaml:"data_type" json:"data_type"`         // uint16/int16/float32/int32/bool
	Quantity     uint16 `yaml:"quantity" json:"quantity"`           // 读取数量

	// 转换参数
	Scale     float64 `yaml:"scale" json:"scale"`
	Offset    float64 `yaml:"offset" offset:"offset"`
	ByteOrder string  `yaml:"byte_order" json:"byte_order"` // ABCD/BADC/CDAB/DCBA

	// 报警配置
	HighLimit float64 `yaml:"high_limit" json:"high_limit"` // 报警上限
	LowLimit  float64 `yaml:"low_limit" json:"low_limit"`   // 报警下限

	// 控制相关属性
	Writable        bool    `yaml:"writable" json:"writable"`                 // 是否可写
	WriteProtected  bool    `yaml:"write_protected" json:"write_protected"`   // 是否写保护
	MinValue        float64 `yaml:"min_value" json:"min_value"`               // 最小允许值
	MaxValue        float64 `yaml:"max_value" json:"max_value"`               // 最大允许值
	ConfirmRequired bool    `yaml:"confirm_required" json:"confirm_required"` // 是否需要确认

	// 元数据
	Description string            `yaml:"description" json:"description"`
	Unit        string            `yaml:"unit" json:"unit"`
	Tags        map[string]string `yaml:"tags" json:"tags"` // 自定义标签
}

// DataPointFileConfig 数据点文件配置
type DataPointFileConfig struct {
	path string
	cfg  *UnifiedDataPointConfig
}

// NewDataPointFileConfig 创建数据点文件配置管理器
func NewDataPointFileConfig(path string) *DataPointFileConfig {
	return &DataPointFileConfig{
		path: path,
	}
}

// Load 加载数据点配置文件
func (m *DataPointFileConfig) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建默认配置
			m.cfg = &UnifiedDataPointConfig{
				DataPoints: make([]UnifiedDataPoint, 0),
			}
			return m.Save()
		}
		return fmt.Errorf("读取数据点配置文件失败: %w", err)
	}

	m.cfg = &UnifiedDataPointConfig{}
	if err := yaml.Unmarshal(data, m.cfg); err != nil {
		return fmt.Errorf("解析数据点配置文件失败: %w", err)
	}

	return nil
}

// Save 保存数据点配置文件
func (m *DataPointFileConfig) Save() error {
	if m.cfg == nil {
		m.cfg = &UnifiedDataPointConfig{
			DataPoints: make([]UnifiedDataPoint, 0),
		}
	}

	data, err := yaml.Marshal(m.cfg)
	if err != nil {
		return fmt.Errorf("序列化数据点配置失败: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("写入数据点配置文件失败: %w", err)
	}

	return nil
}

// Get 获取数据点配置
func (m *DataPointFileConfig) Get() *UnifiedDataPointConfig {
	return m.cfg
}

// GetDataPoints 获取所有数据点
func (m *DataPointFileConfig) GetDataPoints() []UnifiedDataPoint {
	if m.cfg == nil {
		return nil
	}
	return m.cfg.DataPoints
}

// GetDataPointByID 根据ID获取数据点
func (m *DataPointFileConfig) GetDataPointByID(id string) *UnifiedDataPoint {
	if m.cfg == nil {
		return nil
	}
	for i := range m.cfg.DataPoints {
		if m.cfg.DataPoints[i].ID == id {
			return &m.cfg.DataPoints[i]
		}
	}
	return nil
}

// GetDataPointsByDeviceID 根据设备ID获取数据点列表
func (m *DataPointFileConfig) GetDataPointsByDeviceID(deviceID string) []UnifiedDataPoint {
	if m.cfg == nil {
		return nil
	}
	var result []UnifiedDataPoint
	for _, dp := range m.cfg.DataPoints {
		if dp.DeviceID == deviceID {
			result = append(result, dp)
		}
	}
	return result
}

// AddDataPoint 添加数据点
func (m *DataPointFileConfig) AddDataPoint(dp UnifiedDataPoint) error {
	if m.cfg == nil {
		m.cfg = &UnifiedDataPointConfig{
			DataPoints: make([]UnifiedDataPoint, 0),
		}
	}

	// 检查ID是否重复
	if m.GetDataPointByID(dp.ID) != nil {
		return fmt.Errorf("数据点已存在: %s", dp.ID)
	}

	m.cfg.DataPoints = append(m.cfg.DataPoints, dp)
	return m.Save()
}

// UpdateDataPoint 更新数据点
func (m *DataPointFileConfig) UpdateDataPoint(dp UnifiedDataPoint) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}

	for i := range m.cfg.DataPoints {
		if m.cfg.DataPoints[i].ID == dp.ID {
			m.cfg.DataPoints[i] = dp
			return m.Save()
		}
	}
	return fmt.Errorf("数据点不存在: %s", dp.ID)
}

// DeleteDataPoint 删除数据点
func (m *DataPointFileConfig) DeleteDataPoint(id string) error {
	if m.cfg == nil {
		return fmt.Errorf("配置未加载")
	}

	for i := range m.cfg.DataPoints {
		if m.cfg.DataPoints[i].ID == id {
			m.cfg.DataPoints = append(m.cfg.DataPoints[:i], m.cfg.DataPoints[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("数据点不存在: %s", id)
}

// GetDataPointCount 获取数据点数量
func (m *DataPointFileConfig) GetDataPointCount() int {
	if m.cfg == nil {
		return 0
	}
	return len(m.cfg.DataPoints)
}

// GetRegisterCount 获取数据点占用的寄存器数量
func (dp *UnifiedDataPoint) GetRegisterCount() int {
	if dp.Quantity > 0 {
		return int(dp.Quantity)
	}
	switch dp.DataType {
	case "float32", "int32", "uint32":
		return 2
	default:
		return 1
	}
}
