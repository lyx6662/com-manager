# 统一数据共享池架构设计

## 1. 背景与问题

### 1.1 当前架构的问题

当前系统存在以下耦合问题：

```
问题1: 采集与输出强耦合
  Modbus 映射规则 → 既用于采集，又用于 Modbus 输出
  IEC 61850 映射 → 依赖 Modbus 映射规则获取数据

问题2: 重复采集风险
  同一个寄存器被 Modbus 输出和 IEC 61850 同时需要时，可能出现重复采集或遗漏

问题3: 扩展困难
  新增 MQTT、OPC UA 等输出协议时，需要修改采集逻辑
```

### 1.2 目标架构

```
┌─────────────────────────────────────────────────────────────┐
│                      统一采集层                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ Modbus 设备1 │  │ Modbus 设备2 │  │ Modbus 设备3 │  ...    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│         └────────────────┼────────────────┘                 │
│                          ▼                                  │
│              ┌───────────────────────┐                      │
│              │   统一数据共享池      │                      │
│              │  (Unified Data Pool)  │                      │
│              └───────────────────────┘                      │
└─────────────────────────────────────────────────────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
┌────────────────┐ ┌────────────────┐ ┌────────────────┐
│ Modbus TCP/RTU │ │   IEC 61850    │ │     MQTT       │  ...
│    输出适配器   │ │   输出适配器    │ │   输出适配器    │
└────────────────┘ └────────────────┘ └────────────────┘
```

## 2. 核心设计

### 2.1 统一数据点注册表

```go
// UnifiedDataPoint 统一数据点定义
type UnifiedDataPoint struct {
    // 基本信息
    ID          string `yaml:"id" json:"id"`                     // 唯一标识，如 "device1.temperature"
    DeviceID    string `yaml:"device_id" json:"device_id"`      // 所属设备ID
    Name        string `yaml:"name" json:"name"`                 // 点位名称

    // 采集配置
    RegisterType string `yaml:"register_type" json:"register_type"` // holding/coil/input
    RegisterAddr uint16 `yaml:"register_addr" json:"register_addr"` // 寄存器地址
    DataType     string `yaml:"data_type" json:"data_type"`         // uint16/int16/float32/int32/bool
    Quantity     uint16 `yaml:"quantity" json:"quantity"`           // 读取数量

    // 转换参数
    Scale  float64 `yaml:"scale" json:"scale"`
    Offset float64 `yaml:"offset" json:"offset"`
    ByteOrder string `yaml:"byte_order" json:"byte_order"`       // ABCD/BADC/CDAB/DCBA

    // 元数据
    Description string            `yaml:"description" json:"description"`
    Unit        string            `yaml:"unit" json:"unit"`
    Tags        map[string]string `yaml:"tags" json:"tags"`       // 自定义标签，用于分组筛选
}
```

### 2.2 数据共享池

```go
// DataPool 统一数据共享池
type DataPool struct {
    mu          sync.RWMutex
    data        map[string]*DataPointEntry  // dataPointID -> 数据条目
    subscribers []DataChangeSubscriber      // 数据变更订阅者
}

// DataPointEntry 数据条目
type DataPointEntry struct {
    Point       UnifiedDataPoint
    Value       interface{}
    Quality     QualityCode
    Timestamp   time.Time
    UpdateCount uint64
}

// QualityCode 品质码
type QualityCode int

const (
    QualityGood          QualityCode = 0
    QualityBad           QualityCode = 0x80
    QualityUncertain     QualityCode = 0x40
    QualityNotConnected  QualityCode = 0xC0
)

// DataChangeSubscriber 数据变更订阅者接口
type DataChangeSubscriber interface {
    OnDataChanged(dataPointID string, entry *DataPointEntry)
}
```

### 2.3 输出适配器接口

```go
// OutputAdapter 输出适配器接口
type OutputAdapter interface {
    // Init 初始化适配器
    Init(config interface{}) error

    // Start 启动输出
    Start() error

    // Stop 停止输出
    Stop() error

    // SubscribeDataPoints 订阅需要的数据点
    SubscribeDataPoints(pointIDs []string)

    // OnDataChanged 数据变更回调
    OnDataChanged(dataPointID string, entry *DataPointEntry)

    // IsRunning 是否运行中
    IsRunning() bool
}
```

## 3. 配置结构

### 3.1 统一数据点配置 (`configs/data_points.yaml`)

