package core

import (
	"math"
	"sync"

	"github.com/lyx6662/com-manager/lib-modbus/rtu"
	"github.com/lyx6662/com-manager/lib-modbus/tcp"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// ModbusOutputMapping Modbus 输出映射配置
type ModbusOutputMapping struct {
	SourceDevice   string  `yaml:"source_device" json:"source_device"`
	SourceName     string  `yaml:"source_name" json:"source_name"`
	SourceType     string  `yaml:"source_type" json:"source_type"`         // holding/input/coil
	DataType       string  `yaml:"data_type" json:"data_type"`
	TargetRegister uint16  `yaml:"target_register" json:"target_register"`
	Scale          float64 `yaml:"scale" json:"scale"`
	Offset         float64 `yaml:"offset" json:"offset"`
	ByteOrder      string  `yaml:"byte_order" json:"byte_order"`
	MaxPoints      int     `yaml:"max_points" json:"max_points"`
}

// ModbusOutputAdapter Modbus TCP/RTU 输出适配器
type ModbusOutputAdapter struct {
	mu         sync.RWMutex
	log        *logger.Logger
	pool       *DataPool
	name       string
	running    bool
	tcpServer  *tcp.Server
	rtuServer  *rtu.Server
	mappings   []ModbusOutputMapping
	groupID    string
}

// NewModbusOutputAdapter 创建 Modbus 输出适配器
func NewModbusOutputAdapter(log *logger.Logger, name string, groupID string) *ModbusOutputAdapter {
	return &ModbusOutputAdapter{
		log:     log,
		name:    name,
		groupID: groupID,
	}
}

// Name 适配器名称
func (a *ModbusOutputAdapter) Name() string {
	return a.name
}

// Init 初始化适配器
func (a *ModbusOutputAdapter) Init(pool *DataPool) error {
	a.pool = pool
	return nil
}

// Start 启动输出
func (a *ModbusOutputAdapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pool != nil {
		// 订阅所有数据点
		a.pool.SubscribeAll(a)
	}

	a.running = true
	a.log.Info("Modbus 输出适配器已启动", "name", a.name, "group", a.groupID)
	return nil
}

// Stop 停止输出
func (a *ModbusOutputAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pool != nil {
		a.pool.Unsubscribe(a)
	}

	a.running = false
	a.log.Info("Modbus 输出适配器已停止", "name", a.name)
	return nil
}

// IsRunning 是否运行中
func (a *ModbusOutputAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// SetTCPServer 设置 TCP 服务器
func (a *ModbusOutputAdapter) SetTCPServer(srv *tcp.Server) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tcpServer = srv
}

// SetRTUServer 设置 RTU 服务器
func (a *ModbusOutputAdapter) SetRTUServer(srv *rtu.Server) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rtuServer = srv
}

// SetMappings 设置映射规则
func (a *ModbusOutputAdapter) SetMappings(mappings []ModbusOutputMapping) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mappings = mappings
}

// SetMappingsFromConfig 从旧配置格式设置映射规则
func (a *ModbusOutputAdapter) SetMappingsFromConfig(rules []config.MappingRule) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.mappings = make([]ModbusOutputMapping, 0, len(rules))
	for _, rule := range rules {
		a.mappings = append(a.mappings, ModbusOutputMapping{
			SourceDevice:   rule.SourceDevice,
			SourceName:     rule.Name,
			SourceType:     rule.SourceType,
			DataType:       rule.DataType,
			TargetRegister: rule.TargetRegister,
			Scale:          rule.Scale,
			Offset:         rule.Offset,
			ByteOrder:      rule.ByteOrder,
			MaxPoints:      rule.MaxPoints,
		})
	}
}

// SetMappingsFromOutputConfig 从新配置格式设置映射规则
func (a *ModbusOutputAdapter) SetMappingsFromOutputConfig(mappings []config.ModbusOutputMapping, dataPoints []config.UnifiedDataPoint) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 构建数据点索引
	dpIndex := make(map[string]*config.UnifiedDataPoint)
	for i := range dataPoints {
		dpIndex[dataPoints[i].ID] = &dataPoints[i]
	}

	a.mappings = make([]ModbusOutputMapping, 0, len(mappings))
	for _, mapping := range mappings {
		dp, exists := dpIndex[mapping.SourceID]
		if !exists {
			a.log.Warn("数据点不存在，跳过映射", "source_id", mapping.SourceID)
			continue
		}

		// 从数据点获取设备ID和源寄存器信息
		modbusMapping := ModbusOutputMapping{
			SourceDevice:   dp.DeviceID,
			SourceName:     dp.Name,
			SourceType:     dp.RegisterType,
			DataType:       mapping.DataType,
			TargetRegister: mapping.TargetRegister,
			Scale:          mapping.Scale,
			Offset:         mapping.Offset,
			ByteOrder:      mapping.ByteOrder,
		}

		// 如果映射中没有指定数据类型，使用数据点的类型
		if modbusMapping.DataType == "" {
			modbusMapping.DataType = dp.DataType
		}

		a.mappings = append(a.mappings, modbusMapping)
	}
}

