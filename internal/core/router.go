package core

import (
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus/rtu"
	"github.com/lyx6662/com-manager/lib-modbus/tcp"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// RegisterWriter 寄存器写入接口 (TCP Server 和 RTU Server 都实现此接口)
type RegisterWriter interface {
	UpdateRegisters(startAddr uint16, values []uint16)
	UpdateCoils(startAddr uint16, values []bool)
}

// IEC61850Writer IEC 61850 数据写入接口
type IEC61850Writer interface {
	UpdateData(path string, value interface{}, quality uint16, timestamp int64) error
	IsRunning() bool
}

// DeviceStatusProvider 设备状态提供者接口
type DeviceStatusProvider interface {
	GetDeviceStatus(deviceID string) interface{}
	IsDeviceOnline(deviceID string) bool
}

// Router 数据路由器 — 将采集数据按映射规则写入输出服务器
type Router struct {
	log          *logger.Logger
	mu           sync.RWMutex
	mappings     map[string][]config.MappingRule // deviceID -> 映射规则
	groupDevices map[string][]string            // groupID -> 设备ID列表
	servers      map[string]*tcp.Server         // groupID -> TCP Server
	rtuServers   map[string]*rtu.Server         // groupID -> RTU Server
	dataCache    map[string]map[string]model.DataPoint // deviceID -> pointName -> DataPoint
	iec61850Mgr  IEC61850Writer                 // IEC 61850 管理器
	iec61850Rules []config.IEC61850MappingRule  // IEC 61850 映射规则
	deviceStatus DeviceStatusProvider           // 设备状态提供者
}

// NewRouter 创建路由器
func NewRouter(log *logger.Logger) *Router {
	return &Router{
		log:        log,
		mappings:   make(map[string][]config.MappingRule),
		servers:    make(map[string]*tcp.Server),
		rtuServers: make(map[string]*rtu.Server),
		dataCache:  make(map[string]map[string]model.DataPoint),
	}
}

// SetMappings 设置映射配置
func (r *Router) SetMappings(mappings map[string][]config.MappingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mappings = mappings
}

// SetGroupDevices 设置分组设备配置
func (r *Router) SetGroupDevices(groupDevices map[string][]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groupDevices = groupDevices
}

// SetIEC61850Manager 设置 IEC 61850 管理器
func (r *Router) SetIEC61850Manager(mgr IEC61850Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iec61850Mgr = mgr
}

// SetDeviceStatusProvider 设置设备状态提供者
func (r *Router) SetDeviceStatusProvider(provider DeviceStatusProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deviceStatus = provider
}

// SetIEC61850Mappings 设置 IEC 61850 映射规则
func (r *Router) SetIEC61850Mappings(rules []config.IEC61850MappingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.iec61850Rules = rules
}

// findDeviceGroup 查找设备所属的分组ID
func (r *Router) findDeviceGroup(deviceID string) string {
	for groupID, devices := range r.groupDevices {
		for _, devID := range devices {
			if devID == deviceID {
				return groupID
			}
		}
	}
	return ""
}

// RegisterServer 注册输出服务器
func (r *Router) RegisterServer(groupID string, srv *tcp.Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers[groupID] = srv
}

// RegisterRTUServer 注册RTU输出服务器
func (r *Router) RegisterRTUServer(groupID string, srv *rtu.Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rtuServers[groupID] = srv
}

// ClearServers 清空所有服务器注册（热重载前调用，防止残留旧引用）
func (r *Router) ClearServers() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = make(map[string]*tcp.Server)
	r.rtuServers = make(map[string]*rtu.Server)
}

// findTCPServer 按 groupID 查找 TCP 服务器
func (r *Router) findTCPServer(groupID string) (*tcp.Server, bool) {
	srv, ok := r.servers[groupID]
	return srv, ok
}

// findRTUServer 按 groupID 查找 RTU 服务器
func (r *Router) findRTUServer(groupID string) (*rtu.Server, bool) {
	srv, ok := r.rtuServers[groupID]
	return srv, ok
}

// UpdateData 更新设备数据缓存并刷新输出寄存器
func (r *Router) UpdateData(deviceID string, points []model.DataPoint) {
	r.mu.Lock()
	// 更新缓存
	if r.dataCache[deviceID] == nil {
		r.dataCache[deviceID] = make(map[string]model.DataPoint)
	}
	for _, pt := range points {
		r.dataCache[deviceID][pt.Name] = pt
	}
	r.mu.Unlock()


	// 处理批量数据点 (带Extra字段的目标地址)
	r.handleBatchPoints(deviceID, points)

	// 刷新所有引用了该设备的输出组
	r.refreshOutputs(deviceID)
}

// UpdateDataCache 仅更新数据缓存，不触发输出（用于新架构，输出由适配器处理）
func (r *Router) UpdateDataCache(deviceID string, points []model.DataPoint) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.dataCache[deviceID] == nil {
		r.dataCache[deviceID] = make(map[string]model.DataPoint)
	}
	for _, pt := range points {
		r.dataCache[deviceID][pt.Name] = pt
	}
}

