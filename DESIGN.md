# 通讯管理机平台 - 系统设计文档

## 一、项目概述

### 1.1 项目名称

通讯管理机平台 (ComManager)

### 1.2 项目定位

一款通用型工业物联网网关/通讯管理机，支持多协议采集、协议转换、灵活分组输出。

### 1.3 核心能力

- 多串口设备接入 (Modbus RTU / 自定义协议)
- 多网口设备接入 (Modbus TCP / IEC61850 / 自定义协议)
- 协议转换: Modbus RTU ↔ Modbus TCP ↔ IEC61850
- 灵活分组输出: 任意输入 → 任意输出
- 数据映射: 点表驱动，配置文件定义数据流转
- 断点续传: 从机连接断开时本地缓存数据，恢复后主动补传上报

### 1.4 技术选型

| 项目 | 选型 |
|------|------|
| 语言 | Go 1.21+ |
| 配置格式 | YAML |
| Web 管理 | Vue3 + Gin (可选) |
| 日志 | zap / slog |
| 数据库 | SQLite (轻量) / 可选 PostgreSQL |

---

## 二、模块化架构

### 2.1 模块总览

```
com-manager/
├── cmd/                          # 入口
│   └── gateway/
│       └── main.go               # 程序入口
│
├── internal/                     # 核心业务 (不对外暴露)
│   ├── core/                     # 核心引擎
│   ├── protocol/                 # 协议层
│   ├── driver/                   # 驱动层 (硬件通信)
│   ├── mapping/                  # 数据映射引擎
│   ├── group/                    # 分组管理
│   ├── storage/                  # 数据存储
│   └── web/                      # Web 管理 API
│
├── pkg/                          # 公共库 (可对外暴露)
│   ├── logger/                   # 日志
│   ├── config/                   # 配置加载
│   ├── model/                    # 数据模型定义
│   ├── errors/                   # 错误定义
│   └── utils/                    # 工具函数
│
├── configs/                      # 配置文件
│   ├── gateway.yaml              # 主配置
│   ├── devices/                  # 设备配置
│   ├── groups/                   # 分组配置
│   └── mappings/                 # 点表映射
│
├── web/                          # 前端 (可选)
│   └── frontend/
│
├── scripts/                      # 脚本
│   ├── build.sh                  # 编译脚本
│   └── cross-compile.sh          # 交叉编译
│
├── docs/                         # 文档
├── DESIGN.md                     # 本文档
├── CLAUDE.md                     # 项目约定
├── go.mod
└── go.sum
```

---

### 2.2 模块详细设计

#### 模块 1: core (核心引擎)

```
internal/core/
├── engine.go              # 引擎主循环, 生命周期管理
├── scheduler.go           # 任务调度, 定时轮询
├── router.go              # 数据路由, 决定数据去向
└── pipeline.go            # 数据处理管道 (采集→转换→输出)
```

职责:
- 程序生命周期管理 (启动/停止/重载配置)
- 调度各驱动的采集周期
- 数据路由: 收到数据后根据映射表决定发往哪些输出
- 管理所有模块的依赖关系

```go
// 引擎接口定义
type Engine interface {
    Start() error
    Stop() error
    Reload() error  // 热重载配置
    Status() EngineStatus
}
```

---

#### 模块 2: protocol (协议层)

```
internal/protocol/
├── modbus/                    # Modbus 协议栈
│   ├── rtu/
│   │   ├── master.go          # RTU 主站 (采集串口设备)
│   │   └── slave.go           # RTU 从站 (串口输出, 模拟从站)
│   ├── tcp/
│   │   ├── client.go          # TCP 客户端 (采集网口设备)
│   │   └── server.go          # TCP 服务端 (网口输出, 模拟从站)
│   ├── frame.go               # Modbus 帧编解码
│   ├── function.go            # 功能码定义 (03/04/06/10等)
│   └── register.go            # 寄存器模型 (线圈/离散/输入/保持)
│
├── iec61850/                  # IEC 61850 协议栈
│   ├── mms/
│   │   ├── client.go          # MMS 客户端
│   │   └── server.go          # MMS 服务端
│   ├── goose/
│   │   ├── publisher.go       # GOOSE 发布
│   │   └── subscriber.go      # GOOSE 订阅
│   ├── model/
│   │   ├── ld.go              # 逻辑设备
│   │   ├── ln.go              # 逻辑节点
│   │   ├── doi.go             # 数据对象
│   │   └── dai.go             # 数据属性
│   ├── scl/
│   │   ├── parser.go          # SCL 文件解析 (ICD/CID/SCD)
│   │   └── builder.go         # SCL 文件生成
│   └── convert.go             # IEC61850 ↔ 通用数据模型转换
│
├── common/                    # 协议公共部分
│   ├── codec.go               # 编解码接口
│   └── types.go               # 协议公共类型
│
└── registry.go                # 协议注册表 (可扩展)
```

职责:
- 每种协议独立封装
- 统一的采集/输出接口
- 协议间转换通过通用数据模型中转

```go
// 采集器接口 (所有输入协议实现此接口)
type Collector interface {
    Connect() error
    Disconnect() error
    Read(req ReadRequest) ([]DataPoint, error)
    IsConnected() bool
}

// 输出器接口 (所有输出协议实现此接口)
type Outputter interface {
    Listen() error
    Close() error
    Write(points []DataPoint) error
    GetSlaveInfo() SlaveInfo
}
```

---

#### 模块 3: driver (驱动层)

