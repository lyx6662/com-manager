package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/lyx6662/com-manager/internal/storage/buffer"
	"github.com/lyx6662/com-manager/internal/web"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// Engine 核心引擎
type Engine struct {
	cfg     *config.Config
	log     *logger.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// 各管理器
	webServer     *web.Server
	offlineBuffer *buffer.OfflineBuffer
}

// NewEngine 创建引擎
func NewEngine(cfg *config.Config, log *logger.Logger) (*Engine, error) {
	engine := &Engine{
		cfg: cfg,
		log: log,
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

	// 2. 初始化设备管理器
	if err := e.initDeviceManager(); err != nil {
		return fmt.Errorf("初始化设备管理器失败: %w", err)
	}

	// 3. 初始化分组管理器
	if err := e.initGroupManager(); err != nil {
		return fmt.Errorf("初始化分组管理器失败: %w", err)
	}

	// 4. 初始化映射管理器
	if err := e.initMappingManager(); err != nil {
		return fmt.Errorf("初始化映射管理器失败: %w", err)
	}

	// 5. 启动数据采集
	if err := e.startCollectors(); err != nil {
		return fmt.Errorf("启动数据采集失败: %w", err)
	}

	// 6. 启动输出服务
	if err := e.startOutputs(); err != nil {
		return fmt.Errorf("启动输出服务失败: %w", err)
	}

	// 7. 启动Web服务
	if e.cfg.Web.Enabled {
		if err := e.startWebServer(); err != nil {
			return fmt.Errorf("启动Web服务失败: %w", err)
		}
	}

	e.log.Info("引擎启动完成")
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.log.Info("引擎停止中...")
	e.cancel()

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
	// TODO: 实现配置热重载
	return nil
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

func (e *Engine) initDeviceManager() error {
	e.log.Info("初始化设备管理器...",
		"serial_devices", len(e.cfg.SerialDevices),
		"network_devices", len(e.cfg.NetworkDevices),
	)
	// TODO: 创建设备管理器, 注册所有设备
	return nil
}

func (e *Engine) initGroupManager() error {
	e.log.Info("初始化分组管理器...")
	// TODO: 创建分组管理器, 加载分组配置
	return nil
}

func (e *Engine) initMappingManager() error {
	e.log.Info("初始化映射管理器...")
	// TODO: 创建映射管理器, 加载点表
	return nil
}

func (e *Engine) startCollectors() error {
	e.log.Info("启动数据采集...")
	// TODO: 启动所有设备的采集协程
	return nil
}

func (e *Engine) startOutputs() error {
	e.log.Info("启动输出服务...")
	// TODO: 启动Modbus TCP Server, Modbus RTU Slave等
	return nil
}

func (e *Engine) startWebServer() error {
	e.log.Info("启动Web服务...",
		"host", e.cfg.Web.Host,
		"port", e.cfg.Web.Port,
	)

	e.webServer = web.NewServer(e.cfg, e.log, e.offlineBuffer)
	if err := e.webServer.Start(); err != nil {
		return fmt.Errorf("启动Web服务失败: %w", err)
	}

	e.log.Info("Web服务启动成功",
		"addr", fmt.Sprintf("%s:%d", e.cfg.Web.Host, e.cfg.Web.Port),
	)
	return nil
}
