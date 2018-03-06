package logger

import (
	"fmt"
	"io"
	"log/syslog"
	"os"
	"strings"

	fs "github.com/launchdarkly/foundation/statuschecks"
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l *LogLevel) String() string {
	switch *l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "(UNKNOWN)"
	}
}

type LDMessageLogger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
	Output(calldepth int, s string) error
}

type LDLogger interface {
	Debug() LDMessageLogger
	Info() LDMessageLogger
	Warn() LDMessageLogger
	Error() LDMessageLogger
	Status() fs.ServiceStatus
	IsDebug() bool
}

type SyslogConfig struct {
	Host         string
	Port         int
	Tag          string
	LogToConsole bool
	MinLevel     string
}

func (c *SyslogConfig) Level() LogLevel {
	switch strings.ToLower(c.MinLevel) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

var DefaultConfig = SyslogConfig{
	LogToConsole: true,
}
var Logger LDLogger = newConsoleLogger(LevelDebug)
var AccessWriter io.Writer

var Info LDMessageLogger = Logger.Info()

var Warn LDMessageLogger = Logger.Warn()

var Debug LDMessageLogger = Logger.Debug()

var Error LDMessageLogger = Logger.Error()

type noOpLogger struct{}

var NoOpLogger = noOpLogger{}

func (l noOpLogger) Println(v ...interface{})               {}
func (l noOpLogger) Printf(format string, v ...interface{}) {}
func (l noOpLogger) Output(calldepth int, s string) error   { return nil }

func Initialize(cfg SyslogConfig) {
	if cfg.Host == "" {
		Logger = newConsoleLogger(cfg.Level())
		AccessWriter, _ = os.Open("/dev/null")
	} else {
		fmt.Println("creating syslog logger")
		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		writer, err := syslog.Dial("tcp", addr, syslog.LOG_INFO, cfg.Tag+"-application")
		if err != nil {
			Logger = newConsoleLogger(cfg.Level())
			fmt.Println("Error creating syslog logger: ", err)
		} else {
			sysLogger := newSyslogLogger(writer, addr, cfg.Level())
			if cfg.LogToConsole {
				Logger = compositeLogger{[]LDLogger{sysLogger, newConsoleLogger(cfg.Level())}}
			} else {
				Logger = sysLogger
			}
		}
		accessWriter, accessErr := syslog.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), syslog.LOG_INFO, cfg.Tag+"-access")
		if accessErr != nil {
			AccessWriter, _ = os.Open("/dev/null")
			fmt.Println("Error creating syslog access logger: ", accessErr)
		} else {
			AccessWriter = accessWriter
		}
	}
	Info = Logger.Info()
	Warn = Logger.Warn()
	Debug = Logger.Debug()
	Error = Logger.Error()

	registerGodump()
}