```
internal/driver/
├── serial/
│   ├── port.go               # 串口管理 (打开/关闭/配置)
│   ├── pool.go               # 串口连接池 (多设备共享串口)
│   └── watcher.go            # 串口热插拔检测
│
├── network/
│   ├── tcp_conn.go           # TCP 连接管理
│   ├── udp_conn.go           # UDP 连接管理
│   ├── pool.go               # 网络连接池
│   └── keepalive.go          # 心跳/断线重连
│
└── raw/
    ├── raw_socket.go         # 原始套接字 (GOOSE 需要)
    └── ethernet.go           # 以太网帧操作
```

职责:
- 硬件通信的底层操作
- 连接池管理, 复用连接
- 断线重连, 超时控制
- 串口热插拔检测

---

#### 模块 4: mapping (数据映射引擎)

```
internal/mapping/
├── point_table.go            # 点表定义
├── converter.go              # 数据类型转换 (int16→float32等)
├── scaler.go                 # 缩放/偏移计算
├── merger.go                 # 多寄存器合并 (如两个16位→float32)
├── splitter.go               # 寄存器拆分
└── validator.go              # 点表校验
```

职责:
- 定义数据点映射关系
- 数据类型转换 (大端/小端, 16位/32位/浮点)
- 缩放系数和偏移量计算
- 多寄存器合并/拆分

```go
// 点表条目
type PointEntry struct {
    Name         string        `yaml:"name"`          // 数据点名称
    Description  string        `yaml:"description"`   // 描述
    Source       PointSource   `yaml:"source"`        // 来源
    Target       PointTarget   `yaml:"target"`        // 目标
    DataType     string        `yaml:"data_type"`     // 数据类型
    Scale        float64       `yaml:"scale"`         // 缩放系数
    Offset       float64       `yaml:"offset"`        // 偏移量
    ByteOrder    string        `yaml:"byte_order"`    // 字节序
    ReadOnly     bool          `yaml:"read_only"`     // 只读
}

type PointSource struct {
    Protocol    string `yaml:"protocol"`     // modbus-rtu / modbus-tcp / iec61850
    DeviceID    string `yaml:"device_id"`    // 设备标识
    Register    int    `yaml:"register"`     // 寄存器地址
    Function    int    `yaml:"function"`     // 功能码
}

type PointTarget struct {
    Protocol    string `yaml:"protocol"`     // 输出协议
    GroupID     string `yaml:"group_id"`     // 所属分组
    Register    int    `yaml:"register"`     // 输出寄存器地址
    SlaveID     int    `yaml:"slave_id"`     // 从站地址
}
```

---

#### 模块 5: group (分组管理)

```
internal/group/
├── manager.go                # 分组管理器
├── group.go                  # 分组定义
├── port_allocator.go         # 端口分配器
└── policy.go                 # 分组策略 (有序/无序/统一/分离)
```

职责:
- 管理设备分组 (如 GIS-1 + 铁芯-1 → 组1)
- 为每组分配输出端口 (如 50-59)
- 支持多种分组策略

```go
// 分组定义
type Group struct {
    ID          string         `yaml:"id"`           // 组ID
    Name        string         `yaml:"name"`         // 组名称
    Devices     []string       `yaml:"devices"`      // 包含的设备列表
    Output      GroupOutput    `yaml:"output"`       // 输出配置
    Policy      string         `yaml:"policy"`       // 分组策略
}

type GroupOutput struct {
    Protocol    string `yaml:"protocol"`      // modbus-tcp / modbus-rtu
    Port        int    `yaml:"port"`          // TCP端口 或 串口号
    SlaveID     int    `yaml:"slave_id"`      // 从站地址
    BaudRate    int    `yaml:"baud_rate"`     // 串口波特率 (RTU时)
}

// 分组策略
const (
    PolicyOrdered   = "ordered"    // 有序: 按设备顺序排列寄存器
    PolicyUnordered = "unordered"  // 无序: 设备可任意分配寄存器段
    PolicyUnified   = "unified"    // 统一: 所有设备合并到一个输出
    PolicySeparated = "separated"  // 分离: 每个设备独立输出
)
```

---

#### 模块 6: storage (数据存储)

```
internal/storage/
├── database/
│   ├── sqlite.go             # SQLite 实现
│   └── interface.go          # 数据库接口
├── cache/
│   ├── memory.go             # 内存缓存 (最新值)
│   └── ring_buffer.go        # 环形缓冲 (历史数据)
├── history/
│   ├── recorder.go           # 历史数据记录
│   └── query.go              # 历史数据查询
├── buffer/                   # ★ 断点续传缓冲模块
│   ├── store.go              # 离线数据写入 (按时间戳存储)
│   ├── loader.go             # 离线数据加载 (按时间范围查询)
│   ├── cleaner.go            # 过期数据清理 (自动删除超期数据)
│   ├── reporter.go           # 数据补传器 (连接恢复后主动上报)
│   └── queue.go              # 内存队列 (缓存未落盘的最新数据)
└── alarm/
    ├── detector.go           # 报警检测
    ├── handler.go            # 报警处理
    └── record.go             # 报警记录
```

职责:
- 设备最新值缓存 (内存)
- 历史数据存储 (SQLite)
- 报警检测与记录
- 断电数据保持
- **断点续传: 从机离线时数据本地缓存，恢复后主动补传**

---

#### 模块 7: web (Web 管理 API + 前端页面)

