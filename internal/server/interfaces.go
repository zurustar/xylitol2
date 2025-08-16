package server

import (
	"github.com/zurustar/xylitol2/internal/config"
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/proxy"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
	"github.com/zurustar/xylitol2/internal/webadmin"
)

// SIPServer represents the main SIP server that coordinates all components
type SIPServer struct {
	config             *config.Config
	logger             logging.Logger
	transportManager   transport.TransportManager
	messageParser      parser.MessageParser
	transactionManager transaction.TransactionManager
	databaseManager    database.DatabaseManager
	userManager        database.UserManager
	registrar          registrar.Registrar
	proxyEngine        proxy.ProxyEngine
	sessionTimerMgr    sessiontimer.SessionTimerManager
	webAdminServer     webadmin.WebAdminServer
}

// Server defines the interface for the main SIP server
type Server interface {
	Start() error
	Stop() error
	LoadConfig(filename string) error
	RunWithSignalHandling() error
}