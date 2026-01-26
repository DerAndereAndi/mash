package discovery_test

import (
	"net"
	"testing"

	"github.com/enbility/zeroconf/v3/mocks"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/stretchr/testify/mock"
)

// testAdvertiserConfig returns an AdvertiserConfig with mock connections.
// This allows tests to run without binding to real network interfaces.
func testAdvertiserConfig(t *testing.T) discovery.AdvertiserConfig {
	factory := mocks.NewMockConnectionFactory(t)
	provider := mocks.NewMockInterfaceProvider(t)

	// Mock interface provider to return a fake interface
	provider.EXPECT().MulticastInterfaces().Return([]net.Interface{
		{Index: 1, Name: "lo0", Flags: net.FlagUp | net.FlagMulticast},
	}).Maybe()

	// Create mock packet connections
	ipv4Conn := mocks.NewMockPacketConn(t)
	ipv6Conn := mocks.NewMockPacketConn(t)

	// Mock IPv4 connection
	setupMockPacketConn(ipv4Conn)

	// Mock IPv6 connection
	setupMockPacketConn(ipv6Conn)

	// Mock connection factory to return our mock connections
	factory.EXPECT().CreateIPv4Conn(mock.Anything).Return(ipv4Conn, nil).Maybe()
	factory.EXPECT().CreateIPv6Conn(mock.Anything).Return(ipv6Conn, nil).Maybe()

	return discovery.AdvertiserConfig{
		ConnectionFactory: factory,
		InterfaceProvider: provider,
	}
}

// testBrowserConfig returns a BrowserConfig with mock connections.
// This allows tests to run without binding to real network interfaces.
func testBrowserConfig(t *testing.T) discovery.BrowserConfig {
	factory := mocks.NewMockConnectionFactory(t)
	provider := mocks.NewMockInterfaceProvider(t)

	// Mock interface provider to return a fake interface
	provider.EXPECT().MulticastInterfaces().Return([]net.Interface{
		{Index: 1, Name: "lo0", Flags: net.FlagUp | net.FlagMulticast},
	}).Maybe()

	// Create mock packet connections
	ipv4Conn := mocks.NewMockPacketConn(t)
	ipv6Conn := mocks.NewMockPacketConn(t)

	// Mock IPv4 connection
	setupMockPacketConn(ipv4Conn)

	// Mock IPv6 connection
	setupMockPacketConn(ipv6Conn)

	// Mock connection factory to return our mock connections
	factory.EXPECT().CreateIPv4Conn(mock.Anything).Return(ipv4Conn, nil).Maybe()
	factory.EXPECT().CreateIPv6Conn(mock.Anything).Return(ipv6Conn, nil).Maybe()

	return discovery.BrowserConfig{
		ConnectionFactory: factory,
		InterfaceProvider: provider,
	}
}

// setupMockPacketConn configures a mock packet connection with basic expectations.
func setupMockPacketConn(conn *mocks.MockPacketConn) {
	// JoinGroup succeeds
	conn.EXPECT().JoinGroup(mock.Anything, mock.Anything).Return(nil).Maybe()

	// LeaveGroup succeeds
	conn.EXPECT().LeaveGroup(mock.Anything, mock.Anything).Return(nil).Maybe()

	// WriteTo succeeds (simulates sending packets)
	conn.EXPECT().WriteTo(mock.Anything, mock.Anything, mock.Anything).Return(0, nil).Maybe()

	// ReadFrom returns nothing - the context cancellation will stop the browse
	conn.EXPECT().ReadFrom(mock.Anything).RunAndReturn(func(b []byte) (int, int, net.Addr, error) {
		return 0, 0, nil, nil
	}).Maybe()

	// Close succeeds
	conn.EXPECT().Close().Return(nil).Maybe()

	// TTL/HopLimit settings succeed
	conn.EXPECT().SetMulticastTTL(mock.Anything).Return(nil).Maybe()
	conn.EXPECT().SetMulticastHopLimit(mock.Anything).Return(nil).Maybe()
	conn.EXPECT().SetMulticastInterface(mock.Anything).Return(nil).Maybe()
}
