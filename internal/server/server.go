package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/zurustar/xylitol2/internal/auth"
	"github.com/zurustar/xylitol2/internal/config"
	"github.com/zurustar/xylitol2/internal/database"
	"github.com/zurustar/xylitol2/internal/handlers"
	"github.com/zurustar/xylitol2/internal/logging"
	"github.com/zurustar/xylitol2/internal/parser"
	"github.com/zurustar/xylitol2/internal/proxy"
	"github.com/zurustar/xylitol2/internal/registrar"
	"github.com/zurustar/xylitol2/internal/sessiontimer"
	"github.com/zurustar/xylitol2/internal/transaction"
	"github.com/zurustar/xylitol2/internal/transport"
	"github.com/zurustar/xylitol2/internal/webadmin"
)

// SIPServerImpl implements the Server interface
type SIPServerImpl struct {
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
	handlerManager     transport.MessageHandler
	authProcessor      auth.MessageProcessor
	
	// Shutdown coordination
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
	started    bool
	mu         sync.RWMutex
}

// NewSIPServer creates a new SIP server instance
func NewSIPServer() Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &SIPServerImpl{
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
	}
}

// LoadConfig loads and validates the server configuration
func (s *SIPServerImpl) LoadConfig(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.started {
		return fmt.Errorf("cannot load configuration while server is running")
	}
	
	// Create config manager and load configuration
	configManager := config.NewManager()
	cfg, err := configManager.Load(filename)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Validate configuration
	if err := configManager.Validate(cfg); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	s.config = cfg
	return nil
}

// Start initializes all components and starts the server
func (s *SIPServerImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.started {
		return fmt.Errorf("server is already running")
	}
	
	if s.config == nil {
		return fmt.Errorf("configuration not loaded")
	}
	
	// Initialize components in proper order
	if err := s.initializeComponents(); err != nil {
		s.cleanup()
		return fmt.Errorf("failed to initialize components: %w", err)
	}
	
	// Start transport listeners
	if err := s.startTransports(); err != nil {
		s.cleanup()
		return fmt.Errorf("failed to start transports: %w", err)
	}
	
	// Start web admin interface if enabled
	if s.config.WebAdmin.Enabled {
		if err := s.webAdminServer.Start(s.config.WebAdmin.Port); err != nil {
			s.cleanup()
			return fmt.Errorf("failed to start web admin server: %w", err)
		}
		s.logger.Info("Web admin interface started", logging.Field{Key: "port", Value: s.config.WebAdmin.Port})
	}
	
	// Start background cleanup routines
	s.startBackgroundTasks()
	
	s.started = true
	s.logger.Info("SIP Server started successfully",
		logging.Field{Key: "udp_port", Value: s.config.Server.UDPPort},
		logging.Field{Key: "tcp_port", Value: s.config.Server.TCPPort},
	)
	
	return nil
}

// Stop gracefully shuts down the server
func (s *SIPServerImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.started {
		return nil
	}
	
	s.logger.Info("Initiating server shutdown...")
	
	// Signal shutdown to all components
	s.cancel()
	close(s.shutdownCh)
	
	// Stop accepting new connections
	if s.transportManager != nil {
		if err := s.transportManager.Stop(); err != nil {
			s.logger.Error("Error stopping transport manager", logging.Field{Key: "error", Value: err})
		}
	}
	
	// Stop web admin server
	if s.webAdminServer != nil {
		if err := s.webAdminServer.Stop(); err != nil {
			s.logger.Error("Error stopping web admin server", logging.Field{Key: "error", Value: err})
		}
	}
	
	// Wait for background tasks to complete with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		s.logger.Info("All background tasks completed")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for background tasks to complete")
	}
	
	// Cleanup resources
	s.cleanup()
	
	s.started = false
	s.logger.Info("Server shutdown completed")
	
	return nil
}

