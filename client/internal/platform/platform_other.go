//go:build !windows

package platform

import (
	"errors"
	"strings"
)

const ElevatedFlag = "--elevated"

func RelaunchElevated() error { return errors.New("доступно только в Windows") }

func RegisterURLScheme() error { return nil }

func LinkFromArgs(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(arg), "wdtt://") {
			return arg
		}
	}
	return ""
}
