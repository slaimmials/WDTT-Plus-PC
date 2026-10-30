// Package firewall blocks traffic that bypasses the VPN adapter using the
// Windows Filtering Platform.
//
// It is adapted from golang.zx2c4.com/wireguard/windows/tunnel/firewall
// (MIT, WireGuard LLC). The only functional change is in
// getCurrentProcessSecurityDescriptor: the permit rule for this process is
// bound to the process user instead of the WireGuard service SID.
package firewall
