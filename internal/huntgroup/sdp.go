package huntgroup

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zurustar/xylitol2/internal/logging"
)

// SDPSession represents a complete SDP session description
type SDPSession struct {
	Version       int                    `json:"version"`        // v=
	Origin        *SDPOrigin             `json:"origin"`         // o=
	SessionName   string                 `json:"session_name"`   // s=
	SessionInfo   string                 `json:"session_info"`   // i=
	URI           string                 `json:"uri"`            // u=
	Email         string                 `json:"email"`          // e=
	Phone         string                 `json:"phone"`          // p=
	Connection    *SDPConnection         `json:"connection"`     // c=
	Bandwidth     *SDPBandwidth          `json:"bandwidth"`      // b=
	Timing        []*SDPTiming           `json:"timing"`         // t=
	Attributes    map[string]string      `json:"attributes"`     // a=
	MediaDescriptions []*SDPMedia        `json:"media"`          // m=
	RawSDP        string                 `json:"raw_sdp"`        // Original SDP text
}

// SDPOrigin represents the SDP origin field (o=)
type SDPOrigin struct {
	Username       string `json:"username"`
	SessionID      string `json:"session_id"`
	SessionVersion string `json:"session_version"`
	NetworkType    string `json:"network_type"`    // Usually "IN"
	AddressType    string `json:"address_type"`    // Usually "IP4" or "IP6"
	Address        string `json:"address"`
}

// SDPConnection represents the SDP connection field (c=)
type SDPConnection struct {
	NetworkType string `json:"network_type"`    // Usually "IN"
	AddressType string `json:"address_type"`    // Usually "IP4" or "IP6"
	Address     string `json:"address"`
}

// SDPBandwidth represents the SDP bandwidth field (b=)
type SDPBandwidth struct {
	Type  string `json:"type"`   // CT, AS, etc.
	Value int    `json:"value"`  // Bandwidth in kbps
}

// SDPTiming represents the SDP timing field (t=)
type SDPTiming struct {
	StartTime string `json:"start_time"`
	StopTime  string `json:"stop_time"`
}

// SDPMedia represents a media description (m=)
type SDPMedia struct {
	Type       string            `json:"type"`        // audio, video, etc.
	Port       int               `json:"port"`
	PortCount  int               `json:"port_count"`  // For multiple ports
	Protocol   string            `json:"protocol"`    // RTP/AVP, etc.
	Formats    []string          `json:"formats"`     // Payload types
	Connection *SDPConnection    `json:"connection"`  // Media-level connection
	Bandwidth  *SDPBandwidth     `json:"bandwidth"`   // Media-level bandwidth
	Attributes map[string]string `json:"attributes"`  // Media-level attributes
}

// SDPProcessor handles SDP parsing, modification, and generation for B2BUA
type SDPProcessor struct {
	logger     logging.Logger
	serverHost string
	serverPort int
}

// NewSDPProcessor creates a new SDP processor
func NewSDPProcessor(logger logging.Logger, serverHost string, serverPort int) *SDPProcessor {
	return &SDPProcessor{
		logger:     logger,
		serverHost: serverHost,
		serverPort: serverPort,
	}
}

// ParseSDP parses SDP content into an SDPSession structure
func (sp *SDPProcessor) ParseSDP(sdpContent string) (*SDPSession, error) {
	if sdpContent == "" {
		return nil, fmt.Errorf("empty SDP content")
	}

	session := &SDPSession{
		Attributes: make(map[string]string),
		RawSDP:     sdpContent,
	}

	lines := strings.Split(strings.ReplaceAll(sdpContent, "\r\n", "\n"), "\n")
	var currentMedia *SDPMedia

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 2 || line[1] != '=' {
			continue
		}

		fieldType := line[0]
		fieldValue := line[2:]

		switch fieldType {
		case 'v':
			if version, err := strconv.Atoi(fieldValue); err == nil {
				session.Version = version
			}

		case 'o':
			session.Origin = sp.parseOrigin(fieldValue)

		case 's':
			session.SessionName = fieldValue

		case 'i':
			if currentMedia != nil {
				// Media-level session info (not commonly used)
			} else {
				session.SessionInfo = fieldValue
			}

		case 'c':
			connection := sp.parseConnection(fieldValue)
			if currentMedia != nil {
				currentMedia.Connection = connection
			} else {
				session.Connection = connection
			}

		case 'b':
			bandwidth := sp.parseBandwidth(fieldValue)
			if currentMedia != nil {
				currentMedia.Bandwidth = bandwidth
			} else {
				session.Bandwidth = bandwidth
			}

		case 't':
			session.Timing = append(session.Timing, sp.parseTiming(fieldValue))

		case 'm':
			// New media description
			currentMedia = sp.parseMedia(fieldValue)
			if currentMedia != nil {
				session.MediaDescriptions = append(session.MediaDescriptions, currentMedia)
			}

		case 'a':
			if currentMedia != nil {
				sp.parseAttribute(fieldValue, currentMedia.Attributes)
			} else {
				sp.parseAttribute(fieldValue, session.Attributes)
			}
		}
	}

	return session, nil
}