```yaml
# 统一数据点注册表
data_points:
  # 设备1的温度
  - id: "rtu-device-1.temperature"
    device_id: "rtu-device-1"
    name: "temperature"
    register_type: "holding"
    register_addr: 0
    data_type: "float32"
    quantity: 2
    scale: 1.0
    offset: 0.0
    byte_order: "ABCD"
    description: "设备1温度"
    unit: "°C"
    tags:
      group: "environment"
      priority: "high"

  # 设备1的模式
  - id: "rtu-device-1.mode"
    device_id: "rtu-device-1"
    name: "mode"
    register_type: "holding"
    register_addr: 2
    data_type: "int32"
    quantity: 2
    scale: 1.0
    offset: 0.0
    description: "设备1运行模式"
    tags:
      group: "status"

  # 设备2的电压
  - id: "rtu-device-2.voltage"
    device_id: "rtu-device-2"
    name: "voltage"
    register_type: "holding"
    register_addr: 100
    data_type: "float32"
    quantity: 2
    description: "设备2电压"
    unit: "V"
```

### 3.2 输出适配器配置 (`configs/outputs.yaml`)

```yaml
# Modbus TCP/RTU 输出
modbus_output:
  enabled: true
  tcp_port: 502
  rtu_port: "/dev/ttyUSB0"
  # 订阅的数据点及目标寄存器映射
  mappings:
    - source_id: "rtu-device-1.temperature"
      target_register: 0
      target_type: "holding"
    - source_id: "rtu-device-1.mode"
      target_register: 2
      target_type: "holding"

# IEC 61850 输出
iec61850_output:
  enabled: true
  port: 102
  ied_name: "GRID_GATEWAY"
  # 订阅的数据点及 IEC 61850 路径映射
  mappings:
    - source_id: "rtu-device-1.temperature"
      iec61850_path: "GRID_GATEWAY/MMXU1.TotW.mag.f"
      target_type: "float32"
    - source_id: "rtu-device-1.mode"
      iec61850_path: "GRID_GATEWAY/MMXU1.Mod.stVal"
      target_type: "int32"

# MQTT 输出 (未来扩展)
mqtt_output:
  enabled: false
  broker: "mqtt://localhost:1883"
  client_id: "com-manager"
  topics:
    - source_id: "rtu-device-1.temperature"
      topic: "sensors/rtu-device-1/temperature"
      qos: 1
    - source_id: "rtu-device-1.mode"
      topic: "sensors/rtu-device-1/mode"
      qos: 0

# OPC UA 输出 (未来扩展)
opcua_output:
  enabled: false
  endpoint: "opc.tcp://localhost:4840"
  namespace: "ns=2"
  mappings: []
```

## 4. 核心流程

### 4.1 采集流程

```
1. 引擎启动
   ↓
2. 加载 data_points.yaml，注册所有数据点到 DataPool
   ↓
3. 按 device_id 分组，合并同一设备的寄存器读取请求
   ↓
4. 为每个设备创建优化的读取任务（合并连续寄存器）
   ↓
5. 定时轮询设备，更新 DataPool 中的数据
   ↓
6. 通知所有订阅该数据点的输出适配器
```

### 4.2 输出流程

```
1. 输出适配器启动
   ↓
2. 加载输出配置，调用 SubscribeDataPoints() 订阅需要的数据点
   ↓
3. DataPool 记录订阅关系
   ↓
4. 当数据变更时，DataPool 调用 OnDataChanged() 通知适配器
   ↓
5. 适配器根据配置的映射规则，转换并输出数据
```

### 4.3 寄存器合并优化

```go
// RegisterRange 寄存器范围
type RegisterRange struct {
    DeviceID    string
    RegType     string  // holding/coil/input
    StartAddr   uint16
    Quantity    uint16
    DataPoints  []string  // 包含的数据点ID列表
}

// MergeRegisterRanges 合并连续的寄存器读取范围
func MergeRegisterRanges(points []UnifiedDataPoint) []RegisterRange {
    // 按设备ID和寄存器类型分组
    // 按起始地址排序
    // 合并地址连续的范围（考虑最大读取长度限制）
    // 返回优化后的读取任务列表
}
```

## 5. 数据结构变更

### 5.1 DataPool 核心方法

