package tcp

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// ServerConfig TCP服务器配置
type ServerConfig struct {
	ID             string
	Name           string
	ListenPort     int
	SlaveID        byte
	MaxConnections int
}

// Server Modbus TCP服务器 (输出数据给主机)
type Server struct {
	cfg         ServerConfig
	log         *logger.Logger
	mu          sync.RWMutex
	listener    net.Listener
	connections map[net.Conn]bool
	store       *modbus.RegisterStore
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc

	// 断点续传相关
	masterConnected bool
	onConnect       func()
	onDisconnect    func()
}

// NewServer 创建TCP服务器
func NewServer(cfg ServerConfig, log *logger.Logger) *Server {
	return &Server{
		cfg:         cfg,
		log:         log,
		connections: make(map[net.Conn]bool),
		store:       modbus.NewRegisterStore(),
	}
}

// Listen 启动监听
func (s *Server) Listen() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("服务器已在运行: %s", s.cfg.ID)
	}

	addr := fmt.Sprintf(":%d", s.cfg.ListenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}

	s.listener = listener
	s.running = true
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.log.Info("Modbus TCP服务器启动",
		"id", s.cfg.ID,
		"name", s.cfg.Name,
		"port", s.cfg.ListenPort,
	)

	// 接受连接
	go s.acceptConnections()

	return nil
}

// Close 停止服务器
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()
	s.running = false

	// 关闭所有连接
	for conn := range s.connections {
		conn.Close()
	}
	s.connections = make(map[net.Conn]bool)

	// 关闭监听
	if s.listener != nil {
		s.listener.Close()
	}

	s.log.Info("Modbus TCP服务器已停止", "id", s.cfg.ID)
	return nil
}

// IsRunning 是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetConnectionCount 获取当前连接数
func (s *Server) GetConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}

// IsMasterConnected 主机是否连接
func (s *Server) IsMasterConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections) > 0
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

// UpdateRegisters 更新寄存器值
func (s *Server) UpdateRegisters(startAddr uint16, values []uint16) {
	s.store.UpdateRegisters(startAddr, values)
}

// UpdateCoils 更新线圈值
func (s *Server) UpdateCoils(startAddr uint16, values []bool) {
	s.store.UpdateCoils(startAddr, values)
}

// acceptConnections 接受连接
func (s *Server) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.log.Error("接受连接失败", "error", err)
				continue
			}
		}

		// 检查最大连接数
		s.mu.RLock()
		connCount := len(s.connections)
		s.mu.RUnlock()

		if connCount >= s.cfg.MaxConnections {
			s.log.Warn("超过最大连接数，拒绝连接",
				"max", s.cfg.MaxConnections,
				"current", connCount,
			)
			conn.Close()
			continue
		}

		// 添加连接
		s.mu.Lock()
		s.connections[conn] = true
		wasEmpty := connCount == 0
		s.mu.Unlock()

		s.log.Info("新的Modbus TCP连接",
			"remote", conn.RemoteAddr(),
			"id", s.cfg.ID,
		)

		// 触发连接回调
		if wasEmpty && s.onConnect != nil {
			go s.onConnect()
		}

		// 处理连接
		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()

		s.mu.Lock()
		delete(s.connections, conn)
		isEmpty := len(s.connections) == 0
		s.mu.Unlock()

		s.log.Info("Modbus TCP连接断开",
			"remote", conn.RemoteAddr(),
			"id", s.cfg.ID,
		)

		// 触发断开回调
		if isEmpty && s.onDisconnect != nil {
			go s.onDisconnect()
		}
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 设置读超时
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// 读取MBAP头 (6字节: TransactionID + ProtocolID + Length)
		header := make([]byte, 6)
		_, err := io.ReadFull(conn, header)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		// 解析帧长度 (包含 UnitID + PDU)
		frameLength := int(header[4])<<8 | int(header[5])
		if frameLength < 2 || frameLength > 256 {
			s.log.Warn("无效的帧长度", "length", frameLength)
			continue
		}

		// 读取剩余数据 (UnitID + PDU)
		body := make([]byte, frameLength)
		_, err = io.ReadFull(conn, body)
		if err != nil {
			return
		}

		// 组合完整帧
		fullFrame := append(header, body...)
		// s.log.Debug("收到原始数据",
		// 	"header", fmt.Sprintf("%X", header),
		// 	"body", fmt.Sprintf("%X", body),
		// 	"full", fmt.Sprintf("%X", fullFrame),
		// )
		tcpFrame, err := modbus.ParseTCPFrame(fullFrame)
		if err != nil {
			s.log.Warn("解析TCP帧失败", "error", err)
			continue
		}

		// 处理请求
		// s.log.Debug("收到Modbus请求",
		// 	"transaction_id", tcpFrame.TransactionID,
		// 	"function_code", tcpFrame.FunctionCode,
		// 	"unit_id", tcpFrame.UnitID,
		// 	"data_len", len(tcpFrame.Data),
		// )
		response := s.handleRequest(tcpFrame)
		if response != nil {
			encoded := response.Encode()
			// s.log.Debug("编码响应",
			// 	"len", len(encoded),
			// 	"data", fmt.Sprintf("%X", encoded),
			// )
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			n, err := conn.Write(encoded)
			if err != nil {
				s.log.Error("发送响应失败", "error", err, "bytes_written", n)
				return
			}
			// s.log.Debug("发送Modbus响应",
			// 	"transaction_id", response.TransactionID,
			// 	"function_code", response.FunctionCode,
			// 	"data_len", len(response.Data),
			// 	"bytes_written", n,
			// )
		}
	}
}

