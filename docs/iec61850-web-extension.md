# IEC 61850 Web 页面扩展方案

## 1. 背景

当前映射管理页面仅支持 Modbus 输出配置，IEC 61850 输出配置只能通过手动编辑 `modbus_to_61850.yaml` 文件完成。需要将 IEC 61850 输出配置集成到 Web 界面中，实现可视化管理。

## 2. 现有架构分析

### 2.1 当前映射管理页面结构

```
┌─────────────────────────────────────────────────────────────┐
│  映射管理                                    [新增分组]      │
├──────────────┬──────────────────────────────────────────────┤
│  分组设备树   │  右侧内容区                                  │
│              │                                              │
│  ▼ modbustcp │  ┌─────────────────────────────────────────┐│
│    ├ 设备1   │  │  分组详情 / 设备映射                      ││
│    ├ 设备2   │  │                                         ││
│    └ 设备3   │  │  - 输出配置信息                           ││
│              │  │  - 设备列表                               ││
│  ▼ 分组2     │  │  - 映射点表                               ││
│    └ ...     │  └─────────────────────────────────────────┘│
└──────────────┴──────────────────────────────────────────────┘
```

### 2.2 当前 IEC 61850 配置结构 (`modbus_to_61850.yaml`)

```yaml
iec61850:
  enabled: true
  port: 102
  ied_name: "GW"
  max_connections: 10

model:
  logical_devices:
    - name: "GRID_GATEWAY"
      logical_nodes:
        - name: "MMXU1"
          data_objects:
            - name: "TotW"
              data_attributes:
                - name: "mag"
                  children:
                    - name: "f"
                      type: "FLOAT32"
                      fc: "MX"

mappings:
  - source_device: "rtu-device-1"
    source_name: "RTU1-寄存器1"
    source_register: 0
    source_type: "holding"
    data_type: "uint16"
    iec61850_path: "GRID_GATEWAY/MMXU1.TotW.mag.f"
    scale: 1.0
    offset: 0.0
    target_type: "float32"
```

## 3. 改造方案

### 3.1 左侧分组设备树改造

**改造前：**
```
分组设备树
├── modbustcp (Modbus TCP)
│   ├── 设备1
│   └── 设备2
└── modbustcpout (Modbus TCP)
    └── 设备3
```

**改造后：**
```
输出通道
├── Modbus 输出
│   ├── 开关: [启用/禁用]
│   ├── modbustcp (TCP)
│   │   ├── 设备1
│   │   └── 设备2
│   └── modbustcpout (TCP)
│       └── 设备3
│
└── IEC 61850 输出
    ├── 开关: [启用/禁用]
    ├── 服务器配置
    │   ├── 端口: 102
    │   ├── IED名称: GW
    │   └── 最大连接数: 10
    ├── 数据模型
    │   ├── GRID_GATEWAY
    │   │   ├── MMXU1
    │   │   ├── MMXU2
    │   │   └── CSWI1
    │   └── [+ 新增逻辑设备]
    └── 映射规则
        ├── rtu-device-1 (5条规则)
        └── tcp-2 (2条规则)
```

### 3.2 右侧内容区改造

#### 3.2.1 Modbus 输出详情（保持现有）

- 分组配置信息（端口、从站ID、最大连接数）
- 包含设备列表
- 设备映射点表

#### 3.2.2 IEC 61850 服务器配置（新增）

```
┌─────────────────────────────────────────────────────────────┐
│  IEC 61850 服务器配置                       [编辑] [保存]    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  服务状态    │  │  监听端口    │  │  IED名称    │         │
│  │  ● 运行中   │  │  102        │  │  GW         │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  最大连接数  │  │  连接数     │  │  ICD文件    │         │
│  │  10         │  │  2          │  │  已生成     │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 3.2.3 IEC 61850 数据模型编辑（新增）

```
┌─────────────────────────────────────────────────────────────┐
│  数据模型编辑                              [+ 新增逻辑设备]  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  逻辑设备: GRID_GATEWAY                    [编辑] [删除]    │
│  ├── 逻辑节点: MMXU1                       [编辑] [删除]    │
│  │   ├── 数据对象: TotW (CMV)                               │
│  │   │   └── 数据属性: mag.f (FLOAT32, MX)                  │
│  │   └── 数据对象: PPV (CMV)                                │
│  │       └── 数据属性: mag.f (FLOAT32, MX)                  │
│  ├── 逻辑节点: MMXU2                                         │
│  │   └── 数据对象: TotW (CMV)                               │
│  │       └── 数据属性: mag.f (FLOAT32, MX)                  │
│  └── 逻辑节点: CSWI1                                         │
│      ├── 数据对象: Mod (INS)                                 │
│      │   └── 数据属性: stVal (INT32, ST)                    │
│      └── 数据对象: Pos (INS)                                 │
│          └── 数据属性: stVal (INT32, ST)                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 3.2.4 IEC 61850 映射规则编辑（新增）

