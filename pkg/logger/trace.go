package logger

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

// Hook define a Logrus hook
type Hook struct {
	AppName       string
	Prefix        string
	AppField      string
	FunctionField string
	SourceField   string
	Skip          int
	levels        []logrus.Level
	Formatter     func(file, function string, line int) (string, string)
}

// Levels get hook levels
func (hook *Hook) Levels() []logrus.Level {
	return hook.levels
}

// Fire Logrus hook function
func (hook *Hook) Fire(entry *logrus.Entry) error {
	source, function := hook.Formatter(findCaller(hook.Skip))
	entry.Data[hook.AppField] = hook.AppName
	entry.Data[hook.SourceField] = source
	entry.Data[hook.FunctionField] = fmt.Sprintf("%s%s", hook.Prefix, function)
	return nil
}

// NewHook create new hook
func NewHook(levels ...logrus.Level) *Hook {
	hook := Hook{
		AppField:      "app",
		FunctionField: "function",
		SourceField:   "source",
		Skip:          5,
		levels:        levels,
		Formatter: func(file, function string, line int) (string, string) {
			return fmt.Sprintf("%s:%d", file, line), function
		},
	}
	if len(hook.levels) == 0 {
		hook.levels = logrus.AllLevels
	}

	return &hook
}

func findCaller(skip int) (string, string, int) {
	var (
		pc       uintptr
		file     string
		function string
		line     int
	)
	for i := 0; i < 10; i++ {
		pc, file, line = getCaller(skip + i)
		if !strings.HasPrefix(file, "logrus") {
			break
		}
	}
	if pc != 0 {
		frames := runtime.CallersFrames([]uintptr{pc})
		frame, _ := frames.Next()
		function = frame.Function
	}

	return file, function, line
}

func getCaller(skip int) (uintptr, string, int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return 0, "", 0
	}

	n := 0
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			n++
			if n >= 2 {
				file = file[i+1:]
				break
			}
		}
	}

	return pc, file, line
}
