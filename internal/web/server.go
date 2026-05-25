package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/internal/storage/buffer"
	"github.com/lyx6662/com-manager/internal/web/handler"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// Server Web服务器
type Server struct {
	cfgMgr    *config.Manager
	cfg       *config.Config
	log       *logger.Logger
	engine    *gin.Engine
	server    *http.Server
	buf       *buffer.OfflineBuffer

	// 处理器
	deviceHandler  *handler.DeviceHandler
	groupHandler   *handler.GroupHandler
	mappingHandler *handler.MappingHandler
	outputHandler  *handler.OutputHandler
	monitorHandler *handler.MonitorHandler
	alarmHandler   *handler.AlarmHandler
	bufferHandler  *handler.BufferHandler
	systemHandler  *handler.SystemHandler
}

// NewServer 创建Web服务器
func NewServer(cfgMgr *config.Manager, log *logger.Logger, buf *buffer.OfflineBuffer) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		cfgMgr: cfgMgr,
		cfg:    cfgMgr.Get(),
		log:    log,
		buf:    buf,
	}

	s.initHandlers()
	s.initRoutes()

	return s
}

// initHandlers 初始化处理器
func (s *Server) initHandlers() {
	s.deviceHandler = handler.NewDeviceHandler(s.cfgMgr, s.log)
	s.groupHandler = handler.NewGroupHandler(s.cfgMgr, s.log)
	s.mappingHandler = handler.NewMappingHandler(s.cfgMgr, s.log)
	s.outputHandler = handler.NewOutputHandler(s.cfgMgr, s.log)
	s.monitorHandler = handler.NewMonitorHandler(s.log)
	s.alarmHandler = handler.NewAlarmHandler(s.log)
	s.bufferHandler = handler.NewBufferHandler(s.buf, s.log)
	s.systemHandler = handler.NewSystemHandler(s.cfgMgr, s.log)
}

// initRoutes 初始化路由
func (s *Server) initRoutes() {
	s.engine = gin.New()

	// 中间件
	s.engine.Use(gin.Logger())
	s.engine.Use(gin.Recovery())
	s.engine.Use(s.corsMiddleware())

	// 静态文件
	s.engine.Static("/css", "./web/frontend/css")
	s.engine.Static("/js", "./web/frontend/js")
	s.engine.Static("/pages", "./web/frontend/pages")
	s.engine.StaticFile("/", "./web/frontend/index.html")

	// API路由
	api := s.engine.Group("/api/v1")
	{
		// 认证
		auth := api.Group("/auth")
		{
			auth.POST("/login", s.login)
			auth.POST("/logout", s.logout)
		}

		// 设备管理
		devices := api.Group("/devices")
		{
			devices.GET("", s.deviceHandler.List)
			devices.GET("/:id", s.deviceHandler.Get)
			devices.POST("", s.deviceHandler.Create)
			devices.PUT("/:id", s.deviceHandler.Update)
			devices.DELETE("/:id", s.deviceHandler.Delete)
			devices.GET("/:id/status", s.deviceHandler.GetStatus)
		}

		// 分组管理
		groups := api.Group("/groups")
		{
			groups.GET("", s.groupHandler.List)
			groups.GET("/:id", s.groupHandler.Get)
			groups.POST("", s.groupHandler.Create)
			groups.PUT("/:id", s.groupHandler.Update)
			groups.DELETE("/:id", s.groupHandler.Delete)
		}

		// 点表管理
		mappings := api.Group("/mappings")
		{
			mappings.GET("/:deviceId", s.mappingHandler.Get)
			mappings.PUT("/:deviceId", s.mappingHandler.Update)
			mappings.GET("/:deviceId/export", s.mappingHandler.Export)
			mappings.POST("/:deviceId/import", s.mappingHandler.Import)
		}

		// 输出配置
		outputs := api.Group("/outputs")
		{
			outputs.GET("", s.outputHandler.List)
			outputs.GET("/:id", s.outputHandler.Get)
			outputs.POST("", s.outputHandler.Create)
			outputs.PUT("/:id", s.outputHandler.Update)
			outputs.DELETE("/:id", s.outputHandler.Delete)
		}

		// 实时监控
		monitor := api.Group("/monitor")
		{
			monitor.GET("/realtime", s.monitorHandler.GetRealtime)
			monitor.GET("/devices", s.monitorHandler.GetDeviceStatus)
			monitor.GET("/devices/:id", s.monitorHandler.GetDeviceData)
		}

		// 断点续传
		buffer := api.Group("/buffer")
		{
			buffer.GET("/status", s.bufferHandler.GetStatus)
			buffer.GET("/status/:groupId", s.bufferHandler.GetGroupStatus)
			buffer.POST("/retry/:groupId", s.bufferHandler.Retry)
		}

		// 报警管理
		alarm := api.Group("/alarm")
		{
			alarm.GET("", s.alarmHandler.List)
			alarm.PUT("/:id/ack", s.alarmHandler.Ack)
			alarm.GET("/stats", s.alarmHandler.GetStats)
		}

		// 系统管理
		system := api.Group("/system")
		{
			system.GET("/info", s.systemHandler.GetInfo)
			system.GET("/config", s.systemHandler.GetConfig)
			system.PUT("/config", s.systemHandler.UpdateConfig)
			system.POST("/config/reload", s.systemHandler.ReloadConfig)
			system.GET("/logs", s.systemHandler.GetLogs)
			system.POST("/backup", s.systemHandler.Backup)
			system.POST("/restore", s.systemHandler.Restore)
		}
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Web.Host, s.cfg.Web.Port)

	s.server = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}

	s.log.Info("Web服务器启动", "addr", addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("Web服务器错误", "error", err)
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// corsMiddleware CORS中间件
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.Web.CORS.Enabled {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// login 登录
func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	if !s.cfg.Web.Auth.Enabled {
		c.JSON(200, gin.H{
			"code": 0,
			"data": gin.H{
				"token": "disabled",
			},
		})
		return
	}

	if req.Username == s.cfg.Web.Auth.Username && req.Password == s.cfg.Web.Auth.Password {
		// TODO: 生成JWT token
		c.JSON(200, gin.H{
			"code": 0,
			"data": gin.H{
				"token": "mock-token",
			},
		})
	} else {
		c.JSON(401, gin.H{
			"code":    401,
			"message": "用户名或密码错误",
		})
	}
}

// logout 登出
func (s *Server) logout(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "登出成功",
	})
}
