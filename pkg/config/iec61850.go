package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// IEC61850Config IEC 61850 服务器配置
type IEC61850Config struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Port           int    `yaml:"port" json:"port"`
	IEDName        string `yaml:"ied_name" json:"ied_name"`
	MaxConnections int    `yaml:"max_connections" json:"max_connections"`
	ICDOutput      string `yaml:"icd_output" json:"icd_output"` // ICD 文件输出路径
}

// IEC61850ModelConfig 数据模型配置
type IEC61850ModelConfig struct {
	LogicalDevices []LogicalDeviceConfig `yaml:"logical_devices" json:"logical_devices"`
}

// LogicalDeviceConfig 逻辑设备配置
type LogicalDeviceConfig struct {
	Name         string              `yaml:"name" json:"name"`
	LogicalNodes []LogicalNodeConfig `yaml:"logical_nodes" json:"logical_nodes"`
}

// LogicalNodeConfig 逻辑节点配置
type LogicalNodeConfig struct {
	Name        string             `yaml:"name" json:"name"`
	DataObjects []DataObjectConfig `yaml:"data_objects" json:"data_objects"`
}

// DataObjectConfig 数据对象配置
type DataObjectConfig struct {
	Name           string                `yaml:"name" json:"name"`
	DataAttributes []DataAttributeConfig `yaml:"data_attributes" json:"data_attributes"`
}

// DataAttributeConfig 数据属性配置
type DataAttributeConfig struct {
	Name     string                `yaml:"name" json:"name"`
	Type     string                `yaml:"type,omitempty" json:"type"`           // FLOAT32, INT32, INT64, BOOLEAN, VISIBLE_STRING_255
	FC       string                `yaml:"fc,omitempty" json:"fc"`               // MX, ST, SP, CF, DC
	Triggers string                `yaml:"triggers,omitempty" json:"triggers"`   // DATA_CHANGED, QUALITY_CHANGED 等
	Children []DataAttributeConfig `yaml:"children,omitempty" json:"children"` // 容器节点的子属性，叶子节点为 null
}

// IEC61850MappingRule Modbus → IEC 61850 映射规则
type IEC61850MappingRule struct {
	SourceDevice   string  `yaml:"source_device" json:"source_device"`
	SourceName     string  `yaml:"source_name" json:"source_name"`         // 来源点位名称，匹配 DataPoint.Name
	SourceRegister uint16  `yaml:"source_register" json:"source_register"`
	SourceType     string  `yaml:"source_type" json:"source_type"`
	DataType       string  `yaml:"data_type" json:"data_type"`             // 源数据类型: uint16/int16/float32 等
	TargetType     string  `yaml:"target_type" json:"target_type"`         // IEC61850 目标类型: float32/int32/bool (默认 float32)
	IEC61850Path   string  `yaml:"iec61850_path" json:"iec61850_path"`
	Scale          float64 `yaml:"scale" json:"scale"`
	Offset         float64 `yaml:"offset" json:"offset"`
}

// ModbusToIEC61850Config 完整的 Modbus → IEC 61850 配置
type ModbusToIEC61850Config struct {
	IEC61850 IEC61850Config        `yaml:"iec61850"`
	Model    IEC61850ModelConfig   `yaml:"model"`
	Mappings []IEC61850MappingRule `yaml:"mappings"`
}

// LoadModbusToIEC61850 加载 Modbus → IEC 61850 配置文件
func LoadModbusToIEC61850(path string) (*ModbusToIEC61850Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &ModbusToIEC61850Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	setIEC61850Defaults(cfg)

	// 清理叶子节点的空 Children 数组（设为 nil 避免序列化为 []）
	cleanEmptyChildren(&cfg.Model)

	// 确保容器节点（type 为空）的 Children 不为 nil（YAML 反序列化会把 [] 变成 nil）
	ensureContainerChildren(&cfg.Model)

	// 校验映射路径（仅警告，不阻止加载）
	if err := validateModelPaths(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] IEC 61850 配置加载警告: %v\n", err)
	}

	return cfg, nil
}

// cleanEmptyChildren 递归清理叶子节点的空 Children 数组
func cleanEmptyChildren(model *IEC61850ModelConfig) {
	for i := range model.LogicalDevices {
		for j := range model.LogicalDevices[i].LogicalNodes {
			for k := range model.LogicalDevices[i].LogicalNodes[j].DataObjects {
				cleanDAChildren(&model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes)
			}
		}
	}
}

// cleanDAChildren 递归清理数据属性的空 Children
func cleanDAChildren(attrs *[]DataAttributeConfig) {
	for i := range *attrs {
		attr := &(*attrs)[i]
		// 如果 Children 是空数组（不是 nil），且有类型（是叶子节点），则设为 nil
		if len(attr.Children) == 0 && attr.Type != "" {
			attr.Children = nil
		}
		// 递归处理子属性
		if len(attr.Children) > 0 {
			cleanDAChildren(&attr.Children)
		}
	}
}

// ensureContainerChildren 确保容器节点（type 为空）的 Children 不为 nil
// YAML 反序列化会将 [] 序列化为 null，导致前端无法判断为容器节点
func ensureContainerChildren(model *IEC61850ModelConfig) {
	for i := range model.LogicalDevices {
		for j := range model.LogicalDevices[i].LogicalNodes {
			for k := range model.LogicalDevices[i].LogicalNodes[j].DataObjects {
				ensureDAChildren(&model.LogicalDevices[i].LogicalNodes[j].DataObjects[k].DataAttributes)
			}
		}
	}
}

