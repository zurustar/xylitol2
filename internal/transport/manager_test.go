package transport

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManager_StartStopUDP(t *testing.T) {
	manager := NewManager()

	// Test starting UDP
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}

	if !manager.IsUDPRunning() {
		t.Error("UDP should be running after start")
	}

	if !manager.IsRunning() {
		t.Error("Manager should be running when UDP is running")
	}

	// Test stopping
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	if manager.IsUDPRunning() {
		t.Error("UDP should not be running after stop")
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running after stop")
	}
}

func TestManager_StartStopTCP(t *testing.T) {
	manager := NewManager()

	// Test starting TCP
	err := manager.StartTCP(0)
	if err != nil {
		t.Fatalf("Failed to start TCP: %v", err)
	}

	if !manager.IsTCPRunning() {
		t.Error("TCP should be running after start")
	}

	if !manager.IsRunning() {
		t.Error("Manager should be running when TCP is running")
	}

	// Test stopping
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	if manager.IsTCPRunning() {
		t.Error("TCP should not be running after stop")
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running after stop")
	}
}

func TestManager_StartBothTransports(t *testing.T) {
	manager := NewManager()

	// Start both transports
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}

	err = manager.StartTCP(0)
	if err != nil {
		t.Fatalf("Failed to start TCP: %v", err)
	}

	if !manager.IsUDPRunning() {
		t.Error("UDP should be running")
	}

	if !manager.IsTCPRunning() {
		t.Error("TCP should be running")
	}

	if !manager.IsRunning() {
		t.Error("Manager should be running when both transports are running")
	}

	// Test stopping
	err = manager.Stop()
	if err != nil {
		t.Fatalf("Failed to stop manager: %v", err)
	}

	if manager.IsUDPRunning() || manager.IsTCPRunning() {
		t.Error("Both transports should be stopped")
	}

	if manager.IsRunning() {
		t.Error("Manager should not be running after stop")
	}
}

func TestManager_SendMessageUDP(t *testing.T) {
	manager := NewManager()

	// Register a handler
	handler := &mockMessageHandler{}
	manager.RegisterHandler(handler)

	// Start UDP transport
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}
	defer manager.Stop()

	// Get UDP address and convert to IPv4
	udpAddr := manager.GetUDPLocalAddr().(*net.UDPAddr)
	testAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: udpAddr.Port,
	}

	// Create a small message (should use UDP)
	smallMessage := []byte("OPTIONS sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")

	// Send message via UDP
	err = manager.SendMessage(smallMessage, "UDP", testAddr)
	if err != nil {
		t.Fatalf("Failed to send UDP message: %v", err)
	}

	// Wait for message processing
	time.Sleep(100 * time.Millisecond)

	// Verify message was received
	messages := handler.getMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].transport != "UDP" {
		t.Errorf("Expected UDP transport, got %s", messages[0].transport)
	}
}

func TestManager_SendMessageTCP(t *testing.T) {
	manager := NewManager()

	// Register a handler
	handler := &mockMessageHandler{}
	manager.RegisterHandler(handler)

	// Start TCP transport
	err := manager.StartTCP(0)
	if err != nil {
		t.Fatalf("Failed to start TCP: %v", err)
	}
	defer manager.Stop()

	// Get TCP address
	tcpAddr := manager.GetTCPLocalAddr().(*net.TCPAddr)

	// Create a message
	message := []byte("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")

	// Send message via TCP
	err = manager.SendMessage(message, "TCP", tcpAddr)
	if err != nil {
		t.Fatalf("Failed to send TCP message: %v", err)
	}

	// Wait for message processing
	time.Sleep(200 * time.Millisecond)

	// Verify message was received
	messages := handler.getMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].transport != "TCP" {
		t.Errorf("Expected TCP transport, got %s", messages[0].transport)
	}
}

func TestManager_TransportSelection(t *testing.T) {
	manager := NewManager()

	// Test small message - should prefer UDP
	smallMessage := []byte("OPTIONS sip:test@example.com SIP/2.0\r\n\r\n")
	udpAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5060")
	
	transport := manager.GetTransportForMessage(smallMessage, udpAddr)
	if transport != "UDP" {
		t.Errorf("Expected UDP for small message, got %s", transport)
	}

	// Test large message - should prefer TCP
	largeBody := strings.Repeat("a", 1400) // Larger than 1300 byte threshold
	largeMessage := []byte(fmt.Sprintf("INVITE sip:test@example.com SIP/2.0\r\nContent-Length: %d\r\n\r\n%s", len(largeBody), largeBody))
	
	transport = manager.GetTransportForMessage(largeMessage, udpAddr)
	if transport != "TCP" {
		t.Errorf("Expected TCP for large message, got %s", transport)
	}

	// Test with TCP address - should prefer TCP
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:5060")
	transport = manager.GetTransportForMessage(smallMessage, tcpAddr)
	if transport != "TCP" {
		t.Errorf("Expected TCP for TCP address, got %s", transport)
	}
}

func TestManager_SendMessageNotRunning(t *testing.T) {
	manager := NewManager()

	message := []byte("test message")
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5060")

	err := manager.SendMessage(message, "UDP", addr)
	if err == nil {
		t.Error("Expected error when sending message with manager not running")
	}
}

func TestManager_SendMessageUnsupportedTransport(t *testing.T) {
	manager := NewManager()

	// Start UDP transport
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}
	defer manager.Stop()

	message := []byte("test message")
	addr := manager.GetUDPLocalAddr()

	err = manager.SendMessage(message, "SCTP", addr)
	if err == nil {
		t.Error("Expected error for unsupported transport")
	}
}

