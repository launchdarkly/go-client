package logger

import (
	"log/syslog"
	"net"

	fs "github.com/launchdarkly/foundation/statuschecks"
)

type syslogLogger struct {
	LevelLogger
	addr string
}

func newSyslogLogger(w *syslog.Writer, addr string, level LogLevel) LDLogger {
	return syslogLogger{
		LevelLogger: LevelLogger{
			writer: w,
			level:  level,
		},
		addr: addr,
	}
}

func (l syslogLogger) Status() (ret fs.ServiceStatus) {
	conn, err := net.Dial("tcp", l.addr)
	if err != nil {
		ret = fs.DownService()
	} else {
		ret = fs.HealthyService()
		conn.Close()
	}
	return
}