```
internal/web/
├── server.go                 # HTTP 服务器 (Gin), 静态文件托管
├── handler/
│   ├── device.go             # 设备管理 API
│   ├── group.go              # 分组管理 API
│   ├── mapping.go            # 点表管理 API
│   ├── output.go             # 输出配置 API
│   ├── buffer.go             # 断点续传状态 API
│   ├── monitor.go            # 实时监控 API (WebSocket)
│   ├── alarm.go              # 报警管理 API
│   └── system.go             # 系统管理 API
├── middleware/
│   ├── auth.go               # 认证中间件 (用户名密码)
│   └── cors.go               # CORS 中间件
├── dto/
│   ├── request.go            # 请求参数结构体
│   └── response.go           # 响应结构体
└── ws/
    └── realtime.go           # WebSocket 实时数据推送

web/frontend/                 # 前端静态页面 (纯 HTML + JS, 无框架)
├── index.html                # 主页 (导航框架)
├── css/
│   └── style.css             # 样式
├── js/
│   ├── app.js                # 公共逻辑 (API封装, 通用工具)
│   ├── device.js             # 设备管理页逻辑
│   ├── group.js              # 分组管理页逻辑
│   ├── mapping.js            # 点表管理页逻辑
│   ├── output.js             # 输出配置页逻辑
│   ├── monitor.js            # 实时监控页逻辑
│   └── system.js             # 系统设置页逻辑
└── pages/
    ├── device.html           # 设备管理页
    ├── group.html            # 分组管理页
    ├── mapping.html          # 点表管理页
    ├── output.html           # 输出配置页
    ├── monitor.html          # 实时监控页
    ├── buffer.html           # 断点续传状态页
    ├── alarm.html            # 报警记录页
    └── system.html           # 系统设置页
```

职责:
- 提供 RESTful API 供前端和其他系统调用
- 托管前端静态页面
- 设备/分组/点表/输出的在线配置
- 实时数据监控 (WebSocket)
- 断点续传状态查看
- 配置变更后自动生效 (热重载)

前端选型: **纯 HTML + CSS + JavaScript (原生)**, 不依赖 Vue/React 框架
- 轻量, 嵌入式设备资源占用小
- 使用 Bootstrap 或 Layui 做简单 UI 美化
- 所有业务逻辑通过 AJAX 调用后端 API

---

### 2.3 公共库 (pkg)

```
pkg/
├── logger/
│   ├── logger.go             # 日志初始化 (zap/slog)
│   └── rotate.go             # 日志轮转
│
├── config/
│   ├── loader.go             # YAML 配置加载
│   ├── watcher.go            # 配置文件热监听
│   └── validator.go          # 配置校验
│
├── model/
│   ├── data_point.go         # 通用数据点模型
│   ├── device.go             # 设备模型
│   ├── register.go           # 寄存器模型
│   └── alarm.go              # 报警模型
│
├── errors/
│   └── errors.go             # 统一错误定义
│
└── utils/
    ├── hex.go                # 十六进制工具
    ├── convert.go            # 数据类型转换
    ├── crc.go                # CRC 校验
    └── retry.go              # 重试工具
```

---

## 三、数据流转模型

### 3.1 通用数据点 (DataPoint)

所有协议的数据都转换为统一的 DataPoint 进行流转:

```go
type DataPoint struct {
    DeviceID    string      // 设备唯一标识
    Name        string      // 数据点名称
    Value       interface{} // 值 (支持多种类型)
    Quality     Quality     // 数据质量
    Timestamp   time.Time   // 时间戳
    DataType    DataType    // 数据类型
}
```

### 3.2 数据流转管道

```
输入协议                通用模型              输出协议
─────────            ─────────            ─────────

Modbus RTU  ─┐                           ┌─→ Modbus RTU
Modbus TCP  ─┤→ Protocol Adapter ─┐      ├─→ Modbus TCP
IEC61850    ─┤   (协议→DataPoint) │      │   (DataPoint→协议)
自定义协议   ─┘                    │      │
                                  ▼      │
                            ┌──────────┐ │
                            │ DataPoint│ │
                            │  Router  │─┘
                            │ (路由引擎)│
                            └────┬─────┘
                                 │
                            ┌────▼─────┐
                            │  Mapping  │
                            │ (映射转换) │
                            └────┬─────┘
                                 │
                            ┌────▼─────┐
                            │  Group   │
                            │ (分组管理) │
                            └──────────┘
```

---

## 四、配置文件设计

### 4.1 主配置 (gateway.yaml)

```yaml
# 通讯管理机主配置
server:
  name: "com-manager-01"
  log_level: "info"
  log_path: "./logs"

# Web 管理配置
web:
  enabled: true
  port: 8080                     # Web 服务端口
  host: "0.0.0.0"                # 监听地址 (0.0.0.0 允许远程访问)
  auth:
    enabled: true                # 是否启用登录认证
    username: "admin"            # 默认用户名
    password: "admin123"         # 默认密码 (首次登录强制修改)
    token_secret: "change-me"    # JWT 密钥
    token_expire: "24h"          # 登录有效期
  cors:
    enabled: true                # 允许跨域 (其他电脑访问)
    allowed_origins: ["*"]       # 允许的来源

# 串口设备列表
serial_devices:
  - id: "gis-1"
    name: "局放GIS设备1"
    port: "COM3"              # Windows: COM3  Linux: /dev/ttyUSB0
    baud_rate: 9600
    data_bits: 8
    stop_bits: 1
    parity: "none"
    protocol: "modbus-rtu"
    slave_id: 1
    poll_interval: "5s"
    timeout: "3s"
    retry: 3

  - id: "iron-core-1"
    name: "铁芯设备1"
    port: "COM4"
    baud_rate: 9600
    data_bits: 8
    stop_bits: 1
    parity: "none"
    protocol: "modbus-rtu"
    slave_id: 1
    poll_interval: "5s"
    timeout: "3s"
    retry: 3

# 网口设备列表
network_devices:
  - id: "gis-net-1"
    name: "局放GIS网口设备1"
    host: "192.168.1.100"
    port: 502
    protocol: "modbus-tcp"
    slave_id: 1
    poll_interval: "5s"
    timeout: "3s"
    retry: 3

  - id: "ied-1"
    name: "智能电子设备1"
    host: "192.168.2.100"
    port: 102
    protocol: "iec61850-mms"
    poll_interval: "10s"
    timeout: "5s"
    cid_file: "./configs/ied-1.cid"

# 输出配置
outputs:
  modbus_tcp_servers:
    - id: "output-group-1"
      name: "输出组1"
      listen_port: 50         # 对外监听端口 :50
      slave_id: 1
      max_connections: 10

    - id: "output-group-2"
      name: "输出组2"
      listen_port: 51
      slave_id: 1
      max_connections: 10

    # ... 到 :59

  modbus_rtu_servers:
    - id: "output-serial-1"
      name: "串口输出1"
      port: "COM10"
      baud_rate: 9600
      slave_id: 1
```