```go
// RegisterDataPoint 注册数据点
func (dp *DataPool) RegisterDataPoint(point UnifiedDataPoint)

// UnregisterDataPoint 注销数据点
func (dp *DataPool) UnregisterDataPoint(pointID string)

// UpdateData 更新数据（采集器调用）
func (dp *DataPool) UpdateData(pointID string, value interface{}, quality QualityCode)

// BatchUpdateData 批量更新数据（同一设备的一次读取结果）
func (dp *DataPool) BatchUpdateData(deviceID string, results []ReadResult)

// GetData 获取数据
func (dp *DataPool) GetData(pointID string) (*DataPointEntry, bool)

// GetDeviceData 获取设备的所有数据
func (dp *DataPool) GetDeviceData(deviceID string) map[string]*DataPointEntry

// Subscribe 订阅数据变更
func (dp *DataPool) Subscribe(pointIDs []string, subscriber DataChangeSubscriber)

// Unsubscribe 取消订阅
func (dp *DataPool) Unsubscribe(subscriber DataChangeSubscriber)

// GetSubscribedPoints 获取某订阅者订阅的所有数据点
func (dp *DataPool) GetSubscribedPoints(subscriber DataChangeSubscriber) []string
```

### 5.2 优化的采集器

```go
// OptimizedCollector 优化的采集器
type OptimizedCollector struct {
    pool       *DataPool
    devices    map[string]*DeviceConfig
    readTasks  map[string][]RegisterRange  // deviceID -> 读取任务列表
    workers    map[string]*DeviceWorker
}

// RefreshReadTasks 刷新读取任务（数据点配置变更时调用）
func (oc *OptimizedCollector) RefreshReadTasks()

// Worker 设备工作协程
type DeviceWorker struct {
    deviceID   string
    client     ModbusClient
    tasks      []RegisterRange
    interval   time.Duration
    pool       *DataPool
    stopCh     chan struct{}
}

// run 工作循环
func (w *DeviceWorker) run() {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            w.poll()
        case <-w.stopCh:
            return
        }
    }
}

// poll 执行一次轮询
func (w *DeviceWorker) poll() {
    for _, task := range w.tasks {
        // 批量读取寄存器
        rawValues, err := w.client.ReadRegisters(task.StartAddr, task.Quantity, task.RegType)
        if err != nil {
            // 设置所有相关数据点为 Bad 品质
            for _, pointID := range task.DataPoints {
                w.pool.UpdateData(pointID, nil, QualityBad)
            }
            continue
        }

        // 解析各数据点的值
        for _, pointID := range task.DataPoints {
            entry := w.pool.GetData(pointID)
            if entry == nil {
                continue
            }
            value := parseRegisterValue(entry.Point, rawValues)
            w.pool.UpdateData(pointID, value, QualityGood)
        }
    }
}
```

## 6. 输出适配器实现示例

### 6.1 Modbus 输出适配器

```go
// ModbusOutputAdapter Modbus TCP/RTU 输出适配器
type ModbusOutputAdapter struct {
    pool       *DataPool
    tcpServer  *tcp.Server
    rtuServer  *rtu.Server
    mappings   []ModbusOutputMapping
    subscribed []string
    mu         sync.RWMutex
}

// OnDataChanged 数据变更回调
func (a *ModbusOutputAdapter) OnDataChanged(dataPointID string, entry *DataPointEntry) {
    a.mu.RLock()
    defer a.mu.RUnlock()

    // 查找对应的输出映射
    for _, mapping := range a.mappings {
        if mapping.SourceID != dataPointID {
            continue
        }

        // 转换为寄存器值
        regValues := encodeToRegisters(entry, mapping)
        if regValues == nil {
            continue
        }

        // 写入 Modbus 服务器
        if a.tcpServer != nil {
            a.tcpServer.UpdateRegisters(mapping.TargetRegister, regValues)
        }
        if a.rtuServer != nil {
            a.rtuServer.UpdateRegisters(mapping.TargetRegister, regValues)
        }
    }
}
```

### 6.2 IEC 61850 输出适配器

```go
// IEC61850OutputAdapter IEC 61850 输出适配器
type IEC61850OutputAdapter struct {
    pool       *DataPool
    manager    *iec61850.Manager
    mappings   []IEC61850OutputMapping
    subscribed []string
    mu         sync.RWMutex
}

// OnDataChanged 数据变更回调
func (a *IEC61850OutputAdapter) OnDataChanged(dataPointID string, entry *DataPointEntry) {
    a.mu.RLock()
    defer a.mu.RUnlock()

    for _, mapping := range a.mappings {
        if mapping.SourceID != dataPointID {
            continue
        }

        // 转换为目标类型
        value := convertToIEC61850Type(entry.Value, mapping.TargetType)
        if value == nil {
            continue
        }

        // 确定品质码
        quality := mapQualityToIEC61850(entry.Quality)

        // 更新 IEC 61850 数据
        a.manager.UpdateData(mapping.IEC61850Path, value, quality, entry.Timestamp.UnixMilli())
    }
}
```

