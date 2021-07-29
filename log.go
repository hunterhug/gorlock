package gorlock

import (
	"github.com/hunterhug/marmot/util/go-logging"
	"os"
)

// Global logger config for debug
var (
	logger = logging.MustGetLogger("gorlock")

	format = logging.MustStringFormatter(
		"%{color}%{time:2006-01-02 15:04:05.000} %{longpkg}:%{longfunc} [%{level:.5s}]:%{color:reset} %{message}",
	)
)

// Init log record
func init() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
	logging.SetLevel(logging.INFO, "gorlock")
}

// SetLogLevel Set log level
func SetLogLevel(level string) {
	lvl, _ := logging.LogLevel(level)
	logging.SetLevel(lvl, "gorlock")
}

// Log Return global log
func Log() *logging.Logger {
	return logger
}
