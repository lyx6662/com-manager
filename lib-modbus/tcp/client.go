package tcp

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// ClientConfig Modbus TCP客户端配置
type ClientConfig struct {
	DeviceID string
	Host     string
	Port     int
	Timeout  time.Duration
	Retry    int
}

// Client Modbus TCP客户端 (采集网口设备)
type Client struct {
	cfg           ClientConfig
	log           *logger.Logger
	mu            sync.Mutex
	conn          net.Conn
	connected     int32 // 原子操作
	transaction   uint16
	lastConnState bool      // 上次连接状态，用于减少重复日志
	failStartTime time.Time // 首次失败时间
	failCount     int       // 连续失败次数
	nextReconnect time.Time // 下次重连时间
}

// NewClient 创建TCP客户端
func NewClient(cfg ClientConfig, log *logger.Logger) *Client {
	return &Client{
		cfg: cfg,
		log: log,
	}
}

// Connect 连接设备
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.connected) == 1 {
		return nil
	}

	// 检查是否到了重连时间
	if !c.nextReconnect.IsZero() && time.Now().Before(c.nextReconnect) {
		return fmt.Errorf("等待重连时间")
	}

	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)

	// 首次失败时打印日志
	if c.lastConnState {
		c.log.Warn("Modbus TCP设备连接失败，开始重连",
			"device_id", c.cfg.DeviceID,
			"addr", addr,
		)
		c.lastConnState = false
		c.failStartTime = time.Now()
		c.failCount = 0
	}

	conn, err := net.DialTimeout("tcp", addr, c.cfg.Timeout)
	if err != nil {
		c.failCount++
		// 计算下次重连时间
		c.nextReconnect = time.Now().Add(c.getReconnectInterval())
		return fmt.Errorf("连接Modbus TCP设备失败: %w", err)
	}

	c.conn = conn
	atomic.StoreInt32(&c.connected, 1)
	c.lastConnState = true
	c.failCount = 0
	c.nextReconnect = time.Time{}
	c.log.Info("Modbus TCP设备连接成功",
		"device_id", c.cfg.DeviceID,
		"addr", addr,
	)
	return nil
}

// getReconnectInterval 根据失败时间计算重连间隔
func (c *Client) getReconnectInterval() time.Duration {
	if c.failStartTime.IsZero() {
		return 10 * time.Second
	}

	elapsed := time.Since(c.failStartTime)
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
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.connected) == 0 {
		return nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	atomic.StoreInt32(&c.connected, 0)
	c.lastConnState = false
	c.log.Info("Modbus TCP设备已断开", "device_id", c.cfg.DeviceID)
	return nil
}

// IsConnected 是否已连接
func (c *Client) IsConnected() bool {
	return atomic.LoadInt32(&c.connected) == 1
}

// ReadHoldingRegisters 读保持寄存器
func (c *Client) ReadHoldingRegisters(slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error) {
	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)

	frame := &modbus.TCPFrame{
		TransactionID: c.nextTransactionID(),
		ProtocolID:    0x0000,
		UnitID:        slaveID,
		FunctionCode:  modbus.FuncReadHoldingRegs,
		Data:          data,
	}

	response, err := c.sendRequest(frame)
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

	if len(response.Data) < 1 {
		return nil, fmt.Errorf("响应数据长度不足")
	}

	byteCount := int(response.Data[0])
	if len(response.Data) < 1+byteCount {
		return nil, fmt.Errorf("响应数据不完整")
	}

	regs := modbus.DecodeRegisters(response.Data[1:1+byteCount], int(quantity))
	return regs, nil
}

