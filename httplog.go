package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	accesslog "github.com/mash/go-accesslog"
)

var logger *log.Logger

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
	logger.Println(fmt.Sprintf("\b] \033[0;m%d %s %s (%s) %.2fms", record.Status, record.Method, record.Uri, record.Ip,
		float64(record.ElapsedTime.Nanoseconds()/1000)/1000.0))
}
