package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// 优先级常量
const (
	PriorityLow     = 0
	PriorityNormal  = 1
	PriorityHigh    = 2
	PriorityUrgent  = 3
)

// ControlCommand 统一控制指令
type ControlCommand struct {
	// 指令标识
	ID       string `json:"id"`        // 唯一指令ID (UUID)
	Source   string `json:"source"`    // 指令来源: "modbus_tcp", "iec61850", "mqtt", "web", "api"
	SourceID string `json:"source_id"` // 来源标识 (如客户端IP、MQTT clientID)

	// 目标信息
	DeviceID     string `json:"device_id"`     // 目标设备ID
	PointID      string `json:"point_id"`      // 目标数据点ID (可选，优先使用)
	RegisterType string `json:"register_type"` // holding/coil (如未指定point_id)
	RegisterAddr uint16 `json:"register_addr"` // 寄存器地址 (如未指定point_id)

	// 控制值
	ValueType string      `json:"value_type"` // uint16/int16/float32/int32/bool
	Value     interface{} `json:"value"`       // 控制值
	ByteOrder string      `json:"byte_order"`  // 字节序 (可选)

	// 元数据
	Priority  int       `json:"priority"`  // 优先级: 0=低, 1=普通, 2=高, 3=紧急
	Timestamp time.Time `json:"timestamp"` // 指令创建时间
	Timeout   int       `json:"timeout"`   // 超时时间(毫秒)，0表示默认3000ms
	RequestID string    `json:"request_id"` // 请求ID (用于关联响应)

	// 安全
	AuthToken string `json:"auth_token"` // 认证令牌 (可选)
}

// CommandResponse 控制指令响应
type CommandResponse struct {
	CommandID    string      `json:"command_id"`    // 关联的指令ID
	Success      bool        `json:"success"`       // 是否成功
	ErrorCode    string      `json:"error_code"`    // 错误码 (成功时为空)
	ErrorMessage string      `json:"error_message"` // 错误信息
	Timestamp    time.Time   `json:"timestamp"`     // 响应时间

	// 写后读验证 (可选)
	ReadBack   interface{} `json:"read_back"`    // 回读值
	ReadBackOK bool        `json:"read_back_ok"` // 回读是否匹配
}

// CommandHandler 命令处理器接口
type CommandHandler interface {
	// CanHandle 是否能处理该指令
	CanHandle(cmd ControlCommand) bool

	// Handle 处理指令
	Handle(cmd ControlCommand) CommandResponse
}

// ControlSource 控制指令来源接口
type ControlSource interface {
	// Name 来源名称
	Name() string

	// Init 初始化
	Init(bus *CommandBus) error

	// Start 启动监听
	Start() error

	// Stop 停止
	Stop() error

	// OnCommandResponse 指令响应回调 (用于回传给来源)
	OnCommandResponse(response CommandResponse)

	// IsRunning 是否运行中
	IsRunning() bool
}

// CommandAuthorizer 命令授权器接口
type CommandAuthorizer interface {
	// Authorize 授权检查，返回 (是否授权, 错误信息)
	Authorize(cmd ControlCommand) (bool, string)
}

// DefaultAuthorizer 默认授权器
type DefaultAuthorizer struct {
	rules []AuthorizationRule
}

// AuthorizationRule 授权规则
type AuthorizationRule struct {
	Source      string            // 来源 (mqtt/iec61850/web等)
	Devices     []string          // 允许的设备 (空=全部)
	Points      []string          // 允许的数据点 (空=全部)
	PointTags   map[string]string // 按标签筛选
	MaxPriority int               // 最大允许优先级
}

// Authorize 授权检查
func (a *DefaultAuthorizer) Authorize(cmd ControlCommand) (bool, string) {
	if len(a.rules) == 0 {
		return true, ""
	}

	for _, rule := range a.rules {
		if a.matchRule(cmd, rule) {
			return true, ""
		}
	}
	return false, "无操作权限"
}

