package debug

import (
	"fmt"
	"os"

	"github.com/alexdupre/modules2tuple/v2/config"
)

func Print(v ...any) {
	if config.Debug {
		fmt.Fprint(os.Stderr, v...)
	}
}

func Printf(format string, v ...any) {
	if config.Debug {
		fmt.Fprintf(os.Stderr, format, v...)
	}
}
