package model

import "time"

// DataPoint 通用数据点
type DataPoint struct {
	DeviceID  string                 `json:"device_id"`  // 设备唯一标识
	Name      string                 `json:"name"`       // 数据点名称
	Value     interface{}            `json:"value"`      // 值
	Quality   Quality                `json:"quality"`    // 数据质量
	Timestamp time.Time              `json:"timestamp"`  // 时间戳
	DataType  DataType               `json:"data_type"`  // 数据类型
	Extra     map[string]interface{} `json:"extra,omitempty"` // 扩展字段
}

// Quality 数据质量
type Quality int

const (
	QualityGood         Quality = 0    // 良好
	QualityBad          Quality = 1    // 坏
	QualityUncertain    Quality = 2    // 不确定
	QualityTimeout      Quality = 3    // 超时
	QualityDisconnected Quality = 4    // 断开连接
)

func (q Quality) String() string {
	switch q {
	case QualityGood:
		return "good"
	case QualityBad:
		return "bad"
	case QualityUncertain:
		return "uncertain"
	case QualityTimeout:
		return "timeout"
	case QualityDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// DataType 数据类型
type DataType string

const (
	DataTypeBool    DataType = "bool"
	DataTypeInt16   DataType = "int16"
	DataTypeUint16  DataType = "uint16"
	DataTypeInt32   DataType = "int32"
	DataTypeUint32  DataType = "uint32"
	DataTypeFloat32 DataType = "float32"
	DataTypeFloat64 DataType = "float64"
	DataTypeString  DataType = "string"
)

// DeviceStatus 设备状态
type DeviceStatus struct {
	DeviceID    string    `json:"device_id"`
	Online      bool      `json:"online"`
	LastPoll    time.Time `json:"last_poll"`
	ErrorCount  int       `json:"error_count"`
	LastError   string    `json:"last_error"`
}
