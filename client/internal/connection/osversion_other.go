//go:build !windows

package connection

import "runtime"

func osVersion() string { return runtime.GOOS }
