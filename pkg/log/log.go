package log

import (
	"fmt"
	"os"
)

var (
	Silent bool
)

func Debugf(msg string, args ...interface{}) {
	if !Silent {
		fmt.Fprintf(os.Stderr, msg+"\n", args...)
	}
}
