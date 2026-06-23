package core

import (
	"sync"

	"github.com/lyx6662/com-manager/internal/storage/alarm"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// AlarmRule 报警规则
type AlarmRule struct {
	DeviceID   string
	PointName  string
	HighLimit  float64 // 上限报警
	LowLimit   float64 // 下限报警
	Level      alarm.Level
	Enabled    bool
}

// AlarmDetector 报警检测器
type AlarmDetector struct {
	log   *logger.Logger
	store *alarm.Store
	mu    sync.RWMutex
	rules map[string][]AlarmRule // deviceID -> rules
}

// NewAlarmDetector 创建报警检测器
func NewAlarmDetector(log *logger.Logger, store *alarm.Store) *AlarmDetector {
	return &AlarmDetector{
		log:   log,
		store: store,
		rules: make(map[string][]AlarmRule),
	}
}

// LoadRulesFromConfig 从配置加载报警规则
func (d *AlarmDetector) LoadRulesFromConfig(cfg *config.Config) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 清空旧规则
	d.rules = make(map[string][]AlarmRule)

	for _, rules := range cfg.Mappings {
		for _, rule := range rules {
			// 优先使用映射规则中配置的阈值
			if rule.HighLimit != 0 || rule.LowLimit != 0 {
				d.rules[rule.SourceDevice] = append(d.rules[rule.SourceDevice], AlarmRule{
					DeviceID:  rule.SourceDevice,
					PointName: rule.Name,
					HighLimit: rule.HighLimit,
					LowLimit:  rule.LowLimit,
					Level:     alarm.LevelWarning,
					Enabled:   true,
				})
				continue
			}

			// 未配置阈值时，对数值类型自动生成默认规则
			if rule.DataType == "float32" || rule.DataType == "int16" || rule.DataType == "uint16" {
				d.rules[rule.SourceDevice] = append(d.rules[rule.SourceDevice], AlarmRule{
					DeviceID:  rule.SourceDevice,
					PointName: rule.Name,
					HighLimit: 10000,
					LowLimit:  -10000,
					Level:     alarm.LevelWarning,
					Enabled:   true,
				})
			}
		}
	}

	d.log.Info("报警规则加载完成", "devices", len(d.rules))
}

// SetRule 设置报警规则
func (d *AlarmDetector) SetRule(rule AlarmRule) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rules := d.rules[rule.DeviceID]
	for i, r := range rules {
		if r.PointName == rule.PointName {
			rules[i] = rule
			return
		}
	}
	d.rules[rule.DeviceID] = append(rules, rule)
}

// CheckDataPoints 检查数据点是否触发报警
func (d *AlarmDetector) CheckDataPoints(deviceID string, points []model.DataPoint) {
	d.mu.RLock()
	rules := d.rules[deviceID]
	d.mu.RUnlock()

	if len(rules) == 0 {
		return
	}

	for _, pt := range points {
		if pt.Quality != model.QualityGood {
			continue
		}

		value := toFloat64Value(pt.Value)
		if value == nil {
			continue
		}

		for _, rule := range rules {
			if rule.PointName != pt.Name || !rule.Enabled {
				continue
			}

			// 检查上限
			if rule.HighLimit != 0 && *value > rule.HighLimit {
				d.triggerAlarm(alarm.Record{
					DeviceID:  deviceID,
					PointName: pt.Name,
					Level:     rule.Level,
					Message:   pt.Name + " 超上限报警",
					Value:     *value,
					Threshold: rule.HighLimit,
					Timestamp: pt.Timestamp,
				})
			}

			// 检查下限
			if rule.LowLimit != 0 && *value < rule.LowLimit {
				d.triggerAlarm(alarm.Record{
					DeviceID:  deviceID,
					PointName: pt.Name,
					Level:     rule.Level,
					Message:   pt.Name + " 低于下限报警",
					Value:     *value,
					Threshold: rule.LowLimit,
					Timestamp: pt.Timestamp,
				})
			}
		}
	}
}

// triggerAlarm 触发报警
func (d *AlarmDetector) triggerAlarm(record alarm.Record) {
	// 检查是否已有同类未确认报警
	duplicate, err := d.store.CheckDuplicate(record.DeviceID, record.PointName)
	if err != nil {
		d.log.Error("检查重复报警失败", "error", err)
		return
	}

	if duplicate {
		return // 已有同类报警，不重复创建
	}

	if err := d.store.Create(&record); err != nil {
		d.log.Error("创建报警失败", "error", err)
	}
}

// toFloat64Value 将值转为 float64
func toFloat64Value(val interface{}) *float64 {
	switch v := val.(type) {
	case float64:
		return &v
	case float32:
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
	case int:
		f := float64(v)
		return &f
	default:
		return nil
	}
}