### 6.3 MQTT 输出适配器 (示例)

```go
// MQTTOutputAdapter MQTT 输出适配器
type MQTTOutputAdapter struct {
    pool       *DataPool
    client     mqtt.Client
    topics     []MQTTTopicMapping
    subscribed []string
    mu         sync.RWMutex
}

// OnDataChanged 数据变更回调
func (a *MQTTOutputAdapter) OnDataChanged(dataPointID string, entry *DataPointEntry) {
    a.mu.RLock()
    defer a.mu.RUnlock()

    for _, mapping := range a.topics {
        if mapping.SourceID != dataPointID {
            continue
        }

        // 构建 JSON 消息
        payload := map[string]interface{}{
            "value":     entry.Value,
            "quality":   entry.Quality,
            "timestamp": entry.Timestamp.UnixMilli(),
            "unit":      entry.Point.Unit,
        }

        jsonData, err := json.Marshal(payload)
        if err != nil {
            continue
        }

        // 发布到 MQTT
        a.client.Publish(mapping.Topic, byte(mapping.QoS), false, jsonData)
    }
}
```

## 7. 迁移方案

### 7.1 阶段一：核心框架（1周）

1. 实现 `DataPool` 数据共享池
2. 实现 `UnifiedDataPoint` 统一数据点定义
3. 实现 `OutputAdapter` 接口
4. 编写单元测试

### 7.2 阶段二：采集器改造（1周）

1. 实现 `OptimizedCollector` 优化采集器
2. 实现寄存器合并优化
3. 支持从 `data_points.yaml` 加载配置
4. 保持向后兼容旧配置格式

### 7.3 阶段三：输出适配器迁移（1周）

1. 实现 `ModbusOutputAdapter`
2. 实现 `IEC61850OutputAdapter`
3. 迁移现有功能到新架构
4. 集成测试

### 7.4 阶段四：配置迁移工具（可选）

1. 编写配置迁移工具，将旧配置转换为新格式
2. 更新 Web 管理界面
3. 文档更新

## 8. 优势总结

### 8.1 解耦

- 采集层和输出层完全解耦
- 新增输出协议无需修改采集逻辑
- 各输出协议独立配置和管理

### 8.2 性能优化

- 自动合并连续寄存器读取，减少通信次数
- 避免重复采集相同寄存器
- 数据变更时按需通知，减少不必要的处理

### 8.3 可扩展性

- 新增 MQTT、OPC UA、HTTP Webhook 等输出协议非常简单
- 只需实现 `OutputAdapter` 接口
- 配置驱动，无需修改核心代码

### 8.4 可维护性

- 清晰的架构分层
- 统一的数据模型
- 便于测试和调试

## 9. 风险与挑战

### 9.1 向后兼容

- 需要支持旧配置格式的平滑迁移
- 建议提供配置迁移工具

### 9.2 内存占用

- 数据共享池会占用更多内存
- 需要合理设置数据点数量上限

### 9.3 实时性

- 异步通知机制可能引入微小延迟
- 对于极高实时性要求的场景，需要评估

## 10. 双向控制流设计（下行控制）

### 10.1 需求场景

在工业物联网场景中，不仅需要上行采集数据，还需要下行控制设备：

```
┌─────────────────────────────────────────────────────────────┐
│                     上行数据流（已有）                        │
│  Modbus设备 ──→ 数据采集 ──→ 数据共享池 ──→ 输出协议         │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                     下行控制流（新增）                        │
│  控制指令来源 ──→ 命令总线 ──→ 指令路由 ──→ Modbus设备       │
│  (IEC61850/MQTT/Web/SCADA)              (写寄存器/线圈)      │
└─────────────────────────────────────────────────────────────┘
```

**典型场景：**
- SCADA 通过 Modbus TCP 写入保持寄存器 → 控制设备运行参数
- IEC 61850 客户端下发控制命令 → 控制开关状态
- MQTT 订阅控制主题 → 远程参数调整
- Web 界面手动控制 → 调试和维护

