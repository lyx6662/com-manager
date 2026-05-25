package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/lyx6662/com-manager/internal/protocol/modbus/tcp"
	"github.com/lyx6662/com-manager/internal/storage/buffer"
	"github.com/lyx6662/com-manager/internal/web"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
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
	tcpServers    map[string]*tcp.Server  // Modbus TCP 输出服务器
}

// NewEngine 创建引擎
func NewEngine(cfgMgr *config.Manager, log *logger.Logger) (*Engine, error) {
	engine := &Engine{
		cfgMgr:    cfgMgr,
		cfg:       cfgMgr.Get(),
		log:       log,
		tcpServers: make(map[string]*tcp.Server),
	}

	return engine, nil
}

// Start 启动引擎
func (e *Engine) Start(ctx context.Context) error {
	e.ctx, e.cancel = context.WithCancel(ctx)

	e.log.Info("引擎启动中...")

	// 1. 初始化存储
	if err := e.initStorage(); err != nil {
		return fmt.Errorf("初始化存储失败: %w", err)
	}

	// 2. 启动输出服务 (Modbus TCP Server)
	if err := e.startOutputs(); err != nil {
		return fmt.Errorf("启动输出服务失败: %w", err)
	}

	// 3. 启动Web服务
	if e.cfg.Web.Enabled {
		if err := e.startWebServer(); err != nil {
			return fmt.Errorf("启动Web服务失败: %w", err)
		}
	}

	e.log.Info("引擎启动完成",
		"serial_devices", len(e.cfg.SerialDevices),
		"network_devices", len(e.cfg.NetworkDevices),
		"tcp_outputs", len(e.cfg.Outputs.ModbusTCPServers),
	)
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.log.Info("引擎停止中...")
	e.cancel()

	// 停止所有TCP服务器
	for id, srv := range e.tcpServers {
		if err := srv.Close(); err != nil {
			e.log.Error("停止TCP服务器失败", "id", id, "error", err)
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

	// 更新配置
	e.cfg = newCfg

	// TODO: 比较新旧配置，增量更新服务器

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

func (e *Engine) initStorage() error {
	e.log.Info("初始化存储...")

	if e.cfg.OfflineBuffer.Enabled {
		buf, err := buffer.NewOfflineBuffer(
			e.cfg.OfflineBuffer.DBPath,
			e.cfg.OfflineBuffer.RetentionDays,
			e.log,
		)
		if err != nil {
			return fmt.Errorf("初始化离线缓冲失败: %w", err)
		}
		e.offlineBuffer = buf
		e.log.Info("离线缓冲初始化成功",
			"path", e.cfg.OfflineBuffer.DBPath,
			"retention_days", e.cfg.OfflineBuffer.RetentionDays,
		)
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
			e.log.Info("主机已连接", "server", cfg.ID)
			// TODO: 触发数据补传
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

func (e *Engine) startWebServer() error {
	e.log.Info("启动Web服务...",
		"host", e.cfg.Web.Host,
		"port", e.cfg.Web.Port,
	)

	e.webServer = web.NewServer(e.cfgMgr, e.log, e.offlineBuffer)
	if err := e.webServer.Start(); err != nil {
		return fmt.Errorf("启动Web服务失败: %w", err)
	}

	e.log.Info("Web服务启动成功",
		"addr", fmt.Sprintf("%s:%d", e.cfg.Web.Host, e.cfg.Web.Port),
	)
	return nil
}