func (a *DefaultAuthorizer) matchRule(cmd ControlCommand, rule AuthorizationRule) bool {
	if rule.Source != "" && rule.Source != cmd.Source {
		return false
	}
	if len(rule.Devices) > 0 {
		found := false
		for _, d := range rule.Devices {
			if d == cmd.DeviceID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if cmd.Priority > rule.MaxPriority {
		return false
	}
	return true
}

// CommandBus 命令总线
type CommandBus struct {
	mu          sync.RWMutex
	log         *logger.Logger
	pool        *DataPool
	handlers    []CommandHandler
	sources     map[string]ControlSource
	authorizer  CommandAuthorizer
	pending     map[string]chan CommandResponse
	defaultTimeout time.Duration
	maxConcurrent  int
	semaphore      chan struct{}
}

// NewCommandBus 创建命令总线
func NewCommandBus(log *logger.Logger, pool *DataPool) *CommandBus {
	return &CommandBus{
		log:            log,
		pool:           pool,
		handlers:       make([]CommandHandler, 0),
		sources:        make(map[string]ControlSource),
		pending:        make(map[string]chan CommandResponse),
		defaultTimeout: 3 * time.Second,
		maxConcurrent:  10,
		semaphore:      make(chan struct{}, 10),
	}
}

// SetAuthorizer 设置授权器
func (bus *CommandBus) SetAuthorizer(authorizer CommandAuthorizer) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.authorizer = authorizer
}

// SetDefaultTimeout 设置默认超时时间
func (bus *CommandBus) SetDefaultTimeout(timeout time.Duration) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.defaultTimeout = timeout
}

// SetMaxConcurrent 设置最大并发数
func (bus *CommandBus) SetMaxConcurrent(max int) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.maxConcurrent = max
	bus.semaphore = make(chan struct{}, max)
}

// RegisterHandler 注册命令处理器
func (bus *CommandBus) RegisterHandler(handler CommandHandler) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.handlers = append(bus.handlers, handler)
}

// RegisterSource 注册控制来源
func (bus *CommandBus) RegisterSource(source ControlSource) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.sources[source.Name()] = source
}

// Start 启动命令总线和所有来源
func (bus *CommandBus) Start() error {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	for name, source := range bus.sources {
		if err := source.Init(bus); err != nil {
			return fmt.Errorf("初始化控制来源 %s 失败: %w", name, err)
		}
		if err := source.Start(); err != nil {
			return fmt.Errorf("启动控制来源 %s 失败: %w", name, err)
		}
		bus.log.Info("控制来源已启动", "name", name)
	}

	bus.log.Info("命令总线已启动",
		"handlers", len(bus.handlers),
		"sources", len(bus.sources),
	)
	return nil
}

// Stop 停止命令总线和所有来源
func (bus *CommandBus) Stop() {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	for name, source := range bus.sources {
		if err := source.Stop(); err != nil {
			bus.log.Error("停止控制来源失败", "name", name, "error", err)
		}
	}

	bus.log.Info("命令总线已停止")
}

// SubmitCommand 提交控制指令（异步）
func (bus *CommandBus) SubmitCommand(cmd ControlCommand) error {
	// 生成指令ID
	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}
	if cmd.Timestamp.IsZero() {
		cmd.Timestamp = time.Now()
	}

	// 授权检查
	bus.mu.RLock()
	authorizer := bus.authorizer
	bus.mu.RUnlock()

	if authorizer != nil {
		authorized, reason := authorizer.Authorize(cmd)
		if !authorized {
			bus.log.Warn("控制指令被拒绝",
				"command_id", cmd.ID,
				"source", cmd.Source,
				"reason", reason,
			)
			return fmt.Errorf("授权失败: %s", reason)
		}
	}

	// 异步处理
	go bus.processCommand(cmd)

	return nil
}

