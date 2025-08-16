package transport

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

// Manager implements the TransportManager interface
type Manager struct {
	udpTransport *UDPTransport
	tcpTransport *TCPTransport
	handler      MessageHandler
	running      bool
	mu           sync.RWMutex
}

// NewManager creates a new transport manager
func NewManager() *Manager {
	return &Manager{
		udpTransport: NewUDPTransport(),
		tcpTransport: NewTCPTransport(),
	}
}

// StartUDP starts the UDP transport on the specified port
func (m *Manager) StartUDP(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handler != nil {
		m.udpTransport.RegisterHandler(m.handler)
	}

	err := m.udpTransport.Start(port)
	if err != nil {
		return fmt.Errorf("failed to start UDP transport: %w", err)
	}

	m.running = true
	return nil
}

// StartTCP starts the TCP transport on the specified port
func (m *Manager) StartTCP(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handler != nil {
		m.tcpTransport.RegisterHandler(m.handler)
	}

	err := m.tcpTransport.Start(port)
	if err != nil {
		return fmt.Errorf("failed to start TCP transport: %w", err)
	}

	m.running = true
	return nil
}

// SendMessage sends a SIP message using the appropriate transport
func (m *Manager) SendMessage(msg []byte, transport string, addr net.Addr) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return fmt.Errorf("transport manager not running")
	}

	// Determine transport method
	transportMethod := m.selectTransport(msg, transport, addr)

	switch strings.ToUpper(transportMethod) {
	case "UDP":
		if !m.udpTransport.IsRunning() {
			return fmt.Errorf("UDP transport not running")
		}
		return m.udpTransport.SendMessage(msg, addr)
	case "TCP":
		if !m.tcpTransport.IsRunning() {
			return fmt.Errorf("TCP transport not running")
		}
		return m.tcpTransport.SendMessage(msg, addr)
	default:
		return fmt.Errorf("unsupported transport: %s", transportMethod)
	}
}

// RegisterHandler registers a message handler for both transports
func (m *Manager) RegisterHandler(handler MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handler = handler

	// Register with existing transports if they're running
	if m.udpTransport.IsRunning() {
		m.udpTransport.RegisterHandler(handler)
	}
	if m.tcpTransport.IsRunning() {
		m.tcpTransport.RegisterHandler(handler)
	}
}

// Stop stops both UDP and TCP transports
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []string

	// Stop UDP transport
	if m.udpTransport.IsRunning() {
		if err := m.udpTransport.Stop(); err != nil {
			errors = append(errors, fmt.Sprintf("UDP: %v", err))
		}
	}

	// Stop TCP transport
	if m.tcpTransport.IsRunning() {
		if err := m.tcpTransport.Stop(); err != nil {
			errors = append(errors, fmt.Sprintf("TCP: %v", err))
		}
	}

	m.running = false

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping transports: %s", strings.Join(errors, ", "))
	}

	return nil
}

// selectTransport determines which transport to use based on message size and preferences
func (m *Manager) selectTransport(msg []byte, preferredTransport string, addr net.Addr) string {
	// If a specific transport is requested, use it
	if preferredTransport != "" {
		return preferredTransport
	}

	// RFC 3261: Use TCP for messages larger than 1300 bytes (conservative MTU)
	const maxUDPSize = 1300
	if len(msg) > maxUDPSize {
		return "TCP"
	}

	// Check if the address type suggests a specific transport
	switch addr.(type) {
	case *net.UDPAddr:
		return "UDP"
	case *net.TCPAddr:
		return "TCP"
	}

	// Default to UDP for SIP (RFC 3261 recommendation)
	return "UDP"
}

// IsRunning returns true if either transport is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running && (m.udpTransport.IsRunning() || m.tcpTransport.IsRunning())
}

// GetUDPLocalAddr returns the local address of the UDP transport
func (m *Manager) GetUDPLocalAddr() net.Addr {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.udpTransport.LocalAddr()
}

// GetTCPLocalAddr returns the local address of the TCP transport
func (m *Manager) GetTCPLocalAddr() net.Addr {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tcpTransport.LocalAddr()
}

// IsUDPRunning returns true if UDP transport is running
func (m *Manager) IsUDPRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.udpTransport.IsRunning()
}

// IsTCPRunning returns true if TCP transport is running
func (m *Manager) IsTCPRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tcpTransport.IsRunning()
}

// GetTransportForMessage returns the recommended transport for a message
func (m *Manager) GetTransportForMessage(msg []byte, addr net.Addr) string {
	return m.selectTransport(msg, "", addr)
}