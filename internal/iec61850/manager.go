// Package iec61850 管理 IEC 61850 MMS Server 的生命周期和数据更新
package iec61850

import (
	"fmt"
	"strings"
	"sync"

	"github.com/lyx6662/com-manager/lib-iec61850"
	"github.com/lyx6662/com-manager/pkg/config"
)

// Manager IEC 61850 服务管理器
type Manager struct {
	cfg    *config.ModbusToIEC61850Config
	model  *iec61850.Model
	server *iec61850.Server
	mu     sync.RWMutex
	debug  bool // 是否输出调试信息
}

// NewManager 创建 IEC 61850 管理器
func NewManager(cfg *config.ModbusToIEC61850Config) *Manager {
	return &Manager{
		cfg: cfg,
	}
}

// BuildModel 根据配置构建 IEC 61850 数据模型
func (m *Manager) BuildModel() error {
	if m.cfg == nil {
		return fmt.Errorf("IEC 61850 配置为空")
	}

	iedName := m.cfg.IEC61850.IEDName
	if iedName == "" {
		iedName = "GRID_GATEWAY"
	}

	// 创建模型
	model := iec61850.CreateModel(iedName)
	if model == nil {
		return fmt.Errorf("创建 IEC 61850 模型失败")
	}

	// 遍历逻辑设备
	for _, ldCfg := range m.cfg.Model.LogicalDevices {
		ld := model.AddLogicalDevice(ldCfg.Name)
		if ld == nil {
			model.Destroy()
			return fmt.Errorf("创建逻辑设备失败: %s", ldCfg.Name)
		}

		// libiec61850 动态模型会将 IED 名称拼接到 LD 名称前面
		// 例如 IED="GW", LD="GRID_GATEWAY" → MMS 中的 LD 名称为 "GWGRID_GATEWAY"
		mmsLDName := iedName + ldCfg.Name

		// 创建 LLN0 (逻辑设备零节点 - 必须存在)
		lln0 := ld.AddLogicalNode("LLN0")
		if lln0 == nil {
			model.Destroy()
			return fmt.Errorf("创建 LLN0 失败: %s", ldCfg.Name)
		}
		// LLN0 标准数据对象
		lln0Mod := lln0.AddDataObject("Mod")
		if lln0Mod != nil {
			lln0Mod.AddDataAttribute("stVal", iec61850.TypeInt32, iec61850.FCStatus, iec61850.TriggerDataChanged)
			lln0Mod.AddDataAttribute("q", iec61850.TypeQuality, iec61850.FCStatus, iec61850.TriggerQualityChanged)
			lln0Mod.AddDataAttribute("t", iec61850.TypeTimestamp, iec61850.FCStatus, 0)
		}
		lln0Beh := lln0.AddDataObject("Beh")
		if lln0Beh != nil {
			lln0Beh.AddDataAttribute("stVal", iec61850.TypeInt32, iec61850.FCStatus, iec61850.TriggerDataChanged)
			lln0Beh.AddDataAttribute("q", iec61850.TypeQuality, iec61850.FCStatus, iec61850.TriggerQualityChanged)
			lln0Beh.AddDataAttribute("t", iec61850.TypeTimestamp, iec61850.FCStatus, 0)
		}
		lln0Health := lln0.AddDataObject("Health")
		if lln0Health != nil {
			lln0Health.AddDataAttribute("stVal", iec61850.TypeInt32, iec61850.FCStatus, iec61850.TriggerDataChanged)
			lln0Health.AddDataAttribute("q", iec61850.TypeQuality, iec61850.FCStatus, iec61850.TriggerQualityChanged)
			lln0Health.AddDataAttribute("t", iec61850.TypeTimestamp, iec61850.FCStatus, 0)
		}
		lln0NamPlt := lln0.AddDataObject("NamPlt")
		if lln0NamPlt != nil {
			lln0NamPlt.AddDataAttribute("vendor", iec61850.TypeVisibleString255, iec61850.FCDescription, 0)
			lln0NamPlt.AddDataAttribute("swRev", iec61850.TypeVisibleString255, iec61850.FCDescription, 0)
			lln0NamPlt.AddDataAttribute("d", iec61850.TypeVisibleString255, iec61850.FCDescription, 0)
			lln0NamPlt.AddDataAttribute("configRev", iec61850.TypeVisibleString255, iec61850.FCDescription, 0)
			lln0NamPlt.AddDataAttribute("lnNs", iec61850.TypeVisibleString255, iec61850.FCDescription, 0)
		}

		// 遍历逻辑节点
		for _, lnCfg := range ldCfg.LogicalNodes {
			ln := ld.AddLogicalNode(lnCfg.Name)
			if ln == nil {
				model.Destroy()
				return fmt.Errorf("创建逻辑节点失败: %s/%s", ldCfg.Name, lnCfg.Name)
			}

			// 遍历数据对象
			for _, doCfg := range lnCfg.DataObjects {
				if err := m.buildDataObject(model, mmsLDName, lnCfg.Name, doCfg.Name, doCfg, ln); err != nil {
					model.Destroy()
					return err
				}
			}
		}
	}

	m.model = model
	return nil
}

