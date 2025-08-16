package proxy

import (
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/transaction"
)

// ProxyEngine defines the interface for SIP proxy functionality
type ProxyEngine interface {
	ProcessRequest(req *parser.SIPMessage, transaction transaction.Transaction) error
	ForwardRequest(req *parser.SIPMessage, targets []*database.RegistrarContact) error
	ProcessResponse(resp *parser.SIPMessage, transaction transaction.Transaction) error
}