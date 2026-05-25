package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinish/lumberjack.v2"
)

// Logger 日志封装
type Logger struct {
	*zap.SugaredLogger
}

// New 创建日志实例
func New(level, logPath string) (*Logger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 解析日志级别
	logLevel, err := parseLevel(level)
	if err != nil {
		logLevel = zapcore.InfoLevel
	}

	// 配置编码器
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 控制台输出
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 文件输出 (带日志轮转)
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
	fileWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logPath, "com-manager.log"),
		MaxSize:    100, // MB
		MaxBackups: 5,
		MaxAge:     30, // 天
		Compress:   true,
	}

	// 创建核心
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), logLevel),
		zapcore.NewCore(fileEncoder, zapcore.AddSync(fileWriter), logLevel),
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(0))
	return &Logger{logger.Sugar()}, nil
}

func parseLevel(level string) (zapcore.Level, error) {
	switch level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("未知日志级别: %s", level)
	}
}