// handleRequest 处理Modbus请求
func (s *Server) handleRequest(request *modbus.TCPFrame) *modbus.TCPFrame {
	response := &modbus.TCPFrame{
		TransactionID: request.TransactionID,
		ProtocolID:    0x0000,
		UnitID:        request.UnitID,
		FunctionCode:  request.FunctionCode,
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
func (s *Server) handleReadRegs(request, response *modbus.TCPFrame) *modbus.TCPFrame {
	if len(request.Data) < 4 {
		return s.makeExceptionResponse(request, modbus.ExceptionIllegalDataValue)
	}

	startAddr := uint16(request.Data[0])<<8 | uint16(request.Data[1])
	quantity := uint16(request.Data[2])<<8 | uint16(request.Data[3])

	// s.log.Debug("读取寄存器",
	// 	"start_addr", startAddr,
	// 	"quantity", quantity,
	// 	"data_len", len(request.Data),
	// 	"data", fmt.Sprintf("%X", request.Data),
	// )

	data, exCode := s.store.ReadRegisters(startAddr, quantity)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleReadCoils 处理读线圈
func (s *Server) handleReadCoils(request, response *modbus.TCPFrame) *modbus.TCPFrame {
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
func (s *Server) handleReadDiscreteInputs(request, response *modbus.TCPFrame) *modbus.TCPFrame {
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
func (s *Server) handleWriteSingleReg(request, response *modbus.TCPFrame) *modbus.TCPFrame {
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
func (s *Server) handleWriteMultiRegs(request, response *modbus.TCPFrame) *modbus.TCPFrame {
	data, exCode := s.store.WriteMultiRegs(request.Data)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// handleWriteSingleCoil 处理写单个线圈
func (s *Server) handleWriteSingleCoil(request, response *modbus.TCPFrame) *modbus.TCPFrame {
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
func (s *Server) handleWriteMultiCoils(request, response *modbus.TCPFrame) *modbus.TCPFrame {
	data, exCode := s.store.WriteMultiCoils(request.Data)
	if exCode != 0 {
		return s.makeExceptionResponse(request, exCode)
	}

	response.Data = data
	return response
}

// makeExceptionResponse 创建异常响应
func (s *Server) makeExceptionResponse(request *modbus.TCPFrame, code modbus.ExceptionCode) *modbus.TCPFrame {
	return &modbus.TCPFrame{
		TransactionID: request.TransactionID,
		ProtocolID:    0x0000,
		UnitID:        request.UnitID,
		FunctionCode:  request.FunctionCode | 0x80,
		Data:          []byte{byte(code)},
	}
}