// initializeComponents initializes all server components in proper order
func (s *SIPServerImpl) initializeComponents() error {
	var err error
	
	// 1. Initialize logger first
	loggerConfig := logging.LoggerConfig{
		Level: s.config.Logging.Level,
		File:  s.config.Logging.File,
	}
	s.logger, err = logging.NewLoggerFromConfig(loggerConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	s.logger.Info("Logger initialized")
	
	// 2. Initialize database
	s.databaseManager = database.NewSQLiteManager(s.config.Database.Path)
	
	if err := s.databaseManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	s.logger.Info("Database initialized", logging.Field{Key: "path", Value: s.config.Database.Path})
	
	// 3. Initialize user manager
	s.userManager = database.NewSIPUserManager(s.databaseManager)
	s.logger.Info("User manager initialized")
	
	// 4. Initialize message parser
	s.messageParser = parser.NewParser()
	s.logger.Info("Message parser initialized")
	
	// 5. Initialize transaction manager
	s.transactionManager = transaction.NewManager(nil) // Will set send function later
	s.logger.Info("Transaction manager initialized")
	
	// 6. Initialize authentication processor
	s.authProcessor = auth.NewAuthenticatedMessageProcessor(s.userManager, s.config.Authentication.Realm)
	s.logger.Info("Authentication processor initialized")
	
	// 7. Initialize registrar
	registrationDB := database.NewRegistrationDB(s.databaseManager)
	s.registrar = registrar.NewRegistrar(registrationDB, s.logger)
	s.logger.Info("Registrar initialized")
	
	// 8. Initialize session timer manager
	s.sessionTimerMgr = sessiontimer.NewManager(
		s.config.SessionTimer.DefaultExpires,
		s.config.SessionTimer.MinSE,
		s.config.SessionTimer.MaxSE,
		s.logger,
	)
	s.logger.Info("Session timer manager initialized")
	
	// 9. Initialize proxy engine
	s.proxyEngine = proxy.NewRequestForwardingEngine(
		s.registrar,
		s.transportManager,
		s.transactionManager,
		s.messageParser,
		nil, // huntGroupManager - not implemented yet
		nil, // huntGroupEngine - not implemented yet
		"localhost",
		s.config.Server.UDPPort,
	)
	
	// 10. Initialize validated handler manager with validation chain
	validatedManager := handlers.NewValidatedManager()
	
	// Set up the validation chain with default validators
	validationConfig := handlers.DefaultValidationConfig()
	validatedManager.SetupDefaultValidators(validationConfig)
	
	// Set the underlying handler manager
	validatedManager.Manager = handlers.NewManager()
	
	// Register method handlers
	s.setupMethodHandlers(validatedManager.Manager)
	
	// Create transport adapter to bridge ValidatedManager with transport.MessageHandler
	transportAdapter := handlers.NewTransportAdapter(
		validatedManager,
		s.messageParser,
		s.transactionManager,
		s.logger,
	)
	
	s.handlerManager = transportAdapter
	s.logger.Info("Validated handler manager initialized with validation chain")
	
	// 11. Initialize transport manager
	s.transportManager = transport.NewManager()
	s.transportManager.RegisterHandler(s.handlerManager)
	
	// Set transport manager in handler manager for response sending
	if setter, ok := s.handlerManager.(interface{ SetTransportManager(transport.TransportManager) }); ok {
		setter.SetTransportManager(s.transportManager)
	}
	
	s.logger.Info("Transport manager initialized")
	
	// 12. Initialize web admin server
	s.webAdminServer = webadmin.NewServer(s.userManager, s.logger)
	s.logger.Info("Web admin server initialized")
	
	return nil
}

// startTransports starts UDP and TCP transport listeners
func (s *SIPServerImpl) startTransports() error {
	// Start UDP transport
	if err := s.transportManager.StartUDP(s.config.Server.UDPPort); err != nil {
		return fmt.Errorf("failed to start UDP transport: %w", err)
	}
	s.logger.Info("UDP transport started", logging.Field{Key: "port", Value: s.config.Server.UDPPort})
	
	// Start TCP transport
	if err := s.transportManager.StartTCP(s.config.Server.TCPPort); err != nil {
		return fmt.Errorf("failed to start TCP transport: %w", err)
	}
	s.logger.Info("TCP transport started", logging.Field{Key: "port", Value: s.config.Server.TCPPort})
	
	return nil
}

// startBackgroundTasks starts background cleanup and maintenance tasks
func (s *SIPServerImpl) startBackgroundTasks() {
	// Start transaction cleanup routine
	s.wg.Add(1)
	go s.transactionCleanupRoutine()
	
	// Start contact cleanup routine
	s.wg.Add(1)
	go s.contactCleanupRoutine()
	
	// Start session timer cleanup routine
	s.wg.Add(1)
	go s.sessionTimerCleanupRoutine()
}

// transactionCleanupRoutine periodically cleans up expired transactions
func (s *SIPServerImpl) transactionCleanupRoutine() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Transaction cleanup routine stopping")
			return
		case <-ticker.C:
			s.transactionManager.CleanupExpired()
		}
	}
}

// contactCleanupRoutine periodically cleans up expired contacts
func (s *SIPServerImpl) contactCleanupRoutine() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Contact cleanup routine stopping")
			return
		case <-ticker.C:
			s.registrar.CleanupExpired()
		}
	}
}

// sessionTimerCleanupRoutine periodically cleans up expired sessions
func (s *SIPServerImpl) sessionTimerCleanupRoutine() {
	defer s.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Session timer cleanup routine stopping")
			return
		case <-ticker.C:
			s.sessionTimerMgr.CleanupExpiredSessions()
		}
	}
}

// setupMethodHandlers registers method handlers with the handler manager
func (s *SIPServerImpl) setupMethodHandlers(manager *handlers.Manager) {
	// Register REGISTER handler
	registerHandler := handlers.NewRegisterHandler(s.registrar, s.logger)
	manager.RegisterHandler(registerHandler)
	
	// Register session handler for INVITE, ACK, BYE
	sessionHandler := handlers.NewSessionHandler(s.proxyEngine, s.registrar, s.sessionTimerMgr)
	manager.RegisterHandler(sessionHandler)
	
	// Register auxiliary handler for OPTIONS and INFO
	auxHandler := handlers.NewAuxiliaryHandler(s.proxyEngine, s.registrar)
	manager.RegisterHandler(auxHandler)
}

// cleanup performs resource cleanup
func (s *SIPServerImpl) cleanup() {
	if s.databaseManager != nil {
		if err := s.databaseManager.Close(); err != nil {
			if s.logger != nil {
				s.logger.Error("Error closing database", logging.Field{Key: "error", Value: err})
			}
		}
	}
}

// RunWithSignalHandling runs the server with graceful shutdown on signals
func (s *SIPServerImpl) RunWithSignalHandling() error {
	// Start the server
	if err := s.Start(); err != nil {
		return err
	}
	
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Wait for shutdown signal
	sig := <-sigChan
	s.logger.Info("Received shutdown signal", logging.Field{Key: "signal", Value: sig.String()})
	
	// Graceful shutdown
	return s.Stop()
}