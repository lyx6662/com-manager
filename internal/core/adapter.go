package core

// OutputAdapter 输出适配器接口
// 所有输出协议（Modbus TCP/RTU、IEC 61850、MQTT 等）都实现此接口
type OutputAdapter interface {
	// Name 适配器名称
	Name() string

	// Init 初始化适配器
	Init(pool *DataPool) error

	// Start 启动输出
	Start() error

	// Stop 停止输出
	Stop() error

	// IsRunning 是否运行中
	IsRunning() bool

	// OnDataChanged 数据变更回调（由 DataPool 调用）
	OnDataChanged(deviceID string, pointName string, entry *DataPointEntry)
}
