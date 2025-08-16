package handlers

import (
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// MethodHandler defines the interface for handling specific SIP methods
type MethodHandler interface {
	HandleRequest(req *parser.SIPMessage, transaction transaction.Transaction) error
	CanHandle(method string) bool
}

// HandlerManager defines the interface for managing method handlers
type HandlerManager interface {
	RegisterHandler(handler MethodHandler)
	HandleRequest(req *parser.SIPMessage, transaction transaction.Transaction) error
	GetSupportedMethods() []string
}