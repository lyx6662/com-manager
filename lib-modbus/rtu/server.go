package rtu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus"
	"github.com/lyx6662/com-manager/pkg/logger"
	"go.bug.st/serial"
)

// ServerConfig RTU从站配置
type ServerConfig struct {
	ID       string
	Name     string
	Port     string // 串口号，如 COM3 或 /dev/ttyUSB0
	BaudRate int
	DataBits int
	StopBits int
	Parity   string
	SlaveID  byte
}

// Server Modbus RTU从站服务器 (通过串口输出数据给主机)
type Server struct {
	cfg        ServerConfig
	log        *logger.Logger
	mu         sync.RWMutex
	serialPort serial.Port
	running    bool
	ctx        context.Context
	cancel     context.CancelFunc

	store *modbus.RegisterStore

	// 连接状态
	masterConnected bool
	onConnect       func()
	onDisconnect    func()

	// 通信参数
	charTime time.Duration // 1字符传输时间
}

// NewServer 创建RTU从站服务器
func NewServer(cfg ServerConfig, log *logger.Logger) *Server {
	// 计算1字符时间 (微秒级)
	dataBits := cfg.DataBits
	if dataBits == 0 {
		dataBits = 8
	}
	stopBits := cfg.StopBits
	if stopBits == 0 {
		stopBits = 1
	}
	bitsPerChar := 1 + dataBits + stopBits
	if cfg.Parity != "none" && cfg.Parity != "" {
		bitsPerChar++
	}

	charTime := time.Duration(bitsPerChar*1000000/cfg.BaudRate) * time.Microsecond
	if charTime < 1*time.Millisecond {
		charTime = 1 * time.Millisecond
	}

	return &Server{
		cfg:      cfg,
		log:      log,
		store:    modbus.NewRegisterStore(),
		charTime: charTime,
	}
}

// Listen 打开串口并启动监听
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("RTU服务器已在运行: %s", s.cfg.ID)
	}

	// 打开串口
	mode := &serial.Mode{
		BaudRate: s.cfg.BaudRate,
		DataBits: s.cfg.DataBits,
		StopBits: stopBitsToSerial(s.cfg.StopBits),
		Parity:   parityToSerial(s.cfg.Parity),
	}

	port, err := serial.Open(s.cfg.Port, mode)
	if err != nil {
		return fmt.Errorf("打开串口失败 [%s]: %w", s.cfg.Port, err)
	}

	// 设置读超时 (非阻塞，用于帧边界检测)
	port.SetReadTimeout(50 * time.Millisecond)

	s.serialPort = port
	s.running = true
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.log.Info("Modbus RTU从站启动",
		"id", s.cfg.ID,
		"name", s.cfg.Name,
		"port", s.cfg.Port,
		"baud_rate", s.cfg.BaudRate,
		"slave_id", s.cfg.SlaveID,
		"char_time", s.charTime,
	)

	// 启动监听协程
	go s.listenLoop()

	return nil
}

// Close 停止服务器并关闭串口
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()
	s.running = false

	if s.serialPort != nil {
		s.serialPort.Close()
		s.serialPort = nil
	}

	s.log.Info("Modbus RTU从站已停止", "id", s.cfg.ID)
	return nil
}

// IsRunning 是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// UpdateRegisters 更新寄存器值
func (s *Server) UpdateRegisters(startAddr uint16, values []uint16) {
	s.store.UpdateRegisters(startAddr, values)
}

// UpdateCoils 更新线圈值
func (s *Server) UpdateCoils(startAddr uint16, values []bool) {
	s.store.UpdateCoils(startAddr, values)
}

// OnMasterConnected 注册主机连接回调
func (s *Server) OnMasterConnected(callback func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onConnect = callback
}

// OnMasterDisconnected 注册主机断开回调
func (s *Server) OnMasterDisconnected(callback func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDisconnect = callback
}

// IsMasterConnected 主机是否在线 (最近是否有请求)
func (s *Server) IsMasterConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterConnected
}

