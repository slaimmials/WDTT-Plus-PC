package core

import (
	"fmt"
	"log"
	"strings"

	"golang.zx2c4.com/wireguard/device"
)

// WireGuardLogger writes wireguard-go errors and handshake events to the
// application log. device.NewLogger writes to stdout, which a GUI process
// does not have.
func WireGuardLogger(prefix string) *device.Logger {
	return &device.Logger{
		Verbosef: func(format string, args ...any) {
			message := fmt.Sprintf(format, args...)
			if strings.Contains(message, "andshake") {
				log.Print(prefix + message)
			}
		},
		Errorf: func(format string, args ...any) {
			log.Printf(prefix+format, args...)
		},
	}
}
