package core

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/lyx6662/com-manager/lib-modbus/rtu"
	"github.com/lyx6662/com-manager/lib-modbus/tcp"
	"github.com/lyx6662/com-manager/internal/iec61850"
	"github.com/lyx6662/com-manager/internal/storage/alarm"
	"github.com/lyx6662/com-manager/internal/storage/buffer"
	"github.com/lyx6662/com-manager/internal/web"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
	"github.com/lyx6662/com-manager/pkg/model"
)

// Engine 核心引擎
type Engine struct {
	cfgMgr       *config.Manager
	cfg          *config.Config
	log          *logger.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup

	// 各管理器
	webServer     *web.Server
	offlineBuffer *buffer.OfflineBuffer
	alarmStore    *alarm.Store            // 报警存储
	alarmDetector *AlarmDetector          // 报警检测器
	tcpServers    map[string]*tcp.Server  // Modbus TCP 输出服务器
	rtuServers    map[string]*rtu.Server  // Modbus RTU 输出服务器
	router        *Router                 // 数据路由器 (保留用于向后兼容)
	collector     *Collector              // 采集调度器
	iec61850Mgr   *iec61850.Manager       // IEC 61850 管理器

	// 新架构组件
	dataPool      *DataPool               // 统一数据共享池
	modbusAdapter *ModbusOutputAdapter     // Modbus 输出适配器
	iecAdapter    *IEC61850OutputAdapter   // IEC 61850 输出适配器
	commandBus    *CommandBus             // 命令总线
	webControl    *WebControlSource       // Web 控制来源

	// 新配置管理器
	dataPointCfg  *config.DataPointFileConfig    // 数据点配置
	outputCfg     *config.OutputFileConfigManager // 输出配置
	useNewConfig  bool                           // 是否使用新配置格式
}

// NewEngine 创建引擎
func NewEngine(cfgMgr *config.Manager, log *logger.Logger) (*Engine, error) {
	engine := &Engine{
		cfgMgr:     cfgMgr,
		cfg:        cfgMgr.Get(),
		log:        log,
		tcpServers: make(map[string]*tcp.Server),
		rtuServers: make(map[string]*rtu.Server),
		dataPool:   NewDataPool(log),
	}

	// 尝试加载新配置格式
	engine.loadNewConfig()

	return engine, nil
}

