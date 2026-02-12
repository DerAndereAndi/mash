package runner

import (
	"net"
	"testing"
)

func TestHasGlobalIPv6Addr_GlobalUnicast(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
	}
	if !hasGlobalIPv6Addr(addrs) {
		t.Error("expected true for global unicast IPv6 address")
	}
}

func TestHasGlobalIPv6Addr_ULA(t *testing.T) {
	// ULA addresses (fd00::/8) should also count as non-link-local.
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("fd12:3456:789a::1"), Mask: net.CIDRMask(48, 128)},
	}
	if !hasGlobalIPv6Addr(addrs) {
		t.Error("expected true for ULA IPv6 address")
	}
}

func TestHasGlobalIPv6Addr_LinkLocalOnly(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
	}
	if hasGlobalIPv6Addr(addrs) {
		t.Error("expected false for link-local-only IPv6")
	}
}

func TestHasGlobalIPv6Addr_IPv4Only(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.1"), Mask: net.CIDRMask(24, 32)},
	}
	if hasGlobalIPv6Addr(addrs) {
		t.Error("expected false for IPv4-only addresses")
	}
}

func TestHasGlobalIPv6Addr_Mixed(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.1"), Mask: net.CIDRMask(24, 32)},
		&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
	}
	if !hasGlobalIPv6Addr(addrs) {
		t.Error("expected true when global IPv6 is present among mixed addresses")
	}
}

func TestHasGlobalIPv6Addr_Empty(t *testing.T) {
	if hasGlobalIPv6Addr(nil) {
		t.Error("expected false for nil address list")
	}
	if hasGlobalIPv6Addr([]net.Addr{}) {
		t.Error("expected false for empty address list")
	}
}

func TestHasGlobalIPv6Addr_Loopback(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},
	}
	if hasGlobalIPv6Addr(addrs) {
		t.Error("expected false for loopback IPv6")
	}
}