func TestManager_SendMessageTransportNotRunning(t *testing.T) {
	manager := NewManager()

	// Start only UDP
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}
	defer manager.Stop()

	message := []byte("test message")
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:5060")

	// Try to send via TCP when only UDP is running
	err = manager.SendMessage(message, "TCP", addr)
	if err == nil {
		t.Error("Expected error when TCP transport not running")
	}
}

func TestManager_MultiTransportReceive(t *testing.T) {
	manager := NewManager()

	// Register a handler
	handler := &mockMessageHandler{}
	manager.RegisterHandler(handler)

	// Start both transports
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}

	err = manager.StartTCP(0)
	if err != nil {
		t.Fatalf("Failed to start TCP: %v", err)
	}
	defer manager.Stop()

	// Get addresses and convert UDP to IPv4
	udpAddr := manager.GetUDPLocalAddr().(*net.UDPAddr)
	tcpAddr := manager.GetTCPLocalAddr().(*net.TCPAddr)
	
	testUDPAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: udpAddr.Port,
	}

	// Send messages via both transports
	udpMessage := []byte("REGISTER sip:udp@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")
	tcpMessage := []byte("REGISTER sip:tcp@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n")

	// Send UDP message
	err = manager.SendMessage(udpMessage, "UDP", testUDPAddr)
	if err != nil {
		t.Fatalf("Failed to send UDP message: %v", err)
	}

	// Send TCP message
	err = manager.SendMessage(tcpMessage, "TCP", tcpAddr)
	if err != nil {
		t.Fatalf("Failed to send TCP message: %v", err)
	}

	// Wait for message processing
	time.Sleep(300 * time.Millisecond)

	// Verify both messages were received
	messages := handler.getMessages()
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	// Check that we received one UDP and one TCP message
	transports := make(map[string]int)
	for _, msg := range messages {
		transports[msg.transport]++
	}

	if transports["UDP"] != 1 {
		t.Errorf("Expected 1 UDP message, got %d", transports["UDP"])
	}

	if transports["TCP"] != 1 {
		t.Errorf("Expected 1 TCP message, got %d", transports["TCP"])
	}
}

func TestManager_ConcurrentOperations(t *testing.T) {
	manager := NewManager()

	// Register a handler
	handler := &mockMessageHandler{}
	manager.RegisterHandler(handler)

	// Start both transports
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}

	err = manager.StartTCP(0)
	if err != nil {
		t.Fatalf("Failed to start TCP: %v", err)
	}
	defer manager.Stop()

	// Get addresses and convert UDP to IPv4
	udpAddr := manager.GetUDPLocalAddr().(*net.UDPAddr)
	tcpAddr := manager.GetTCPLocalAddr().(*net.TCPAddr)
	
	testUDPAddr := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: udpAddr.Port,
	}

	// Send messages concurrently
	var wg sync.WaitGroup
	numMessages := 10

	// Send UDP messages
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			message := []byte(fmt.Sprintf("OPTIONS sip:udp%d@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n", id))
			err := manager.SendMessage(message, "UDP", testUDPAddr)
			if err != nil {
				t.Errorf("Failed to send UDP message %d: %v", id, err)
			}
		}(i)
	}

	// Send TCP messages
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			message := []byte(fmt.Sprintf("OPTIONS sip:tcp%d@example.com SIP/2.0\r\nContent-Length: 0\r\n\r\n", id))
			err := manager.SendMessage(message, "TCP", tcpAddr)
			if err != nil {
				t.Errorf("Failed to send TCP message %d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for message processing
	time.Sleep(500 * time.Millisecond)

	// Verify all messages were received
	messages := handler.getMessages()
	expectedTotal := numMessages * 2 // UDP + TCP messages
	if len(messages) != expectedTotal {
		t.Fatalf("Expected %d messages, got %d", expectedTotal, len(messages))
	}

	// Count messages by transport
	transports := make(map[string]int)
	for _, msg := range messages {
		transports[msg.transport]++
	}

	if transports["UDP"] != numMessages {
		t.Errorf("Expected %d UDP messages, got %d", numMessages, transports["UDP"])
	}

	if transports["TCP"] != numMessages {
		t.Errorf("Expected %d TCP messages, got %d", numMessages, transports["TCP"])
	}
}

func TestManager_LocalAddresses(t *testing.T) {
	manager := NewManager()

	// Should return nil when not running
	if addr := manager.GetUDPLocalAddr(); addr != nil {
		t.Error("Expected nil UDP address when not running")
	}

	if addr := manager.GetTCPLocalAddr(); addr != nil {
		t.Error("Expected nil TCP address when not running")
	}

	// Start both transports
	err := manager.StartUDP(0)
	if err != nil {
		t.Fatalf("Failed to start UDP: %v", err)
	}

	err = manager.StartTCP(0)
	if err != nil {
		t.Fatalf("Failed to start TCP: %v", err)
	}
	defer manager.Stop()

	// Should return valid addresses when running
	udpAddr := manager.GetUDPLocalAddr()
	if udpAddr == nil {
		t.Error("Expected non-nil UDP address when running")
	}

	tcpAddr := manager.GetTCPLocalAddr()
	if tcpAddr == nil {
		t.Error("Expected non-nil TCP address when running")
	}

	// Verify address types
	if _, ok := udpAddr.(*net.UDPAddr); !ok {
		t.Errorf("Expected UDP address, got %T", udpAddr)
	}

	if _, ok := tcpAddr.(*net.TCPAddr); !ok {
		t.Errorf("Expected TCP address, got %T", tcpAddr)
	}
}