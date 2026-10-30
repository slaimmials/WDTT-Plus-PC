package tunnel

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// wintun holds wintun-<arch>.dll placed there by the build script.
//
//go:embed all:wintun
var wintun embed.FS

// ensureWintun puts wintun.dll next to the executable, where wireguard-go
// loads it from.
func ensureWintun() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	target := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	data, err := wintun.ReadFile("wintun/wintun-" + runtime.GOARCH + ".dll")
	if err != nil {
		return errors.New("не найден wintun.dll: положите его рядом с программой")
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("не удалось сохранить wintun.dll: %w", err)
	}
	return nil
}
