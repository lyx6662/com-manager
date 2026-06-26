package modbus

// DeviceCollector 设备采集器接口（TCP client 和 RTU master 共用）
type DeviceCollector interface {
	Connect() error
	Disconnect() error
	IsConnected() bool
	ReadHoldingRegisters(slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error)
	ReadInputRegisters(slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error)
	ReadCoils(slaveID byte, startAddr uint16, quantity uint16) ([]bool, error)
	GetRecentPackets() []PacketEntry
}