// GenerateSDP generates SDP content from an SDPSession structure
func (sp *SDPProcessor) GenerateSDP(session *SDPSession) (string, error) {
	if session == nil {
		return "", fmt.Errorf("session cannot be nil")
	}

	var lines []string

	// Version (required)
	lines = append(lines, fmt.Sprintf("v=%d", session.Version))

	// Origin (required)
	if session.Origin != nil {
		lines = append(lines, fmt.Sprintf("o=%s %s %s %s %s %s",
			session.Origin.Username,
			session.Origin.SessionID,
			session.Origin.SessionVersion,
			session.Origin.NetworkType,
			session.Origin.AddressType,
			session.Origin.Address))
	}

	// Session name (required)
	lines = append(lines, fmt.Sprintf("s=%s", session.SessionName))

	// Session info (optional)
	if session.SessionInfo != "" {
		lines = append(lines, fmt.Sprintf("i=%s", session.SessionInfo))
	}

	// Connection (optional at session level)
	if session.Connection != nil {
		lines = append(lines, fmt.Sprintf("c=%s %s %s",
			session.Connection.NetworkType,
			session.Connection.AddressType,
			session.Connection.Address))
	}

	// Bandwidth (optional)
	if session.Bandwidth != nil {
		lines = append(lines, fmt.Sprintf("b=%s:%d", session.Bandwidth.Type, session.Bandwidth.Value))
	}

	// Timing (required)
	for _, timing := range session.Timing {
		lines = append(lines, fmt.Sprintf("t=%s %s", timing.StartTime, timing.StopTime))
	}

	// Session attributes
	for name, value := range session.Attributes {
		if value == "" {
			lines = append(lines, fmt.Sprintf("a=%s", name))
		} else {
			lines = append(lines, fmt.Sprintf("a=%s:%s", name, value))
		}
	}

	// Media descriptions
	for _, media := range session.MediaDescriptions {
		// Media line
		if media.PortCount > 1 {
			lines = append(lines, fmt.Sprintf("m=%s %d/%d %s %s",
				media.Type, media.Port, media.PortCount, media.Protocol, strings.Join(media.Formats, " ")))
		} else {
			lines = append(lines, fmt.Sprintf("m=%s %d %s %s",
				media.Type, media.Port, media.Protocol, strings.Join(media.Formats, " ")))
		}

		// Media connection
		if media.Connection != nil {
			lines = append(lines, fmt.Sprintf("c=%s %s %s",
				media.Connection.NetworkType,
				media.Connection.AddressType,
				media.Connection.Address))
		}

		// Media bandwidth
		if media.Bandwidth != nil {
			lines = append(lines, fmt.Sprintf("b=%s:%d", media.Bandwidth.Type, media.Bandwidth.Value))
		}

		// Media attributes
		for name, value := range media.Attributes {
			if value == "" {
				lines = append(lines, fmt.Sprintf("a=%s", name))
			} else {
				lines = append(lines, fmt.Sprintf("a=%s:%s", name, value))
			}
		}
	}

	return strings.Join(lines, "\r\n") + "\r\n", nil
}