// ReadInputRegisters 读输入寄存器
func (c *Client) ReadInputRegisters(slaveID byte, startAddr uint16, quantity uint16) ([]uint16, error) {
	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)

	frame := &modbus.TCPFrame{
		TransactionID: c.nextTransactionID(),
		ProtocolID:    0x0000,
		UnitID:        slaveID,
		FunctionCode:  modbus.FuncReadInputRegs,
		Data:          data,
	}

	response, err := c.sendRequest(frame)
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
func (c *Client) ReadCoils(slaveID byte, startAddr uint16, quantity uint16) ([]bool, error) {
	data := make([]byte, 4)
	data[0] = byte(startAddr >> 8)
	data[1] = byte(startAddr & 0xFF)
	data[2] = byte(quantity >> 8)
	data[3] = byte(quantity & 0xFF)

	frame := &modbus.TCPFrame{
		TransactionID: c.nextTransactionID(),
		ProtocolID:    0x0000,
		UnitID:        slaveID,
		FunctionCode:  modbus.FuncReadCoils,
		Data:          data,
	}

	response, err := c.sendRequest(frame)
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

// sendRequest 发送请求并接收响应
func (c *Client) sendRequest(request *modbus.TCPFrame) (*modbus.TCPFrame, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.connected) == 0 {
		return nil, fmt.Errorf("设备未连接: %s", c.cfg.DeviceID)
	}

	// 设置写超时
	c.conn.SetWriteDeadline(time.Now().Add(c.cfg.Timeout))

	// 编码并发送请求
	requestData := request.Encode()
	_, err := c.conn.Write(requestData)
	if err != nil {
		// 写入失败，关闭连接并标记断开
		c.conn.Close()
		c.conn = nil
		atomic.StoreInt32(&c.connected, 0)
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 设置读超时
	c.conn.SetReadDeadline(time.Now().Add(c.cfg.Timeout))

	// 读取MBAP头前6字节 (Transaction ID + Protocol ID + Length)
	mbapHeader := make([]byte, 6)
	_, err = io.ReadFull(c.conn, mbapHeader)
	if err != nil {
		// 读取失败，关闭连接并标记断开
		c.conn.Close()
		c.conn = nil
		atomic.StoreInt32(&c.connected, 0)
		return nil, fmt.Errorf("接收MBAP头失败: %w", err)
	}

	// 解析帧长度 (Length字段表示后续字节数，包括Unit ID)
	frameLength := int(mbapHeader[4])<<8 | int(mbapHeader[5])
	if frameLength < 2 || frameLength > 256 {
		// 帧长度无效，连接已损坏，关闭连接并标记断开以触发重连
		c.log.Warn("无效的帧长度",
			"device_id", c.cfg.DeviceID,
			"frame_length", frameLength,
			"header", fmt.Sprintf("%02X %02X %02X %02X %02X %02X",
				mbapHeader[0], mbapHeader[1], mbapHeader[2],
				mbapHeader[3], mbapHeader[4], mbapHeader[5]),
		)
		c.conn.Close()
		c.conn = nil
		atomic.StoreInt32(&c.connected, 0)
		return nil, fmt.Errorf("无效的帧长度: %d", frameLength)
	}

	// 读取剩余数据 (Unit ID + PDU)
	remaining := make([]byte, frameLength)
	_, err = io.ReadFull(c.conn, remaining)
	if err != nil {
		// 读取失败，关闭连接并标记断开
		c.conn.Close()
		c.conn = nil
		atomic.StoreInt32(&c.connected, 0)
		return nil, fmt.Errorf("接收帧体失败: %w", err)
	}

	// 组合完整帧: MBAP Header (6 bytes) + Unit ID + PDU
	fullFrame := make([]byte, 0, 6+frameLength)
	fullFrame = append(fullFrame, mbapHeader...)
	fullFrame = append(fullFrame, remaining...)

	response, err := modbus.ParseTCPFrame(fullFrame)
	if err != nil {
		return nil, fmt.Errorf("解析响应帧失败: %w", err)
	}

	return response, nil
}

// nextTransactionID 获取下一个事务ID
func (c *Client) nextTransactionID() uint16 {
	c.transaction++
	return c.transaction
}