### 10.2 双向架构设计

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            控制指令来源                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐  │
│  │ Modbus TCP   │  │  IEC 61850   │  │    MQTT      │  │  Web API   │  │
│  │ (上位机写)   │  │  (客户端控制) │  │ (订阅控制)   │  │ (手动控制) │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └─────┬──────┘  │
└─────────┼─────────────────┼─────────────────┼────────────────┼──────────┘
          │                 │                 │                │
          └─────────────────┼─────────────────┼────────────────┘
                            ▼
          ┌─────────────────────────────────────────┐
          │           统一命令总线                   │
          │        (Command Bus / Command Queue)     │
          └────────────────────┬────────────────────┘
                               ▼
          ┌─────────────────────────────────────────┐
          │           控制指令处理器                 │
          │  - 权限验证                              │
          │  - 指令校验                              │
          │  - 地址映射                              │
          │  - 超时管理                              │
          │  - 确认回调                              │
          └────────────────────┬────────────────────┘
                               ▼
          ┌─────────────────────────────────────────┐
          │           统一数据共享池                 │
          │  - 更新本地缓存（写后读一致）            │
          │  - 触发数据变更通知                      │
          └────────────────────┬────────────────────┘
                               ▼
          ┌─────────────────────────────────────────┐
          │           底层设备通信                   │
          │  - Modbus RTU/TCP 写寄存器              │
          │  - Modbus RTU/TCP 写线圈               │
          └─────────────────────────────────────────┘
```

### 10.3 控制指令定义

```go
// ControlCommand 统一控制指令
type ControlCommand struct {
    // 指令标识
    ID          string    `json:"id"`           // 唯一指令ID (UUID)
    Source      string    `json:"source"`       // 指令来源: "modbus_tcp", "iec61850", "mqtt", "web", "api"
    SourceID    string    `json:"source_id"`    // 来源标识 (如客户端IP、MQTT clientID)

    // 目标信息
    DeviceID    string    `json:"device_id"`    // 目标设备ID
    PointID     string    `json:"point_id"`     // 目标数据点ID (可选，优先使用)
    RegisterType string   `json:"register_type"` // holding/coil (如未指定point_id)
    RegisterAddr uint16   `json:"register_addr"` // 寄存器地址 (如未指定point_id)

    // 控制值
    ValueType   string      `json:"value_type"`   // uint16/int16/float32/int32/bool
    Value       interface{} `json:"value"`         // 控制值
    ByteOrder   string      `json:"byte_order"`    // 字节序 (可选)

    // 元数据
    Priority    int       `json:"priority"`     // 优先级: 0=低, 1=普通, 2=高, 3=紧急
    Timestamp   time.Time `json:"timestamp"`    // 指令创建时间
    Timeout     int       `json:"timeout"`      // 超时时间(毫秒)，0表示默认3000ms
    RequestID   string    `json:"request_id"`   // 请求ID (用于关联响应)

    // 安全
    AuthToken   string    `json:"auth_token"`   // 认证令牌 (可选)
}

// CommandResponse 控制指令响应
type CommandResponse struct {
    CommandID   string    `json:"command_id"`   // 关联的指令ID
    Success     bool      `json:"success"`      // 是否成功
    ErrorCode   string    `json:"error_code"`   // 错误码 (成功时为空)
    ErrorMessage string   `json:"error_message"` // 错误信息
    Timestamp   time.Time `json:"timestamp"`    // 响应时间

    // 写后读验证 (可选)
    ReadBack    interface{} `json:"read_back"`   // 回读值
    ReadBackOK  bool        `json:"read_back_ok"` // 回读是否匹配
}
```

### 10.4 控制适配器接口

```go
// ControlSource 控制指令来源接口
// 各协议实现此接口，接收外部控制指令并转发到命令总线
type ControlSource interface {
    // Init 初始化
    Init(config interface{}, bus *CommandBus) error

    // Start 启动监听
    Start() error

    // Stop 停止
    Stop() error

    // OnCommandResponse 指令响应回调 (用于回传给来源)
    OnCommandResponse(response CommandResponse)

    // IsRunning 是否运行中
    IsRunning() bool
}

// CommandBus 命令总线
type CommandBus struct {
    mu          sync.RWMutex
    queue       chan ControlCommand           // 指令队列
    handlers    []CommandHandler              // 指令处理器列表
    sources     map[string]ControlSource      // 指令来源注册表
    pending     map[string]chan CommandResponse // 等待响应的指令
    logger      *logger.Logger
}

// CommandHandler 命令处理器接口
type CommandHandler interface {
    // CanHandle 是否能处理该指令
    CanHandle(cmd ControlCommand) bool

    // Handle 处理指令
    Handle(cmd ControlCommand) CommandResponse
}

// SubmitCommand 提交控制指令 (由 ControlSource 调用)
func (bus *CommandBus) SubmitCommand(cmd ControlCommand) error