// handleBatchPoints 处理批量数据点 (用于int32_dcba_batch和raw_batch类型)
func (r *Router) handleBatchPoints(deviceID string, points []model.DataPoint) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 查找设备所属分组
	groupID := r.findDeviceGroup(deviceID)
	if groupID == "" {
		return
	}

	for _, pt := range points {
		// 检查是否有目标寄存器地址
		if pt.Extra == nil {
			continue
		}
		targetReg, ok := pt.Extra["target_register"]
		if !ok {
			continue
		}
		targetAddr, ok := targetReg.(uint16)
		if !ok {
			r.log.Debug("target_register类型断言失败", "name", pt.Name, "type", fmt.Sprintf("%T", targetReg))
			continue
		}

		r.log.Debug("处理批量数据点", "name", pt.Name, "target", targetAddr, "value", pt.Value)

		// 获取寄存器值
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
			// 写入TCP服务器
			if srv, exists := r.findTCPServer(groupID); exists {
				srv.UpdateRegisters(targetAddr, regValues)
			}
			// 写入RTU服务器
			if srv, exists := r.findRTUServer(groupID); exists {
				srv.UpdateRegisters(targetAddr, regValues)
			}
			r.log.Debug("写入寄存器", "group", groupID, "addr", targetAddr, "values", regValues)
		}
	}
}

// refreshOutputs 刷新指定设备的输出
func (r *Router) refreshOutputs(deviceID string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 转发数据到 IEC 61850 服务器 (独立于 Modbus 映射和输出)
	if r.iec61850Mgr != nil && r.iec61850Mgr.IsRunning() {
		r.log.Debug("IEC 61850 转发检查",
			"device_id", deviceID,
			"rules_count", len(r.iec61850Rules),
		)

		// 获取设备在线状态，决定品质码
		var quality uint16 = 0 // 0 = Good
		if r.deviceStatus != nil && !r.deviceStatus.IsDeviceOnline(deviceID) {
			quality = 0x80 // 品质码: 0x80 = Bad (设备离线)
		}

		// 获取当前时间戳 (毫秒)
		now := time.Now().UnixMilli()

		// 获取设备数据缓存
		deviceCache, hasCache := r.dataCache[deviceID]

		for _, iecRule := range r.iec61850Rules {
			if iecRule.SourceDevice != deviceID {
				continue
			}

			// 通过 SourceName 直接从数据缓存查找数据点，不依赖 Modbus 映射规则
			var pt model.DataPoint
			var exists bool
			if hasCache && iecRule.SourceName != "" {
				pt, exists = deviceCache[iecRule.SourceName]
			}
			if !exists || pt.Quality != model.QualityGood {
				r.log.Debug("IEC 61850 数据跳过",
					"source_name", iecRule.SourceName,
					"exists", exists,
				)
				continue
			}
			scaledValue := r.applyIEC61850Scale(iecRule, pt.Value)
			if scaledValue != nil {
				r.log.Info("IEC 61850 转发数据",
					"path", iecRule.IEC61850Path,
					"value", scaledValue,
					"quality", quality,
					"source_value", pt.Value,
				)
				r.iec61850Mgr.UpdateData(iecRule.IEC61850Path, scaledValue, quality, now)
			}
		}
	} else {
		r.log.Debug("IEC 61850 转发跳过",
			"mgr_nil", r.iec61850Mgr == nil,
			"running", r.iec61850Mgr != nil && r.iec61850Mgr.IsRunning(),
		)
	}

	// 获取设备的 Modbus 映射规则
	rules, hasMappings := r.mappings[deviceID]
	if !hasMappings || len(rules) == 0 {
		return
	}

	// 查找设备所属分组
	groupID := r.findDeviceGroup(deviceID)
	if groupID == "" {
		return
	}

	// 检查是否有对应的输出服务器 (TCP 或 RTU)
	tcpSrv, hasTCP := r.findTCPServer(groupID)
	rtuSrv, hasRTU := r.findRTUServer(groupID)

	// 如果没有 Modbus 输出服务器，直接返回
	if !hasTCP && !hasRTU {
		return
	}

	for _, rule := range rules {
		pt, exists := r.dataCache[deviceID][rule.Name]

		// 线圈类型走独立输出路径
		if rule.SourceType == "coil" {
			var coilVal bool
			if exists && pt.Quality == model.QualityGood {
				if v, ok := pt.Value.(bool); ok {
					coilVal = v
				}
			}
			if hasTCP {
				tcpSrv.UpdateCoils(rule.TargetRegister, []bool{coilVal})
			}
			if hasRTU {
				rtuSrv.UpdateCoils(rule.TargetRegister, []bool{coilVal})
			}
			continue
		}

		if !exists || pt.Quality != model.QualityGood {
			// 数据不存在或质量差，清零输出寄存器
			regCount := rule.GetRegisterCount()
			zeroValues := make([]uint16, regCount)
			if hasTCP {
				tcpSrv.UpdateRegisters(rule.TargetRegister, zeroValues)
			}
			if hasRTU {
				rtuSrv.UpdateRegisters(rule.TargetRegister, zeroValues)
			}
			continue
		}

		// 执行映射转换
		regValues := r.applyMapping(rule, pt)
		if len(regValues) > 0 {
			if hasTCP {
				tcpSrv.UpdateRegisters(rule.TargetRegister, regValues)
			}
			if hasRTU {
				rtuSrv.UpdateRegisters(rule.TargetRegister, regValues)
			}
		}
	}
}

