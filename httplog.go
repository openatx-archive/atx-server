package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"

	accesslog "github.com/mash/go-accesslog"
	isatty "github.com/mattn/go-isatty"
)

var logger *log.Logger
var isaTTY = isatty.IsTerminal(os.Stdout.Fd())

func init() {
	if runtime.GOOS == "windows" {
		logger = log.New(os.Stdout, "", log.Ltime)
	} else {
		logger = log.New(os.Stdout, "\033[0;32m[", log.Ltime)
	}
}

type HTTPLogger struct {
}

// Example
// [I 170227 14:47:16 web:1946] 200 GET /api/v1/devices (10.240.185.65) 28.00ms
func (l HTTPLogger) Log(record accesslog.LogRecord) {
	// update info too many just ignore
	if record.Method == "POST" && regexp.MustCompile(`/devices/[^/]+/info`).MatchString(record.Uri) {
		return
	}
	if strings.HasSuffix(record.Uri, "/heartbeat") {
		return
	}

	if isaTTY {
		logger.Println(fmt.Sprintf("\b] \033[0;m%d %s %s (%s) %.2fms", record.Status, record.Method, record.Uri, record.Ip,
			float64(record.ElapsedTime.Nanoseconds()/1000)/1000.0))
	} else {
		logger.Println(fmt.Sprintf("%d %s %s (%s) %.2fms", record.Status, record.Method, record.Uri, record.Ip,
			float64(record.ElapsedTime.Nanoseconds()/1000)/1000.0))
	}
}