// RegisterSource 注册控制来源
func (bus *CommandBus) RegisterSource(name string, source ControlSource)

// RegisterHandler 注册命令处理器
func (bus *CommandBus) RegisterHandler(handler CommandHandler)

// WaitForResponse 等待指令响应 (带超时)
func (bus *CommandBus) WaitForResponse(commandID string, timeout time.Duration) (CommandResponse, error)
```

### 10.5 Modbus 写入处理器

```go
// ModbusWriteHandler Modbus 写入处理器
type ModbusWriteHandler struct {
    pool        *DataPool
    devices     map[string]*DeviceConnection  // deviceID -> 设备连接
    logger      *logger.Logger
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

    // 2. 解析数据点配置 (如果指定了 PointID)
    var regType string
    var regAddr uint16
    if cmd.PointID != "" {
        entry, exists := h.pool.GetData(cmd.PointID)
        if !exists {
            return CommandResponse{
                CommandID:    cmd.ID,
                Success:      false,
                ErrorCode:    "POINT_NOT_FOUND",
                ErrorMessage: "数据点不存在",
                Timestamp:    time.Now(),
            }
        }
        regType = entry.Point.RegisterType
        regAddr = entry.Point.RegisterAddr
    } else {
        regType = cmd.RegisterType
        regAddr = cmd.RegisterAddr
    }

    // 3. 编码寄存器值
    regValues := encodeCommandValue(cmd.ValueType, cmd.Value, cmd.ByteOrder)

    // 4. 执行写入
    var err error
    if regType == "coil" {
        coilValue := cmd.Value.(bool)
        err = conn.WriteCoil(regAddr, coilValue)
    } else {
        err = conn.WriteRegisters(regAddr, regValues)
    }

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

        // 6. 更新数据共享池 (写后读一致)
        h.pool.UpdateData(cmd.PointID, cmd.Value, QualityGood)

        // 7. 可选: 回读验证
        if cmd.Timeout > 0 {
            readBack, readErr := h.readBack(conn, regType, regAddr, len(regValues))
            if readErr == nil {
                response.ReadBack = readBack
                response.ReadBackOK = h.verifyReadBack(regValues, readBack)
            }
        }
    }

    h.logger.Info("Modbus 写入完成",
        "command_id", cmd.ID,
        "device", cmd.DeviceID,
        "success", response.Success,
        "duration", time.Since(startTime),
    )

    return response
}
```

### 10.6 协议控制来源实现

#### Modbus TCP 控制来源

```go
// ModbusTCPControlSource Modbus TCP 控制来源
// 监听上位机写入请求，转换为统一控制指令
type ModbusTCPControlSource struct {
    bus       *CommandBus
    server    *tcp.Server
    logger    *logger.Logger
}

// OnWriteRequest Modbus TCP 服务器写入回调
func (s *ModbusTCPControlSource) OnWriteRequest(slaveID byte, funcCode byte, addr uint16, values []interface{}) error {
    // 构建控制指令
    cmd := ControlCommand{
        ID:           generateUUID(),
        Source:       "modbus_tcp",
        SourceID:     "client_ip",  // 从连接获取
        DeviceID:     s.resolveDeviceID(slaveID, addr),
        RegisterType: s.resolveRegType(funcCode),
        RegisterAddr: addr,
        ValueType:    "uint16",
        Value:        values,
        Priority:     PriorityNormal,
        Timestamp:    time.Now(),
        Timeout:      3000,
    }

    // 提交到命令总线
    response, err := s.bus.SubmitAndWait(cmd, 3*time.Second)
    if err != nil {
        return err
    }

    if !response.Success {
        return fmt.Errorf("写入失败: %s", response.ErrorMessage)
    }

    return nil
}
```

#### IEC 61850 控制来源

```go
// IEC61850ControlSource IEC 61850 控制来源
type IEC61850ControlSource struct {
    bus       *CommandBus
    manager   *iec61850.Manager
    logger    *logger.Logger
}

