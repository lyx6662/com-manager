package rtu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
	"go.bug.st/serial"
)

// MasterConfig RTU主站配置
type MasterConfig struct {
	DeviceID string
	Port     string
	BaudRate int
	DataBits int
	StopBits int
	Parity   string
	SlaveID  byte
	Timeout  time.Duration
	Retry    int
}

// Master Modbus RTU主站 (采集串口设备)
type Master struct {
	cfg           MasterConfig
	log           *logger.Logger
	mu            sync.Mutex
	connected     bool
	serialPort    serial.Port
	lastConnState bool      // 上次连接状态，用于减少重复日志
	failStartTime time.Time // 首次失败时间
	failCount     int       // 连续失败次数
	nextReconnect time.Time // 下次重连时间
}

// NewMaster 创建RTU主站
func NewMaster(cfg MasterConfig, log *logger.Logger) *Master {
	return &Master{
		cfg: cfg,
		log: log,
	}
}

// parityToSerial 将配置中的奇偶校验字符串转为 serial 库的类型
func parityToSerial(parity string) serial.Parity {
	switch parity {
	case "even", "Even":
		return serial.EvenParity
	case "odd", "Odd":
		return serial.OddParity
	case "mark", "Mark":
		return serial.MarkParity
	case "space", "Space":
		return serial.SpaceParity
	default:
		return serial.NoParity
	}
}

// stopBitsToSerial 将停止位配置转为 serial 库的类型
func stopBitsToSerial(stopBits int) serial.StopBits {
	switch stopBits {
	case 2:
		return serial.TwoStopBits
	default:
		return serial.OneStopBit
	}
}

// Connect 连接串口
func (m *Master) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connected {
		return nil
	}

	// 检查是否到了重连时间
	if !m.nextReconnect.IsZero() && time.Now().Before(m.nextReconnect) {
		return fmt.Errorf("等待重连时间")
	}

	// 首次失败时打印日志
	if m.lastConnState {
		m.log.Warn("串口设备连接失败，开始重连",
			"device_id", m.cfg.DeviceID,
			"port", m.cfg.Port,
			"baud_rate", m.cfg.BaudRate,
			"error", "连接失败",
		)
		m.lastConnState = false
		m.failStartTime = time.Now()
		m.failCount = 0
	}

	mode := &serial.Mode{
		BaudRate: m.cfg.BaudRate,
		DataBits: m.cfg.DataBits,
		StopBits: stopBitsToSerial(m.cfg.StopBits),
		Parity:   parityToSerial(m.cfg.Parity),
	}

	port, err := serial.Open(m.cfg.Port, mode)
	if err != nil {
		m.failCount++
		// 计算下次重连时间
		m.nextReconnect = time.Now().Add(m.getReconnectInterval())
		return fmt.Errorf("打开串口失败: %w", err)
	}

	// 设置读超时
	port.SetReadTimeout(m.cfg.Timeout)

	m.serialPort = port
	m.connected = true
	m.lastConnState = true
	m.failCount = 0
	m.nextReconnect = time.Time{}
	m.log.Info("串口设备连接成功", "device_id", m.cfg.DeviceID)
	return nil
}

// getReconnectInterval 根据失败时间计算重连间隔
func (m *Master) getReconnectInterval() time.Duration {
	if m.failStartTime.IsZero() {
		return 10 * time.Second
	}

	elapsed := time.Since(m.failStartTime)
	switch {
	case elapsed < 1*time.Minute:
		// 第一分钟：10秒一次
		return 10 * time.Second
	case elapsed < 10*time.Minute:
		// 1-10分钟：60秒一次
		return 60 * time.Second
	default:
		// 10分钟后：5分钟一次
		return 5 * time.Minute
	}
}