// applyMapping 应用映射规则，将数据点转换为寄存器值
func (r *Router) applyMapping(rule config.MappingRule, pt model.DataPoint) []uint16 {
	// 检查是否是直接透传的寄存器数据
	if pt.DataType == "uint16_pair" || pt.DataType == "uint16" {
		if v, ok := pt.Value.([]uint16); ok {
			return v
		}
		// 单个uint16值
		if v, ok := pt.Value.(uint16); ok {
			return []uint16{v}
		}
		return nil
	}

	// 获取原始数值
	rawValue := r.toFloat64(pt.Value)
	if rawValue == nil {
		return nil
	}

	// 应用缩放和偏移
	converted := *rawValue*rule.Scale + rule.Offset

	// 按数据类型编码为寄存器值
	switch rule.DataType {
	case "float32":
		bits := math.Float32bits(float32(converted))
		return encodeUint32WithByteOrder(bits, rule.ByteOrder)
	case "int32", "uint32":
		v := uint32(int32(converted))
		return encodeUint32WithByteOrder(v, rule.ByteOrder)
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

// encodeUint32WithByteOrder 按指定字节序将uint32编码为2个寄存器值
// ABCD: 大端 (默认), BADC: 字交换, CDAB: 字节交换, DCBA: 小端
func encodeUint32WithByteOrder(val uint32, byteOrder string) []uint16 {
	switch byteOrder {
	case "BADC":
		// 字交换: 低16位在前，高16位在后，各自大端
		return []uint16{uint16(val & 0xFFFF), uint16(val >> 16)}
	case "CDAB":
		// 字节交换: 保持寄存器顺序，每个寄存器内字节反转
		reg0 := uint16(((val >> 16) & 0xFF00) | ((val >> 24) & 0xFF))
		reg1 := uint16(((val & 0xFF) << 8) | ((val >> 8) & 0xFF))
		return []uint16{reg0, reg1}
	case "DCBA":
		// 小端: 寄存器交换 + 每个寄存器内字节反转
		reg0 := uint16(((val & 0xFF) << 8) | ((val >> 8) & 0xFF))
		reg1 := uint16(((val >> 16) & 0xFF) << 8 | ((val >> 24) & 0xFF))
		return []uint16{reg0, reg1}
	default:
		// ABCD 大端 (默认)
		return []uint16{uint16(val >> 16), uint16(val & 0xFFFF)}
	}
}

// toFloat64 将各种类型的值转为 float64
func (r *Router) toFloat64(val interface{}) *float64 {
	switch v := val.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int16:
		f := float64(v)
		return &f
	case uint16:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	case uint32:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case uint64:
		f := float64(v)
		return &f
	case bool:
		if v {
			f := 1.0
			return &f
		}
		f := 0.0
		return &f
	default:
		return nil
	}
}

// applyIEC61850Scale 对 IEC 61850 映射值应用缩放和偏移
func (r *Router) applyIEC61850Scale(rule config.IEC61850MappingRule, value interface{}) interface{} {
	raw := r.toFloat64(value)
	if raw == nil {
		// 非数值类型 (如 bool/string) 直接透传
		return value
	}
	scaled := *raw*rule.Scale + rule.Offset
	// 根据 IEC61850 目标类型返回对应类型
	targetType := rule.TargetType
	if targetType == "" {
		targetType = "float32" // 默认 float32
	}
	switch targetType {
	case "float32":
		return float32(scaled)
	case "int32":
		return int32(scaled)
	case "int16":
		return int16(scaled)
	case "uint16":
		return uint16(scaled)
	case "bool":
		return scaled != 0
	default:
		return float32(scaled)
	}
}

// GetDeviceStatus 获取所有设备的缓存数据 (供监控API使用)
func (r *Router) GetDeviceStatus() map[string]map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]map[string]interface{})
	for devID, points := range r.dataCache {
		result[devID] = make(map[string]interface{})
		for name, pt := range points {
			result[devID][name] = pt
		}
	}
	return result
}

// EncodeDataPointValue 将数据点值编码为字节 (用于离线缓冲存储)
func EncodeDataPointValue(pt model.DataPoint) []byte {
	switch v := pt.Value.(type) {
	case float32:
		bits := math.Float32bits(v)
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, bits)
		return buf
	case float64:
		bits := math.Float64bits(v)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, bits)
		return buf
	case int16:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(v))
		return buf
	case uint16:
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, v)
		return buf
	case int32:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(v))
		return buf
	case uint32:
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, v)
		return buf
	case bool:
		if v {
			return []byte{1}
		}
		return []byte{0}
	default:
		return nil
	}
}
