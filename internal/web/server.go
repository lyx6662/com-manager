package web

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lyx6662/com-manager/internal/storage/alarm"
	"github.com/lyx6662/com-manager/internal/storage/buffer"
	"github.com/lyx6662/com-manager/internal/web/auth"
	"github.com/lyx6662/com-manager/internal/web/handler"
	"github.com/lyx6662/com-manager/pkg/config"
	"github.com/lyx6662/com-manager/pkg/logger"
)

// FrontendFS 内嵌前端资源 (由 main 包在启动前设置)
var FrontendFS fs.FS

// DeviceStatusProvider 设备状态提供者接口
type DeviceStatusProvider interface {
	GetDeviceStatus(deviceID string) interface{}
	GetAllDeviceStatus() map[string]interface{}
}

// DataCacheProvider 数据缓存提供者接口
type DataCacheProvider interface {
	GetDeviceStatus() map[string]map[string]interface{}
}

// Server Web服务器
type Server struct {
	cfgMgr    *config.Manager
	cfg       *config.Config
	log       *logger.Logger
	engine    *gin.Engine
	server    *http.Server
	buf       *buffer.OfflineBuffer
	alarmStore *alarm.Store
	tokenMgr  *auth.TokenManager

	// 处理器
	deviceHandler   *handler.DeviceHandler
	groupHandler    *handler.GroupHandler
	mappingHandler  *handler.MappingHandler
	outputHandler   *handler.OutputHandler
	monitorHandler  *handler.MonitorHandler
	alarmHandler    *handler.AlarmHandler
	bufferHandler   *handler.BufferHandler
	systemHandler   *handler.SystemHandler
	iec61850Handler *handler.IEC61850Handler
}

// NewServer 创建Web服务器
func NewServer(cfgMgr *config.Manager, log *logger.Logger, buf *buffer.OfflineBuffer, alarmStore *alarm.Store, collector interface{}, router interface{}, restartFunc func()) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		cfgMgr:     cfgMgr,
		cfg:        cfgMgr.Get(),
		log:        log,
		buf:        buf,
		alarmStore: alarmStore,
		tokenMgr:   auth.NewTokenManager(cfgMgr.Get().Web.Auth.TokenSecret, 24),
	}

	s.initHandlers(collector, router, restartFunc)
	s.initRoutes()

	return s
}

// initHandlers 初始化处理器
func (s *Server) initHandlers(collector interface{}, router interface{}, restartFunc func()) {
	s.deviceHandler = handler.NewDeviceHandler(s.cfgMgr, s.log, collector)
	s.groupHandler = handler.NewGroupHandler(s.cfgMgr, s.log)
	s.mappingHandler = handler.NewMappingHandler(s.cfgMgr, s.log)
	s.outputHandler = handler.NewOutputHandler(s.cfgMgr, s.log)
	s.monitorHandler = handler.NewMonitorHandler(s.log, collector, router)
	s.alarmHandler = handler.NewAlarmHandler(s.log, s.alarmStore)
	s.bufferHandler = handler.NewBufferHandler(s.buf, s.log)
	s.systemHandler = handler.NewSystemHandler(s.cfgMgr, s.log, restartFunc)
	s.iec61850Handler = handler.NewIEC61850Handler(s.cfgMgr, s.log, restartFunc)
}

