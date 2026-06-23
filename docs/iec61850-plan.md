# IEC 61850 模块开发规划

## 一、目标概述

在现有 Modbus 通讯管理机基础上，新增 IEC 61850 (MMS) 输出能力。系统从底层设备（Modbus RTU/TCP）采集数据后，既可通过 Modbus TCP/RTU Server 对外输出，也可通过 IEC 61850 MMS Server 对外输出，供支持 61850 协议的上位机/调度系统读取。

数据流：
```
底层设备 (Modbus) → Collector → Router → IEC 61850 MMS Server (输出)
                                      → Modbus TCP/RTU Server (输出，已有)
```

## 二、关键决策

### 2.1 为什么选 libiec61850 (C 库)

| 方案 | 优点 | 缺点 |
|------|------|------|
| **libiec61850 (C)** | 成熟稳定、功能完整、社区活跃、支持 MMS/GOOSE/SV | 需要 CGO，交叉编译复杂度高 |
| 纯 Go 实现 | 无 CGO 依赖，编译简单 | 61850 协议栈复杂（MMS/ASN.1BER/COTP），纯 Go 实现不成熟 |
| 其他商业库 | 支持好 | 授权费用高，不适合开源项目 |

结论：libiec61850 是目前最务实的选择。

### 2.2 为什么用 zig cc + musl

| 方案 | glibc 动态链接 | musl 静态链接 |
|------|---------------|--------------|
| 目标机器兼容性 | 需匹配 glibc 版本（如 2.17/2.28/2.31） | 零依赖，任意 Linux 发行版 |
| 部署复杂度 | 需确保目标机器有所需 .so | 单一二进制，scp 即用 |
| 二进制大小 | 较小 | 稍大（+1~2MB），可接受 |
| 交叉编译工具链 | 需安装对应平台的 gcc 交叉编译器 | zig cc 内置全部目标 |

工业网关场景下，目标设备通常是定制 Linux，glibc 版本不可控。**musl 静态链接是最佳选择**。

zig cc 的优势：
- 无需安装额外交叉编译工具链（arm-linux-gnueabihf-gcc 等）
- 一行命令切换目标架构
- 内置 musl 和 glibc 头文件，可指定最低 glibc 版本
- 编译出的 musl 静态二进制可在任意 Linux 上运行

### 2.3 目标架构

| 架构 | zig target | 常见设备 |
|------|-----------|---------|
| ARM64 (AArch64) | `aarch64-linux-musl` | 新一代网关（研华、摩莎、华为） |
| ARMv7 (hf) | `arm-linux-musleabihf` | 老旧网关、树莓派 |
| x86_64 | `x86_64-linux-musl` | 工控机、虚拟机 |

优先支持 **aarch64-linux-musl**（主流），其余按需。

## 三、架构设计

### 3.1 整体架构

```
Engine (核心引擎)
  ├── Collector (采集调度器) ── 不变
  ├── Router (数据路由器) ── 扩展：支持 IEC61850 输出
  ├── AlarmDetector ── 不变
  ├── OfflineBuffer ── 不变
  ├── tcp.Server (Modbus TCP 输出) ── 不变
  ├── rtu.Server (Modbus RTU 输出) ── 不变
  ├── iec61850.Server (IEC 61850 MMS 输出) ── 新增
  └── web.Server ── 扩展：61850 配置页面
```

### 3.2 模块分层

```
┌─────────────────────────────────────────────────────────┐
│  Go 业务层                                                │
│  internal/iec61850/server.go   IEC61850 Server 管理       │
│  internal/iec61850/model.go    数据模型映射                 │
│  internal/iec61850/config.go   配置结构体                   │
├─────────────────────────────────────────────────────────┤
│  CGO 桥接层                                               │
│  lib-iec61850/wrapper.go       Go ↔ C 函数绑定             │
│  lib-iec61850/wrapper.c        C wrapper (薄封装)          │
├─────────────────────────────────────────────────────────┤
│  C 库层                                                   │
│  third_party/libiec61850/      libiec61850 源码            │
│  → 编译产物: libiec61850.a (musl 静态库)                   │
└─────────────────────────────────────────────────────────┘
```

### 3.3 数据模型映射

IEC 61850 使用面向对象的数据模型（LD/LN/DO/DA），与 Modbus 的寄存器地址完全不同。