// ModifySDPForB2BUA modifies SDP for B2BUA operation
func (sp *SDPProcessor) ModifySDPForB2BUA(originalSDP string, newAddress string, newPort int) (string, error) {
	session, err := sp.ParseSDP(originalSDP)
	if err != nil {
		return "", fmt.Errorf("failed to parse SDP: %w", err)
	}

	// Update session-level connection if present
	if session.Connection != nil {
		session.Connection.Address = newAddress
	}

	// Update media-level connections and ports
	for _, media := range session.MediaDescriptions {
		if media.Connection != nil {
			media.Connection.Address = newAddress
		}
		
		// Update media port (for audio, typically)
		if media.Type == "audio" && newPort > 0 {
			media.Port = newPort
		}
	}

	// Update origin address
	if session.Origin != nil {
		session.Origin.Address = newAddress
		// Increment session version for modification
		if sessionVersion, err := strconv.ParseInt(session.Origin.SessionVersion, 10, 64); err == nil {
			session.Origin.SessionVersion = strconv.FormatInt(sessionVersion+1, 10)
		}
	}

	return sp.GenerateSDP(session)
}

// RelaySDPOffer processes and relays an SDP offer from caller to callee
func (sp *SDPProcessor) RelaySDPOffer(callerSDP string, session *B2BUASession) (string, error) {
	if callerSDP == "" {
		return "", nil // No SDP to relay
	}

	sp.logger.Debug("Relaying SDP offer",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "original_sdp_length", Value: len(callerSDP)})

	// For basic B2BUA operation, we can relay the SDP as-is
	// In more advanced scenarios, we might need to modify media addresses
	modifiedSDP, err := sp.ModifySDPForB2BUA(callerSDP, sp.serverHost, 0)
	if err != nil {
		sp.logger.Warn("Failed to modify SDP, using original",
			logging.Field{Key: "session_id", Value: session.SessionID},
			logging.Field{Key: "error", Value: err.Error()})
		return callerSDP, nil
	}

	sp.logger.Debug("SDP offer relayed successfully",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "modified_sdp_length", Value: len(modifiedSDP)})

	return modifiedSDP, nil
}

// RelaySDPAnswer processes and relays an SDP answer from callee to caller
func (sp *SDPProcessor) RelaySDPAnswer(calleeSDP string, session *B2BUASession) (string, error) {
	if calleeSDP == "" {
		return "", nil // No SDP to relay
	}

	sp.logger.Debug("Relaying SDP answer",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "original_sdp_length", Value: len(calleeSDP)})

	// Store the SDP answer in the session
	session.Lock()
	session.SDPAnswer = calleeSDP
	session.Unlock()

	// For basic B2BUA operation, we can relay the SDP as-is
	// In more advanced scenarios, we might need to modify media addresses
	modifiedSDP, err := sp.ModifySDPForB2BUA(calleeSDP, sp.serverHost, 0)
	if err != nil {
		sp.logger.Warn("Failed to modify SDP answer, using original",
			logging.Field{Key: "session_id", Value: session.SessionID},
			logging.Field{Key: "error", Value: err.Error()})
		return calleeSDP, nil
	}

	sp.logger.Debug("SDP answer relayed successfully",
		logging.Field{Key: "session_id", Value: session.SessionID},
		logging.Field{Key: "modified_sdp_length", Value: len(modifiedSDP)})

	return modifiedSDP, nil
}

// ValidateSDP performs basic SDP validation
func (sp *SDPProcessor) ValidateSDP(sdpContent string) error {
	if sdpContent == "" {
		return fmt.Errorf("empty SDP content")
	}

	session, err := sp.ParseSDP(sdpContent)
	if err != nil {
		return fmt.Errorf("SDP parsing failed: %w", err)
	}

	// Check required fields
	if session.Origin == nil {
		return fmt.Errorf("missing required origin field (o=)")
	}

	if session.SessionName == "" {
		return fmt.Errorf("missing required session name field (s=)")
	}

	if len(session.Timing) == 0 {
		return fmt.Errorf("missing required timing field (t=)")
	}

	// Validate media descriptions
	for i, media := range session.MediaDescriptions {
		if media.Type == "" {
			return fmt.Errorf("media description %d missing type", i)
		}
		if media.Port <= 0 {
			return fmt.Errorf("media description %d has invalid port: %d", i, media.Port)
		}
		if media.Protocol == "" {
			return fmt.Errorf("media description %d missing protocol", i)
		}
	}

	return nil
}