### 4.2 分组配置 (groups/group-1.yaml)

```yaml
# 分组1: GIS-1 + 铁芯-1 → 端口50
id: "group-1"
name: "第1组 - GIS1+铁芯1"
policy: "ordered"             # ordered / unordered / unified / separated

# 包含的设备
devices:
  - device_id: "gis-1"
    role: "primary"
  - device_id: "iron-core-1"
    role: "secondary"

# 输出配置
output:
  protocol: "modbus-tcp"
  port: 50
  slave_id: 1

# 寄存器分配策略
register_range:
  gis_start: 0                # GIS 设备起始寄存器
  gis_count: 100              # GIS 设备占 100 个寄存器
  iron_start: 100             # 铁芯设备起始寄存器
  iron_count: 100             # 铁芯设备占 100 个寄存器
```

### 4.3 点表配置 (mappings/gis-1.yaml)

```yaml
# GIS-1 设备点表
device_id: "gis-1"
description: "局放GIS设备1 点表映射"

points:
  - name: "partial_discharge_a"
    description: "A相局放量"
    source:
      protocol: "modbus-rtu"
      function: 3             # 读保持寄存器
      register: 0
      count: 1                # 1个寄存器
    target:
      protocol: "modbus-tcp"
      group_id: "group-1"
      register: 0
      slave_id: 1
    data_type: "uint16"
    byte_order: "big"
    scale: 0.1
    offset: 0
    unit: "pC"

  - name: "partial_discharge_b"
    description: "B相局放量"
    source:
      protocol: "modbus-rtu"
      function: 3
      register: 1
      count: 1
    target:
      protocol: "modbus-tcp"
      group_id: "group-1"
      register: 1
      slave_id: 1
    data_type: "uint16"
    byte_order: "big"
    scale: 0.1
    offset: 0
    unit: "pC"

  - name: "temperature"
    description: "设备温度"
    source:
      protocol: "modbus-rtu"
      function: 3
      register: 10
      count: 2                 # 2个寄存器合并为 float32
    target:
      protocol: "modbus-tcp"
      group_id: "group-1"
      register: 10
      slave_id: 1
    data_type: "float32"
    byte_order: "big_endian_word_swap"
    scale: 0.01
    offset: -40.0
    unit: "°C"
```

---

## 五、协议转换设计

### 5.1 Modbus RTU ↔ Modbus TCP

```
Modbus RTU 帧:  [设备地址][功能码][数据][CRC16]
Modbus TCP 帧:  [MBAP头][功能码][数据]

转换只需要:
  1. 去掉 CRC, 加上 MBAP 头 (或反向)
  2. 地址映射 (串口从站地址 → TCP 端口/从站地址)
```

### 5.2 Modbus ↔ IEC61850

```
Modbus 寄存器          IEC61850 数据模型
──────────            ──────────────────
HR0 (uint16)    ←→    GGIO1.AnIn1.mag.f     (模拟量)
HR1 (uint16)    ←→    GGIO1.AnIn2.mag.f     (模拟量)
Coil0 (bool)    ←→    GGIO1.Ind1.stVal      (状态量)
Coil1 (bool)    ←→    GGIO1.Ind2.stVal      (状态量)

转换流程:
  1. 解析 SCL/CID 文件, 建立 IEC61850 数据模型
  2. 读取 Modbus 寄存器值
  3. 按映射表写入对应的 IEC61850 数据对象
  4. 数据类型转换 (uint16 → float, bool → boolean)
```

---

## 六、断点续传设计 (数据补传)

### 6.1 业务场景

通讯管理机作为**从机**对外输出数据时 (Modbus TCP Server / Modbus RTU Slave)，与**主机** (SCADA/组态/DCS) 的连接可能中断。

```
正常状态:
  通讯管理机(从机) ──连接正常──→ 主机(SCADA)
  实时数据正常上报

断线状态:
  通讯管理机(从机) ──连接断开──✗ 主机(SCADA)
  数据写入本地 SQLite 缓存 (带时间戳)

恢复状态:
  通讯管理机(从机) ──连接恢复──→ 主机(SCADA)
  1. 恢复实时数据上报
  2. 主动补传断线期间的历史数据 (按时间顺序)
```

### 6.2 数据补传流程

