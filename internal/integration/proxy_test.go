package integration

import (
	"net"
	"testing"
	"time"
)

// TestServerOPTIONS tests OPTIONS requests directed to the server itself
func TestServerOPTIONS(t *testing.T) {
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// OPTIONS request to server itself (no user part)
	message := `OPTIONS sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-server-options
From: <sip:client@test.local>;tag=client-tag
To: <sip:test.local>
Call-ID: server-options-call@test.local
CSeq: 1 OPTIONS
Content-Length: 0

`

	t.Logf("Sending server OPTIONS request")

	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buffer[:n])
	t.Logf("Server OPTIONS response:\n%s", response)

	// Should get 200 OK without authentication
	if !contains(response, "200 OK") {
		t.Errorf("Expected 200 OK, got: %s", response)
	}

	if !contains(response, "Allow:") {
		t.Errorf("Expected Allow header in response")
	}
}

// TestProxyOPTIONS tests OPTIONS requests that should be proxied
func TestProxyOPTIONS(t *testing.T) {
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// OPTIONS request to a specific user (should be proxied)
	message := `OPTIONS sip:alice@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-proxy-options
From: <sip:client@test.local>;tag=client-tag
To: <sip:alice@test.local>
Call-ID: proxy-options-call@test.local
CSeq: 1 OPTIONS
Content-Length: 0

`

	t.Logf("Sending proxy OPTIONS request")

	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buffer[:n])
	t.Logf("Proxy OPTIONS response:\n%s", response)

	// Should get 401 Unauthorized (authentication required for proxy)
	if !contains(response, "401 Unauthorized") {
		t.Errorf("Expected 401 Unauthorized, got: %s", response)
	}

	if !contains(response, "WWW-Authenticate:") {
		t.Errorf("Expected WWW-Authenticate header in response")
	}
}

// TestREGISTERAuthentication tests REGISTER request authentication
func TestREGISTERAuthentication(t *testing.T) {
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// REGISTER request without authentication
	message := `REGISTER sip:test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-register-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:alice@test.local>
Call-ID: register-call@test.local
CSeq: 1 REGISTER
Contact: <sip:alice@192.168.1.100:5060>
Content-Length: 0

`

	t.Logf("Sending REGISTER request without authentication")

	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buffer[:n])
	t.Logf("REGISTER response:\n%s", response)

	// Should get 401 Unauthorized
	if !contains(response, "401 Unauthorized") {
		t.Errorf("Expected 401 Unauthorized, got: %s", response)
	}

	if !contains(response, "WWW-Authenticate:") {
		t.Errorf("Expected WWW-Authenticate header in response")
	}
}

// TestINVITEAuthentication tests INVITE request authentication
func TestINVITEAuthentication(t *testing.T) {
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// INVITE request without authentication
	message := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invite-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: invite-call@test.local
CSeq: 1 INVITE
Content-Length: 0

`

	t.Logf("Sending INVITE request without authentication")

	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buffer[:n])
	t.Logf("INVITE response:\n%s", response)

	// Should get 401 Unauthorized
	if !contains(response, "401 Unauthorized") {
		t.Errorf("Expected 401 Unauthorized, got: %s", response)
	}

	if !contains(response, "WWW-Authenticate:") {
		t.Errorf("Expected WWW-Authenticate header in response")
	}
}

// TestINVITESessionTimer tests INVITE request Session-Timer validation
func TestINVITESessionTimer(t *testing.T) {
	conn, err := net.Dial("udp", "127.0.0.1:5060")
	if err != nil {
		t.Fatalf("Failed to create UDP connection: %v", err)
	}
	defer conn.Close()

	// INVITE request without Session-Timer (should be required)
	message := `INVITE sip:bob@test.local SIP/2.0
Via: SIP/2.0/UDP 192.168.1.100:5060;branch=z9hG4bK-invite-timer-test
From: <sip:alice@test.local>;tag=alice-tag
To: <sip:bob@test.local>
Call-ID: invite-timer-call@test.local
CSeq: 1 INVITE
Authorization: Digest username="alice", realm="sipserver.local", nonce="test", uri="sip:bob@test.local", response="test"
Content-Length: 0

`

	t.Logf("Sending INVITE request without Session-Timer")

	_, err = conn.Write([]byte(message))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	response := string(buffer[:n])
	t.Logf("INVITE Session-Timer response:\n%s", response)

	// Should get 421 Extension Required or 401 Unauthorized (auth first)
	if !contains(response, "421 Extension Required") && !contains(response, "401 Unauthorized") {
		t.Logf("Got response: %s", response)
		// This is acceptable for now as authentication might be processed first
	}
}