// Helper parsing methods

func (sp *SDPProcessor) parseOrigin(value string) *SDPOrigin {
	parts := strings.Fields(value)
	if len(parts) != 6 {
		return nil
	}

	return &SDPOrigin{
		Username:       parts[0],
		SessionID:      parts[1],
		SessionVersion: parts[2],
		NetworkType:    parts[3],
		AddressType:    parts[4],
		Address:        parts[5],
	}
}

func (sp *SDPProcessor) parseConnection(value string) *SDPConnection {
	parts := strings.Fields(value)
	if len(parts) != 3 {
		return nil
	}

	return &SDPConnection{
		NetworkType: parts[0],
		AddressType: parts[1],
		Address:     parts[2],
	}
}

func (sp *SDPProcessor) parseBandwidth(value string) *SDPBandwidth {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return nil
	}

	if bandwidthValue, err := strconv.Atoi(parts[1]); err == nil {
		return &SDPBandwidth{
			Type:  parts[0],
			Value: bandwidthValue,
		}
	}

	return nil
}

func (sp *SDPProcessor) parseTiming(value string) *SDPTiming {
	parts := strings.Fields(value)
	if len(parts) != 2 {
		return &SDPTiming{StartTime: "0", StopTime: "0"}
	}

	return &SDPTiming{
		StartTime: parts[0],
		StopTime:  parts[1],
	}
}

func (sp *SDPProcessor) parseMedia(value string) *SDPMedia {
	parts := strings.Fields(value)
	if len(parts) < 4 {
		return nil
	}

	media := &SDPMedia{
		Type:       parts[0],
		Protocol:   parts[2],
		Attributes: make(map[string]string),
	}

	// Parse port and port count
	portParts := strings.Split(parts[1], "/")
	if port, err := strconv.Atoi(portParts[0]); err == nil {
		media.Port = port
	}
	if len(portParts) > 1 {
		if portCount, err := strconv.Atoi(portParts[1]); err == nil {
			media.PortCount = portCount
		}
	} else {
		media.PortCount = 1
	}

	// Parse formats (payload types)
	if len(parts) > 3 {
		media.Formats = parts[3:]
	}

	return media
}

func (sp *SDPProcessor) parseAttribute(value string, attributes map[string]string) {
	if colonIndex := strings.Index(value, ":"); colonIndex != -1 {
		name := value[:colonIndex]
		attrValue := value[colonIndex+1:]
		attributes[name] = attrValue
	} else {
		attributes[value] = ""
	}
}

// GetMediaAddress extracts the media address from SDP
func (sp *SDPProcessor) GetMediaAddress(sdpContent string) (string, error) {
	session, err := sp.ParseSDP(sdpContent)
	if err != nil {
		return "", err
	}

	// Check session-level connection first
	if session.Connection != nil {
		return session.Connection.Address, nil
	}

	// Check media-level connections
	for _, media := range session.MediaDescriptions {
		if media.Connection != nil {
			return media.Connection.Address, nil
		}
	}

	return "", fmt.Errorf("no media address found in SDP")
}

// GetMediaPorts extracts media ports from SDP
func (sp *SDPProcessor) GetMediaPorts(sdpContent string) ([]int, error) {
	session, err := sp.ParseSDP(sdpContent)
	if err != nil {
		return nil, err
	}

	var ports []int
	for _, media := range session.MediaDescriptions {
		ports = append(ports, media.Port)
	}

	return ports, nil
}

// CreateBasicSDP creates a basic SDP session for testing or fallback
func (sp *SDPProcessor) CreateBasicSDP(address string, port int) string {
	sessionID := strconv.FormatInt(time.Now().Unix(), 10)
	sessionVersion := sessionID

	sdp := fmt.Sprintf(`v=0
o=- %s %s IN IP4 %s
s=SIP Call
c=IN IP4 %s
t=0 0
m=audio %d RTP/AVP 0 8
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
`, sessionID, sessionVersion, address, address, port)

	return sdp
}