```
┌─────────────────────────────────────────────────────────────┐
│                      断点续传流程                             │
│                                                             │
│  ┌──────────┐    正常输出     ┌──────────┐                  │
│  │ 数据采集  │───────────────→│ 主机连接  │──→ 主机(SCADA)    │
│  └──────────┘                └────┬─────┘                  │
│       │                          │                         │
│       │                     连接断开?                        │
│       │                          │                         │
│       │                          ▼                         │
│       │                   ┌──────────┐                     │
│       │                   │ 切换模式  │                     │
│       │                   └────┬─────┘                     │
│       │                        │                           │
│       ▼                        ▼                           │
│  ┌──────────┐           ┌──────────┐                       │
│  │ 数据采集  │──写入──→  │ 离线缓冲  │ (SQLite, 带时间戳)    │
│  │ (继续运行)│           │          │                       │
│  └──────────┘           └────┬─────┘                       │
│                              │                             │
│                         连接恢复?                            │
│                              │                             │
│                              ▼                             │
│                       ┌──────────┐                         │
│                       │ 补传调度  │                         │
│                       └────┬─────┘                         │
│                            │                               │
│                 ┌──────────┼──────────┐                    │
│                 ▼          ▼          ▼                    │
│            按时间顺序   批量发送    发送确认                  │
│            读取缓存    (避免拥塞)  (标记已传)                │
│                                                             │
│  补传策略:                                                   │
│  1. 优先恢复实时数据上报                                      │
│  2. 后台异步补传历史数据                                      │
│  3. 按时间戳从小到大顺序发送                                  │
│  4. 每批数据发送间隔可配置 (避免拥塞主机)                      │
│  5. 补传完成后切换回纯实时模式                                │
└─────────────────────────────────────────────────────────────┘
```

### 6.3 存储设计

```go
// 离线数据记录 (SQLite 表结构)
// 表名: offline_data
type OfflineRecord struct {
    ID          int64     `db:"id"`           // 自增主键
    GroupID     string    `db:"group_id"`     // 所属分组 (哪个输出端口)
    DeviceID    string    `db:"device_id"`    // 设备标识
    PointName   string    `db:"point_name"`   // 数据点名称
    Value       []byte    `db:"value"`        // 值 (序列化)
    DataType    string    `db:"data_type"`    // 数据类型
    Quality     int       `db:"quality"`      // 数据质量
    Timestamp   int64     `db:"timestamp"`    // 数据采集时间戳 (Unix ms)
    CreatedAt   int64     `db:"created_at"`   // 写入缓冲时间戳
    Transmitted bool      `db:"transmitted"`  // 是否已补传
    TransAt     int64     `db:"trans_at"`     // 补传完成时间戳
}

// 索引:
//   idx_group_time     (group_id, timestamp)  -- 按组+时间查询
//   idx_transmitted    (transmitted)           -- 查询未补传数据
//   idx_created_at     (created_at)            -- 过期清理
```

### 6.4 核心接口

```go
// 断点续传缓冲存储
type OfflineBuffer interface {
    // 写入离线数据 (连接断开时调用)
    Store(records []OfflineRecord) error

    // 查询未补传的数据 (按时间顺序)
    LoadUntransmitted(groupID string, limit int) ([]OfflineRecord, error)

    // 标记已补传
    MarkTransmitted(ids []int64) error

    // 查询最早未补传的时间戳
    EarliestUntransmitted(groupID string) (time.Time, error)

    // 统计未补传数据量
    CountUntransmitted(groupID string) (int64, error)

    // 清理过期数据 (超过保留天数)
    Cleanup(retentionDays int) (int64, error)

    // 获取缓冲数据大小
    Size() (int64, error)
}

// 数据补传器
type DataReporter interface {
    // 启动补传 (连接恢复时调用)
    StartReport(ctx context.Context) error

    // 停止补传
    StopReport()

    // 补传是否完成
    IsCompleted() bool

    // 补传进度
    Progress() ReportProgress
}

type ReportProgress struct {
    Total       int64     // 待补传总数
    Completed   int64     // 已补传数
    Failed      int64     // 补传失败数
    StartTime   time.Time // 补传开始时间
    Estimated   time.Duration // 预计剩余时间
}

// 输出器接口 (扩展, 新增连接状态管理)
type Outputter interface {
    Listen() error
    Close() error
    Write(points []DataPoint) error
    GetSlaveInfo() SlaveInfo
    Protocol() string

    // ★ 新增: 连接状态管理
    IsMasterConnected() bool                              // 主机是否连接
    OnMasterConnected(callback func())                    // 主机连接回调
    OnMasterDisconnected(callback func())                 // 主机断开回调
    GetOfflineBuffer() OfflineBuffer                      // 获取离线缓冲
}
```

### 6.5 补传调度策略

```go
type ReportStrategy struct {
    BatchSize       int           `yaml:"batch_size"`        // 每批发送条数, 默认 100
    BatchInterval   time.Duration `yaml:"batch_interval"`    // 批次间隔, 默认 500ms
    MaxRetries      int           `yaml:"max_retries"`       // 单条最大重试次数, 默认 3
    PriorityMode    string        `yaml:"priority_mode"`     // 优先级模式: time / device
    ConcurrentGroup int           `yaml:"concurrent_group"`  // 并发补传的组数, 默认 1
}
```

补传优先级:
- **time 模式** (默认): 按时间戳从小到大，先产生的数据先补传
- **device 模式**: 按设备优先级，重要设备的数据先补传

### 6.6 配置示例

```yaml
# gateway.yaml 中新增配置
offline_buffer:
  enabled: true
  db_path: "./data/offline.db"       # SQLite 文件路径
  retention_days: 10                  # 数据保留天数 (至少10天)
  max_db_size_mb: 500                 # 数据库最大容量 (MB)
  memory_queue_size: 10000            # 内存队列大小 (待落盘)
  flush_interval: "1s"                # 内存队列刷盘间隔

  report_strategy:
    batch_size: 100                   # 每批补传条数
    batch_interval: "500ms"           # 批次间隔
    max_retries: 3                    # 单条重试次数
    priority_mode: "time"             # 优先级: time / device
    concurrent_group: 1               # 并发补传组数

  # 每个输出组可单独配置
  group_override:
    output-group-1:
      retention_days: 15              # 组1保留15天
      priority_mode: "device"
    output-group-2:
      retention_days: 10
```

