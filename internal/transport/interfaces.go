package transport

import (
	"net"
)

// MessageHandler defines the interface for handling incoming SIP messages
type MessageHandler interface {
	HandleMessage(data []byte, transport string, addr net.Addr) error
}

// TransportManager defines the interface for managing UDP and TCP transport
type TransportManager interface {
	StartUDP(port int) error
	StartTCP(port int) error
	SendMessage(msg []byte, transport string, addr net.Addr) error
	RegisterHandler(handler MessageHandler)
	Stop() error
}