// ensureDAChildren 递归确保容器节点的 Children 不为 nil
func ensureDAChildren(attrs *[]DataAttributeConfig) {
	for i := range *attrs {
		attr := &(*attrs)[i]
		if attr.Type == "" && attr.Children == nil {
			attr.Children = []DataAttributeConfig{}
		}
		if len(attr.Children) > 0 {
			ensureDAChildren(&attr.Children)
		}
	}
}

// SaveModbusToIEC61850 保存 Modbus → IEC 61850 配置文件
func SaveModbusToIEC61850(path string, cfg *ModbusToIEC61850Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// setIEC61850Defaults 设置 IEC 61850 配置默认值
func setIEC61850Defaults(cfg *ModbusToIEC61850Config) {
	if cfg.IEC61850.Port == 0 {
		cfg.IEC61850.Port = 102
	}
	if cfg.IEC61850.IEDName == "" {
		cfg.IEC61850.IEDName = "GRID_GATEWAY"
	}
	if cfg.IEC61850.MaxConnections == 0 {
		cfg.IEC61850.MaxConnections = 10
	}
	for i := range cfg.Mappings {
		if cfg.Mappings[i].Scale == 0 {
			cfg.Mappings[i].Scale = 1.0
		}
	}
}

// validateModelPaths 校验 mappings 中的 iec61850_path 是否在 model 定义中存在
func validateModelPaths(cfg *ModbusToIEC61850Config) error {
	if len(cfg.Mappings) == 0 {
		return nil
	}

	validPaths := collectAllPaths(&cfg.Model)
	pathSet := make(map[string]bool, len(validPaths))
	for _, p := range validPaths {
		pathSet[p] = true
	}

	var invalidPaths []string
	for _, rule := range cfg.Mappings {
		if rule.IEC61850Path != "" && !pathSet[rule.IEC61850Path] {
			invalidPaths = append(invalidPaths, rule.IEC61850Path)
		}
	}

	if len(invalidPaths) > 0 {
		return fmt.Errorf("映射规则中存在无效的 IEC 61850 路径: %s", strings.Join(invalidPaths, ", "))
	}

	return nil
}

// collectAllPaths 递归遍历 model 定义，收集所有有效的叶属性对象引用路径
func collectAllPaths(model *IEC61850ModelConfig) []string {
	var paths []string
	for _, ld := range model.LogicalDevices {
		for _, ln := range ld.LogicalNodes {
			for _, do := range ln.DataObjects {
				collectDAPaths(ld.Name, ln.Name, do.Name, do.DataAttributes, &paths)
			}
		}
	}
	return paths
}

// collectDAPaths 递归收集数据属性路径
func collectDAPaths(ldName, lnName, doPath string, attrs []DataAttributeConfig, paths *[]string) {
	for _, da := range attrs {
		fullPath := ldName + "/" + lnName + "." + doPath + "." + da.Name
		if len(da.Children) > 0 {
			// 有子属性，继续递归 (如 mag -> f)
			collectDAPaths(ldName, lnName, doPath+"."+da.Name, da.Children, paths)
		} else {
			// 叶节点，记录路径
			*paths = append(*paths, fullPath)
		}
	}
}

// GenerateDefaultIEC61850Config 生成默认的（disabled）配置文件
func GenerateDefaultIEC61850Config(path string) error {
	cfg := &ModbusToIEC61850Config{
		IEC61850: IEC61850Config{
			Enabled:        false,
			Port:           102,
			IEDName:        "GRID_GATEWAY",
			MaxConnections: 10,
			ICDOutput:      "./configs/GRID_GATEWAY.icd",
		},
		Model: IEC61850ModelConfig{
			LogicalDevices: []LogicalDeviceConfig{
				{
					Name: "GRID_GATEWAY",
					LogicalNodes: []LogicalNodeConfig{
						{
							Name: "MMXU1",
							DataObjects: []DataObjectConfig{
								{
									Name: "TotW",
									DataAttributes: []DataAttributeConfig{
										{
											Name: "mag",
											Children: []DataAttributeConfig{
												{
													Name:     "f",
													Type:     "FLOAT32",
													FC:       "MX",
													Triggers: "DATA_CHANGED",
												},
											},
										},
									},
								},
								{
									Name: "Mod",
									DataAttributes: []DataAttributeConfig{
										{
											Name:     "stVal",
											Type:     "INT32",
											FC:       "ST",
											Triggers: "DATA_CHANGED",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Mappings: []IEC61850MappingRule{
			{
				SourceDevice:   "rtu-device-1",
				SourceName:     "temperature",
				SourceRegister: 0,
				SourceType:     "holding",
				DataType:       "float32",
				IEC61850Path:   "GRID_GATEWAY/MMXU1.TotW.mag.f",
				Scale:          1.0,
				Offset:         0.0,
			},
			{
				SourceDevice:   "rtu-device-1",
				SourceName:     "mode",
				SourceRegister: 2,
				SourceType:     "holding",
				DataType:       "int32",
				IEC61850Path:   "GRID_GATEWAY/MMXU1.Mod.stVal",
				Scale:          1.0,
				Offset:         0.0,
			},
		},
	}

	return SaveModbusToIEC61850(path, cfg)
}
