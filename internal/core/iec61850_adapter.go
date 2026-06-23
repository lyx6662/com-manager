package core

import (
	"sync"
	"time"

	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// IEC61850OutputAdapter IEC 61850 输出适配器
type IEC61850OutputAdapter struct {
	mu           sync.RWMutex
	log          *logger.Logger
	pool         *DataPool
	name         string
	running      bool
	iec61850Mgr  IEC61850Writer
	mappings     []config.IEC61850MappingRule
	deviceStatus DeviceStatusProvider
}

// NewIEC61850OutputAdapter 创建 IEC 61850 输出适配器
func NewIEC61850OutputAdapter(log *logger.Logger) *IEC61850OutputAdapter {
	return &IEC61850OutputAdapter{
		log:  log,
		name: "iec61850",
	}
}

// Name 适配器名称
func (a *IEC61850OutputAdapter) Name() string {
	return a.name
}

// Init 初始化适配器
func (a *IEC61850OutputAdapter) Init(pool *DataPool) error {
	a.pool = pool
	return nil
}

// Start 启动输出
func (a *IEC61850OutputAdapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pool != nil {
		a.pool.SubscribeAll(a)
	}

	a.running = true
	a.log.Info("IEC 61850 输出适配器已启动")
	return nil
}

// Stop 停止输出
func (a *IEC61850OutputAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pool != nil {
		a.pool.Unsubscribe(a)
	}

	a.running = false
	a.log.Info("IEC 61850 输出适配器已停止")
	return nil
}

// IsRunning 是否运行中
func (a *IEC61850OutputAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// SetIEC61850Manager 设置 IEC 61850 管理器
func (a *IEC61850OutputAdapter) SetIEC61850Manager(mgr IEC61850Writer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.iec61850Mgr = mgr
}

// SetMappings 设置映射规则
func (a *IEC61850OutputAdapter) SetMappings(mappings []config.IEC61850MappingRule) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mappings = mappings
}

// SetDeviceStatusProvider 设置设备状态提供者
func (a *IEC61850OutputAdapter) SetDeviceStatusProvider(provider DeviceStatusProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deviceStatus = provider
}

// OnDataChanged 数据变更回调
func (a *IEC61850OutputAdapter) OnDataChanged(deviceID string, pointName string, entry *DataPointEntry) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.running {
		return
	}

	if a.iec61850Mgr == nil || !a.iec61850Mgr.IsRunning() {
		return
	}

	// 查找对应的映射规则
	for _, rule := range a.mappings {
		if rule.SourceDevice != deviceID || rule.SourceName != pointName {
			continue
		}

		if entry.Quality != model.QualityGood {
			continue
		}

		// 获取设备在线状态，决定品质码
		var quality uint16 = 0 // 0 = Good
		if a.deviceStatus != nil && !a.deviceStatus.IsDeviceOnline(deviceID) {
			quality = 0x80 // 品质码: 0x80 = Bad (设备离线)
		}

		// 应用缩放
		scaledValue := a.applyScale(rule, entry.Value)
		if scaledValue == nil {
			continue
		}

		// 更新 IEC 61850 数据
		now := time.Now().UnixMilli()
		a.iec61850Mgr.UpdateData(rule.IEC61850Path, scaledValue, quality, now)

		a.log.Debug("IEC 61850 转发数据",
			"path", rule.IEC61850Path,
			"value", scaledValue,
			"quality", quality,
		)
	}
}

// applyScale 对映射值应用缩放和偏移
func (a *IEC61850OutputAdapter) applyScale(rule config.IEC61850MappingRule, value interface{}) interface{} {
	raw := toFloat64(value)
	if raw == nil {
		// 非数值类型（如 bool/string）直接透传
		return value
	}
	scaled := *raw*rule.Scale + rule.Offset

	targetType := rule.TargetType
	if targetType == "" {
		targetType = "float32"
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