// listenLoop 主监听循环 — 读取串口数据并处理请求
func (s *Server) listenLoop() {
	s.log.Info("RTU从站监听协程启动", "id", s.cfg.ID)

	buffer := make([]byte, 0, 256)
	lastReadTime := time.Now()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 读取串口数据
		readBuf := make([]byte, 128)
		s.mu.RLock()
		port := s.serialPort
		s.mu.RUnlock()

		if port == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		n, err := port.Read(readBuf)
		if err != nil {
			if n == 0 {
				if len(buffer) > 0 && time.Since(lastReadTime) > s.charTime*4 {
					s.processFrame(buffer)
					buffer = buffer[:0]
				}
				continue
			}
			s.log.Error("RTU串口读取错误", "id", s.cfg.ID, "error", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if n > 0 {
			buffer = append(buffer, readBuf[:n]...)
			lastReadTime = time.Now()
		}

		// 检测帧边界
		if len(buffer) >= 4 {
			time.Sleep(s.charTime * 4)

			extra := make([]byte, 128)
			extraN, _ := port.Read(extra)
			if extraN > 0 {
				buffer = append(buffer, extra[:extraN]...)
			}

			if len(buffer) >= 4 {
				s.processFrame(buffer)
				buffer = buffer[:0]
			}
		}
	}
}

// processFrame 处理接收到的RTU帧
func (s *Server) processFrame(data []byte) {
	// 解析RTU帧
	frame, err := modbus.ParseRTUFrame(data)
	if err != nil {
		s.log.Debug("RTU帧解析失败", "id", s.cfg.ID, "error", err)
		return
	}

	// 检查是否是发给本从站的
	if frame.SlaveID != s.cfg.SlaveID && frame.SlaveID != 0 {
		return
	}

	// 标记主机已连接
	s.mu.Lock()
	wasConnected := s.masterConnected
	s.masterConnected = true
	s.mu.Unlock()

	if !wasConnected && s.onConnect != nil {
		go s.onConnect()
	}

	// 处理请求
	response := s.handleRequest(frame)
	if response == nil {
		return
	}

	// 编码并发送响应
	responseData := response.Encode()

	s.mu.RLock()
	port := s.serialPort
	s.mu.RUnlock()

	if port != nil {
		time.Sleep(s.charTime * 4)

		_, err := port.Write(responseData)
		if err != nil {
			s.log.Error("RTU发送响应失败", "id", s.cfg.ID, "error", err)
		}
	}
}

// handleRequest 处理Modbus请求
func (s *Server) handleRequest(request *modbus.RTUFrame) *modbus.RTUFrame {
	response := &modbus.RTUFrame{
		SlaveID:      request.SlaveID,
		FunctionCode: request.FunctionCode,
	}

	switch request.FunctionCode {
	case modbus.FuncReadHoldingRegs:
		return s.handleReadRegs(request, response)
	case modbus.FuncReadInputRegs:
		return s.handleReadRegs(request, response)
	case modbus.FuncReadCoils:
		return s.handleReadCoils(request, response)
	case modbus.FuncReadDiscreteInputs:
		return s.handleReadDiscreteInputs(request, response)
	case modbus.FuncWriteSingleReg:
		return s.handleWriteSingleReg(request, response)
	case modbus.FuncWriteMultiRegs:
		return s.handleWriteMultiRegs(request, response)
	case modbus.FuncWriteSingleCoil:
		return s.handleWriteSingleCoil(request, response)
	case modbus.FuncWriteMultiCoils:
		return s.handleWriteMultiCoils(request, response)
	default:
		response.FunctionCode = request.FunctionCode | 0x80
		response.Data = []byte{byte(modbus.ExceptionIllegalFunction)}
		return response
	}
}

// handleReadRegs 处理读保持/输入寄存器
func (s *Server) handleReadRegs(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	if len(request.Data) < 4 {
		return s.makeExceptionResponse(request, modbus.ExceptionIllegalDataValue)
	}

	startAddr := uint16(request.Data[0])<<8 | uint16(request.Data[1])
	quantity := uint16(request.Data[2])<<8 | uint16(request.Data[3])

	data, exCode := s.store.ReadRegisters(startAddr, quantity)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleReadCoils 处理读线圈
func (s *Server) handleReadCoils(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	if len(request.Data) < 4 {
		return s.makeExceptionResponse(request, modbus.ExceptionIllegalDataValue)
	}

	startAddr := uint16(request.Data[0])<<8 | uint16(request.Data[1])
	quantity := uint16(request.Data[2])<<8 | uint16(request.Data[3])

	data, exCode := s.store.ReadCoils(startAddr, quantity)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleReadDiscreteInputs 处理读离散输入
func (s *Server) handleReadDiscreteInputs(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	if len(request.Data) < 4 {
		return s.makeExceptionResponse(request, modbus.ExceptionIllegalDataValue)
	}

	startAddr := uint16(request.Data[0])<<8 | uint16(request.Data[1])
	quantity := uint16(request.Data[2])<<8 | uint16(request.Data[3])

	data, exCode := s.store.ReadDiscreteInputs(startAddr, quantity)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleWriteSingleReg 处理写单个寄存器
func (s *Server) handleWriteSingleReg(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	if len(request.Data) < 4 {
		return s.makeExceptionResponse(request, modbus.ExceptionIllegalDataValue)
	}

	addr := uint16(request.Data[0])<<8 | uint16(request.Data[1])
	value := uint16(request.Data[2])<<8 | uint16(request.Data[3])

	data, exCode := s.store.WriteSingleReg(addr, value)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleWriteMultiRegs 处理写多个寄存器
func (s *Server) handleWriteMultiRegs(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	data, exCode := s.store.WriteMultiRegs(request.Data)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleWriteSingleCoil 处理写单个线圈
func (s *Server) handleWriteSingleCoil(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	if len(request.Data) < 4 {
		return s.makeExceptionResponse(request, modbus.ExceptionIllegalDataValue)
	}

	addr := uint16(request.Data[0])<<8 | uint16(request.Data[1])
	value := uint16(request.Data[2])<<8 | uint16(request.Data[3])

	data, exCode := s.store.WriteSingleCoil(addr, value)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleWriteMultiCoils 处理写多个线圈
func (s *Server) handleWriteMultiCoils(request, response *modbus.RTUFrame) *modbus.RTUFrame {
	data, exCode := s.store.WriteMultiCoils(request.Data)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// makeExceptionResponse 创建异常响应
func (s *Server) makeExceptionResponse(request *modbus.RTUFrame, code modbus.ExceptionCode) *modbus.RTUFrame {
	return &modbus.RTUFrame{
		SlaveID:      request.SlaveID,
		FunctionCode: request.FunctionCode | 0x80,
		Data:         []byte{byte(code)},
	}
}
