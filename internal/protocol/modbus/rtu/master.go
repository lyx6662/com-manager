package rtu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/internal/protocol/modbus"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// MasterConfig RTU主站配置
type MasterConfig struct {
	DeviceID   string
	Port       string
	BaudRate   int
	DataBits   int
	StopBits   int
	Parity     string
	SlaveID    byte
	Timeout    time.Duration
	Retry      int
}

// Master Modbus RTU主站 (采集串口设备)
type Master struct {
	cfg      MasterConfig
	log      *logger.Logger
	mu       sync.Mutex
	connected bool
	// serialPort serial.Port // 串口连接
}

// NewMaster 创建RTU主站
func NewMaster(cfg MasterConfig, log *logger.Logger) *Master {
	return &Master{
		cfg: cfg,
		log: log,
	}
}

// Connect 连接串口
func (m *Master) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	m.log.Info("连接串口设备...",
		"device_id", m.cfg.DeviceID,
		"port", m.cfg.Port,
		"baud_rate", m.cfg.BaudRate,
	)

	// TODO: 打开串口连接
	// port, err := serial.OpenPort(&serial.Config{
	//     Name:     m.cfg.Port,
	//     Baud:     m.cfg.BaudRate,
	//     DataBits: m.cfg.DataBits,
	//     StopBits: m.cfg.StopBits,
	//     Parity:   m.cfg.Parity,
	// })
	// if err != nil {
	//     return fmt.Errorf("打开串口失败: %w", err)
	// }
	// m.serialPort = port

	m.connected = true
	m.log.Info("串口设备连接成功", "device_id", m.cfg.DeviceID)
	return nil
}

// Disconnect 断开连接
func (m *Master) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return nil
	}

	// TODO: 关闭串口连接
	// if m.serialPort != nil {
	//     m.serialPort.Close()
	// }

	m.connected = false
	m.log.Info("串口设备已断开", "device_id", m.cfg.DeviceID)
	return nil
}

// IsConnected 是否已连接
func (m *Master) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// ReadHoldingRegisters 读保持寄存器
func (m *Master) ReadHoldingRegisters(slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error) {
	if !m.connected {
		return nil, fmt.Errorf("设备未连接: %s", m.cfg.DeviceID)
	}

	// 构建请求帧
	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)

	frame := &modbus.RTUFrame{
		SlaveID:      slaveID,
		FunctionCode: modbus.FuncReadHoldingRegs,
		Data:         data,
	}

	// 发送请求并接收响应
	response, err := m.sendRequest(frame)
	if err != nil {
		return nil, err
	}

	// 解析响应
	if response.FunctionCode&0x80 != 0 {
		return nil, &modbus.ModbusException{
			Function:  response.FunctionCode & 0x7F,
			Exception: modbus.ExceptionCode(response.Data[0]),
		}
	}

	if len(response.Data) < 1 {
		return nil, fmt.Errorf("响应数据长度不足")
	}

	byteCount := int(response.Data[0])
	if len(response.Data) < 1+byteCount {
		return nil, fmt.Errorf("响应数据不完整")
	}

	// 解析寄存器值
	regs := modbus.DecodeRegisters(response.Data[1:1+byteCount], int(quantity))
	return regs, nil
}

// ReadInputRegisters 读输入寄存器
func (m *Master) ReadInputRegisters(slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error) {
	if !m.connected {
		return nil, fmt.Errorf("设备未连接: %s", m.cfg.DeviceID)
	}

	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)

	frame := &modbus.RTUFrame{
		SlaveID:      slaveID,
		FunctionCode: modbus.FuncReadInputRegs,
		Data:         data,
	}

	response, err := m.sendRequest(frame)
	if err != nil {
		return nil, err
	}

	if response.FunctionCode&0x80 != 0 {
		return nil, &modbus.ModbusException{
			Function:  response.FunctionCode & 0x7F,
			Exception: modbus.ExceptionCode(response.Data[0]),
		}
	}

	byteCount := int(response.Data[0])
	regs := modbus.DecodeRegisters(response.Data[1:1+byteCount], int(quantity))
	return regs, nil
}

