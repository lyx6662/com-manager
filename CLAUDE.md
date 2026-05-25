# 通讯管理机平台 (ComManager)

## 项目概述

基于 Go 语言开发的工业物联网网关/通讯管理机平台，支持多协议采集、协议转换、灵活分组输出。

## 技术栈

- 语言: Go 1.21+
- Web框架: Gin
- 数据库: SQLite
- 日志: zap
- 配置: YAML
- 前端: 原生 HTML/CSS/JS + Bootstrap

## 项目结构

```
cmd/gateway/        入口程序
internal/           核心业务模块
  core/             核心引擎
  protocol/         协议层 (Modbus, IEC61850)
  driver/           驱动层 (串口, 网络)
  mapping/          数据映射引擎
  group/            分组管理
  storage/          数据存储 (SQLite, 缓存, 断点续传)
  web/              Web管理API
pkg/                公共库
configs/            配置文件
web/frontend/       前端页面
```

## 开发规范

- 所有代码注释使用中文
- 错误处理使用统一的错误类型
- API响应格式: `{"code": 0, "message": "success", "data": ...}`
- 配置变更后支持热重载

## 构建与运行

```bash
# 安装依赖
go mod tidy

# 本地运行
go run cmd/gateway/main.go

# 编译
go build -o com-manager cmd/gateway/main.go

# 交叉编译到ARM
GOOS=linux GOARCH=arm GOARM=7 go build -o com-manager-arm cmd/gateway/main.go
```

## 访问Web管理

- 地址: http://localhost:8080
- 默认用户名: admin
- 默认密码: admin123