// OnDataChanged 数据变更回调
func (a *ModbusOutputAdapter) OnDataChanged(deviceID string, pointName string, entry *DataPointEntry) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return
	}

	// 查找对应的映射规则
	for _, mapping := range a.mappings {
		if mapping.SourceDevice != deviceID || mapping.SourceName != pointName {
			continue
		}

		// 处理批量类型
		if mapping.DataType == "int32_dcba_batch" || mapping.DataType == "raw_batch" ||
			mapping.DataType == "int32_dcba_batch_passthrough" || mapping.DataType == "passthrough" {
			// 批量类型由 BatchUpdatePoints 处理
			continue
		}

		// 线圈类型
		if mapping.SourceType == "coil" {
			var coilVal bool
			if entry.Quality == model.QualityGood {
				if v, ok := entry.Value.(bool); ok {
					coilVal = v
				}
			}
			if a.tcpServer != nil {
				a.tcpServer.UpdateCoils(mapping.TargetRegister, []bool{coilVal})
			}
			if a.rtuServer != nil {
				a.rtuServer.UpdateCoils(mapping.TargetRegister, []bool{coilVal})
			}
			continue
		}

		// 普通寄存器类型
		if entry.Quality != model.QualityGood {
			// 数据质量差，清零输出寄存器
			regCount := a.getRegisterCount(mapping)
			zeroValues := make([]uint16, regCount)
			if a.tcpServer != nil {
				a.tcpServer.UpdateRegisters(mapping.TargetRegister, zeroValues)
			}
			if a.rtuServer != nil {
				a.rtuServer.UpdateRegisters(mapping.TargetRegister, zeroValues)
			}
			continue
		}

		// 执行映射转换
		regValues := a.applyMapping(mapping, entry)
		if len(regValues) > 0 {
			if a.tcpServer != nil {
				a.tcpServer.UpdateRegisters(mapping.TargetRegister, regValues)
			}
			if a.rtuServer != nil {
				a.rtuServer.UpdateRegisters(mapping.TargetRegister, regValues)
			}
		}
	}
}

// BatchUpdatePoints 批量更新数据点（用于 batch 类型）
func (a *ModbusOutputAdapter) BatchUpdatePoints(points []model.DataPoint) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return
	}

	for _, pt := range points {
		if pt.Extra == nil {
			continue
		}
		targetReg, ok := pt.Extra["target_register"]
		if !ok {
			continue
		}
		targetAddr, ok := targetReg.(uint16)
		if !ok {
			continue
		}

		var regValues []uint16

		if _, isRaw := pt.Extra["raw_value"]; isRaw {
			if v, ok := pt.Value.(uint16); ok {
				regValues = []uint16{v}
			}
		} else if _, isPassthrough := pt.Extra["raw_passthrough"]; isPassthrough {
			if v, ok := pt.Value.([]uint16); ok {
				regValues = v
			}
		} else {
			if v, ok := pt.Value.(int32); ok {
				regValues = []uint16{uint16(uint32(v) >> 16), uint16(uint32(v) & 0xFFFF)}
			}
		}

		if len(regValues) > 0 {
			if a.tcpServer != nil {
				a.tcpServer.UpdateRegisters(targetAddr, regValues)
			}
			if a.rtuServer != nil {
				a.rtuServer.UpdateRegisters(targetAddr, regValues)
			}
		}
	}
}

// applyMapping 应用映射规则，将数据点值转换为寄存器值
func (a *ModbusOutputAdapter) applyMapping(mapping ModbusOutputMapping, entry *DataPointEntry) []uint16 {
	// 检查是否是直接透传的寄存器数据
	if entry.DataType == "uint16_pair" || entry.DataType == "uint16" {
		if v, ok := entry.Value.([]uint16); ok {
			return v
		}
		if v, ok := entry.Value.(uint16); ok {
			return []uint16{v}
		}
		return nil
	}

	// 获取原始数值
	rawValue := toFloat64(entry.Value)
	if rawValue == nil {
		return nil
	}

	// 应用缩放和偏移
	converted := *rawValue*mapping.Scale + mapping.Offset

	// 按数据类型编码为寄存器值
	switch mapping.DataType {
	case "float32":
		bits := math.Float32bits(float32(converted))
		return encodeModbusUint32(bits, mapping.ByteOrder)
	case "int32", "uint32":
		v := uint32(int32(converted))
		return encodeModbusUint32(v, mapping.ByteOrder)
	case "int16":
		return []uint16{uint16(int16(converted))}
	case "uint16":
		return []uint16{uint16(converted)}
	case "bool":
		if converted != 0 {
			return []uint16{0xFF00}
		}
		return []uint16{0x0000}
	default:
		return []uint16{uint16(converted)}
	}
}

// getRegisterCount 获取映射规则占用的寄存器数量
func (a *ModbusOutputAdapter) getRegisterCount(mapping ModbusOutputMapping) int {
	switch mapping.DataType {
	case "float32", "int32", "uint32":
		return 2
	default:
		return 1
	}
}

// encodeModbusUint32 按指定字节序将 uint32 编码为 2 个寄存器值
func encodeModbusUint32(val uint32, byteOrder string) []uint16 {
	switch byteOrder {
	case "BADC":
		return []uint16{uint16(val & 0xFFFF), uint16(val >> 16)}
	case "CDAB":
		reg0 := uint16(((val >> 16) & 0xFF00) | ((val >> 24) & 0xFF))
		reg1 := uint16(((val & 0xFF) << 8) | ((val >> 8) & 0xFF))
		return []uint16{reg0, reg1}
	case "DCBA":
		reg0 := uint16(((val & 0xFF) << 8) | ((val >> 8) & 0xFF))
		reg1 := uint16(((val >> 16) & 0xFF) << 8 | ((val >> 24) & 0xFF))
		return []uint16{reg0, reg1}
	default:
		return []uint16{uint16(val >> 16), uint16(val & 0xFFFF)}
	}
}