```
IEC 61850 模型:
  LD (逻辑设备) ── 例如 "GRID_GATEWAY"
    LN (逻辑节点) ── 例如 "MMXU1" (测量)
      DO (数据对象) ── 例如 "TotW" (总有功功率)
        DA (数据属性) ── 例如 "mag.f" (浮点值)

Modbus 侧:
  设备1 → 寄存器 100-101 → float32 → 25.5 (温度)
```

配置映射关系（在 gateway.yaml 中扩展）：

```yaml
iec61850:
  enabled: true
  port: 102
  model_file: "./configs/model.scl"  # SCL 模型文件
  mappings:
    - logical_device: "GRID_GATEWAY"
      logical_node: "MMXU1"
      data_object: "TotW.mag.f"
      source_device: "meter1"
      source_point: "active_power"
      scale: 1.0
      offset: 0.0
```

### 3.4 CGO 封装策略

**核心原则：C wrapper 薄封装，Go 侧只调用高层 API。**

不要在 Go 中直接 `#include` libiec61850 的全部头文件。写一个精简的 C wrapper：

```c
// lib-iec61850/wrapper.h
typedef void* IedServerHandle;

// 创建/销毁
IedServerHandle iec61850_server_create(int port, const char* model_file);
void iec61850_server_destroy(IedServerHandle handle);

// 控制
int iec61850_server_start(IedServerHandle handle);
void iec61850_server_stop(IedServerHandle handle);

// 数据更新
int iec61850_update_float(IedServerHandle handle,
    const char* ld, const char* ln, const char* do_name, float value);
int iec61850_update_int32(IedServerHandle handle,
    const char* ld, const char* ln, const char* do_name, int32_t value);
int iec61850_update_bool(IedServerHandle handle,
    const char* ld, const char* ln, const char* do_name, int value);
```

Go 侧通过 `// #cgo` 指令链接静态库：

```go
// lib-iec61850/wrapper.go
package iec61850

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/libiec61850/include
#cgo LDFLAGS: -L${SRCDIR}/../../build -liec61850 -lm -lpthread
#include "wrapper.h"
*/
import "C"
```

## 四、目录结构规划

```
com-manager/
├── main.go                              入口 (不变)
├── Makefile                             构建脚本 (新增交叉编译目标)
├── configs/
│   ├── gateway.yaml                     主配置 (扩展 iec61850 段)
│   └── model.scl                        IEC 61850 SCL 模型文件 (新增)
├── third_party/
│   └── libiec61850/                     libiec61850 源码 (git submodule)
│       ├── include/                     头文件
│       └── src/                         源码
├── lib-iec61850/                        Go + C wrapper 层
│   ├── wrapper.h                        C wrapper 头文件
│   ├── wrapper.c                        C wrapper 实现
│   ├── wrapper.go                       CGO 绑定
│   └── server.go                        Go 封装的 Server 类型
├── internal/
│   ├── core/
│   │   ├── engine.go                    扩展：启动/停止 61850 Server
│   │   ├── router.go                    扩展：61850 输出路径
│   │   └── ...                          (现有文件不变)
│   └── iec61850/                        61850 业务逻辑 (新增)
│       ├── server.go                    IEC61850 Server 管理
│       ├── model_mapper.go              数据模型映射
│       └── config.go                    配置结构体
├── pkg/config/
│   └── config.go                        扩展：IEC61850Config 结构体
├── build/                               编译产物 (.a 静态库)
│   ├── libiec61850-aarch64-musl.a
│   ├── libiec61850-armv7-musl.a
│   └── libiec61850-x86_64-musl.a
└── docs/
    └── iec61850-plan.md                 本文档
```

## 五、构建流程

### 5.1 一次性准备：编译 libiec61850 静态库

```bash
# 1. 引入 libiec61850 源码 (git submodule)
git submodule add https://github.com/mz-automation/libiec61850.git third_party/libiec61850

# 2. 编译静态库 (以 aarch64-musl 为例)
make libiec61850 ARCH=aarch64
```

Makefile 中的关键规则：

