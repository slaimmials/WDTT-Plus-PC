// Package platform contains Windows integration: elevation and the wdtt://
// URL handler.
package platform

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const ElevatedFlag = "--elevated"

// RelaunchElevated starts this executable again with administrator rights.
// The caller must exit afterwards.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	args, _ := windows.UTF16PtrFromString(ElevatedFlag)
	return windows.ShellExecute(0, verb, file, args, nil, windows.SW_SHOWNORMAL)
}

// RegisterURLScheme makes wdtt:// links open this executable for the current user.
func RegisterURLScheme() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\wdtt`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue("", "URL:WDTT Plus"); err != nil {
		return err
	}
	if err := key.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	command, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\wdtt\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer command.Close()
	return command.SetStringValue("", `"`+exe+`" "%1"`)
}

// LinkFromArgs returns a wdtt:// link passed on the command line.
func LinkFromArgs(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(arg), "wdtt://") {
			return arg
		}
	}
	return ""
}
