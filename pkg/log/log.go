package log

import (
	"fmt"
	"io"
	"os"
)

type Logger struct {
	output io.Writer
}

type Config struct {
	Output io.Writer
}

func New(c Config) *Logger {
	var w io.Writer = os.Stderr
	if c.Output != nil {
		w = c.Output
	}
	return &Logger{
		output: w,
	}
}

func (l *Logger) Debugf(msg string, args ...interface{}) {
	_, _ = fmt.Fprintf(l.output, msg+"\n", args...)
}