// loadNewConfig 加载新配置格式
func (e *Engine) loadNewConfig() {
	// 加载数据点配置
	dataPointCfgPath := "./configs/data_points.yaml"
	e.dataPointCfg = config.NewDataPointFileConfig(dataPointCfgPath)
	if err := e.dataPointCfg.Load(); err != nil {
		e.log.Warn("加载数据点配置失败，将使用旧配置格式", "error", err)
		e.dataPointCfg = nil
		return
	}

	// 加载输出配置
	outputCfgPath := "./configs/outputs.yaml"
	e.outputCfg = config.NewOutputFileConfigManager(outputCfgPath)
	if err := e.outputCfg.Load(); err != nil {
		e.log.Warn("加载输出配置失败，将使用旧配置格式", "error", err)
		e.outputCfg = nil
		e.dataPointCfg = nil
		return
	}

	// 检查新配置是否有数据
	if e.dataPointCfg.GetDataPointCount() > 0 {
		e.useNewConfig = true
		e.log.Info("使用新配置格式",
			"data_points", e.dataPointCfg.GetDataPointCount(),
		)
	} else {
		e.log.Info("新配置文件为空，将使用旧配置格式")
		e.useNewConfig = false
	}
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.ctx, e.cancel = context.WithCancel(ctx)

	e.log.Info("引擎启动中...")

	// 1. 初始化存储
	if err := e.initStorage(); err != nil {
		return fmt.Errorf("初始化存储失败: %w", err)
	}

	// 2. 启动输出服务 (Modbus TCP/RTU Server)
	if e.cfg.Outputs.Enabled {
		if err := e.startOutputs(); err != nil {
			return fmt.Errorf("启动输出服务失败: %w", err)
		}
	} else {
		e.log.Info("Modbus 输出服务已禁用，跳过")
	}

	// 启动心跳更新
	e.startHeartbeat()

	// 2.5 启动 IEC 61850 服务
	if e.cfgMgr.IsIEC61850Enabled() {
		if err := e.startIEC61850(); err != nil {
			e.log.Error("启动 IEC 61850 服务失败，继续运行", "error", err)
		}
	} else {
		e.log.Info("IEC 61850 服务已禁用，跳过")
	}

	// 3. 初始化数据路由器 (保留用于向后兼容)
	e.initRouter()

	// 3.5 初始化输出适配器 (新架构)
	e.initOutputAdapters()

	// 3.6 初始化命令总线 (双向控制)
	e.initCommandBus()

	// 4. 初始化报警检测器
	e.initAlarmDetector()

	// 5. 启动采集调度器
	if err := e.startCollector(); err != nil {
		return fmt.Errorf("启动采集调度器失败: %w", err)
	}

	// 5.5 设置设备状态提供者 (用于 IEC 61850 品质码)
	if e.router != nil && e.collector != nil {
		e.router.SetDeviceStatusProvider(e.collector)
	}
	if e.iecAdapter != nil && e.collector != nil {
		e.iecAdapter.SetDeviceStatusProvider(e.collector)
	}

	// 6. 启动Web服务
	if e.cfg.Web.Enabled {
		if err := e.startWebServer(); err != nil {
			return fmt.Errorf("启动Web服务失败: %w", err)
		}
	}

	e.log.Info("引擎启动完成",
		"serial_devices", len(e.cfg.SerialDevices),
		"network_devices", len(e.cfg.NetworkDevices),
		"tcp_outputs", len(e.cfg.Outputs.ModbusTCPServers),
		"data_pool_points", e.dataPool.GetDataPointCount(),
	)
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.log.Info("引擎停止中...")
	e.cancel()

	// 停止命令总线
	if e.commandBus != nil {
		e.commandBus.Stop()
		e.log.Info("命令总线已停止")
	}

	// 停止输出适配器
	if e.modbusAdapter != nil {
		e.modbusAdapter.Stop()
		e.log.Info("Modbus 输出适配器已停止")
	}
	if e.iecAdapter != nil {
		e.iecAdapter.Stop()
		e.log.Info("IEC 61850 输出适配器已停止")
	}

	// 停止 IEC 61850 服务
	if e.iec61850Mgr != nil {
		e.iec61850Mgr.Stop()
		e.log.Info("IEC 61850 服务已停止")
	}

	// 停止采集调度器
	if e.collector != nil {
		e.collector.Stop()
	}

	// 停止所有TCP服务器
	for id, srv := range e.tcpServers {
		if err := srv.Close(); err != nil {
			e.log.Error("停止TCP服务器失败", "id", id, "error", err)
		}
	}

	// 停止所有RTU服务器
	for id, srv := range e.rtuServers {
		if err := srv.Close(); err != nil {
			e.log.Error("停止RTU服务器失败", "id", id, "error", err)
		}
	}

	if e.webServer != nil {
		if err := e.webServer.Stop(); err != nil {
			e.log.Error("停止Web服务失败", "error", err)
		}
	}

	if e.offlineBuffer != nil {
		if err := e.offlineBuffer.Close(); err != nil {
			e.log.Error("关闭离线缓冲失败", "error", err)
		}
	}

	e.wg.Wait()
	e.log.Info("引擎已停止")
}

