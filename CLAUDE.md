# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

基于 Go 语言开发的工业物联网网关/通讯管理机平台。从底层设备（串口/网口 Modbus）采集数据，经映射转换后，通过 Modbus TCP/RTU Server 对外输出，供上位机（SCADA）读取。

## 技术栈

- 语言: Go 1.21+
- Web框架: Gin
- 数据库: SQLite (断点续传 + 报警存储)
- 日志: zap
- 配置: YAML (`configs/gateway.yaml`)
- 前端: 原生 HTML/CSS/JS + Bootstrap 5.3，SPA 模式

## 架构

```
Engine (核心引擎)
  ├── Collector (采集调度器) — 轮询串口/网口设备，读取 Modbus 寄存器
  │     └── deviceWorker (每设备独立 goroutine)
  ├── Router (数据路由器) — 将采集数据按映射规则写入输出寄存器
  │     └── refreshOutputs() — holding 寄存器 + coil 线圈
  ├── AlarmDetector (报警检测) — 检查 HighLimit/LowLimit 阈值
  ├── OfflineBuffer (断点续传) — 内存队列 + 定时批量刷盘到 SQLite
  ├── tcp.Server (Modbus TCP 输出) — 供上位机以 TCP 方式读取
  ├── rtu.Server (Modbus RTU 输出) — 供上位机以串口方式读取
  └── web.Server (Web 管理界面)
```

数据流: `设备轮询 → Collector → Router → 输出寄存器 (TCP/RTU Server)`
缓冲流: `设备轮询 → Engine.onDeviceData → 过滤 QualityGood → 内存队列 → 定时 Flush → SQLite`

## 目录结构

```
main.go                       入口程序
internal/
  core/
    engine.go                  核心引擎，负责启动/停止/热重载所有组件
    collector.go               采集调度器，管理 deviceWorker 生命周期
    router.go                  数据路由器，映射源寄存器→目标寄存器
    alarm_detector.go          报警检测器，基于阈值规则触发报警
  storage/
    buffer/store.go            断点续传存储 (SQLite + 内存队列)
    alarm/store.go             报警记录存储 (SQLite)
  web/
    server.go                  Gin 路由注册 + 中间件
    handler/                   各业务处理器 (device/group/mapping/output/monitor/alarm/buffer/system)
    auth/token.go              JWT Token 管理
lib-modbus/                    自研 Modbus 协议库
  tcp/client.go, server.go     Modbus TCP 客户端/服务端
  rtu/master.go, server.go     Modbus RTU 主站/从站
pkg/
  config/config.go             配置结构体 + 加载/保存/管理
  model/data_point.go          DataPoint 数据模型
  logger/logger.go             zap 日志封装
configs/gateway.yaml           主配置文件
web/frontend/
  index.html                   SPA 壳页面
  js/app.js                    前端主逻辑 (路由、API 封装、登录)
  pages/*.html                 各功能页面 (device/group/mapping/output/monitor/alarm/buffer/system)
```

## 开发规范

- 所有代码注释使用中文
- API 响应格式: `{"code": 0, "message": "...", "data": ...}`
- 配置变更后支持热重载 (`Engine.Reload()`)
- 前端使用 `loadPage()` 动态加载页面 HTML，不使用前端路由库
- 前端 API 调用使用封装的 `api(method, path, data)` 函数（带 JWT token 自动附加）

## 构建与运行

```bash
go mod tidy
go run main.go                          # 本地运行
go build -o com-manager main.go         # 编译

# 交叉编译到 ARM
GOOS=linux GOARCH=arm GOARM=7 go build -o com-manager-arm main.go
```

## Web 管理

- 地址: http://localhost:18080
- 默认用户名: admin / 密码: admin123
- 认证通过 JWT token，前端登录后存入 localStorage

## 关键设计点

- **断点续传**: 仅在设备采集成功 (`QualityGood`) 且上位机未连接时缓存数据。内存队列按配置间隔（默认 10 分钟）批量刷入 SQLite。
- **补传逻辑**: `handleMasterConnected()` 目前为占位实现，仅记录日志。后续需在该方法中实现从 SQLite 读取待补传数据写入输出寄存器的逻辑。
- **热重载**: `Engine.Reload()` 支持增量更新 TCP/RTU 输出服务器、重建采集调度器、更新路由映射和报警规则。
- **分组映射**: `config.Mappings` 是 `map[string][]MappingRule`，key 为分组 ID（对应输出服务器 ID），value 为该组的点表映射规则列表。
