# 通讯管理机 (Com-Manager)

基于 Go 语言开发的工业物联网网关/通讯管理机平台。从底层设备（串口/网口 Modbus）采集数据，经映射转换后，通过 Modbus TCP/RTU Server 对外输出，供上位机（SCADA）读取，同时支持 IEC 61850 协议输出。

## 功能特性

- **多设备采集**: 支持串口 Modbus RTU 和网口 Modbus TCP 设备
- **数据映射**: 灵活的寄存器映射配置，支持缩放、偏移、数据类型转换
- **Modbus TCP 输出**: 供上位机以 TCP 方式读取数据
- **Modbus RTU 输出**: 供上位机以串口方式读取数据
- **IEC 61850 输出**: 支持 IEC 61850 MMS 协议输出
- **Web 管理界面**: 基于 Bootstrap 5.3 的现代化管理界面
- **断点续传**: SQLite 存储，支持断线数据缓存
- **热重载**: 支持配置文件热重载，无需重启
- **ARM 支持**: 支持交叉编译到 ARM 平台（工控机）

## 技术栈

- **语言**: Go 1.21+
- **Web 框架**: Gin
- **数据库**: SQLite (断点续传 + 报警存储)
- **日志**: zap
- **配置**: YAML
- **前端**: 原生 HTML/CSS/JS + Bootstrap 5.3，SPA 模式

## 项目结构

```
com-manager/
├── main.go                          # 入口程序
├── go.mod                           # Go 模块文件
├── configs/                         # 配置文件目录
│   ├── gateway.yaml                 # 主配置文件（设备、采集点表）
│   ├── outputs.yaml                 # 输出配置（Modbus TCP/RTU 服务器、映射）
│   ├── modbus_to_61850.yaml         # IEC 61850 配置
│   └── GW.icd                       # IEC 61850 ICD 文件
├── internal/
│   ├── core/                        # 核心业务逻辑
│   │   ├── engine.go                # 核心引擎
│   │   ├── collector.go             # 采集调度器
│   │   ├── router.go                # 数据路由器
│   │   ├── datapool.go              # 数据共享池
│   │   ├── modbus_adapter.go        # Modbus 输出适配器
│   │   └── iec61850_adapter.go      # IEC 61850 输出适配器
│   ├── storage/                     # 存储层
│   │   ├── buffer/                  # 断点续传存储
│   │   └── alarm/                   # 报警记录存储
│   └── web/                         # Web 服务
│       ├── server.go                # Gin 路由注册
│       └── handler/                 # 业务处理器
├── lib-modbus/                      # 自研 Modbus 协议库
│   ├── tcp/                         # Modbus TCP 客户端/服务端
│   └── rtu/                         # Modbus RTU 主站/从站
├── lib-iec61850/                    # IEC 61850 封装库
├── pkg/                             # 公共包
│   ├── config/                      # 配置管理
│   ├── model/                       # 数据模型
│   └── logger/                      # 日志封装
├── web/frontend/                    # 前端静态文件
│   ├── index.html                   # SPA 壳页面
│   ├── js/app.js                    # 前端主逻辑
│   └── pages/                       # 各功能页面
├── third_party/                     # 第三方依赖
│   └── libiec61850/                 # IEC 61850 C 库
└── .claude/                         # Claude Code 配置
    └── build-arm.sh                 # ARM 交叉编译脚本
```

## 快速开始

### 环境要求

- Go 1.21+
- GCC (用于 CGO 编译)
- Git

### 编译

```bash
# 克隆项目
git clone https://github.com/lyx6662/com-manager.git
cd com-manager

# 安装依赖
go mod tidy

# 编译 Linux AMD64 版本
go build -o com-manager main.go

# 编译 Windows 版本
go build -o com-manager.exe main.go
```

### 交叉编译 ARM 版本（工控机）

```bash
# 方式一：使用 Docker（推荐）
bash .claude/build-arm.sh

# 方式二：手动交叉编译
GOOS=linux GOARCH=arm GOARM=5 CGO_ENABLED=1 \
  CC=arm-linux-gnueabi-gcc \
  go build -o com-manager-arm main.go
```

### 运行

```bash
# 本地运行
./com-manager

# 或者直接运行
go run main.go
```

## 配置说明

### 主配置文件 (configs/gateway.yaml)

```yaml
server:
    name: com-manager
    log_level: info
    log_path: ./logs

web:
    enabled: true
    port: 18080
    host: 0.0.0.0
    auth:
        enabled: true
        username: admin
        password: "123456"

serial_devices:
    - id: device1
      name: 设备1
      port: /dev/ttyS1
      baud_rate: 9600
      data_bits: 8
      stop_bits: 1
      parity: none
      protocol: modbus_rtu
      slave_id: 1
      poll_interval: 1s
      timeout: 3s
      retry: 3
      enabled: true

data_points:
    device1:
        - name: DEVICE1-H0
          source_device: device1
          source_register: 0
          source_type: holding
          data_type: uint16
          register_count: 1
```