// buildDataObject 递归构建数据对象及其子节点
// parent 可以是 *iec61850.LogicalNode 或 *iec61850.DataObject
func (m *Manager) buildDataObject(model *iec61850.Model, ldName, lnName, doPath string, doCfg config.DataObjectConfig, parent interface{}) error {
	// 创建数据对象
	var do *iec61850.DataObject
	switch p := parent.(type) {
	case *iec61850.LogicalNode:
		do = p.AddDataObject(doCfg.Name)
	case *iec61850.DataObject:
		do = p.AddDataObject(doCfg.Name)
	default:
		return fmt.Errorf("不支持的父节点类型: %T", parent)
	}

	if do == nil {
		return fmt.Errorf("创建数据对象失败: %s/%s.%s", ldName, lnName, doPath)
	}

	// 遍历数据属性
	for _, daCfg := range doCfg.DataAttributes {
		if err := m.buildDataAttribute(model, ldName, lnName, doPath, daCfg, do); err != nil {
			return err
		}
	}

	return nil
}

// buildDataAttribute 递归构建数据属性
// 如果有 Children，创建子 DataObject；如果是叶子节点，创建 DataAttribute 并注册
func (m *Manager) buildDataAttribute(model *iec61850.Model, ldName, lnName, doPath string, daCfg config.DataAttributeConfig, do *iec61850.DataObject) error {
	if len(daCfg.Children) > 0 {
		// 有子属性 → 当前属性作为中间 DataObject (如 "mag")
		subDO := do.AddDataObject(daCfg.Name)
		if subDO == nil {
			return fmt.Errorf("创建子数据对象失败: %s/%s.%s.%s", ldName, lnName, doPath, daCfg.Name)
		}

		newDoPath := doPath + "." + daCfg.Name
		for _, child := range daCfg.Children {
			if err := m.buildDataAttribute(model, ldName, lnName, newDoPath, child, subDO); err != nil {
				return err
			}
		}
		return nil
	}

	// 叶子节点 → 创建 DataAttribute
	dataType := parseDataType(daCfg.Type)
	fc := parseFC(daCfg.FC)
	triggers := parseTriggers(daCfg.Triggers)

	da := do.AddDataAttribute(daCfg.Name, dataType, fc, triggers)
	if da == nil {
		return fmt.Errorf("创建数据属性失败: %s/%s.%s.%s", ldName, lnName, doPath, daCfg.Name)
	}

	// 注册完整路径 (如 GWWG/MMXU1.TotW.mag.f)
	// 需要加上 IED 名称前缀，因为 libiec61850 的 MMS 路径格式为 IED+LD/LN.DO...
	fullPath := ldName + "/" + lnName + "." + doPath + "." + daCfg.Name
	iedName := m.cfg.IEC61850.IEDName
	if iedName != "" {
		fullPath = iedName + fullPath
	}
	model.RegisterDA(fullPath, da)

	return nil
}

// Start 创建并启动 IEC 61850 MMS 服务器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.model == nil {
		return fmt.Errorf("数据模型未构建，请先调用 BuildModel")
	}

	port := m.cfg.IEC61850.Port
	if port == 0 {
		port = 102
	}

	srv := iec61850.NewServer(m.model, iec61850.ServerConfig{
		TCPPort:        port,
		MaxConnections: m.cfg.IEC61850.MaxConnections,
	})
	if srv == nil {
		return fmt.Errorf("创建 IEC 61850 服务器失败")
	}

	if err := srv.Start(); err != nil {
		srv.Destroy()
		return fmt.Errorf("启动 IEC 61850 服务器失败: %w", err)
	}

	m.server = srv

	// 启用读取访问日志
	srv.EnableReadLog()

	// 生成 ICD 文件
	if icdPath := m.cfg.IEC61850.ICDOutput; icdPath != "" {
		if err := GenerateICD(m.cfg, icdPath); err != nil {
			fmt.Printf("[IEC61850] 生成 ICD 文件失败: %v\n", err)
		} else {
			fmt.Printf("[IEC61850] ICD 文件已生成: %s\n", icdPath)
		}
	}

	return nil
}

// Stop 停止并销毁 IEC 61850 服务器
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.server != nil {
		m.server.Stop()
		m.server.Destroy()
		m.server = nil
	}
	if m.model != nil {
		m.model.Destroy()
		m.model = nil
	}
}