// ReadCoils 读线圈
func (m *Master) ReadCoils(slaveID byte, startAddr uint16, quantity uint16) ([]bool, error) {
	if !m.connected {
		return nil, fmt.Errorf("设备未连接: %s", m.cfg.DeviceID)
	}

	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)

	frame := &modbus.RTUFrame{
		SlaveID:      slaveID,
		FunctionCode: modbus.FuncReadCoils,
		Data:         data,
	}

	response, err := m.sendRequest(frame)
	if err != nil {
		return nil, err
	}

	if response.FunctionCode&0x80 != 0 {
		return nil, &modbus.ModbusException{
			Function:  response.FunctionCode & 0x7F,
			Exception: modbus.ExceptionCode(response.Data[0]),
		}
	}

	// 解析线圈状态
	byteCount := int(response.Data[0])
	coils := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		byteIndex := 1 + i/8
		bitIndex := i % 8
		if byteIndex < uint16(len(response.Data)) {
			coils[i] = (response.Data[byteIndex]>>bitIndex)&0x01 != 0
		}
	}

	return coils, nil
}

// WriteSingleRegister 写单个寄存器
func (m *Master) WriteSingleRegister(slaveID byte, addr uint16, value uint16) error {
	if !m.connected {
		return fmt.Errorf("设备未连接: %s", m.cfg.DeviceID)
	}

	data := make([]byte, 4)
	data[0] = byte(addr >> 8)
	data[1] = byte(addr & 0xFF)
	data[2] = byte(value >> 8)
	data[3] = byte(value & 0xFF)

	frame := &modbus.RTUFrame{
		SlaveID:      slaveID,
		FunctionCode: modbus.FuncWriteSingleReg,
		Data:         data,
	}

	_, err := m.sendRequest(frame)
	return err
}

// WriteMultipleRegisters 写多个寄存器
func (m *Master) WriteMultipleRegisters(slaveID byte, startAddr uint16, values []uint16) error {
	if !m.connected {
		return fmt.Errorf("设备未连接: %s", m.cfg.DeviceID)
	}

	quantity := len(values)
	byteCount := quantity * 2

	data := make([]byte, 5+byteCount)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)
	data[4] = byte(byteCount)

	for i, val := range values {
		data[5+i*2] = byte(val >> 8)
		data[5+i*2+1] = byte(val & 0xFF)
	}

	frame := &modbus.RTUFrame{
		SlaveID:      slaveID,
		FunctionCode: modbus.FuncWriteMultiRegs,
		Data:         data,
	}

	_, err := m.sendRequest(frame)
	return err
}

// sendRequest 发送请求并接收响应
func (m *Master) sendRequest(request *modbus.RTUFrame) (*modbus.RTUFrame, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 编码请求
	requestData := request.Encode()

	// TODO: 发送数据到串口
	// _, err := m.serialPort.Write(requestData)
	// if err != nil {
	//     return nil, fmt.Errorf("发送请求失败: %w", err)
	// }

	// TODO: 接收响应
	// buffer := make([]byte, 256)
	// m.serialPort.SetReadTimeout(m.cfg.Timeout)
	// n, err := m.serialPort.Read(buffer)
	// if err != nil {
	//     return nil, fmt.Errorf("接收响应失败: %w", err)
	// }

	// 临时返回空响应
	_ = requestData
	response := &modbus.RTUFrame{
		SlaveID:      request.SlaveID,
		FunctionCode: request.FunctionCode,
		Data:         []byte{0x02, 0x00, 0x00},
	}

	return response, nil
}

// PollDevice 轮询设备数据
func (m *Master) PollDevice(ctx context.Context) ([]model.DataPoint, error) {
	points := make([]model.DataPoint, 0)

	// TODO: 根据点表读取设备数据
	// regs, err := m.ReadHoldingRegisters(m.cfg.SlaveID, 0, 10)
	// if err != nil {
	//     return nil, err
	// }

	return points, nil
}