// Reload 热重载配置
func (e *Engine) Reload() error {
	e.log.Info("重新加载配置...")

	// 重新加载配置文件
	newCfg, err := config.Load("./configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	oldCfg := e.cfg
	e.cfg = newCfg

	// 更新配置管理器中的配置
	*e.cfgMgr.Get() = *newCfg

	// 更新路由器映射配置
	if e.router != nil {
		e.router.SetMappings(newCfg.Mappings)
		e.router.SetGroupDevices(newCfg.Outputs.GroupDevices)
		e.log.Info("路由映射已更新")
	}

	// 重建采集调度器
	if e.collector != nil {
		e.collector.Stop()
		e.collector.ClearDevices()

		for _, dev := range newCfg.SerialDevices {
			e.collector.AddSerialDevice(dev)
			mappings := e.getDeviceMappings(dev.ID)
			if len(mappings) > 0 {
				e.collector.SetDeviceMappings(dev.ID, mappings)
			}
		}
		for _, dev := range newCfg.NetworkDevices {
			e.collector.AddNetworkDevice(dev)
			mappings := e.getDeviceMappings(dev.ID)
			if len(mappings) > 0 {
				e.collector.SetDeviceMappings(dev.ID, mappings)
			}
		}

		e.collector.Start()
		e.log.Info("采集调度器已重建",
			"serial_devices", len(newCfg.SerialDevices),
			"network_devices", len(newCfg.NetworkDevices),
		)
	}

	// 增量更新 TCP 输出服务器
	oldTCPIDs := make(map[string]bool)
	newTCPIDs := make(map[string]bool)
	for id := range oldCfg.Outputs.ModbusTCPServers {
		oldTCPIDs[oldCfg.Outputs.ModbusTCPServers[id].ID] = true
	}
	for _, srv := range newCfg.Outputs.ModbusTCPServers {
		newTCPIDs[srv.ID] = true
	}

	// 关闭已移除的 TCP 服务器
	for id, srv := range e.tcpServers {
		if !newTCPIDs[id] {
			if err := srv.Close(); err != nil {
				e.log.Error("关闭TCP服务器失败", "id", id, "error", err)
			}
			delete(e.tcpServers, id)
			e.log.Info("TCP服务器已移除", "id", id)
		}
	}

	// 启动新增的 TCP 服务器
	for _, srvCfg := range newCfg.Outputs.ModbusTCPServers {
		if !oldTCPIDs[srvCfg.ID] {
			if err := e.startTCPServer(srvCfg); err != nil {
				e.log.Error("启动TCP服务器失败", "id", srvCfg.ID, "error", err)
			}
		}
	}

	// 增量更新 RTU 输出服务器
	oldRTUIDs := make(map[string]bool)
	newRTUIDs := make(map[string]bool)
	for id := range oldCfg.Outputs.ModbusRTUServers {
		oldRTUIDs[oldCfg.Outputs.ModbusRTUServers[id].ID] = true
	}
	for _, srv := range newCfg.Outputs.ModbusRTUServers {
		newRTUIDs[srv.ID] = true
	}

	// 关闭已移除的 RTU 服务器
	for id, srv := range e.rtuServers {
		if !newRTUIDs[id] {
			if err := srv.Close(); err != nil {
				e.log.Error("关闭RTU服务器失败", "id", id, "error", err)
			}
			delete(e.rtuServers, id)
			e.log.Info("RTU服务器已移除", "id", id)
		}
	}

	// 启动新增的 RTU 服务器
	for _, srvCfg := range newCfg.Outputs.ModbusRTUServers {
		if !oldRTUIDs[srvCfg.ID] {
			if err := e.startRTUServer(srvCfg); err != nil {
				e.log.Error("启动RTU服务器失败", "id", srvCfg.ID, "error", err)
			}
		}
	}

	// 更新路由器中的服务器注册
	e.initRouter()

	// 更新输出适配器配置
	e.reloadOutputAdapters()

	// 重新加载报警规则
	if e.alarmDetector != nil {
		e.alarmDetector.LoadRulesFromConfig(newCfg)
		e.log.Info("报警规则已更新")
	}

	e.log.Info("配置重载成功")
	return nil
}

// GetConfigManager 获取配置管理器
func (e *Engine) GetConfigManager() *config.Manager {
	return e.cfgMgr
}

// GetOfflineBuffer 获取离线缓冲
func (e *Engine) GetOfflineBuffer() *buffer.OfflineBuffer {
	return e.offlineBuffer
}

// GetCollector 获取采集调度器
func (e *Engine) GetCollector() *Collector {
	return e.collector
}

// GetRouter 获取路由器
func (e *Engine) GetRouter() *Router {
	return e.router
}

func (e *Engine) initStorage() error {
	e.log.Info("初始化存储...")

	if e.cfg.OfflineBuffer.Enabled {
		flushInterval, err := time.ParseDuration(e.cfg.OfflineBuffer.FlushInterval)
		if err != nil {
			flushInterval = 10 * time.Minute
		}

		buf, err := buffer.NewOfflineBuffer(
			e.cfg.OfflineBuffer.DBPath,
			e.cfg.OfflineBuffer.RetentionDays,
			flushInterval,
			e.log,
		)
		if err != nil {
			return fmt.Errorf("初始化离线缓冲失败: %w", err)
		}

		// 设置刷盘条件：上位机未连接 且 底层设备已连接
		buf.SetShouldFlush(func() (bool, string) {
			// 检查是否有上位机在线
			for groupID := range e.cfg.Mappings {
				if e.isMasterConnected(groupID) {
					return false, "上位机在线，不需要写入"
				}
			}

			// 检查是否有底层设备在线
			if e.collector != nil {
				statuses := e.collector.GetAllDeviceStatus()
				for _, s := range statuses {
					if ds, ok := s.(*DeviceStatus); ok && ds.Online {
						return true, "上位机未连接且设备在线"
					}
				}
			}
			return false, "底层设备全部离线"
		})

		e.offlineBuffer = buf
		e.log.Info("离线缓冲初始化成功",
			"path", e.cfg.OfflineBuffer.DBPath,
			"retention_days", e.cfg.OfflineBuffer.RetentionDays,
		)

		// 初始化报警存储 (使用同一个数据库)
		db := buf.GetDB()
		if db != nil {
			store, err := alarm.NewStore(db, e.log)
			if err != nil {
				return fmt.Errorf("初始化报警存储失败: %w", err)
			}
			e.alarmStore = store
			e.log.Info("报警存储初始化成功")
		}
	}

	return nil
}

func (e *Engine) startOutputs() error {
	e.log.Info("启动输出服务...")

	// 启动所有 Modbus TCP Server
	for _, srvCfg := range e.cfg.Outputs.ModbusTCPServers {
		if err := e.startTCPServer(srvCfg); err != nil {
			e.log.Error("启动TCP服务器失败",
				"id", srvCfg.ID,
				"port", srvCfg.ListenPort,
				"error", err,
			)
			continue
		}
	}

	// 启动所有 Modbus RTU Server
	for _, srvCfg := range e.cfg.Outputs.ModbusRTUServers {
		if err := e.startRTUServer(srvCfg); err != nil {
			e.log.Error("启动RTU服务器失败",
				"id", srvCfg.ID,
				"port", srvCfg.Port,
				"error", err,
			)
			continue
		}
	}

	return nil
}

func (e *Engine) startTCPServer(cfg config.ModbusTCPServerConfig) error {
	srvCfg := tcp.ServerConfig{
		ID:             cfg.ID,
		Name:           cfg.Name,
		ListenPort:     cfg.ListenPort,
		SlaveID:        byte(cfg.SlaveID),
		MaxConnections: cfg.MaxConnections,
	}

	srv := tcp.NewServer(srvCfg, e.log)

	// 设置断点续传回调
	if e.offlineBuffer != nil {
		srv.OnMasterConnected(func() {
			e.log.Info("主机已连接，触发数据补传", "server", cfg.ID)
			e.handleMasterConnected(cfg.ID)
		})
		srv.OnMasterDisconnected(func() {
			e.log.Warn("主机断开连接", "server", cfg.ID)
		})
	}

	if err := srv.Listen(); err != nil {
		return err
	}

	e.tcpServers[cfg.ID] = srv
	e.log.Info("TCP服务器启动成功",
		"id", cfg.ID,
		"port", cfg.ListenPort,
	)
	return nil
}

func (e *Engine) startRTUServer(cfg config.ModbusRTUServerConfig) error {
	srvCfg := rtu.ServerConfig{
		ID:       cfg.ID,
		Name:     cfg.Name,
		Port:     cfg.Port,
		BaudRate: cfg.BaudRate,
		DataBits: 8,
		StopBits: 1,
		Parity:   "none",
		SlaveID:  byte(cfg.SlaveID),
	}

	// 使用默认值
	if srvCfg.BaudRate == 0 {
		srvCfg.BaudRate = 9600
	}

	srv := rtu.NewServer(srvCfg, e.log)

	// 设置断点续传回调
	if e.offlineBuffer != nil {
		srv.OnMasterConnected(func() {
			e.log.Info("RTU主机已连接，触发数据补传", "server", cfg.ID)
			e.handleMasterConnected(cfg.ID)
		})
		srv.OnMasterDisconnected(func() {
			e.log.Warn("RTU主机断开连接", "server", cfg.ID)
		})
	}

	if err := srv.Listen(); err != nil {
		return err
	}

	e.rtuServers[cfg.ID] = srv
	e.log.Info("RTU服务器启动成功",
		"id", cfg.ID,
		"port", cfg.Port,
		"baud_rate", srvCfg.BaudRate,
		"slave_id", cfg.SlaveID,
	)
	return nil
}

// initRouter 初始化或更新数据路由器
func (e *Engine) initRouter() {
	if e.router == nil {
		e.router = NewRouter(e.log)
	}

	// 清空旧的服务器注册，防止切换类型时残留
	e.router.ClearServers()

	// 注册当前TCP输出服务器
	for groupID, srv := range e.tcpServers {
		e.router.RegisterServer(groupID, srv)
	}

	// 注册当前RTU输出服务器
	for groupID, srv := range e.rtuServers {
		e.router.RegisterRTUServer(groupID, srv)
	}

	// 设置映射配置
	e.router.SetMappings(e.cfg.Mappings)
	e.router.SetGroupDevices(e.cfg.Outputs.GroupDevices)

	// 注入 IEC 61850 管理器和映射规则
	if e.iec61850Mgr != nil {
		e.router.SetIEC61850Manager(e.iec61850Mgr)
		e.router.SetIEC61850Mappings(e.cfgMgr.GetIEC61850Mappings())
	}

	e.log.Info("数据路由器初始化完成",
		"mapping_groups", len(e.cfg.Mappings),
		"tcp_servers", len(e.tcpServers),
		"rtu_servers", len(e.rtuServers),
	)
}

// startCollector 启动采集调度器
func (e *Engine) startCollector() error {
	// 创建采集器，注册数据回调
	e.collector = NewCollector(e.log, func(deviceID string, points []model.DataPoint) {
		e.onDeviceData(deviceID, points)
	})

	// 注册串口设备
	for _, dev := range e.cfg.SerialDevices {
		e.collector.AddSerialDevice(dev)
		// 设置该设备的映射规则
		mappings := e.getDeviceMappings(dev.ID)
		if len(mappings) > 0 {
			e.collector.SetDeviceMappings(dev.ID, mappings)
		}
	}

	// 注册网口设备
	for _, dev := range e.cfg.NetworkDevices {
		e.collector.AddNetworkDevice(dev)
		mappings := e.getDeviceMappings(dev.ID)
		if len(mappings) > 0 {
			e.collector.SetDeviceMappings(dev.ID, mappings)
		}
	}

	// 启动采集
	e.collector.Start()

	return nil
}

// getDeviceMappings 获取指定设备的映射规则
func (e *Engine) getDeviceMappings(deviceID string) []config.MappingRule {
	if e.cfg.Mappings == nil {
		return nil
	}
	return e.cfg.Mappings[deviceID]
}

// initAlarmDetector 初始化报警检测器
func (e *Engine) initAlarmDetector() {
	if e.alarmStore == nil {
		return
	}

	e.alarmDetector = NewAlarmDetector(e.log, e.alarmStore)
	e.alarmDetector.LoadRulesFromConfig(e.cfg)

	e.log.Info("报警检测器初始化完成")
}

// initOutputAdapters 初始化输出适配器
func (e *Engine) initOutputAdapters() {
	// 初始化 Modbus 输出适配器
	e.modbusAdapter = NewModbusOutputAdapter(e.log, "modbus", "default")
	e.modbusAdapter.Init(e.dataPool)

	// 设置 Modbus 服务器
	for groupID, srv := range e.tcpServers {
		e.modbusAdapter.SetTCPServer(srv)
		e.log.Debug("Modbus 适配器绑定 TCP 服务器", "group", groupID)
	}
	for groupID, srv := range e.rtuServers {
		e.modbusAdapter.SetRTUServer(srv)
		e.log.Debug("Modbus 适配器绑定 RTU 服务器", "group", groupID)
	}

	// 根据配置格式设置映射规则
	if e.useNewConfig && e.outputCfg != nil && e.dataPointCfg != nil {
		// 使用新配置格式
		modbusOutput := e.outputCfg.GetModbusOutput()
		if modbusOutput != nil && modbusOutput.Enabled {
			e.modbusAdapter.SetMappingsFromOutputConfig(
				modbusOutput.Mappings,
				e.dataPointCfg.GetDataPoints(),
			)
			e.log.Info("Modbus 输出适配器使用新配置格式", "mappings", len(modbusOutput.Mappings))
		}
	} else {
		// 使用旧配置格式
		allMappings := make([]ModbusOutputMapping, 0)
		for _, rules := range e.cfg.Mappings {
			for _, rule := range rules {
				allMappings = append(allMappings, ModbusOutputMapping{
					SourceDevice:   rule.SourceDevice,
					SourceName:     rule.Name,
					SourceType:     rule.SourceType,
					DataType:       rule.DataType,
					TargetRegister: rule.TargetRegister,
					Scale:          rule.Scale,
					Offset:         rule.Offset,
					ByteOrder:      rule.ByteOrder,
					MaxPoints:      rule.MaxPoints,
				})
			}
		}
		e.modbusAdapter.SetMappings(allMappings)
		e.log.Info("Modbus 输出适配器使用旧配置格式", "mappings", len(allMappings))
	}

	e.modbusAdapter.Start()

	// 初始化 IEC 61850 输出适配器
	if e.iec61850Mgr != nil {
		e.iecAdapter = NewIEC61850OutputAdapter(e.log)
		e.iecAdapter.Init(e.dataPool)
		e.iecAdapter.SetIEC61850Manager(e.iec61850Mgr)

		// 根据配置格式设置映射规则
		if e.useNewConfig && e.outputCfg != nil && e.dataPointCfg != nil {
			iecOutput := e.outputCfg.GetIEC61850Output()
			if iecOutput != nil && iecOutput.Enabled {
				e.iecAdapter.SetMappingsFromOutputConfig(
					iecOutput.Mappings,
					e.dataPointCfg.GetDataPoints(),
				)
				e.log.Info("IEC 61850 输出适配器使用新配置格式", "mappings", len(iecOutput.Mappings))
			}
		} else {
			e.iecAdapter.SetMappings(e.cfgMgr.GetIEC61850Mappings())
			e.log.Info("IEC 61850 输出适配器使用旧配置格式")
		}

		e.iecAdapter.Start()
		e.log.Info("IEC 61850 输出适配器初始化完成")
	}
}

// initCommandBus 初始化命令总线
func (e *Engine) initCommandBus() {
	e.commandBus = NewCommandBus(e.log, e.dataPool)

	// 注册 Modbus 写入处理器
	modbusHandler := NewModbusWriteHandler(e.log, e.dataPool)
	// 注册设备写入连接（这里需要根据实际设备配置来注册）
	for _, dev := range e.cfg.NetworkDevices {
		// 网络设备的写入功能需要通过采集器的连接来实现
		// 暂时跳过，后续可以通过扩展采集器接口来支持
		_ = dev
	}
	e.commandBus.RegisterHandler(modbusHandler)

	// 注册 Web 控制来源
	e.webControl = NewWebControlSource()
	e.commandBus.RegisterSource(e.webControl)

	// 启动命令总线
	if err := e.commandBus.Start(); err != nil {
		e.log.Error("启动命令总线失败", "error", err)
	} else {
		e.log.Info("命令总线初始化完成")
	}
}

// reloadOutputAdapters 热重载输出适配器配置
func (e *Engine) reloadOutputAdapters() {
	// 更新 Modbus 输出适配器的服务器绑定
	if e.modbusAdapter != nil {
		// 重新绑定服务器
		for groupID, srv := range e.tcpServers {
			e.modbusAdapter.SetTCPServer(srv)
			e.log.Debug("Modbus 适配器重新绑定 TCP 服务器", "group", groupID)
		}
		for groupID, srv := range e.rtuServers {
			e.modbusAdapter.SetRTUServer(srv)
			e.log.Debug("Modbus 适配器重新绑定 RTU 服务器", "group", groupID)
		}

		// 更新映射规则
		allMappings := make([]ModbusOutputMapping, 0)
		for _, rules := range e.cfg.Mappings {
			for _, rule := range rules {
				allMappings = append(allMappings, ModbusOutputMapping{
					SourceDevice:   rule.SourceDevice,
					SourceName:     rule.Name,
					SourceType:     rule.SourceType,
					DataType:       rule.DataType,
					TargetRegister: rule.TargetRegister,
					Scale:          rule.Scale,
					Offset:         rule.Offset,
					ByteOrder:      rule.ByteOrder,
					MaxPoints:      rule.MaxPoints,
				})
			}
		}
		e.modbusAdapter.SetMappings(allMappings)
		e.log.Info("Modbus 输出适配器映射已更新", "mappings", len(allMappings))
	}

	// 更新 IEC 61850 输出适配器的映射规则
	if e.iecAdapter != nil && e.iec61850Mgr != nil {
		e.iecAdapter.SetIEC61850Manager(e.iec61850Mgr)
		e.iecAdapter.SetMappings(e.cfgMgr.GetIEC61850Mappings())
		e.log.Info("IEC 61850 输出适配器映射已更新")
	}

	e.log.Info("输出适配器配置重载完成")
}

// GetCommandBus 获取命令总线
func (e *Engine) GetCommandBus() *CommandBus {
	return e.commandBus
}

// GetWebControlSource 获取 Web 控制来源
func (e *Engine) GetWebControlSource() *WebControlSource {
	return e.webControl
}

// GetDataPool 获取数据共享池
func (e *Engine) GetDataPool() *DataPool {
	return e.dataPool
}

// onDeviceData 设备数据回调 — 将数据写入共享池并处理后续逻辑
func (e *Engine) onDeviceData(deviceID string, points []model.DataPoint) {
	// 写入统一数据共享池（会自动通知订阅的输出适配器，处理普通数据点）
	e.dataPool.BatchUpdateData(deviceID, points)

	// 处理批量数据点（带 Extra 字段的特殊数据点，如 batch 类型）
	if e.modbusAdapter != nil {
		e.modbusAdapter.BatchUpdatePoints(points)
	}

	// 更新路由器数据缓存（仅用于查询，不触发输出）
	if e.router != nil {
		e.router.UpdateDataCache(deviceID, points)
	}

	// 报警检测
	if e.alarmDetector != nil {
		e.alarmDetector.CheckDataPoints(deviceID, points)
	}

	// 存入离线缓冲 (仅当设备连接成功且上位机未连接时)
	if e.offlineBuffer != nil {
		// 只缓存质量良好的数据
		goodPoints := make([]model.DataPoint, 0, len(points))
		for _, pt := range points {
			if pt.Quality == model.QualityGood {
				goodPoints = append(goodPoints, pt)
			}
		}

		if len(goodPoints) > 0 {
			storedGroups := make(map[string]bool)
			for groupID, rules := range e.cfg.Mappings {
				if storedGroups[groupID] {
					continue
				}
				// 检查该分组是否有上位机在线
				if e.isMasterConnected(groupID) {
					continue // 上位机在线，不需要缓冲
				}
				for _, rule := range rules {
					if rule.SourceDevice == deviceID {
						e.offlineBuffer.StoreDataPoints(groupID, goodPoints)
						storedGroups[groupID] = true
						break
					}
				}
			}
		}
	}
}

// isMasterConnected 检查指定分组的上位机是否在线（支持自动匹配唯一服务器）
func (e *Engine) isMasterConnected(groupID string) bool {
	// 精确匹配
	if srv, ok := e.tcpServers[groupID]; ok {
		if srv.IsMasterConnected() {
			return true
		}
	}
	if srv, ok := e.rtuServers[groupID]; ok {
		if srv.IsMasterConnected() {
			return true
		}
	}
	// 唯一服务器时自动匹配
	if len(e.tcpServers) == 1 {
		for _, srv := range e.tcpServers {
			if srv.IsMasterConnected() {
				return true
			}
		}
	}
	if len(e.rtuServers) == 1 {
		for _, srv := range e.rtuServers {
			if srv.IsMasterConnected() {
				return true
			}
		}
	}
	return false
}

// handleMasterConnected 主机连接时触发数据补传
func (e *Engine) handleMasterConnected(groupID string) {
	if e.offlineBuffer == nil {
		return
	}

	count, err := e.offlineBuffer.CountUntransmitted(groupID)
	if err != nil {
		e.log.Error("查询待补传数据量失败", "group_id", groupID, "error", err)
		return
	}

	if count == 0 {
		return
	}

	e.log.Info("开始数据补传", "group_id", groupID, "pending_count", count)

	// 构建映射索引: deviceID+pointName -> MappingRule
	mappingIndex := e.buildMappingIndex(groupID)

	// 分批读取并补传
	const batchSize = 200
	var totalTransmitted int64

	for {
		records, err := e.offlineBuffer.LoadUntransmitted(groupID, batchSize)
		if err != nil {
			e.log.Error("读取待补传数据失败", "group_id", groupID, "error", err)
			break
		}
		if len(records) == 0 {
			break
		}

		ids := make([]int64, 0, len(records))
		for _, rec := range records {
			key := rec.DeviceID + "." + rec.PointName
			rule, ok := mappingIndex[key]
			if !ok {
				// 没有映射规则，跳过但仍标记为已传
				ids = append(ids, rec.ID)
				continue
			}

			// 反序列化值并写入输出寄存器
			val := buffer.DeserializeValue(rec.Value, rec.DataType)
			if val == nil {
				ids = append(ids, rec.ID)
				continue
			}

			e.writeValueToOutputs(groupID, rule, val)
			ids = append(ids, rec.ID)
		}

		// 批量标记已补传
		if len(ids) > 0 {
			if err := e.offlineBuffer.MarkTransmitted(ids); err != nil {
				e.log.Error("标记已补传失败", "group_id", groupID, "error", err)
				break
			}
			totalTransmitted += int64(len(ids))
		}

		// 不足一批说明已读完
		if len(records) < batchSize {
			break
		}
	}

	e.log.Info("数据补传完成", "group_id", groupID, "transmitted", totalTransmitted)
}

// buildMappingIndex 构建 deviceID.pointName -> MappingRule 的索引
func (e *Engine) buildMappingIndex(groupID string) map[string]config.MappingRule {
	index := make(map[string]config.MappingRule)
	rules, ok := e.cfg.Mappings[groupID]
	if !ok {
		return index
	}
	for _, rule := range rules {
		key := rule.SourceDevice + "." + rule.Name
		index[key] = rule
	}
	return index
}

// writeValueToOutputs 将单个值写入指定分组的输出寄存器（支持自动匹配唯一服务器）
func (e *Engine) writeValueToOutputs(groupID string, rule config.MappingRule, val interface{}) {
	// 转换为寄存器值
	regValues := e.valueToRegisters(rule, val)
	if len(regValues) == 0 {
		return
	}

	// 写入TCP服务器（精确匹配或唯一服务器自动匹配）
	if srv, ok := e.tcpServers[groupID]; ok {
		srv.UpdateRegisters(rule.TargetRegister, regValues)
	} else if len(e.tcpServers) == 1 {
		for _, srv := range e.tcpServers {
			srv.UpdateRegisters(rule.TargetRegister, regValues)
		}
	}
	// 写入RTU服务器（精确匹配或唯一服务器自动匹配）
	if srv, ok := e.rtuServers[groupID]; ok {
		srv.UpdateRegisters(rule.TargetRegister, regValues)
	} else if len(e.rtuServers) == 1 {
		for _, srv := range e.rtuServers {
			srv.UpdateRegisters(rule.TargetRegister, regValues)
		}
	}
}

// valueToRegisters 将值按映射规则编码为寄存器值
func (e *Engine) valueToRegisters(rule config.MappingRule, val interface{}) []uint16 {
	// 获取原始数值
	rawValue := toFloat64(val)
	if rawValue == nil {
		return nil
	}

	// 应用缩放和偏移
	converted := *rawValue*rule.Scale + rule.Offset

	switch rule.DataType {
	case "float32":
		bits := math.Float32bits(float32(converted))
		return []uint16{uint16(bits >> 16), uint16(bits & 0xFFFF)}
	case "int32", "uint32":
		v := uint32(int32(converted))
		return []uint16{uint16(v >> 16), uint16(v & 0xFFFF)}
	case "int16":
		return []uint16{uint16(int16(converted))}
	case "uint16":
		return []uint16{uint16(converted)}
	case "bool":
		if converted != 0 {
			return []uint16{0xFF00}
		}
		return []uint16{0x0000}
	default:
		return []uint16{uint16(converted)}
	}
}

// toFloat64 将各种类型的值转为 float64
func toFloat64(val interface{}) *float64 {
	switch v := val.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int16:
		f := float64(v)
		return &f
	case uint16:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	case uint32:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case uint64:
		f := float64(v)
		return &f
	case bool:
		if v {
			f := 1.0
			return &f
		}
		f := 0.0
		return &f
	default:
		return nil
	}
}