// IsRunning 检查服务器是否正在运行
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.server == nil {
		return false
	}
	return m.server.IsRunning()
}

// GetConnectionCount 获取当前客户端连接数
func (m *Manager) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.server == nil {
		return 0
	}
	return m.server.GetConnectionCount()
}

// UpdateData 更新 IEC 61850 数据属性值
// path: IEC 61850 对象引用 (如 "GRID_GATEWAY/MMXU1.TotW.mag.f")
// value: 数据值
// quality: 质量码 (0=Good, 0x80=Bad)
// timestamp: 时间戳 (毫秒)
func (m *Manager) UpdateData(path string, value interface{}, quality uint16, timestamp int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.server == nil {
		return fmt.Errorf("IEC 61850 服务器未启动")
	}

	// 更新值 (加锁确保线程安全)
	m.server.LockDataModel()
	if err := m.updateValue(path, value); err != nil {
		m.server.UnlockDataModel()
		return err
	}
	m.server.UnlockDataModel()

	// 更新质量码 (如果有对应的 .q 属性)
	m.updateQuality(path, quality)

	// 更新时标 (如果有对应的 .t 属性)
	m.updateTimestamp(path, timestamp)

	return nil
}

// updateValue 根据值类型更新对应的 IEC 61850 数据属性
// path 是配置中的 IEC61850Path (如 "GRID_GATEWAY/MMXU1.TotW.mag.f")
// 需要转换为 libiec61850 的 MMS 路径 (如 "GWGRID_GATEWAY/MMXU1.TotW.mag.f")
func (m *Manager) updateValue(path string, value interface{}) error {
	// 添加 IED 名称前缀到 LD 部分
	// libiec61850 动态模型: IED="GW", LD="GRID_GATEWAY" → MMS LD="GWGRID_GATEWAY"
	// 添加 IED 名称前缀到 LD 部分
	// libiec61850 动态模型: IED="GW", LD="GRID_GATEWAY" → MMS LD="GWGRID_GATEWAY"
	iedName := m.cfg.IEC61850.IEDName
	if iedName != "" {
		slashIdx := strings.Index(path, "/")
		if slashIdx > 0 {
			ldPart := path[:slashIdx]
			rest := path[slashIdx+1:]
			path = iedName + ldPart + "/" + rest
		}
	}
	// 根据值类型选择更新方法
	// float → UpdateFloat (用于 MX 测量值)
	// 整数 → UpdateInt32 (用于 ST 状态值)
	switch v := value.(type) {
	case float32:
		return m.server.UpdateFloatByPath(path, v)
	case float64:
		return m.server.UpdateFloatByPath(path, float32(v))
	case int32:
		return m.server.UpdateInt32ByPath(path, v)
	case int:
		return m.server.UpdateInt32ByPath(path, int32(v))
	case uint16:
		return m.server.UpdateInt32ByPath(path, int32(v))
	case uint32:
		return m.server.UpdateInt32ByPath(path, int32(v))
	case int16:
		return m.server.UpdateInt32ByPath(path, int32(v))
	case bool:
		return m.server.UpdateBoolByPath(path, v)
	case string:
		return m.server.UpdateStringByPath(path, v)
	default:
		return fmt.Errorf("不支持的数据类型: %T (路径: %s)", value, path)
	}
}

// updateQuality 更新质量码
// 根据 IEC 61850 模型结构，q 和 t 在数据对象级别
// 例如 "GRID_GATEWAY/MMXU1.TotW.mag.f" → "GRID_GATEWAY/MMXU1.TotW.q"
// 例如 "GRID_GATEWAY/CSWI1.Mod.stVal" → "GRID_GATEWAY/CSWI1.Mod.q"
func (m *Manager) updateQuality(path string, quality uint16) {
	if m.model == nil {
		return
	}
	qPath := buildDataObjectAttrPath(path, "q")
	if qPath == "" {
		return
	}
	// 添加 IED 名称前缀
	iedName := m.cfg.IEC61850.IEDName
	fullPath := qPath
	if iedName != "" {
		slashIdx := strings.Index(qPath, "/")
		if slashIdx > 0 {
			fullPath = iedName + qPath
		}
	}
	// 查找数据属性并使用专门的 UpdateQuality 方法
	da := m.model.FindDA(fullPath)
	if da != nil {
		m.server.UpdateQuality(da, quality)
	} else {
		fmt.Printf("[IEC61850] 品质属性未找到: %s (原始路径: %s)\n", fullPath, path)
	}
}