### 6.7 容量估算

```
假设场景:
  - 10个输出组, 每组2台设备, 每台设备20个数据点
  - 采集周期 5秒
  - 每条记录约 200 字节

计算:
  每秒数据量 = 10组 × 2设备 × 20点 / 5秒 = 80 条/秒
  每天数据量 = 80 × 86400 = 6,912,000 条/天
  10天数据量 = 69,120,000 条
  10天存储空间 = 69,120,000 × 200字节 ≈ 13.8 GB

优化措施:
  1. 值未变化时不存储 (变化检测)
  2. 定期压缩 (SQLite VACUUM)
  3. 超过保留天数自动清理
  4. 可配置最大数据库容量

优化后估算 (变化率30%):
  实际存储 = 13.8 GB × 30% ≈ 4.1 GB (10天)
```

---

## 七、Web 管理 API 设计

### 7.1 访问方式

```
浏览器访问: http://<通讯管理机IP>:8080

示例:
  http://192.168.1.100:8080         → 跳转到设备管理页
  http://192.168.1.100:8080/api/v1  → API 根路径

认证方式: 用户名 + 密码 (首次登录可配置)
```

### 7.2 API 总览

```
/api/v1/
├── auth/                         # 认证
│   ├── POST   /login             # 登录
│   └── POST   /logout            # 登出
│
├── devices/                      # 设备管理
│   ├── GET    /                  # 获取设备列表
│   ├── GET    /:id               # 获取单个设备
│   ├── POST   /                  # 新增设备
│   ├── PUT    /:id               # 修改设备
│   ├── DELETE /:id               # 删除设备
│   └── GET    /:id/status        # 获取设备在线状态
│
├── groups/                       # 分组管理
│   ├── GET    /                  # 获取分组列表
│   ├── GET    /:id               # 获取单个分组
│   ├── POST   /                  # 新增分组
│   ├── PUT    /:id               # 修改分组
│   ├── DELETE /:id               # 删除分组
│   └── GET    /:id/devices       # 获取分组内设备
│
├── mappings/                     # 点表管理
│   ├── GET    /                  # 获取点表列表 (按设备)
│   ├── GET    /:deviceId         # 获取指定设备点表
│   ├── PUT    /:deviceId         # 更新设备点表
│   ├── POST   /:deviceId/import  # 导入点表 (CSV/JSON)
│   └── GET    /:deviceId/export  # 导出点表 (CSV/JSON)
│
├── outputs/                      # 输出配置
│   ├── GET    /                  # 获取输出配置列表
│   ├── GET    /:id               # 获取单个输出配置
│   ├── POST   /                  # 新增输出
│   ├── PUT    /:id               # 修改输出
│   └── DELETE /:id               # 删除输出
│
├── monitor/                      # 实时监控
│   ├── GET    /realtime          # WebSocket 实时数据推送
│   ├── GET    /devices           # 所有设备实时状态
│   └── GET    /devices/:id       # 单设备实时数据
│
├── buffer/                       # 断点续传状态
│   ├── GET    /status            # 各输出组补传状态
│   ├── GET    /status/:groupId   # 指定组补传状态
│   └── POST   /retry/:groupId    # 手动触发重传
│
├── alarm/                        # 报警管理
│   ├── GET    /                  # 报警记录列表
│   ├── PUT    /:id/ack           # 确认报警
│   └── GET    /stats             # 报警统计
│
└── system/                       # 系统管理
    ├── GET    /info              # 系统信息 (版本/运行时间/CPU/内存)
    ├── GET    /config            # 获取系统配置
    ├── PUT    /config            # 更新系统配置
    ├── POST   /config/reload     # 热重载配置
    ├── GET    /logs               # 查看日志 (最近N条)
    ├── POST   /backup            # 备份配置 (导出)
    └── POST   /restore           # 恢复配置 (导入)
```

### 7.3 关键 API 详细设计

#### 设备管理

```json
// POST /api/v1/devices - 新增设备
{
    "id": "gis-1",
    "name": "局放GIS设备1",
    "type": "serial",                // serial / network
    "protocol": "modbus-rtu",        // modbus-rtu / modbus-tcp / iec61850-mms
    "connection": {
        "port": "COM3",              // 串口号 (serial 类型)
        "baud_rate": 9600,
        "data_bits": 8,
        "stop_bits": 1,
        "parity": "none"
        // 或网络类型:
        // "host": "192.168.1.100",
        // "port": 502
    },
    "slave_id": 1,
    "poll_interval": "5s",
    "timeout": "3s",
    "retry": 3,
    "enabled": true
}

// GET /api/v1/devices/:id/status - 设备状态
{
    "id": "gis-1",
    "online": true,
    "last_poll": "2026-05-25T14:30:00Z",
    "error_count": 0,
    "last_error": ""
}
```

#### 分组管理

```json
// POST /api/v1/groups - 新增分组
{
    "id": "group-1",
    "name": "第1组 - GIS1+铁芯1",
    "devices": [
        {"device_id": "gis-1", "role": "primary"},
        {"device_id": "iron-core-1", "role": "secondary"}
    ],
    "output": {
        "protocol": "modbus-tcp",
        "port": 50,
        "slave_id": 1
    },
    "register_range": {
        "gis_start": 0,
        "gis_count": 100,
        "iron_start": 100,
        "iron_count": 100
    }
}
```

