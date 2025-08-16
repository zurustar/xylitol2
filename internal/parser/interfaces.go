package parser

// MessageParser defines the interface for parsing and serializing SIP messages
type MessageParser interface {
	Parse(data []byte) (*SIPMessage, error)
	Serialize(msg *SIPMessage) ([]byte, error)
	Validate(msg *SIPMessage) error
}