// updateTimestamp 更新时标
// 根据 IEC 61850 模型结构，q 和 t 在数据对象级别
// 例如 "GRID_GATEWAY/MMXU1.TotW.mag.f" → "GRID_GATEWAY/MMXU1.TotW.t"
// 例如 "GRID_GATEWAY/CSWI1.Mod.stVal" → "GRID_GATEWAY/CSWI1.Mod.t"
func (m *Manager) updateTimestamp(path string, timestamp int64) {
	if m.model == nil {
		return
	}
	tPath := buildDataObjectAttrPath(path, "t")
	if tPath == "" {
		return
	}
	// 添加 IED 名称前缀
	iedName := m.cfg.IEC61850.IEDName
	fullPath := tPath
	if iedName != "" {
		slashIdx := strings.Index(tPath, "/")
		if slashIdx > 0 {
			fullPath = iedName + tPath
		}
	}
	// 查找数据属性并使用专门的 UpdateTimestamp 方法
	da := m.model.FindDA(fullPath)
	if da != nil {
		m.server.UpdateTimestamp(da, timestamp)
	} else {
		fmt.Printf("[IEC61850] 时标属性未找到: %s (原始路径: %s)\n", fullPath, path)
	}
}

// buildDataObjectAttrPath 构建数据对象级别的属性路径
// IEC 61850 结构: LD/LN.DO.subDO.leaf (如 MMXU1.TotW.mag.f)
// q 和 t 在 DO 级别: LD/LN.DO.q, LD/LN.DO.t
//
// 规则: 从路径中提取 LD/LN.DO 部分，然后拼接属性名
// "LD/LN.DO.mag.f" → "LD/LN.DO.q" (mag 是中间容器，跳过)
// "LD/LN.DO.stVal" → "LD/LN.DO.q" (stVal 直接在 DO 下)
func buildDataObjectAttrPath(valuePath string, attrName string) string {
	// 分离 LD 部分和 LN.DO 部分
	slashIdx := strings.Index(valuePath, "/")
	if slashIdx < 0 {
		return ""
	}
	ldPart := valuePath[:slashIdx+1] // 包含 "/"
	lnDoPart := valuePath[slashIdx+1:]

	// 按 "." 分割 LN.DO 部分
	parts := strings.Split(lnDoPart, ".")
	if len(parts) < 2 {
		// 至少需要 LN 和 DO
		return ""
	}

	// LN 是第一部分，DO 是第二部分
	// q 和 t 在 DO 级别，即 LD/LN.DO.q
	return ldPart + parts[0] + "." + parts[1] + "." + attrName
}

// === 类型转换辅助函数 ===

// parseDataType 将配置中的类型字符串转为 iec61850.DataType
func parseDataType(typeStr string) iec61850.DataType {
	switch strings.ToUpper(typeStr) {
	case "BOOLEAN":
		return iec61850.TypeBoolean
	case "INT8":
		return iec61850.TypeInt8
	case "INT16":
		return iec61850.TypeInt16
	case "INT32":
		return iec61850.TypeInt32
	case "INT64":
		return iec61850.TypeInt64
	case "UINT8", "INT8U":
		return iec61850.TypeUint8
	case "UINT16", "INT16U":
		return iec61850.TypeUint16
	case "UINT32", "INT32U":
		return iec61850.TypeUint32
	case "FLOAT32":
		return iec61850.TypeFloat32
	case "FLOAT64":
		return iec61850.TypeFloat64
	case "VISIBLE_STRING_255", "STRING":
		return iec61850.TypeVisibleString255
	case "QUALITY":
		return iec61850.TypeQuality
	case "TIMESTAMP":
		return iec61850.TypeTimestamp
	default:
		return iec61850.TypeFloat32
	}
}

// parseFC 将配置中的功能约束字符串转为 iec61850.FC
func parseFC(fcStr string) iec61850.FC {
	switch strings.ToUpper(fcStr) {
	case "ST":
		return iec61850.FCStatus
	case "MX":
		return iec61850.FCMeasurand
	case "SP":
		return iec61850.FCSetpoint
	case "CF":
		return iec61850.FCConfig
	case "DC":
		return iec61850.FCDescription
	default:
		return iec61850.FCMeasurand
	}
}

// parseTriggers 将配置中的触发选项字符串转为 iec61850.TriggerOptions
func parseTriggers(triggerStr string) iec61850.TriggerOptions {
	switch strings.ToUpper(triggerStr) {
	case "DATA_CHANGED":
		return iec61850.TriggerDataChanged
	case "QUALITY_CHANGED":
		return iec61850.TriggerQualityChanged
	case "DATA_UPDATE":
		return iec61850.TriggerDataUpdate
	case "INTEGRITY":
		return iec61850.TriggerIntegrity
	case "GI":
		return iec61850.TriggerGI
	default:
		return iec61850.TriggerDataChanged
	}
}