// Disconnect 断开连接
func (m *Master) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.connected {
		return nil
	}

	if m.serialPort != nil {
		m.serialPort.Close()
		m.serialPort = nil
	}

	m.connected = false
	m.lastConnState = false
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
		if len(response.Data) < 1 {
			return nil, fmt.Errorf("异常响应数据不完整")
		}
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
		if len(response.Data) < 1 {
			return nil, fmt.Errorf("异常响应数据不完整")
		}
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
		if len(response.Data) < 1 {
			return nil, fmt.Errorf("异常响应数据不完整")
		}
		return nil, &modbus.ModbusException{
			Function:  response.FunctionCode & 0x7F,
			Exception: modbus.ExceptionCode(response.Data[0]),
		}
	}

	// 解析线圈状态
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

	if !m.connected || m.serialPort == nil {
		return nil, fmt.Errorf("设备未连接: %s", m.cfg.DeviceID)
	}

	// 编码请求
	requestData := request.Encode()

	// 清空接收缓冲区
	m.serialPort.ResetInputBuffer()

	// 发送数据到串口
	_, err := m.serialPort.Write(requestData)
	if err != nil {
		m.connected = false
		m.serialPort.Close()
		m.serialPort = nil
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 计算字符时间 (用于帧边界检测)
	charTime := time.Duration(11000000/m.cfg.BaudRate) * time.Microsecond
	if charTime < 1*time.Millisecond {
		charTime = 1 * time.Millisecond
	}

	// 使用短超时循环读取，通过帧边界检测判断帧结束
	buffer := make([]byte, 256)
	totalRead := 0
	deadline := time.Now().Add(m.cfg.Timeout)

	// 设置短超时，快速检测数据到达
	m.serialPort.SetReadTimeout(20 * time.Millisecond)

	for time.Now().Before(deadline) {
		n, _ := m.serialPort.Read(buffer[totalRead:])
		if n > 0 {
			totalRead += n
			// 重置deadline，因为收到了数据
			deadline = time.Now().Add(m.cfg.Timeout)

			// 最小 RTU 帧长度: SlaveID(1) + FuncCode(1) + ByteCount(1) + CRC(2) = 5
			if totalRead >= 5 {
				// 检查是否可以根据协议推算完整帧长度
				expectedLen := m.expectedFrameLength(buffer[:totalRead])
				if expectedLen > 0 && totalRead >= expectedLen {
					break // 帧已完整
				}
				// 等待 3.5 字符时间无新数据 = 帧结束
				time.Sleep(charTime * 4)
				extra, _ := m.serialPort.Read(buffer[totalRead:])
				if extra == 0 {
					break // 帧结束
				}
				totalRead += extra
				break
			}
		}
	}

	// 恢复原始超时
	m.serialPort.SetReadTimeout(m.cfg.Timeout)

	if totalRead < 5 {
		if totalRead == 0 {
			return nil, fmt.Errorf("串口无响应，请检查串口连接或设备状态: %s", m.cfg.DeviceID)
		}
		return nil, fmt.Errorf("响应数据太短: %d字节，期望至少5字节", totalRead)
	}

	// 解析 RTU 帧
	response, err := modbus.ParseRTUFrame(buffer[:totalRead])
	if err != nil {
		return nil, fmt.Errorf("解析响应帧失败: %w", err)
	}

	return response, nil
}

// expectedFrameLength 根据响应数据推算期望的帧长度
func (m *Master) expectedFrameLength(data []byte) int {
	if len(data) < 3 {
		return 0
	}

	funcCode := data[1] & 0x7F
	switch modbus.FunctionCode(funcCode) {
	case modbus.FuncReadCoils, modbus.FuncReadDiscreteInputs,
		modbus.FuncReadHoldingRegs, modbus.FuncReadInputRegs:
		// 响应格式: SlaveID + FuncCode + ByteCount + Data + CRC(2)
		if len(data) >= 3 {
			byteCount := int(data[2])
			return 3 + byteCount + 2 // +2 for CRC
		}
	case modbus.FuncWriteSingleReg, modbus.FuncWriteSingleCoil:
		// 响应格式: SlaveID + FuncCode + Addr(2) + Value(2) + CRC(2) = 8
		return 8
	case modbus.FuncWriteMultiRegs, modbus.FuncWriteMultiCoils:
		// 响应格式: SlaveID + FuncCode + Addr(2) + Quantity(2) + CRC(2) = 8
		return 8
	}
	return 0
}

// PollDevice 轮询设备数据
func (m *Master) PollDevice(ctx context.Context) ([]model.DataPoint, error) {
	points := make([]model.DataPoint, 0)

	// 此方法保留接口兼容性，实际采集由 Collector 调度器完成
	return points, nil
}