// startIEC61850 启动 IEC 61850 MMS 服务
func (e *Engine) startIEC61850() error {
	iecCfg := e.cfgMgr.GetIEC61850Config()
	if iecCfg == nil {
		return fmt.Errorf("IEC 61850 配置为空")
	}

	e.log.Info("启动 IEC 61850 服务...",
		"port", iecCfg.IEC61850.Port,
		"ied_name", iecCfg.IEC61850.IEDName,
	)

	mgr := iec61850.NewManager(iecCfg)

	// 构建数据模型
	if err := mgr.BuildModel(); err != nil {
		return fmt.Errorf("构建 IEC 61850 数据模型失败: %w", err)
	}

	// 启动服务器
	if err := mgr.Start(); err != nil {
		return fmt.Errorf("启动 IEC 61850 服务器失败: %w", err)
	}

	e.iec61850Mgr = mgr
	e.log.Info("IEC 61850 服务启动成功",
		"port", iecCfg.IEC61850.Port,
		"ied_name", iecCfg.IEC61850.IEDName,
		"mappings", len(iecCfg.Mappings),
	)
	return nil
}

func (e *Engine) startWebServer() error {
	e.log.Info("启动Web服务...",
		"host", e.cfg.Web.Host,
		"port", e.cfg.Web.Port,
	)

	// 构建重启回调：清理资源后退出，看门狗会自动重启
	restartFunc := func() {
		e.log.Info("收到重启请求，正在清理资源...")
		e.Stop()
		os.Exit(0)
	}

	e.webServer = web.NewServer(e.cfgMgr, e.log, e.offlineBuffer, e.alarmStore, e.collector, e.router, restartFunc)
	if err := e.webServer.Start(); err != nil {
		return fmt.Errorf("启动Web服务失败: %w", err)
	}

	e.log.Info("Web服务启动成功",
		"addr", fmt.Sprintf("%s:%d", e.cfg.Web.Host, e.cfg.Web.Port),
	)
	return nil
}

// startHeartbeat 启动心跳更新 goroutine，每 10 秒写入当前时间到 SQLite
func (e *Engine) startHeartbeat() {
	if e.offlineBuffer == nil {
		return
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		// 启动时立即写入一次
		e.offlineBuffer.UpdateHeartbeat()
		e.log.Info("心跳服务启动")

		for {
			select {
			case <-e.ctx.Done():
				return
			case <-ticker.C:
				e.offlineBuffer.UpdateHeartbeat()
			}
		}
	}()
}