// SubmitAndWait 提交控制指令并等待响应
func (bus *CommandBus) SubmitAndWait(cmd ControlCommand, timeout time.Duration) (CommandResponse, error) {
	// 生成指令ID
	if cmd.ID == "" {
		cmd.ID = uuid.New().String()
	}
	if cmd.Timestamp.IsZero() {
		cmd.Timestamp = time.Now()
	}

	// 授权检查
	bus.mu.RLock()
	authorizer := bus.authorizer
	bus.mu.RUnlock()

	if authorizer != nil {
		authorized, reason := authorizer.Authorize(cmd)
		if !authorized {
			return CommandResponse{
				CommandID:    cmd.ID,
				Success:      false,
				ErrorCode:    "UNAUTHORIZED",
				ErrorMessage: reason,
				Timestamp:    time.Now(),
			}, fmt.Errorf("授权失败: %s", reason)
		}
	}

	// 创建响应通道
	responseCh := make(chan CommandResponse, 1)

	bus.mu.Lock()
	bus.pending[cmd.ID] = responseCh
	bus.mu.Unlock()

	defer func() {
		bus.mu.Lock()
		delete(bus.pending, cmd.ID)
		bus.mu.Unlock()
	}()

	// 处理指令
	bus.processCommand(cmd)

	// 等待响应或超时
	if timeout == 0 {
		timeout = bus.defaultTimeout
	}

	select {
	case resp := <-responseCh:
		return resp, nil
	case <-time.After(timeout):
		return CommandResponse{
			CommandID:    cmd.ID,
			Success:      false,
			ErrorCode:    "TIMEOUT",
			ErrorMessage: "指令执行超时",
			Timestamp:    time.Now(),
		}, fmt.Errorf("指令执行超时")
	}
}

// processCommand 处理控制指令
func (bus *CommandBus) processCommand(cmd ControlCommand) {
	// 并发控制
	bus.semaphore <- struct{}{}
	defer func() { <-bus.semaphore }()

	bus.log.Info("处理控制指令",
		"command_id", cmd.ID,
		"source", cmd.Source,
		"device", cmd.DeviceID,
		"point", cmd.PointID,
	)

	// 查找处理器
	bus.mu.RLock()
	handlers := make([]CommandHandler, len(bus.handlers))
	copy(handlers, bus.handlers)
	bus.mu.RUnlock()

	var response CommandResponse
	handled := false

	for _, handler := range handlers {
		if handler.CanHandle(cmd) {
			response = handler.Handle(cmd)
			handled = true
			break
		}
	}

	if !handled {
		response = CommandResponse{
			CommandID:    cmd.ID,
			Success:      false,
			ErrorCode:    "NO_HANDLER",
			ErrorMessage: "没有找到能处理该指令的处理器",
			Timestamp:    time.Now(),
		}
	}

	// 通知等待者
	bus.mu.RLock()
	if ch, ok := bus.pending[cmd.ID]; ok {
		select {
		case ch <- response:
		default:
		}
	}

	// 通知来源
	for _, source := range bus.sources {
		source.OnCommandResponse(response)
	}
	bus.mu.RUnlock()

	bus.log.Info("控制指令处理完成",
		"command_id", cmd.ID,
		"success", response.Success,
		"error_code", response.ErrorCode,
	)
}

// GetDataPool 获取数据池
func (bus *CommandBus) GetDataPool() *DataPool {
	return bus.pool
}

// ModbusWriteHandler Modbus 写入处理器
type ModbusWriteHandler struct {
	pool    *DataPool
	devices map[string]*DeviceWriteConnection
	log     *logger.Logger
}

// DeviceWriteConnection 设备写入连接
type DeviceWriteConnection struct {
	DeviceID string
	WriteFn  func(slaveID byte, regType string, addr uint16, values []uint16) error
	SlaveID  byte
}

// NewModbusWriteHandler 创建 Modbus 写入处理器
func NewModbusWriteHandler(log *logger.Logger, pool *DataPool) *ModbusWriteHandler {
	return &ModbusWriteHandler{
		pool:    pool,
		devices: make(map[string]*DeviceWriteConnection),
		log:     log,
	}
}

// RegisterDevice 注册设备写入连接
func (h *ModbusWriteHandler) RegisterDevice(conn *DeviceWriteConnection) {
	h.devices[conn.DeviceID] = conn
}

// CanHandle 判断是否处理该指令
func (h *ModbusWriteHandler) CanHandle(cmd ControlCommand) bool {
	_, exists := h.devices[cmd.DeviceID]
	return exists
}

