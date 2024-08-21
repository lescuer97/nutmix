package routes

import "log/syslog"

type Logger struct {
	Sysloger *syslog.Writer
}