#### 点表管理

```json
// PUT /api/v1/mappings/gis-1 - 更新点表
{
    "points": [
        {
            "name": "partial_discharge_a",
            "description": "A相局放量",
            "source": {
                "function": 3,
                "register": 0,
                "count": 1
            },
            "target": {
                "group_id": "group-1",
                "register": 0,
                "slave_id": 1
            },
            "data_type": "uint16",
            "byte_order": "big",
            "scale": 0.1,
            "offset": 0,
            "unit": "pC"
        }
    ]
}

// GET /api/v1/mappings/gis-1/export - 导出点表
// 返回 CSV 或 JSON 文件下载
```

#### 实时监控 (WebSocket)

```javascript
// WebSocket 连接
ws://192.168.1.100:8080/api/v1/monitor/realtime

// 服务端推送数据格式
{
    "type": "data",                   // data / alarm / status
    "device_id": "gis-1",
    "points": [
        {
            "name": "partial_discharge_a",
            "value": 12.5,
            "quality": "good",
            "timestamp": "2026-05-25T14:30:00Z"
        }
    ]
}

// 设备状态变更推送
{
    "type": "status",
    "device_id": "gis-1",
    "online": false,
    "reason": "connection timeout"
}
```

#### 系统信息

```json
// GET /api/v1/system/info
{
    "version": "1.0.0",
    "uptime": "72h30m",
    "cpu_usage": 5.2,
    "memory_usage": 45.8,
    "memory_total_mb": 2048,
    "disk_usage": 1.2,
    "disk_total_mb": 16384,
    "device_count": 20,
    "group_count": 10,
    "online_devices": 18,
    "offline_devices": 2,
    "offline_buffer": {
        "total_records": 12345,
        "transmitted": 10000,
        "pending": 2345
    }
}
```

### 7.4 前端页面设计

```
页面结构 (轻量级, 功能优先):

┌─────────────────────────────────────────────────────┐
│  通讯管理机平台 v1.0                    [用户名] [退出]│
├──────────┬──────────────────────────────────────────┤
│          │                                          │
│  导航栏   │           内容区域                        │
│          │                                          │
│  ● 设备   │  根据左侧导航切换不同页面                  │
│  ● 分组   │                                          │
│  ● 点表   │  每个页面包含:                            │
│  ● 输出   │    - 数据表格 (列表展示)                  │
│  ● 监控   │    - 新增/编辑 弹窗表单                   │
│  ● 补传   │    - 删除确认                             │
│  ● 报警   │    - 导入/导出按钮                        │
│  ● 系统   │                                          │
│          │                                          │
└──────────┴──────────────────────────────────────────┘
```

各页面功能:

| 页面 | 功能 |
|------|------|
| **设备管理** | 串口/网口设备列表, 新增/编辑/删除, 在线状态显示, 测试连接 |
| **分组管理** | 分组列表, 拖拽分配设备, 配置输出端口, 寄存器范围分配 |
| **点表管理** | 按设备查看/编辑点表, CSV 导入/导出, 数据类型/缩放配置 |
| **输出配置** | 输出端口列表, TCP/RTU 输出配置, 从站地址设置 |
| **实时监控** | WebSocket 实时数据表格, 设备在线状态, 数据变化高亮 |
| **断点续传** | 各组补传状态, 未补传数据量, 手动触发重传, 补传进度条 |
| **报警记录** | 报警列表, 确认/清除, 按设备/时间筛选 |
| **系统设置** | 系统信息, 修改密码, 配置备份/恢复, 日志查看, 热重载 |

### 7.5 配置变更热重载机制

```
Web 修改配置 → API 保存到 YAML 文件 → 通知引擎热重载
                                      ↓
                               引擎检测配置变更
                                      ↓
                            ┌─────────┼─────────┐
                            ▼         ▼         ▼
                         重新加载   断开旧连接  启动新连接
                         点表映射   (如有变更)  (如有新增)
```

```go
// 配置变更通知
type ConfigChangeNotifier interface {
    // 注册变更回调
    OnChange(callback func(changeType string, target string))

    // 触发变更通知
    Notify(changeType string, target string)
}

// 变更类型
const (
    ConfigChangeDevice  = "device"   // 设备配置变更
    ConfigChangeGroup   = "group"    // 分组配置变更
    ConfigChangeMapping = "mapping"  // 点表变更
    ConfigChangeOutput  = "output"   // 输出配置变更
    ConfigChangeSystem  = "system"   // 系统配置变更
)
```

---

## 八、开发路线图

### 第一阶段: 基础框架 (第1-2周)

- [x] 项目初始化 (go mod, 目录结构)
- [ ] 配置加载 (YAML 解析)
- [ ] 日志系统
- [ ] 串口驱动 (serial driver)
- [ ] 网络驱动 (TCP driver)
- [ ] Modbus RTU 协议编解码
- [ ] Modbus TCP 协议编解码
- [ ] 核心引擎骨架

### 第二阶段: Modbus 采集 + 输出 (第3-4周)

- [ ] Modbus RTU 主站 (串口采集)
- [ ] Modbus TCP 客户端 (网口采集)
- [ ] Modbus TCP 服务端 (网口输出, 多端口)
- [ ] Modbus RTU 从站 (串口输出)
- [ ] 数据映射引擎
- [ ] 分组管理
- [ ] 简单配置文件驱动

### 第三阶段: Web 管理 + 高级功能 (第5-6周)