```
┌─────────────────────────────────────────────────────────────┐
│  映射规则                                  [+ 新增映射]      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  筛选: [全部设备 ▼]  [全部类型 ▼]                           │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ 源设备    │ 源名称      │ IEC61850路径              │ 类型 │
│  ├──────────┼────────────┼───────────────────────────┼─────┤
│  │ rtu-1    │ RTU1-寄存器1│ MMXU1.TotW.mag.f         │ FLT │
│  │ rtu-1    │ RTU1-寄存器2│ MMXU1.PPV.mag.f          │ FLT │
│  │ rtu-1    │ RTU1-寄存器5│ CSWI1.Mod.stVal          │ INT │
│  │ tcp-2    │ 0           │ MMXU2.TotW.mag.f         │ FLT │
│  │ tcp-2    │ 1           │ CSWI1.Pos.stVal          │ INT │
│  └──────────┴────────────┴───────────────────────────┴─────┘
│                                                             │
│  [编辑] [删除]                                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 映射规则编辑弹窗

```
┌─────────────────────────────────────────────────────────────┐
│  编辑 IEC 61850 映射规则                                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  源配置                                                      │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  源设备     │  │  源名称     │  │  源寄存器   │         │
│  │  [▼ rtu-1]  │  │  [RTU1-寄..]│  │  [0      ]  │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│  ┌─────────────┐  ┌─────────────┐                          │
│  │  源类型     │  │  数据类型   │                          │
│  │  [▼ holding]│  │  [▼ uint16] │                          │
│  └─────────────┘  └─────────────┘                          │
│                                                             │
│  目标配置 (IEC 61850)                                        │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  IEC 61850 路径                                      │   │
│  │  [GRID_GATEWAY/MMXU1.TotW.mag.f                ]    │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  目标类型   │  │  缩放系数   │  │  偏移量     │         │
│  │  [▼ float32]│  │  [1.0    ]  │  │  [0.0    ]  │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│                                                             │
│  说明                                                        │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  [如: 有功功率 (kW)                             ]    │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│                          [取消]  [保存]                      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## 4. 后端 API 设计

### 4.1 IEC 61850 配置 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/iec61850/config` | 获取 IEC 61850 完整配置 |
| PUT | `/api/v1/iec61850/config` | 更新 IEC 61850 服务器配置 |
| POST | `/api/v1/iec61850/restart` | 重启 IEC 61850 服务 |

### 4.2 IEC 61850 数据模型 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/iec61850/model` | 获取数据模型 |
| PUT | `/api/v1/iec61850/model` | 更新数据模型 |
| POST | `/api/v1/iec61850/model/devices` | 新增逻辑设备 |
| PUT | `/api/v1/iec61850/model/devices/{name}` | 更新逻辑设备 |
| DELETE | `/api/v1/iec61850/model/devices/{name}` | 删除逻辑设备 |
| POST | `/api/v1/iec61850/model/devices/{device}/nodes` | 新增逻辑节点 |
| PUT | `/api/v1/iec61850/model/devices/{device}/nodes/{node}` | 更新逻辑节点 |
| DELETE | `/api/v1/iec61850/model/devices/{device}/nodes/{node}` | 删除逻辑节点 |

### 4.3 IEC 61850 映射规则 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/iec61850/mappings` | 获取所有映射规则 |
| GET | `/api/v1/iec61850/mappings?device={id}` | 按设备筛选映射规则 |
| POST | `/api/v1/iec61850/mappings` | 新增映射规则 |
| PUT | `/api/v1/iec61850/mappings/{index}` | 更新映射规则 |
| DELETE | `/api/v1/iec61850/mappings/{index}` | 删除映射规则 |

### 4.4 IEC 61850 状态 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/iec61850/status` | 获取 IEC 61850 运行状态 |
| GET | `/api/v1/iec61850/paths` | 获取可用的 IEC 61850 路径列表 |