// OnControlCommand IEC 61850 控制命令回调
func (s *IEC61850ControlSource) OnControlCommand(path string, value interface{}, ctlModel string) error {
    // 查找对应的数据点
    pointID := s.resolvePointID(path)
    if pointID == "" {
        return fmt.Errorf("未找到 IEC 61850 路径对应的映射: %s", path)
    }

    // 构建控制指令
    cmd := ControlCommand{
        ID:        generateUUID(),
        Source:    "iec61850",
        SourceID:  "iec61850_client",
        PointID:   pointID,
        Value:     value,
        Priority:  PriorityNormal,
        Timestamp: time.Now(),
        Timeout:   5000,
    }

    // 根据控制模型决定处理方式
    switch ctlModel {
    case "direct-with-normal-security":
        response, err := s.bus.SubmitAndWait(cmd, 5*time.Second)
        if err != nil {
            return err
        }
        if !response.Success {
            return fmt.Errorf("控制失败: %s", response.ErrorMessage)
        }
        return nil

    case "sbo-with-normal-security":
        // Select-Before-Operate 模式
        // 先检查是否可操作
        // 再执行操作
        return s.handleSBO(cmd)

    default:
        return fmt.Errorf("不支持的控制模型: %s", ctlModel)
    }
}
```

#### MQTT 控制来源

```go
// MQTTControlSource MQTT 控制来源
type MQTTControlSource struct {
    bus       *CommandBus
    client    mqtt.Client
    topics    []MQTTControlTopic
    logger    *logger.Logger
}

// MQTTControlTopic MQTT 控制主题配置
type MQTTControlTopic struct {
    Topic       string `yaml:"topic"`
    PointID     string `yaml:"point_id"`
    DeviceID    string `yaml:"device_id"`
    QoS         byte   `yaml:"qos"`
}

// onMessage MQTT 消息回调
func (s *MQTTControlSource) onMessage(client mqtt.Client, msg mqtt.Message) {
    // 解析控制主题
    topicConfig := s.findTopicConfig(msg.Topic())
    if topicConfig == nil {
        return
    }

    // 解析消息负载
    var payload struct {
        Value     interface{} `json:"value"`
        ValueType string      `json:"value_type"`
        RequestID string      `json:"request_id"`
    }
    if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
        s.logger.Error("解析 MQTT 控制消息失败", "error", err)
        return
    }

    // 构建控制指令
    cmd := ControlCommand{
        ID:        generateUUID(),
        Source:    "mqtt",
        SourceID:  "mqtt_client",
        PointID:   topicConfig.PointID,
        DeviceID:  topicConfig.DeviceID,
        ValueType: payload.ValueType,
        Value:     payload.Value,
        Priority:  PriorityNormal,
        Timestamp: time.Now(),
        RequestID: payload.RequestID,
    }

    // 提交到命令总线
    response, err := s.bus.SubmitAndWait(cmd, 5*time.Second)
    if err != nil {
        s.logger.Error("MQTT 控制指令执行失败", "error", err)
        return
    }

    // 发布响应
    s.publishResponse(topicConfig, response)
}
```

### 10.7 安全与权限控制

```go
// CommandAuthorizer 命令授权器
type CommandAuthorizer interface {
    Authorize(cmd ControlCommand) (bool, string)
}

// DefaultAuthorizer 默认授权器
type DefaultAuthorizer struct {
    rules []AuthorizationRule
}

// AuthorizationRule 授权规则
type AuthorizationRule struct {
    Source      string   // 来源 (mqtt/iec61850/web等)
    Devices     []string // 允许的设备 (空=全部)
    Points      []string // 允许的数据点 (空=全部)
    PointTags   map[string]string // 按标签筛选
    MaxPriority int      // 最大允许优先级
}

// Authorize 授权检查
func (a *DefaultAuthorizer) Authorize(cmd ControlCommand) (bool, string) {
    for _, rule := range a.rules {
        if a.matchRule(cmd, rule) {
            return true, ""
        }
    }
    return false, "无操作权限"
}
```

### 10.8 控制指令配置

```yaml
# configs/control.yaml
control:
  # 命令队列配置
  queue_size: 1000
  default_timeout: 3000  # 毫秒
  max_concurrent: 10

  # Modbus 写入配置
  modbus_write:
    enabled: true
    write_timeout: 1000  # 写入超时
    retry_count: 2       # 重试次数
    read_back: true      # 写后回读验证

  # IEC 61850 控制配置
  iec61850_control:
    enabled: true
    default_ctl_model: "direct-with-normal-security"

  # MQTT 控制配置
  mqtt_control:
    enabled: false
    control_topics:
      - topic: "control/{device_id}/{point_name}"
        pattern: true  # 支持通配符

  # 权限配置
  authorization:
    enabled: true
    rules:
      - source: "web"
        max_priority: 3  # Web 界面最高优先级
      - source: "mqtt"
        max_priority: 1  # MQTT 普通优先级
        point_tags:
          writable: "true"  # 只允许写入标记为可写的数据点

  # 安全配置
  security:
    rate_limit:
      enabled: true
      max_per_second: 10  # 每秒最大指令数
      max_per_minute: 100 # 每分钟最大指令数
    audit_log:
      enabled: true       # 记录所有控制指令
