// Package iec61850 封装 libiec61850 C 库，提供 IEC 61850 MMS Server 功能。
// 使用动态模型 API 在运行时构建数据模型，避免静态模型的编译依赖。
package iec61850

/*
#include "wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// DataType 数据属性类型
type DataType int

const (
	TypeBoolean         DataType = C.IEC61850_WRAPPER_BOOLEAN
	TypeInt8            DataType = C.IEC61850_WRAPPER_INT8
	TypeInt16           DataType = C.IEC61850_WRAPPER_INT16
	TypeInt32           DataType = C.IEC61850_WRAPPER_INT32
	TypeInt64           DataType = C.IEC61850_WRAPPER_INT64
	TypeUint8           DataType = C.IEC61850_WRAPPER_INT8U
	TypeUint16          DataType = C.IEC61850_WRAPPER_INT16U
	TypeUint32          DataType = C.IEC61850_WRAPPER_INT32U
	TypeFloat32         DataType = C.IEC61850_WRAPPER_FLOAT32
	TypeFloat64         DataType = C.IEC61850_WRAPPER_FLOAT64
	TypeVisibleString255 DataType = C.IEC61850_WRAPPER_VISIBLE_STRING_255
	TypeQuality         DataType = 23 // IEC61850_QUALITY - 质量码 (BITSTRING)
	TypeTimestamp       DataType = 22 // IEC61850_TIMESTAMP - 时间戳
)

// FC 功能约束
type FC int

const (
	FCStatus     FC = C.IEC61850_WRAPPER_FC_ST     // 状态
	FCMeasurand  FC = C.IEC61850_WRAPPER_FC_MX     // 测量值
	FCSetpoint   FC = C.IEC61850_WRAPPER_FC_SP     // 设定值
	FCConfig     FC = C.IEC61850_WRAPPER_FC_CF     // 配置
	FCDescription FC = C.IEC61850_WRAPPER_FC_DC    // 描述
)

// TriggerOptions 触发选项
type TriggerOptions uint8

const (
	TriggerDataChanged    TriggerOptions = C.IEC61850_WRAPPER_TRG_OPT_DATA_CHANGED
	TriggerQualityChanged TriggerOptions = C.IEC61850_WRAPPER_TRG_OPT_QUALITY_CHANGED
	TriggerDataUpdate     TriggerOptions = C.IEC61850_WRAPPER_TRG_OPT_DATA_UPDATE
	TriggerIntegrity      TriggerOptions = C.IEC61850_WRAPPER_TRG_OPT_INTEGRITY
	TriggerGI             TriggerOptions = C.IEC61850_WRAPPER_TRG_OPT_GI
)

// ModelNode 模型节点 (包装 C 不透明指针)
type ModelNode struct {
	handle unsafe.Pointer
}

// DataAttribute 数据属性节点
type DataAttribute struct {
	ModelNode
}

// DataObject 数据对象节点
type DataObject struct {
	ModelNode
}

// LogicalNode 逻辑节点
type LogicalNode struct {
	ModelNode
}

// LogicalDevice 逻辑设备
type LogicalDevice struct {
	ModelNode
}

// Model IED 数据模型
type Model struct {
	handle   unsafe.Pointer
	devices  map[string]*LogicalDevice
	nodes    map[string]*DataAttribute // 完整对象引用 -> DataAttribute
	mu       sync.RWMutex
}

// newModel 创建模型包装器
func newModel(handle unsafe.Pointer) *Model {
	return &Model{
		handle:  handle,
		devices: make(map[string]*LogicalDevice),
		nodes:   make(map[string]*DataAttribute),
	}
}

// CreateModel 创建空的 IED 数据模型
func CreateModel(name string) *Model {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.iec61850_wrapper_model_create(cName)
	if handle == nil {
		return nil
	}
	return newModel(handle)
}

// Destroy 销毁动态创建的模型
func (m *Model) Destroy() {
	if m == nil || m.handle == nil {
		return
	}
	C.iec61850_wrapper_model_destroy(m.handle)
	m.handle = nil
}

// AddLogicalDevice 添加逻辑设备
func (m *Model) AddLogicalDevice(name string) *LogicalDevice {
	if m == nil || m.handle == nil {
		return nil
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.iec61850_wrapper_ld_create(cName, m.handle)
	if handle == nil {
		return nil
	}

	ld := &LogicalDevice{ModelNode{handle: handle}}
	m.mu.Lock()
	m.devices[name] = ld
	m.mu.Unlock()
	return ld
}

// AddLogicalNode 添加逻辑节点
func (ld *LogicalDevice) AddLogicalNode(name string) *LogicalNode {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.iec61850_wrapper_ln_create(cName, ld.handle)
	if handle == nil {
		return nil
	}
	return &LogicalNode{ModelNode{handle: handle}}
}

// AddDataObject 添加数据对象
func (ln *LogicalNode) AddDataObject(name string) *DataObject {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.iec61850_wrapper_do_create(cName, ln.handle)
	if handle == nil {
		return nil
	}
	return &DataObject{ModelNode{handle: handle}}
}

// AddDataObject 添加子数据对象
func (do *DataObject) AddDataObject(name string) *DataObject {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.iec61850_wrapper_do_create(cName, do.handle)
	if handle == nil {
		return nil
	}
	return &DataObject{ModelNode{handle: handle}}
}

// AddDataAttribute 添加数据属性
func (do *DataObject) AddDataAttribute(name string, dataType DataType, fc FC, triggers TriggerOptions) *DataAttribute {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.iec61850_wrapper_da_create(cName, do.handle,
		C.int(dataType), C.int(fc), C.uint8_t(triggers))
	if handle == nil {
		return nil
	}
	return &DataAttribute{ModelNode{handle: handle}}
}

// RegisterDA 注册数据属性到模型索引 (用于后续按路径查找)
func (m *Model) RegisterDA(objectRef string, da *DataAttribute) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.nodes[objectRef] = da
	m.mu.Unlock()
}

// FindDA 按对象引用查找数据属性 (如 "LD1/MMXU1.TotW.mag.f")
func (m *Model) FindDA(objectRef string) *DataAttribute {
	if m == nil || m.handle == nil {
		return nil
	}
	// 先从缓存查找
	m.mu.RLock()
	if da, ok := m.nodes[objectRef]; ok {
		m.mu.RUnlock()
		return da
	}
	m.mu.RUnlock()

	// 通过 C API 查找
	cRef := C.CString(objectRef)
	defer C.free(unsafe.Pointer(cRef))

	handle := C.iec61850_wrapper_find_da(m.handle, cRef)
	if handle == nil {
		return nil
	}

	da := &DataAttribute{ModelNode{handle: handle}}
	m.mu.Lock()
	m.nodes[objectRef] = da
	m.mu.Unlock()
	return da
}

// FindDO 按对象引用查找数据对象
func (m *Model) FindDO(objectRef string) *DataObject {
	if m == nil || m.handle == nil {
		return nil
	}
	cRef := C.CString(objectRef)
	defer C.free(unsafe.Pointer(cRef))

	handle := C.iec61850_wrapper_find_do(m.handle, cRef)
	if handle == nil {
		return nil
	}
	return &DataObject{ModelNode{handle: handle}}
}

// ServerConfig 服务器配置
type ServerConfig struct {
	TCPPort          int // MMS 监听端口，默认 102
	MaxConnections   int // 最大连接数，0 表示默认
	ReportBufferSize int // 报告缓冲区大小，0 表示默认
}

// Server IEC 61850 MMS 服务器
type Server struct {
	handle unsafe.Pointer
	model  *Model
	config ServerConfig
}

// NewServer 创建 IEC 61850 服务器
func NewServer(model *Model, cfg ServerConfig) *Server {
	if model == nil || model.handle == nil {
		return nil
	}

	// 设置默认端口
	if cfg.TCPPort == 0 {
		cfg.TCPPort = 102
	}

	var handle unsafe.Pointer
	if cfg.MaxConnections > 0 || cfg.ReportBufferSize > 0 {
		handle = C.iec61850_wrapper_server_create_with_config(
			model.handle,
			C.int(cfg.MaxConnections),
			C.int(cfg.ReportBufferSize),
		)
	} else {
		handle = C.iec61850_wrapper_server_create(model.handle)
	}

	if handle == nil {
		return nil
	}

	return &Server{
		handle: handle,
		model:  model,
		config: cfg,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	if s == nil || s.handle == nil {
		return fmt.Errorf("服务器句柄为空")
	}

	ret := C.iec61850_wrapper_server_start(s.handle, C.int(s.config.TCPPort))
	if ret == 0 {
		return fmt.Errorf("启动 IEC 61850 服务器失败，端口 %d", s.config.TCPPort)
	}
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() {
	if s == nil || s.handle == nil {
		return
	}
	C.iec61850_wrapper_server_stop(s.handle)
}

// Destroy 销毁服务器，释放所有资源
func (s *Server) Destroy() {
	if s == nil || s.handle == nil {
		return
	}
	C.iec61850_wrapper_server_destroy(s.handle)
	s.handle = nil
}

// IsRunning 检查服务器是否正在运行
func (s *Server) IsRunning() bool {
	if s == nil || s.handle == nil {
		return false
	}
	return bool(C.iec61850_wrapper_server_is_running(s.handle))
}

// EnableReadLog 启用读取访问日志 (客户端读取数据时打印日志到控制台)
func (s *Server) EnableReadLog() {
	if s == nil || s.handle == nil {
		return
	}
	C.iec61850_wrapper_enable_read_log(s.handle)
}

// GetConnectionCount 获取当前连接数
func (s *Server) GetConnectionCount() int {
	if s == nil || s.handle == nil {
		return 0
	}
	return int(C.iec61850_wrapper_server_get_connection_count(s.handle))
}

// GetModel 获取关联的数据模型
func (s *Server) GetModel() *Model {
	if s == nil {
		return nil
	}
	return s.model
}

// LockDataModel 锁定数据模型 (批量更新前调用)
func (s *Server) LockDataModel() {
	if s == nil || s.handle == nil {
		return
	}
	C.iec61850_wrapper_lock_data_model(s.handle)
}

// UnlockDataModel 解锁数据模型 (批量更新后调用)
func (s *Server) UnlockDataModel() {
	if s == nil || s.handle == nil {
		return
	}
	C.iec61850_wrapper_unlock_data_model(s.handle)
}

// UpdateFloat 更新浮点值
func (s *Server) UpdateFloat(da *DataAttribute, value float32) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_float(s.handle, da.handle, C.float(value))
}

// UpdateInt32 更新 int32 值
func (s *Server) UpdateInt32(da *DataAttribute, value int32) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_int32(s.handle, da.handle, C.int32_t(value))
}

// UpdateUint32 更新 uint32 值
func (s *Server) UpdateUint32(da *DataAttribute, value uint32) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_uint32(s.handle, da.handle, C.uint32_t(value))
}

// UpdateBool 更新布尔值
func (s *Server) UpdateBool(da *DataAttribute, value bool) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_bool(s.handle, da.handle, C.bool(value))
}

// UpdateString 更新可见字符串值
func (s *Server) UpdateString(da *DataAttribute, value string) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	C.iec61850_wrapper_update_string(s.handle, da.handle, cValue)
}

// UpdateInt64 更新 int64 值
func (s *Server) UpdateInt64(da *DataAttribute, value int64) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_int64(s.handle, da.handle, C.int64_t(value))
}

// UpdateTimestamp 更新时标 (毫秒时间戳)
func (s *Server) UpdateTimestamp(da *DataAttribute, msTimestamp int64) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_timestamp(s.handle, da.handle, C.int64_t(msTimestamp))
}

// UpdateQuality 更新质量码
func (s *Server) UpdateQuality(da *DataAttribute, quality uint16) {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return
	}
	C.iec61850_wrapper_update_quality(s.handle, da.handle, C.uint16_t(quality))
}

// GetFloat 获取浮点值
func (s *Server) GetFloat(da *DataAttribute) float32 {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return 0.0
	}
	return float32(C.iec61850_wrapper_get_float(s.handle, da.handle))
}

// GetInt32 获取 int32 值
func (s *Server) GetInt32(da *DataAttribute) int32 {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return 0
	}
	return int32(C.iec61850_wrapper_get_int32(s.handle, da.handle))
}

// GetBool 获取布尔值
func (s *Server) GetBool(da *DataAttribute) bool {
	if s == nil || s.handle == nil || da == nil || da.handle == nil {
		return false
	}
	return bool(C.iec61850_wrapper_get_bool(s.handle, da.handle))
}

// UpdateFloatByPath 按对象引用更新浮点值
func (s *Server) UpdateFloatByPath(objectRef string, value float32) error {
	if s == nil || s.model == nil {
		return fmt.Errorf("服务器未初始化")
	}
	da := s.model.FindDA(objectRef)
	if da == nil {
		return fmt.Errorf("未找到数据属性: %s", objectRef)
	}
	s.UpdateFloat(da, value)
	return nil
}

// UpdateInt32ByPath 按对象引用更新 int32 值
func (s *Server) UpdateInt32ByPath(objectRef string, value int32) error {
	if s == nil || s.model == nil {
		return fmt.Errorf("服务器未初始化")
	}
	da := s.model.FindDA(objectRef)
	if da == nil {
		return fmt.Errorf("未找到数据属性: %s", objectRef)
	}
	s.UpdateInt32(da, value)
	return nil
}

// UpdateBoolByPath 按对象引用更新布尔值
func (s *Server) UpdateBoolByPath(objectRef string, value bool) error {
	if s == nil || s.model == nil {
		return fmt.Errorf("服务器未初始化")
	}
	da := s.model.FindDA(objectRef)
	if da == nil {
		return fmt.Errorf("未找到数据属性: %s", objectRef)
	}
	s.UpdateBool(da, value)
	return nil
}

// UpdateStringByPath 按对象引用更新字符串值
func (s *Server) UpdateStringByPath(objectRef string, value string) error {
	if s == nil || s.model == nil {
		return fmt.Errorf("服务器未初始化")
	}
	da := s.model.FindDA(objectRef)
	if da == nil {
		return fmt.Errorf("未找到数据属性: %s", objectRef)
	}
	s.UpdateString(da, value)
	return nil
}

// UpdateInt64ByPath 按对象引用更新 int64 值 (用于时标)
func (s *Server) UpdateInt64ByPath(objectRef string, value int64) error {
	if s == nil || s.model == nil {
		return fmt.Errorf("服务器未初始化")
	}
	da := s.model.FindDA(objectRef)
	if da == nil {
		// 时标属性可能不存在，静默返回
		return nil
	}
	s.UpdateInt64(da, value)
	return nil
}
