package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// Init 初始化全局日志器
func Init(level string) {
	log = logrus.New()

	// 设置输出格式
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	// 设置输出目标
	log.SetOutput(os.Stdout)

	// 设置日志级别
	logLevel, err := logrus.ParseLevel(strings.ToLower(level))
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	log.SetLevel(logLevel)
}

// Debug 输出调试级别日志
func Debug(args ...interface{}) {
	if log != nil {
		log.Debug(args...)
	}
}

// Debugf 输出格式化调试级别日志
func Debugf(format string, args ...interface{}) {
	if log != nil {
		log.Debugf(format, args...)
	}
}

// Info 输出信息级别日志
func Info(args ...interface{}) {
	if log != nil {
		log.Info(args...)
	}
}

// Infof 输出格式化信息级别日志
func Infof(format string, args ...interface{}) {
	if log != nil {
		log.Infof(format, args...)
	}
}

// Warn 输出警告级别日志
func Warn(args ...interface{}) {
	if log != nil {
		log.Warn(args...)
	}
}

// Warnf 输出格式化警告级别日志
func Warnf(format string, args ...interface{}) {
	if log != nil {
		log.Warnf(format, args...)
	}
}

// Error 输出错误级别日志
func Error(args ...interface{}) {
	if log != nil {
		log.Error(args...)
	}
}

// Errorf 输出格式化错误级别日志
func Errorf(format string, args ...interface{}) {
	if log != nil {
		log.Errorf(format, args...)
	}
}

// Fatal 输出致命错误日志并退出程序
func Fatal(args ...interface{}) {
	if log != nil {
		log.Fatal(args...)
	}
}

// Fatalf 输出格式化致命错误日志并退出程序
func Fatalf(format string, args ...interface{}) {
	if log != nil {
		log.Fatalf(format, args...)
	}
}

// WithField 创建带有字段的日志条目
func WithField(key string, value interface{}) *logrus.Entry {
	if log != nil {
		return log.WithField(key, value)
	}
	return nil
}

// WithFields 创建带有多个字段的日志条目
func WithFields(fields logrus.Fields) *logrus.Entry {
	if log != nil {
		return log.WithFields(fields)
	}
	return nil
}