// initRoutes 初始化路由
func (s *Server) initRoutes() {
	s.engine = gin.New()

	// 中间件
	s.engine.Use(gin.Logger())
	s.engine.Use(gin.Recovery())
	s.engine.Use(s.corsMiddleware())

	// 静态文件 (从内嵌文件系统加载)
	rootFS, _ := fs.Sub(FrontendFS, "web/frontend")
	cssFS, _ := fs.Sub(rootFS, "css")
	jsFS, _ := fs.Sub(rootFS, "js")
	pagesFS, _ := fs.Sub(rootFS, "pages")
	s.engine.StaticFS("/css", http.FS(cssFS))
	s.engine.StaticFS("/js", http.FS(jsFS))
	s.engine.StaticFS("/pages", http.FS(pagesFS))
	s.engine.GET("/", func(c *gin.Context) {
		c.FileFromFS("/", http.FS(rootFS))
	})
	// SPA 路由兜底：非API路径统一返回 index.html
	s.engine.NoRoute(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.FileFromFS("/", http.FS(rootFS))
		}
	})

	// API路由
	api := s.engine.Group("/api/v1")
	{
		// 认证 (不需要token)
		auth := api.Group("/auth")
		{
			auth.POST("/login", s.login)
			auth.POST("/logout", s.logout)
		}

		// 以下路由需要认证 (如果启用)
		protected := api.Group("")
		protected.Use(s.authMiddleware())
		{
			// 设备管理
			devices := protected.Group("/devices")
			{
				devices.GET("", s.deviceHandler.List)
				devices.GET("/:id", s.deviceHandler.Get)
				devices.POST("", s.deviceHandler.Create)
				devices.PUT("/:id", s.deviceHandler.Update)
				devices.DELETE("/:id", s.deviceHandler.Delete)
				devices.GET("/:id/status", s.deviceHandler.GetStatus)
			}

			// 分组管理
			groups := protected.Group("/groups")
			{
				groups.GET("", s.groupHandler.List)
				groups.GET("/:id", s.groupHandler.Get)
				groups.POST("", s.groupHandler.Create)
				groups.PUT("/:id", s.groupHandler.Update)
				groups.DELETE("/:id", s.groupHandler.Delete)
			}

			// 点表管理
			mappings := protected.Group("/mappings")
			{
				mappings.GET("/:deviceId", s.mappingHandler.Get)
				mappings.PUT("/:deviceId", s.mappingHandler.Update)
				mappings.GET("/:deviceId/export", s.mappingHandler.Export)
				mappings.POST("/:deviceId/import", s.mappingHandler.Import)
			}

			// 输出配置
			outputs := protected.Group("/outputs")
			{
				outputs.GET("", s.outputHandler.List)
				outputs.GET("/:id", s.outputHandler.Get)
				outputs.POST("", s.outputHandler.Create)
				outputs.PUT("", s.outputHandler.BatchUpdate) // 批量更新
				outputs.PUT("/:id", s.outputHandler.Update)
				outputs.DELETE("/:id", s.outputHandler.Delete)
				outputs.POST("/toggle", s.outputHandler.ToggleEnabled)
			}

			// 实时监控
			monitor := protected.Group("/monitor")
			{
				monitor.GET("/realtime", s.monitorHandler.GetRealtime)
				monitor.GET("/devices", s.monitorHandler.GetDeviceStatus)
				monitor.GET("/devices/:id", s.monitorHandler.GetDeviceData)
			}

			// 断点续传
			buffer := protected.Group("/buffer")
			{
				buffer.GET("/heartbeat", s.bufferHandler.GetHeartbeat)
				buffer.GET("/status", s.bufferHandler.GetStatus)
				buffer.GET("/status/:groupId", s.bufferHandler.GetGroupStatus)
				buffer.GET("/records/:groupId", s.bufferHandler.GetRecords)
				buffer.POST("/retry/:groupId", s.bufferHandler.Retry)
				buffer.POST("/mark-transmitted/:groupId", s.bufferHandler.MarkAllTransmitted)
				buffer.POST("/purge-transmitted", s.bufferHandler.PurgeTransmitted)
			}

			// 报警管理
			alarm := protected.Group("/alarm")
			{
				alarm.GET("", s.alarmHandler.List)
				alarm.PUT("/:id/ack", s.alarmHandler.Ack)
				alarm.GET("/stats", s.alarmHandler.GetStats)
			}

			// 系统管理
			system := protected.Group("/system")
			{
				system.GET("/info", s.systemHandler.GetInfo)
				system.GET("/config", s.systemHandler.GetConfig)
				system.PUT("/config", s.systemHandler.UpdateConfig)
				system.POST("/config/reload", s.systemHandler.ReloadConfig)
				system.GET("/logs", s.systemHandler.GetLogs)
				system.GET("/serial-ports", s.systemHandler.GetSerialPorts)
				system.POST("/backup", s.systemHandler.Backup)
				system.POST("/restore", s.systemHandler.Restore)
				system.POST("/restart", s.systemHandler.Restart)
			}

			// IEC 61850 管理
			iec61850 := protected.Group("/iec61850")
			{
				iec61850.GET("/config", s.iec61850Handler.GetConfig)
				iec61850.PUT("/config", s.iec61850Handler.UpdateServerConfig)
				iec61850.POST("/restart", s.iec61850Handler.Restart)
				iec61850.GET("/status", s.iec61850Handler.GetStatus)
				iec61850.GET("/paths", s.iec61850Handler.GetPaths)

				// 数据模型
				iec61850.GET("/model", s.iec61850Handler.GetModel)
				iec61850.PUT("/model", s.iec61850Handler.UpdateModel)
				iec61850.POST("/model/devices", s.iec61850Handler.AddLogicalDevice)
				iec61850.PUT("/model/devices/:name", s.iec61850Handler.UpdateLogicalDevice)
				iec61850.DELETE("/model/devices/:name", s.iec61850Handler.DeleteLogicalDevice)
				iec61850.POST("/model/devices/:name/nodes", s.iec61850Handler.AddLogicalNode)
				iec61850.PUT("/model/devices/:name/nodes/:node", s.iec61850Handler.UpdateLogicalNode)
				iec61850.DELETE("/model/devices/:name/nodes/:node", s.iec61850Handler.DeleteLogicalNode)
				iec61850.POST("/model/devices/:name/nodes/:node/copy", s.iec61850Handler.CopyLogicalNode)
				iec61850.POST("/model/devices/:name/nodes/:node/objects", s.iec61850Handler.AddDataObject)
				iec61850.PUT("/model/devices/:name/nodes/:node/objects/:object", s.iec61850Handler.UpdateDataObject)
				iec61850.DELETE("/model/devices/:name/nodes/:node/objects/:object", s.iec61850Handler.DeleteDataObject)
				iec61850.POST("/model/devices/:name/nodes/:node/objects/:object/attrs", s.iec61850Handler.AddDataAttribute)
				iec61850.PUT("/model/devices/:name/nodes/:node/objects/:object/attrs/:attr", s.iec61850Handler.UpdateDataAttribute)
				iec61850.DELETE("/model/devices/:name/nodes/:node/objects/:object/attrs/:attr", s.iec61850Handler.DeleteDataAttribute)

				// 映射规则
				iec61850.GET("/mappings", s.iec61850Handler.GetMappings)
				iec61850.POST("/mappings", s.iec61850Handler.AddMapping)
				iec61850.PUT("/mappings/:index", s.iec61850Handler.UpdateMapping)
				iec61850.DELETE("/mappings/:index", s.iec61850Handler.DeleteMapping)
			}
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
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	if !s.cfg.Web.Auth.Enabled {
		token, _ := s.tokenMgr.Generate(req.Username)
		c.JSON(200, gin.H{
			"code": 0,
			"data": gin.H{"token": token},
		})
		return
	}

	if req.Username == s.cfg.Web.Auth.Username && req.Password == s.cfg.Web.Auth.Password {
		token, err := s.tokenMgr.Generate(req.Username)
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "message": "生成token失败"})
			return
		}
		c.JSON(200, gin.H{
			"code": 0,
			"data": gin.H{"token": token},
		})
	} else {
		c.JSON(401, gin.H{"code": 401, "message": "用户名或密码错误"})
	}
}

// authMiddleware 认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cfg.Web.Auth.Enabled {
			c.Next()
			return
		}

		// 从Header获取token
		token := c.GetHeader("Authorization")
		if token == "" {
			token = c.Query("token")
		}

		// 去掉 "Bearer " 前缀
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "未提供认证token"})
			return
		}

		claims, err := s.tokenMgr.Validate(token)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "message": "token无效: " + err.Error()})
			return
		}

		// 将用户名存入上下文
		c.Set("username", claims.Username)
		c.Next()
	}
}

// logout 登出
func (s *Server) logout(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "登出成功",
	})
}