```makefile
# 可用架构: aarch64, armv7, x86_64
ARCH ?= aarch64

# 架构 → zig target 映射
ifeq ($(ARCH),aarch64)
    ZIG_TARGET := aarch64-linux-musl
else ifeq ($(ARCH),armv7)
    ZIG_TARGET := arm-linux-musleabihf
else ifeq ($(ARCH),x86_64)
    ZIG_TARGET := x86_64-linux-musl
endif

LIBIEC_SRC  := third_party/libiec61850
LIBIEC_INC  := $(LIBIEC_SRC)/include
BUILD_DIR   := build
STATIC_LIB  := $(BUILD_DIR)/libiec61850-$(ARCH)-musl.a

# 收集 libiec61850 所有 .c 源文件
LIBIEC_SRCS := $(wildcard $(LIBIEC_SRC)/src/*.c) \
               $(wildcard $(LIBIEC_SRC)/src/iso/*.c) \
               $(wildcard $(LIBIEC_SRC)/src/mms/*.c) \
               $(wildcard $(LIBIEC_SRC)/src/iec61850/*.c) \
               $(wildcard $(LIBIEC_SRC)/src/logging/*.c)

# 编译静态库
.PHONY: libiec61850
libiec61850:
    @mkdir -p $(BUILD_DIR)/obj
    @echo "编译 libiec61850 ($(ZIG_TARGET))..."
    @for src in $(LIBIEC_SRCS); do \
        zig cc -target $(ZIG_TARGET) -static -O2 \
            -I$(LIBIEC_INC) \
            -I$(LIBIEC_SRC)/src \
            -c $$src -o $(BUILD_DIR)/obj/$$(basename $$src .c).o || exit 1; \
    done
    zig ar rcs $(STATIC_LIB) $(BUILD_DIR)/obj/*.o
    @echo "静态库已生成: $(STATIC_LIB)"
    @rm -rf $(BUILD_DIR)/obj
```

### 5.2 编译 Go 程序

```makefile
# 交叉编译 Go 程序
.PHONY: build
build: libiec61850
    @echo "编译 com-manager ($(ZIG_TARGET))..."
    CC="zig cc -target $(ZIG_TARGET)" \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=$(GOARCH) \
    go build \
        -ldflags='-linkmode external -extldflags "-static -L$(BUILD_DIR) -liec61850-$(ARCH)-musl -lm -lpthread"' \
        -o $(BUILD_DIR)/com-manager-$(ARCH) \
        main.go

# 架构 → GOARCH 映射
ifeq ($(ARCH),aarch64)
    GOARCH := arm64
else ifeq ($(ARCH),armv7)
    GOARCH := arm
else ifeq ($(ARCH),x86_64)
    GOARCH := amd64
endif
```

### 5.3 完整构建命令

```bash
# 编译 ARM64 版本
make build ARCH=aarch64

# 编译 ARMv7 版本
make build ARCH=armv7

# 编译 x86_64 版本
make build ARCH=x86_64

# 一键编译全部架构
make build-all
```

产物：
```
build/
├── com-manager-aarch64          ARM64 二进制
├── com-manager-armv7            ARMv7 二进制
├── com-manager-x86_64           x86_64 二进制
├── libiec61850-aarch64-musl.a   ARM64 静态库
├── libiec61850-armv7-musl.a     ARMv7 静态库
└── libiec61850-x86_64-musl.a    x86_64 静态库
```

## 六、跨平台兼容性验证

### 6.1 musl 静态链接的兼容性保证

```
编译产物 (静态链接 musl)
  ├── 不依赖目标机器的 glibc
  ├── 不依赖目标机器的 libc.so
  ├── 不依赖目标机器的 libm.so / libpthread.so
  └── 唯一要求: Linux 内核版本 >= 3.2 (支持相关 syscall)
```

验证方法：

```bash
# 检查二进制是否完全静态
file build/com-manager-aarch64
# 期望输出: ELF 64-bit LSB executable, ARM aarch64, statically linked

# 检查是否还有动态依赖
readelf -d build/com-manager-aarch64
# 期望输出: (无动态段)

# 用 QEMU 模拟运行 (开发机上验证)
qemu-aarch64-static build/com-manager-aarch64 --help
```

### 6.2 目标设备兼容矩阵

| 目标环境 | 内核版本 | glibc 版本 | 是否兼容 | 说明 |
|---------|---------|-----------|---------|------|
| Ubuntu 18.04+ | 4.15+ | 2.27+ | 兼容 | musl 静态链接不依赖 glibc |
| Debian 10+ | 4.19+ | 2.28+ | 兼容 | 同上 |
| CentOS 7 | 3.10 | 2.17 | 兼容 | 内核版本满足最低要求即可 |
| Alpine Linux | 4.x+ | musl | 兼容 | 原生 musl 环境 |
| OpenWrt | 4.x+ | musl | 兼容 | 嵌入式网关常用 |
| 自定义嵌入式 Linux | 3.2+ | 任意 | 兼容 | 只要内核支持相关 syscall |