### 4.5 API 响应格式

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "iec61850": {
            "enabled": true,
            "port": 102,
            "ied_name": "GW",
            "max_connections": 10,
            "icd_output": "./configs/GW.icd"
        },
        "model": {
            "logical_devices": [...]
        },
        "mappings": [...]
    }
}
```

## 5. 前端实现细节

### 5.1 左侧树结构改造

```javascript
// 渲染左侧树
function renderOutputTree() {
    return `
        <div class="output-section">
            <!-- Modbus 输出 -->
            <div class="section-header">
                <i class="bi bi-ethernet"></i> Modbus 输出
                <div class="form-check form-switch ms-auto">
                    <input class="form-check-input" type="checkbox" 
                           id="modbusEnabled" ${modbusEnabled ? 'checked' : ''}
                           onchange="toggleModbusOutput(this.checked)">
                </div>
            </div>
            <div class="section-body ${modbusEnabled ? '' : 'disabled'}">
                ${renderModbusGroups()}
            </div>
            
            <!-- IEC 61850 输出 -->
            <div class="section-header">
                <i class="bi bi-broadcast"></i> IEC 61850 输出
                <div class="form-check form-switch ms-auto">
                    <input class="form-check-input" type="checkbox" 
                           id="iec61850Enabled" ${iec61850Enabled ? 'checked' : ''}
                           onchange="toggleIEC61850Output(this.checked)">
                </div>
            </div>
            <div class="section-body ${iec61850Enabled ? '' : 'disabled'}">
                ${renderIEC61850Tree()}
            </div>
        </div>
    `;
}
```

### 5.2 IEC 61850 数据模型编辑器

```javascript
// 数据模型编辑器组件
class IEC61850ModelEditor {
    constructor(container) {
        this.container = container;
        this.model = null;
    }
    
    // 渲染模型树
    renderModelTree(model) {
        let html = '<div class="model-tree">';
        model.logical_devices.forEach(device => {
            html += this.renderLogicalDevice(device);
        });
        html += '</div>';
        return html;
    }
    
    // 渲染逻辑设备
    renderLogicalDevice(device) {
        return `
            <div class="tree-node device-node">
                <div class="node-header">
                    <i class="bi bi-collection"></i>
                    <span>${device.name}</span>
                    <div class="node-actions">
                        <button onclick="editLogicalDevice('${device.name}')">编辑</button>
                        <button onclick="deleteLogicalDevice('${device.name}')">删除</button>
                    </div>
                </div>
                <div class="node-children">
                    ${device.logical_nodes.map(node => this.renderLogicalNode(device.name, node)).join('')}
                    <button onclick="addLogicalNode('${device.name}')">+ 新增节点</button>
                </div>
            </div>
        `;
    }
    
    // 渲染逻辑节点
    renderLogicalNode(deviceName, node) {
        return `
            <div class="tree-node node-node">
                <div class="node-header">
                    <i class="bi bi-diagram-2"></i>
                    <span>${node.name}</span>
                </div>
                <div class="node-children">
                    ${node.data_objects.map(obj => this.renderDataObject(deviceName, node.name, obj)).join('')}
                </div>
            </div>
        `;
    }
}
```

### 5.3 映射规则编辑弹窗

```javascript
// 映射规则编辑弹窗
function showIEC61850MappingModal(rule = null) {
    const isEdit = rule !== null;
    const title = isEdit ? '编辑 IEC 61850 映射规则' : '新增 IEC 61850 映射规则';
    
    // 填充源设备下拉列表
    const deviceSelect = document.getElementById('iec_source_device');
    deviceSelect.innerHTML = allDevices.map(d => 
        `<option value="${d.id}" ${rule?.source_device === d.id ? 'selected' : ''}>${d.name} (${d.id})</option>`
    ).join('');
    
    // 填充 IEC 61850 路径下拉列表
    const pathSelect = document.getElementById('iec_path');
    pathSelect.innerHTML = availablePaths.map(p => 
        `<option value="${p}" ${rule?.iec61850_path === p ? 'selected' : ''}>${p}</option>`
    ).join('');
    
    // 显示弹窗
    new bootstrap.Modal(document.getElementById('iec61850MappingModal')).show();
}
```

## 6. 实现步骤

### 阶段 1: 后端 API 开发

1. 新增 `internal/web/handler/iec61850.go` 处理器
2. 在 `internal/web/server.go` 注册新路由
3. 实现配置读取、保存、重启等功能

### 阶段 2: 前端页面改造

1. 修改 `web/frontend/pages/mapping.html` 左侧树结构
2. 新增 IEC 61850 配置面板
3. 新增数据模型编辑器
4. 新增映射规则编辑器

### 阶段 3: 联调测试

1. 测试配置读取和保存
2. 测试数据模型编辑
3. 测试映射规则编辑
4. 测试服务重启功能

## 7. 注意事项

1. **热重载支持**: 修改 IEC 61850 配置后需要重启服务才能生效
2. **路径校验**: 映射规则中的 IEC 61850 路径必须在数据模型中存在
3. **类型兼容**: 源数据类型和目标类型需要兼容
4. **配置备份**: 修改配置前建议自动备份原配置文件
5. **错误处理**: 提供友好的错误提示和回滚机制

## 8. 预期效果

改造完成后，用户可以在 Web 界面中：

1. **一键开关**: 启用/禁用 Modbus 和 IEC 61850 输出
2. **可视化配置**: 通过表单编辑 IEC 61850 服务器参数
3. **模型管理**: 图形化编辑 IEC 61850 数据模型
4. **映射配置**: 可视化配置 Modbus → IEC 61850 映射规则
5. **实时状态**: 查看 IEC 61850 服务运行状态和连接数