```

### 10.9 数据点配置扩展

```yaml
# data_points.yaml 扩展
data_points:
  - id: "rtu-device-1.temperature_setpoint"
    device_id: "rtu-device-1"
    name: "temperature_setpoint"
    register_type: "holding"
    register_addr: 10
    data_type: "float32"
    quantity: 2
    description: "温度设定值"
    unit: "°C"

    # 控制相关属性
    writable: true                    # 是否可写
    write_protected: false            # 是否写保护
    min_value: 0.0                    # 最小允许值
    max_value: 100.0                  # 最大允许值
    step: 0.1                         # 步进值
    confirm_required: false           # 是否需要确认

    tags:
      group: "control"
      priority: "high"
```

## 11. 迁移方案（更新）

### 11.1 阶段一：核心框架（1周）

1. 实现 `DataPool` 数据共享池
2. 实现 `UnifiedDataPoint` 统一数据点定义
3. 实现 `OutputAdapter` 接口
4. 编写单元测试

### 11.2 阶段二：采集器改造（1周）

1. 实现 `OptimizedCollector` 优化采集器
2. 实现寄存器合并优化
3. 支持从 `data_points.yaml` 加载配置
4. 保持向后兼容旧配置格式

### 11.3 阶段三：输出适配器迁移（1周）

1. 实现 `ModbusOutputAdapter`
2. 实现 `IEC61850OutputAdapter`
3. 迁移现有功能到新架构
4. 集成测试

### 11.4 阶段四：双向控制框架（1-2周）

1. 实现 `CommandBus` 命令总线
2. 实现 `ModbusWriteHandler` 写入处理器
3. 实现 `ControlSource` 接口
4. 实现 Modbus TCP 控制来源
5. 实现权限控制和审计日志
6. 集成测试

### 11.5 阶段五：协议控制来源（2周）

1. 实现 IEC 61850 控制来源
2. 实现 MQTT 控制来源
3. 实现 Web API 控制来源
4. 端到端测试

### 11.6 阶段六：配置迁移工具（可选）

1. 编写配置迁移工具
2. 更新 Web 管理界面
3. 文档更新

## 12. 优势总结（更新）

### 12.1 完全解耦

- 上行采集和下行控制完全分离
- 各协议独立实现，互不影响
- 新增协议只需实现对应接口

### 12.2 统一管理

- 所有控制指令通过统一命令总线
- 统一的权限控制和审计
- 统一的超时和重试机制

### 12.3 高可扩展性

- 新增控制来源：实现 `ControlSource` 接口
- 新增控制目标：实现 `CommandHandler` 接口
- 配置驱动，无需修改核心代码

### 12.4 安全可靠

- 细粒度的权限控制
- 写后读验证
- 完整的审计日志
- 速率限制防止误操作

## 13. 风险与挑战（更新）

### 13.1 向后兼容

- 需要支持旧配置格式的平滑迁移
- 建议提供配置迁移工具

### 13.2 实时性

- 命令总线引入额外延迟
- 对于毫秒级控制响应，可能需要直接通道

### 13.3 复杂度

- 双向控制增加了系统复杂度
- 需要充分的测试覆盖

### 13.4 安全性

- 控制指令涉及设备安全
- 需要严格的权限控制和审计

## 14. 后续扩展（更新）

### 14.1 支持的协议

**上行采集：**
- [x] Modbus RTU/TCP
- [ ] IEC 61850 (作为数据源)
- [ ] OPC UA
- [ ] MQTT (订阅)

**下行输出：**
- [x] Modbus TCP/RTU
- [x] IEC 61850
- [ ] MQTT
- [ ] OPC UA
- [ ] HTTP Webhook
- [ ] InfluxDB/TimescaleDB
- [ ] WebSocket

**控制来源：**
- [x] Modbus TCP (上位机写)
- [ ] IEC 61850 客户端
- [ ] MQTT 订阅
- [ ] Web API
- [ ] 定时任务
- [ ] 报警联动

### 14.2 高级功能

- [ ] 数据点分组和批量操作
- [ ] 数据过滤和聚合
- [ ] 历史数据查询
- [ ] 报警规则引擎集成
- [ ] 数据点标签系统
- [ ] 控制指令队列优先级
- [ ] 批量控制指令
- [ ] 控制指令回滚
- [ ] 操作票系统集成
