package tunnel

import (
	"fmt"
	"log"
	"net/netip"
	"sync/atomic"

	"golang.zx2c4.com/wireguard/tun"
)

const packetLogLimit = 6

// packetLogDevice logs the first packets in each direction so the log shows
// which addresses Windows actually uses on the adapter.
type packetLogDevice struct {
	tun.Device
	outbound atomic.Int32
	inbound  atomic.Int32
}

func (d *packetLogDevice) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	n, err := d.Device.Read(bufs, sizes, offset)
	for i := 0; i < n && d.outbound.Load() < packetLogLimit; i++ {
		if d.outbound.Add(1) <= packetLogLimit {
			log.Printf("[VPN] Windows → туннель: %s", describePacket(bufs[i][offset:offset+sizes[i]]))
		}
	}
	return n, err
}

func (d *packetLogDevice) Write(bufs [][]byte, offset int) (int, error) {
	for i := 0; i < len(bufs) && d.inbound.Load() < packetLogLimit; i++ {
		if d.inbound.Add(1) <= packetLogLimit {
			log.Printf("[VPN] туннель → Windows: %s", describePacket(bufs[i][offset:]))
		}
	}
	n, err := d.Device.Write(bufs, offset)
	if err != nil {
		log.Printf("[VPN] Адаптер не принял пакеты: %v", err)
	}
	return n, err
}

func describePacket(packet []byte) string {
	if len(packet) < 20 || packet[0]>>4 != 4 {
		if len(packet) > 0 && packet[0]>>4 == 6 {
			return fmt.Sprintf("IPv6, %d Б", len(packet))
		}
		return fmt.Sprintf("не IPv4, %d Б", len(packet))
	}
	src, _ := netip.AddrFromSlice(packet[12:16])
	dst, _ := netip.AddrFromSlice(packet[16:20])
	proto := map[byte]string{1: "ICMP", 6: "TCP", 17: "UDP"}[packet[9]]
	if proto == "" {
		proto = fmt.Sprintf("proto %d", packet[9])
	}
	return fmt.Sprintf("%s %s → %s, %d Б", proto, src, dst, len(packet))
}