// Handle 执行 Modbus 写入
func (h *ModbusWriteHandler) Handle(cmd ControlCommand) CommandResponse {
	startTime := time.Now()

	// 1. 获取设备连接
	conn, exists := h.devices[cmd.DeviceID]
	if !exists {
		return CommandResponse{
			CommandID:    cmd.ID,
			Success:      false,
			ErrorCode:    "DEVICE_NOT_FOUND",
			ErrorMessage: "设备不存在",
			Timestamp:    time.Now(),
		}
	}

	// 2. 解析数据点配置（如果指定了 PointID）
	var regType string
	var regAddr uint16
	if cmd.PointID != "" {
		// 从数据池获取数据点信息
		// 这里需要根据实际的数据点配置来获取寄存器类型和地址
		// 暂时使用命令中指定的值
		regType = cmd.RegisterType
		regAddr = cmd.RegisterAddr
	} else {
		regType = cmd.RegisterType
		regAddr = cmd.RegisterAddr
	}

	// 3. 编码寄存器值
	regValues := encodeCommandValue(cmd.ValueType, cmd.Value, cmd.ByteOrder)

	// 4. 执行写入
	err := conn.WriteFn(conn.SlaveID, regType, regAddr, regValues)

	// 5. 构建响应
	response := CommandResponse{
		CommandID: cmd.ID,
		Timestamp: time.Now(),
	}

	if err != nil {
		response.Success = false
		response.ErrorCode = "WRITE_FAILED"
		response.ErrorMessage = err.Error()
	} else {
		response.Success = true

		// 6. 更新数据共享池（写后读一致）
		if cmd.PointID != "" {
			h.pool.UpdateData(cmd.DeviceID, cmd.PointID, cmd.Value, 0, "")
		}

		h.log.Info("Modbus 写入完成",
			"command_id", cmd.ID,
			"device", cmd.DeviceID,
			"success", true,
			"duration", time.Since(startTime),
		)
	}

	return response
}

// encodeCommandValue 将控制值编码为寄存器值
func encodeCommandValue(valueType string, value interface{}, byteOrder string) []uint16 {
	switch valueType {
	case "float32":
		if v, ok := value.(float32); ok {
			bits := math.Float32bits(v)
			return encodeModbusUint32(bits, byteOrder)
		}
	case "int32":
		if v, ok := value.(int32); ok {
			return encodeModbusUint32(uint32(v), byteOrder)
		}
	case "uint32":
		if v, ok := value.(uint32); ok {
			return encodeModbusUint32(v, byteOrder)
		}
	case "int16":
		if v, ok := value.(int16); ok {
			return []uint16{uint16(v)}
		}
	case "uint16":
		if v, ok := value.(uint16); ok {
			return []uint16{v}
		}
	case "bool":
		if v, ok := value.(bool); ok {
			if v {
				return []uint16{0xFF00}
			}
			return []uint16{0x0000}
		}
	}
	return nil
}

// WebControlSource Web API 控制来源
type WebControlSource struct {
	bus     *CommandBus
	running bool
	mu      sync.RWMutex
}

// NewWebControlSource 创建 Web 控制来源
func NewWebControlSource() *WebControlSource {
	return &WebControlSource{}
}

// Name 来源名称
func (s *WebControlSource) Name() string {
	return "web"
}

// Init 初始化
func (s *WebControlSource) Init(bus *CommandBus) error {
	s.bus = bus
	return nil
}

// Start 启动
func (s *WebControlSource) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = true
	return nil
}

// Stop 停止
func (s *WebControlSource) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	return nil
}

// OnCommandResponse 响应回调
func (s *WebControlSource) OnCommandResponse(response CommandResponse) {
	// Web 来源的响应通过 API 返回，不需要额外处理
}

// IsRunning 是否运行中
func (s *WebControlSource) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ExecuteCommand 执行控制命令（供 Web API 调用）
func (s *WebControlSource) ExecuteCommand(ctx context.Context, cmd ControlCommand) (CommandResponse, error) {
	cmd.Source = "web"
	timeout := time.Duration(cmd.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return s.bus.SubmitAndWait(cmd, timeout)
}
