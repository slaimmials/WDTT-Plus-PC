package tunnel

import "testing"

func TestDescribePacket(t *testing.T) {
	packet := make([]byte, 40)
	packet[0] = 0x45
	packet[9] = 6
	copy(packet[12:16], []byte{10, 66, 66, 6})
	copy(packet[16:20], []byte{1, 1, 1, 1})
	if got, want := describePacket(packet), "TCP 10.66.66.6 → 1.1.1.1, 40 Б"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := describePacket([]byte{0x60, 0, 0}); got != "IPv6, 3 Б" {
		t.Fatalf("got %q", got)
	}
}