- [ ] Gin HTTP 服务器 + 静态文件托管
- [ ] 用户登录认证 (JWT)
- [ ] 设备管理 API + 前端页面 (CRUD, 在线状态)
- [ ] 分组管理 API + 前端页面 (设备分配, 端口配置)
- [ ] 点表管理 API + 前端页面 (在线编辑, CSV 导入/导出)
- [ ] 输出配置 API + 前端页面
- [ ] 实时监控页面 (WebSocket 数据推送)
- [ ] 系统设置页面 (系统信息, 修改密码, 配置备份/恢复)
- [ ] 配置变更热重载 (Web 修改 → YAML 保存 → 引擎重载)
- [ ] 内存缓存 + 历史数据
- [ ] 报警检测 + 报警页面
- [ ] 断线重连

### 第四阶段: 断点续传 (第7-8周)

- [ ] SQLite 离线缓冲存储 (OfflineBuffer)
- [ ] 从机连接状态检测 (主机断开/恢复)
- [ ] 断线时自动写入本地缓存 (带时间戳)
- [ ] 连接恢复后自动补传调度 (DataReporter)
- [ ] 按时间顺序批量补传 (BatchSize + BatchInterval)
- [ ] 过期数据自动清理 (retention_days)
- [ ] 补传进度查询 API + 前端页面
- [ ] 变化检测优化 (值未变化不存储)

### 第五阶段: IEC 61850 集成 (第9-12周)

- [ ] IEC 61850 MMS 客户端 (采集61850设备)
- [ ] IEC 61850 MMS 服务端 (对外提供61850接口)
- [ ] SCL/CID 文件解析
- [ ] Modbus ↔ IEC61850 数据模型映射
- [ ] GOOSE 发布/订阅 (可选)

### 第六阶段: 完善 + 部署 (第13-14周)

- [ ] 交叉编译 (ARM/MIPS)
- [ ] 性能优化
- [ ] 压力测试
- [ ] 部署脚本
- [ ] 使用文档

---

## 九、接口定义汇总

### 9.1 协议层接口

```go
// 采集器 - 所有输入协议实现
type Collector interface {
    Connect() error
    Disconnect() error
    Read(req ReadRequest) ([]DataPoint, error)
    IsConnected() bool
    Protocol() string
}

// 输出器 - 所有输出协议实现
type Outputter interface {
    Listen() error
    Close() error
    Write(points []DataPoint) error
    GetSlaveInfo() SlaveInfo
    Protocol() string

    // 连接状态管理 (断点续传依赖)
    IsMasterConnected() bool                              // 主机是否连接
    OnMasterConnected(callback func())                    // 主机连接回调
    OnMasterDisconnected(callback func())                 // 主机断开回调
    GetOfflineBuffer() OfflineBuffer                      // 获取离线缓冲
}

// 协议转换器
type Converter interface {
    Convert(points []DataPoint, target string) ([]DataPoint, error)
    SourceProtocol() string
    TargetProtocol() string
}
```

### 9.2 驱动层接口

```go
// 串口驱动
type SerialDriver interface {
    Open(config SerialConfig) error
    Close() error
    Read(buf []byte) (int, error)
    Write(data []byte) (int, error)
    Flush() error
}

// 网络驱动
type NetworkDriver interface {
    Connect(addr string, timeout time.Duration) error
    Close() error
    Read(buf []byte) (int, error)
    Write(data []byte) (int, error)
    SetDeadline(t time.Time) error
}
```

### 9.3 存储层接口

```go
// 缓存
type Cache interface {
    Get(key string) (DataPoint, bool)
    Set(key string, point DataPoint)
    GetAll(prefix string) []DataPoint
    Delete(key string)
}

// 历史存储
type HistoryStore interface {
    Save(point DataPoint) error
    Query(deviceID string, start, end time.Time) ([]DataPoint, error)
    Latest(deviceID string) (DataPoint, error)
}

// 离线缓冲 (断点续传)
type OfflineBuffer interface {
    Store(records []OfflineRecord) error                 // 写入离线数据
    LoadUntransmitted(groupID string, limit int) ([]OfflineRecord, error) // 查询未补传
    MarkTransmitted(ids []int64) error                   // 标记已补传
    EarliestUntransmitted(groupID string) (time.Time, error) // 最早未补传时间
    CountUntransmitted(groupID string) (int64, error)    // 未补传数量
    Cleanup(retentionDays int) (int64, error)            // 清理过期数据
    Size() (int64, error)                                // 缓冲大小
}

// 数据补传器
type DataReporter interface {
    StartReport(ctx context.Context) error               // 启动补传
    StopReport()                                          // 停止补传
    IsCompleted() bool                                    // 是否完成
    Progress() ReportProgress                             // 补传进度
}
```

---

## 十、错误处理策略

```go
// 统一错误类型
type ComError struct {
    Code    int
    Module  string
    Message string
    Err     error
}

// 错误级别
const (
    LevelWarn  = "warn"   // 告警: 设备断线, 超时 (继续运行)
    LevelError = "error"  // 错误: 协议解析失败 (跳过当前数据)
    LevelFatal = "fatal"  // 致命: 配置错误, 端口占用 (程序退出)
)
```

策略:
- 单个设备故障不影响其他设备
- 断线自动重连, 指数退避
- 协议解析错误记录日志, 跳过当前帧
- 配置错误在启动时校验并报错

---

## 十一、性能指标目标

| 指标 | 目标值 |
|------|--------|
| 最大串口设备数 | 64+ |
| 最大网口设备数 | 256+ |
| 最大输出端口数 | 100+ |
| 单设备采集延迟 | < 100ms |
| 内存占用 | < 100MB (50台设备) |
| CPU 占用 | < 10% (正常负载) |
| 断线重连时间 | < 30s |
| 配置热重载 | < 5s |
