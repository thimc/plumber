package internal

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	DefaultEnvVar = "PLUMBERD"
	DefaultDir    = "/mnt/plumb"
	DefaultShell  = "/bin/sh -c"
)

// Defaults reads the value of the OS environment variable [DefaultEnvVar]
// and reinitializes the default values for file-related flags.
func Defaults() {
	dir := os.Getenv(DefaultEnvVar)
	if dir == "" {
		dir = DefaultDir
	}
	flag.VisitAll(func(f *flag.Flag) {
		if strings.Contains(f.Usage, "file") {
			f.DefValue = fmt.Sprintf("%s/%s", dir, f.Value)
			if err := f.Value.Set(f.DefValue); err != nil {
				panic(err)
			}
		}
	})
}