### 输出配置 (configs/outputs.yaml)

```yaml
outputs:
    enabled: true
    modbus_tcp_servers:
        - id: output_1
          name: Modbus TCP输出
          listen_port: 502
          slave_id: 1
          max_connections: 10
    group_devices:
        output_1:
            - device1
            - device2

output_mappings:
    device1:
        - name: DEVICE1-H0    # 必须与 data_points 中的 name 一致
          target_register: 0
          scale: 1
          offset: 0
```

### IEC 61850 配置 (configs/modbus_to_61850.yaml)

```yaml
iec61850:
    enabled: true
    port: 102
    ied_name: GW_IED
    max_connections: 10
    icd_output: ./configs/GW.icd

model:
    logical_devices:
        - name: GW
          logical_nodes:
            - name: MMXU1
              data_objects:
                - name: TotW0
                  data_attributes:
                    - name: mag
                      children:
                        - name: f
                          type: FLOAT32
                          fc: MX

mappings:
    - source_device: device1
      source_name: DEVICE1-H0
      target_type: float32
      iec61850_path: GW/MMXU1.TotW0.mag.f
      scale: 1
      offset: 0
```

## Web 管理

- **地址**: http://localhost:18080
- **默认用户名**: admin
- **默认密码**: 123456

### 功能页面

- **设备管理**: 添加/删除/修改设备，配置采集点表
- **数据池**: 查看实时采集数据
- **输出管理**: 配置 Modbus TCP/RTU 输出服务器和映射规则
- **系统设置**: 查看系统状态、日志

## 部署到工控机

### 1. 编译 ARM 版本

```bash
bash .claude/build-arm.sh
```

### 2. 上传到工控机

```bash
# 上传程序
curl -T com-manager-arm ftp://工控机IP/com-manager-arm --user 用户名:密码

# 上传配置文件
curl -T configs/gateway.yaml ftp://工控机IP/home/com/configs/gateway.yaml --user 用户名:密码
curl -T configs/outputs.yaml ftp://工控机IP/home/com/configs/outputs.yaml --user 用户名:密码
curl -T configs/modbus_to_61850.yaml ftp://工控机IP/home/com/configs/modbus_to_61850.yaml --user 用户名:密码
```

### 3. 工控机上部署

```bash
# 移动程序
mv /com-manager-arm /home/com/com-manager-arm
chmod +x /home/com/com-manager-arm

# 启动程序
cd /home/com && nohup ./com-manager-arm > /dev/null 2>&1 &
```

### 4. 设置开机自启动

创建守护脚本 `/home/com/com-manager.sh`:

```bash
#!/bin/sh
APP_PATH="/home/com/com-manager-arm"
APP_DIR="/home/com"
APP_NAME="com-manager-arm"

while true; do
    cd "$APP_DIR"
    nohup $APP_PATH > /dev/null 2>&1 &
    APP_PID=$!
    wait $APP_PID
    sleep 2
done
```

在 `/etc/init.d/rcS` 末尾添加：

```bash
# 启动通讯管理机
mkdir -p /home/com/logs
/home/com/com-manager.sh &
```

## 常见问题

### Q: 输出寄存器全是 0？

A: 检查 `outputs.yaml` 中的 `output_mappings` 的 `name` 字段是否与 `gateway.yaml` 中 `data_points` 的 `name` 字段完全一致。

### Q: 设备显示离线？

A: 检查：
1. 串口设备是否正确连接
2. 串口路径是否正确（如 `/dev/ttyS1`）
3. 波特率、数据位、停止位、校验位是否与设备一致
4. 从站 ID 是否正确

### Q: 如何查看日志？

A: 
```bash
# 查看实时日志
tail -f logs/com-manager.log

# 通过 Web API 查看
curl http://localhost:18080/api/v1/system/logs?lines=100 -H "Authorization: Bearer TOKEN"
```

### Q: 如何热重载配置？

A: 通过 Web 界面修改配置后会自动热重载，或者调用 API：

```bash
curl -X POST http://localhost:18080/api/v1/system/reload -H "Authorization: Bearer TOKEN"
```

## API 接口

### 认证

```bash
# 登录获取 Token
curl -X POST http://localhost:18080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}'
```

### 设备管理

```bash
# 获取设备列表
curl http://localhost:18080/api/v1/devices -H "Authorization: Bearer TOKEN"

# 获取数据池
curl http://localhost:18080/api/v1/datapool -H "Authorization: Bearer TOKEN"
```

## 许可证

MIT License

## 作者

- GitHub: [lyx6662](https://github.com/lyx6662)