### 6.3 已知限制

- **cgo 与 UPX 冲突**：UPX 压缩含 CGO 的二进制可能失败，不建议压缩
- **DNS 解析**：musl 的 DNS 解析行为与 glibc 略有差异（`/etc/nsswitch.conf` 不生效），但 61850 服务端不需要 DNS
- **信号处理差异**：musl 的某些信号语义与 glibc 不同，但对本项目无影响

## 七、开发阶段规划

### 阶段 1：基础设施 (预计 2-3 天)

- [ ] 引入 libiec61850 源码 (git submodule)
- [ ] 编写 Makefile 交叉编译规则
- [ ] 验证 musl 静态库编译成功
- [ ] 验证 Go CGO 能链接静态库并运行

里程碑：`make build ARCH=aarch64` 能生成可运行的二进制（即使还没有 61850 功能）

### 阶段 2：CGO Wrapper 层 (预计 2-3 天)

- [ ] 编写 `wrapper.h` / `wrapper.c` (C wrapper)
- [ ] 编写 `wrapper.go` (CGO 绑定)
- [ ] 编写 `lib-iec61850/server.go` (Go 封装)
- [ ] 用 libiec61850 的 `simple_server` example 验证 wrapper 能正常工作

里程碑：Go 程序能创建 IEC 61850 Server 并被 IEDScout 连接

### 阶段 3：业务集成 (预计 3-5 天)

- [ ] 定义 `IEC61850Config` 配置结构体 (pkg/config)
- [ ] 实现 `internal/iec61850/server.go` (Server 生命周期管理)
- [ ] 实现 `internal/iec61850/model_mapper.go` (数据模型映射)
- [ ] 扩展 `Router` 支持 61850 输出路径
- [ ] 扩展 `Engine` 启动/停止/热重载 61850 Server
- [ ] 编写 SCL 模型文件 (model.scl)

里程碑：数据能从 Modbus 设备 → Router → 61850 Server → IEDScout 读到

### 阶段 4：Web 管理 (预计 2-3 天)

- [ ] 新增 61850 配置页面 (web/frontend/pages/iec61850.html)
- [ ] 实现 61850 相关的 API handler
- [ ] 监控页面展示 61850 连接状态

里程碑：通过 Web 界面可配置和监控 61850 模块

### 阶段 5：测试与优化 (预计 2-3 天)

- [ ] 使用 IEDScout 完整测试 MMS 读写
- [ ] 压力测试：多客户端并发读取
- [ ] 跨架构验证：在真实 ARM 设备上运行
- [ ] 内存泄漏检测

里程碑：生产可用

## 八、风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|---------|
| libiec61850 编译在 musl 下有兼容问题 | 阻塞 | libiec61850 是纯 C 实现，musl 兼容性好；如有问题可 patch 源码 |
| CGO 内存管理导致 Go 程序崩溃 | 高 | wrapper 层做好指针生命周期管理，避免 Go GC 回收 C 指针 |
| SCL 模型配置复杂 | 中 | 先提供几个预置模板，降低配置门槛 |
| 交叉编译的二进制在目标设备上行为异常 | 中 | 阶段 1 就在目标设备上验证基础运行 |
| libiec61850 API 变更 | 低 | git submodule 锁定特定版本 (v1.x) |

## 九、开发环境准备清单

### 开发机 (Windows/macOS/Linux)

```bash
# 1. 安装 zig (用于交叉编译 C 库)
# Windows: scoop install zig 或 choco install zig
# macOS:   brew install zig
# Linux:   snap install zig 或从官网下载

# 2. 验证 zig 可用
zig version
# 期望: 0.13.x 或更高

# 3. 验证 zig cc 支持目标架构
zig cc -print-targets | grep aarch64
zig cc -print-targets | grep arm

# 4. Go 1.21+ (已有)
go version

# 5. 安装测试工具
# IEDScout: https://www.omicronenergy.com/en/products/iedscout/ (免费演示版)
# Wireshark: https://www.wireshark.org/ (带 MMS 协议解析)
```

### 目标设备

```
无需预装任何依赖。
只要能 scp 二进制文件上去，chmod +x 后直接运行。
```
