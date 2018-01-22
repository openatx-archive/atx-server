package inforus

import (
	"path"
	"runtime"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Skip the following 3 frames.
// runtime.Callers
// github.com/178inaba/inforus.Hook.Fire
// github.com/178inaba/inforus.(*Hook).Fire
const skipFrameCnt = 3

// AddHookDefault is ...
func AddHookDefault() {
	log.AddHook(Hook{
		mu:       &sync.Mutex{},
		file:     true,
		line:     true,
		function: false,
		levels:   log.AllLevels,
	})
}

// AddHook is ...
func AddHook(file, line, function bool, levels []log.Level) {
	log.AddHook(Hook{
		mu:       &sync.Mutex{},
		file:     file,
		line:     line,
		function: function,
		levels:   levels,
	})
}

// Hook is ...
type Hook struct {
	mu       *sync.Mutex
	file     bool
	line     bool
	function bool
	levels   []log.Level
}

// Levels is ...
func (h Hook) Levels() []log.Level {
	return h.levels
}

// Fire is ...
func (h Hook) Fire(entry *log.Entry) error {
	pc := make([]uintptr, 64)
	cnt := runtime.Callers(skipFrameCnt, pc)

	for i := 0; i < cnt; i++ {
		fu := runtime.FuncForPC(pc[i])
		name := fu.Name()
		if !strings.Contains(name, "github.com/Sirupsen/logrus") {
			file, line := fu.FileLine(pc[i] - 1)
			if h.file {
				h.mu.Lock()
				entry.Data["file"] = path.Base(file)
				h.mu.Unlock()
			}

			if h.function {
				h.mu.Lock()
				entry.Data["func"] = path.Base(name)
				h.mu.Unlock()
			}

			if h.line {
				h.mu.Lock()
				entry.Data["line"] = line
				h.mu.Unlock()
			}

			break
		}
	}

	return nil
}
