package mylog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
)

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var currentLevel = LogLevelInfo // 默认日志级别为 INFO

type MyLogAdapter struct {
	Level         LogLevel
	EnableFileLog bool
	FileLogPath   string
}

func GetLogLevelFromConfig(logLevel int) LogLevel {
	switch logLevel {
	case 0:
		return LogLevelDebug
	case 1:
		return LogLevelInfo
	case 2:
		return LogLevelWarn
	case 3:
		return LogLevelError
	default:
		return LogLevelInfo // 默认为 Info
	}
}

var logPath string

func init() {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	exeDir := filepath.Dir(exePath)
	logPath = filepath.Join(exeDir, "log")
}

// 全局变量，用于存储日志启用状态
var enableFileLogGlobal bool

// SetEnableFileLog 设置 enableFileLogGlobal 的值
func SetEnableFileLog(value bool) {
	enableFileLogGlobal = value
}

// 接收新参数，并设置文件日志路径
func NewMyLogAdapter(level LogLevel, enableFileLog bool) *MyLogAdapter {
	if enableFileLog {
		SetEnableFileLog(true)
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			err := os.Mkdir(logPath, 0755)
			if err != nil {
				panic(err)
			}
		}
	}

	return &MyLogAdapter{
		Level:         level,
		EnableFileLog: enableFileLog,
		FileLogPath:   logPath,
	}
}

// 获取当前日志文件名
func getCurrentLogFilename() string {
	suffixMins := config.GetLogSuffixPerMins()
	baseFilename := time.Now().Format("2006-01-02")
	if suffixMins == 0 {
		return baseFilename + ".log"
	}

	currentTime := time.Now()
	currentMinutes := currentTime.Hour()*60 + currentTime.Minute()   // 当前时间的总分钟数
	windowStartMinutes := (currentMinutes / suffixMins) * suffixMins // 计算当前时间窗口的起始分钟数
	windowStartHour := windowStartMinutes / 60
	windowStartMinute := windowStartMinutes % 60

	suffix := fmt.Sprintf("%02d-%02d", windowStartHour, windowStartMinute) // 格式化时间窗口后缀
	return fmt.Sprintf("%s-%s.log", baseFilename, suffix)
}

// 文件日志记录函数
func (adapter *MyLogAdapter) logToFile(level, message string) {
	if !adapter.EnableFileLog {
		return
	}
	filename := getCurrentLogFilename()
	filepath := adapter.FileLogPath + "/" + filename

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer file.Close()

	logEntry := fmt.Sprintf("[%s] %s: %s\n", time.Now().Format("2006-01-02T15:04:05"), level, message)
	if _, err := file.WriteString(logEntry); err != nil {
		fmt.Println("Error writing to log file:", err)
	}
}

// 独立的文件日志记录函数
func LogToFile(level, message string) {
	if !enableFileLogGlobal {
		return
	}
	filename := getCurrentLogFilename()
	filepath := logPath + "/" + filename

	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer file.Close()

	logEntry := fmt.Sprintf("[%s] %s: %s\n", time.Now().Format("2006-01-02T15:04:05"), level, message)
	if _, err := file.WriteString(logEntry); err != nil {
		fmt.Println("Error writing to log file:", err)
	}
}

// Debug logs a message at the debug level.
func (adapter *MyLogAdapter) Debug(v ...interface{}) {
	if adapter.Level <= LogLevelDebug {
		message := fmt.Sprint(v...)
		Println(v...)
		adapter.logToFile("DEBUG", message)
	}
}

// Info logs a message at the info level.
func (adapter *MyLogAdapter) Info(v ...interface{}) {
	if adapter.Level <= LogLevelInfo {
		message := fmt.Sprint(v...)
		Println(v...)
		adapter.logToFile("INFO", message)
	}
}

// Warn logs a message at the warn level.
func (adapter *MyLogAdapter) Warn(v ...interface{}) {
	if adapter.Level <= LogLevelWarn {
		message := fmt.Sprint(v...)
		Printf("WARN: %v\n", v...)
		adapter.logToFile("WARN", message)
	}
}

// Error logs a message at the error level.
func (adapter *MyLogAdapter) Error(v ...interface{}) {
	if adapter.Level <= LogLevelError {
		message := fmt.Sprint(v...)
		Printf("ERROR: %v\n", v...)
		adapter.logToFile("ERROR", message)
	}
}

// Debugf logs a formatted message at the debug level.
func (adapter *MyLogAdapter) Debugf(format string, v ...interface{}) {
	if adapter.Level <= LogLevelDebug {
		message := fmt.Sprintf(format, v...)
		Printf("DEBUG: "+format, v...)
		adapter.logToFile("DEBUG", message)
	}
}

// Infof logs a formatted message at the info level.
func (adapter *MyLogAdapter) Infof(format string, v ...interface{}) {
	if adapter.Level <= LogLevelInfo {
		message := fmt.Sprintf(format, v...)
		Printf("INFO: "+format, v...)
		adapter.logToFile("INFO", message)
	}
}

// Warnf logs a formatted message at the warn level.
func (adapter *MyLogAdapter) Warnf(format string, v ...interface{}) {
	if adapter.Level <= LogLevelWarn {
		message := fmt.Sprintf(format, v...)
		Printf("WARN: "+format, v...)
		adapter.logToFile("WARN", message)
	}
}

// Errorf logs a formatted message at the error level.
func (adapter *MyLogAdapter) Errorf(format string, v ...interface{}) {
	if adapter.Level <= LogLevelError {
		message := fmt.Sprintf(format, v...)
		Printf("ERROR: "+format, v...)
		adapter.logToFile("ERROR", message)
	}
}

// Sync 实现 Botgo SDK 的 Sync 方法
func (adapter *MyLogAdapter) Sync() error {
	// 如果日志系统有需要执行的同步操作，在这里添加相应的代码
	// 例如，可能需要刷新或关闭文件，或执行其他清理操作
	// 如果没有特别的同步需求，可以直接返回 nil
	return nil
}

func SetLogLevel(level LogLevel) {
	if level >= LogLevelDebug && level <= LogLevelError {
		currentLevel = level
	} else {
		log.Printf("Invalid log level: %d", level)
	}
}

func Println(v ...interface{}) {
	if currentLevel <= LogLevelInfo {
		log.Println(v...)
		message := fmt.Sprint(v...)
		LogToFile("INFO", message)
	}
}

func Printf(format string, v ...interface{}) {
	if currentLevel <= LogLevelInfo {
		log.Printf(format, v...)
		message := fmt.Sprintf(format, v...)
		LogToFile("INFO", message)
	}
}

func Warnf(format string, v ...interface{}) {
	if currentLevel <= LogLevelWarn {
		log.Printf(format, v...)
		message := fmt.Sprintf(format, v...)
		LogToFile("WARN", message)
	}
}

func Errorf(format string, v ...interface{}) {
	if currentLevel <= LogLevelError {
		log.Printf(format, v...)
		message := fmt.Sprintf(format, v...)
		LogToFile("ERROR", message)
	}
}

func Fatalf(format string, v ...interface{}) {
	log.Printf(format, v...)
	message := fmt.Sprintf(format, v...)
	LogToFile("FATAL", message)
	os.Exit(1) // Fatal logs usually terminate the program
}
