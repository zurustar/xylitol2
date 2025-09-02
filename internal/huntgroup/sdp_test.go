package huntgroup

import (
	"bytes"
	"testing"

	"github.com/zurustar/xylitol2/internal/logging"
)

// TestLogger is a simple logger for testing
type TestLogger struct {
	buf bytes.Buffer
}

func (l *TestLogger) Debug(msg string, fields ...logging.Field) {
	l.buf.WriteString("DEBUG: " + msg + "\n")
}

func (l *TestLogger) Info(msg string, fields ...logging.Field) {
	l.buf.WriteString("INFO: " + msg + "\n")
}

func (l *TestLogger) Warn(msg string, fields ...logging.Field) {
	l.buf.WriteString("WARN: " + msg + "\n")
}

func (l *TestLogger) Error(msg string, fields ...logging.Field) {
	l.buf.WriteString("ERROR: " + msg + "\n")
}

func TestSDPProcessor_ParseSDP(t *testing.T) {
	logger := &TestLogger{}
	processor := NewSDPProcessor(logger, "192.168.1.1", 5060)

	testSDP := `v=0
o=alice 2890844526 2890844527 IN IP4 host.atlanta.com
s=Session Description
c=IN IP4 host.atlanta.com
t=0 0
m=audio 49170 RTP/AVP 0
a=rtpmap:0 PCMU/8000`

	session, err := processor.ParseSDP(testSDP)
	if err != nil {
		t.Fatalf("Failed to parse SDP: %v", err)
	}

	if session.Version != 0 {
		t.Errorf("Expected version 0, got %d", session.Version)
	}

	if session.Origin == nil {
		t.Fatal("Origin should not be nil")
	}

	if session.Origin.Username != "alice" {
		t.Errorf("Expected username 'alice', got '%s'", session.Origin.Username)
	}

	if len(session.MediaDescriptions) != 1 {
		t.Errorf("Expected 1 media description, got %d", len(session.MediaDescriptions))
	}

	media := session.MediaDescriptions[0]
	if media.Type != "audio" {
		t.Errorf("Expected media type 'audio', got '%s'", media.Type)
	}

	if media.Port != 49170 {
		t.Errorf("Expected port 49170, got %d", media.Port)
	}
}

func TestSDPProcessor_ModifySDPForB2BUA(t *testing.T) {
	logger := &TestLogger{}
	processor := NewSDPProcessor(logger, "192.168.1.1", 5060)

	originalSDP := `v=0
o=alice 2890844526 2890844527 IN IP4 host.atlanta.com
s=Session Description
c=IN IP4 host.atlanta.com
t=0 0
m=audio 49170 RTP/AVP 0
a=rtpmap:0 PCMU/8000`

	modifiedSDP, err := processor.ModifySDPForB2BUA(originalSDP, "192.168.1.100", 5004)
	if err != nil {
		t.Fatalf("Failed to modify SDP: %v", err)
	}

	// Parse the modified SDP to verify changes
	session, err := processor.ParseSDP(modifiedSDP)
	if err != nil {
		t.Fatalf("Failed to parse modified SDP: %v", err)
	}

	// Check that connection address was updated
	if session.Connection.Address != "192.168.1.100" {
		t.Errorf("Expected connection address '192.168.1.100', got '%s'", session.Connection.Address)
	}

	// Check that origin address was updated
	if session.Origin.Address != "192.168.1.100" {
		t.Errorf("Expected origin address '192.168.1.100', got '%s'", session.Origin.Address)
	}

	// Check that audio port was updated
	if len(session.MediaDescriptions) > 0 && session.MediaDescriptions[0].Type == "audio" {
		if session.MediaDescriptions[0].Port != 5004 {
			t.Errorf("Expected audio port 5004, got %d", session.MediaDescriptions[0].Port)
		}
	}
}

func TestSDPProcessor_ValidateSDP(t *testing.T) {
	logger := &TestLogger{}
	processor := NewSDPProcessor(logger, "192.168.1.1", 5060)

	validSDP := `v=0
o=alice 2890844526 2890844527 IN IP4 host.atlanta.com
s=Session Description
t=0 0
m=audio 49170 RTP/AVP 0`

	err := processor.ValidateSDP(validSDP)
	if err != nil {
		t.Errorf("Valid SDP should not return error: %v", err)
	}

	// Test invalid SDP (missing origin)
	invalidSDP := `v=0
s=Session Description
t=0 0`

	err = processor.ValidateSDP(invalidSDP)
	if err == nil {
		t.Error("Invalid SDP should return error")
	}
}

func TestSDPProcessor_CreateBasicSDP(t *testing.T) {
	logger := &TestLogger{}
	processor := NewSDPProcessor(logger, "192.168.1.1", 5060)

	sdp := processor.CreateBasicSDP("192.168.1.100", 5004)
	
	// Validate the created SDP
	err := processor.ValidateSDP(sdp)
	if err != nil {
		t.Errorf("Created SDP should be valid: %v", err)
	}

	// Parse and check content
	session, err := processor.ParseSDP(sdp)
	if err != nil {
		t.Fatalf("Failed to parse created SDP: %v", err)
	}

	if session.Connection.Address != "192.168.1.100" {
		t.Errorf("Expected address '192.168.1.100', got '%s'", session.Connection.Address)
	}

	if len(session.MediaDescriptions) > 0 && session.MediaDescriptions[0].Port != 5004 {
		t.Errorf("Expected port 5004, got %d", session.MediaDescriptions[0].Port)
	}
}