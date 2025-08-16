package transport

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// UDPTransport handles UDP transport for SIP messages
type UDPTransport struct {
	conn     *net.UDPConn
	handler  MessageHandler
	running  bool
	mu       sync.RWMutex
	wg       sync.WaitGroup
	stopChan chan struct{}
}

// NewUDPTransport creates a new UDP transport handler
func NewUDPTransport() *UDPTransport {
	return &UDPTransport{
		stopChan: make(chan struct{}),
	}
}

// Start starts the UDP listener on the specified port
func (u *UDPTransport) Start(port int) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.running {
		return fmt.Errorf("UDP transport already running")
	}

	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP port %d: %w", port, err)
	}

	u.conn = conn
	u.running = true

	// Start the message receiving goroutine
	u.wg.Add(1)
	go u.receiveMessages()

	return nil
}

// Stop stops the UDP transport
func (u *UDPTransport) Stop() error {
	u.mu.Lock()
	if !u.running {
		u.mu.Unlock()
		return nil
	}

	u.running = false
	close(u.stopChan)

	if u.conn != nil {
		u.conn.Close()
	}
	u.mu.Unlock()

	u.wg.Wait()
	return nil
}

// SendMessage sends a SIP message over UDP
func (u *UDPTransport) SendMessage(data []byte, addr net.Addr) error {
	u.mu.RLock()
	defer u.mu.RUnlock()

	if !u.running || u.conn == nil {
		return fmt.Errorf("UDP transport not running")
	}

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		return fmt.Errorf("invalid address type for UDP transport: %T", addr)
	}

	_, err := u.conn.WriteToUDP(data, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to send UDP message: %w", err)
	}

	return nil
}

// RegisterHandler registers a message handler for incoming messages
func (u *UDPTransport) RegisterHandler(handler MessageHandler) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.handler = handler
}

// receiveMessages handles incoming UDP messages
func (u *UDPTransport) receiveMessages() {
	defer u.wg.Done()

	buffer := make([]byte, 65536) // Maximum UDP packet size

	for {
		select {
		case <-u.stopChan:
			return
		default:
		}

		u.mu.RLock()
		conn := u.conn
		handler := u.handler
		u.mu.RUnlock()

		if conn == nil {
			break
		}

		// Set read timeout to allow periodic checking of stop signal
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected, continue loop to check stop signal
				continue
			}
			// Check if we're stopping
			select {
			case <-u.stopChan:
				return
			default:
			}
			// Log error but continue receiving
			continue
		}

		if n > 0 && handler != nil {
			// Make a copy of the received data
			data := make([]byte, n)
			copy(data, buffer[:n])

			// Handle the message in a separate goroutine
			go func() {
				if err := handler.HandleMessage(data, "UDP", addr); err != nil {
					// Log error handling message
					// In a real implementation, this would use proper logging
				}
			}()
		}
	}
}

// IsRunning returns true if the UDP transport is running
func (u *UDPTransport) IsRunning() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.running
}

// LocalAddr returns the local address of the UDP connection
func (u *UDPTransport) LocalAddr() net.Addr {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if u.conn != nil {
		return u.conn.LocalAddr()
	}
	return nil
}