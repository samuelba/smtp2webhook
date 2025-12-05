package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"

	"smtp-webhook-forwarder/internal/auth"
	"smtp-webhook-forwarder/internal/config"
	"smtp-webhook-forwarder/internal/logger"
)

// Server represents the SMTP server
type Server struct {
	config        *config.ServerConfig
	handler       *Handler
	authenticator auth.Authenticator
	logger        logger.Logger
	smtpServer    *smtp.Server
	semaphore     chan struct{} // For limiting concurrent connections
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewServer creates a new SMTP server
func NewServer(
	cfg *config.ServerConfig,
	handler *Handler,
	authenticator auth.Authenticator,
	logger logger.Logger,
) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:        cfg,
		handler:       handler,
		authenticator: authenticator,
		logger:        logger,
		semaphore:     make(chan struct{}, cfg.MaxConcurrentConnections),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Create the backend
	backend := &smtpBackend{
		server: s,
	}

	// Create the SMTP server
	smtpServer := smtp.NewServer(backend)
	smtpServer.Addr = fmt.Sprintf(":%d", cfg.Port)
	smtpServer.Domain = cfg.Hostname
	smtpServer.ReadTimeout = time.Duration(cfg.ReadTimeout) * time.Second
	smtpServer.WriteTimeout = time.Duration(cfg.WriteTimeout) * time.Second
	smtpServer.MaxMessageBytes = cfg.MaxEmailSize
	smtpServer.MaxRecipients = 100                              // Reasonable default
	smtpServer.AllowInsecureAuth = (cfg.SecurityMode == "none") // Allow plain auth only if no encryption

	// Configure TLS based on security mode
	if err := s.configureTLS(smtpServer, cfg); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to configure TLS: %w", err)
	}

	s.smtpServer = smtpServer

	return s, nil
}

// configureTLS configures TLS settings based on the security mode
func (s *Server) configureTLS(smtpServer *smtp.Server, cfg *config.ServerConfig) error {
	switch cfg.SecurityMode {
	case "none":
		// No TLS configuration needed
		s.logger.Info("SMTP server configured without encryption")
		return nil

	case "tls", "ssl":
		// Load TLS certificate
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		smtpServer.TLSConfig = tlsConfig
		s.logger.Info("SMTP server configured with TLS/SSL",
			logger.Str("cert_path", cfg.TLSCertPath),
		)
		return nil

	case "starttls":
		// Load TLS certificate
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		smtpServer.TLSConfig = tlsConfig
		s.logger.Info("SMTP server configured with STARTTLS",
			logger.Str("cert_path", cfg.TLSCertPath),
		)
		return nil

	default:
		return fmt.Errorf("unsupported security mode: %s", cfg.SecurityMode)
	}
}

// Start starts the SMTP server
func (s *Server) Start() error {
	authEnabled := "disabled"
	if s.authenticator != nil {
		authEnabled = "enabled"
	}

	s.logger.Info("starting SMTP server",
		logger.Int("port", s.config.Port),
		logger.Str("hostname", s.config.Hostname),
		logger.Str("security_mode", s.config.SecurityMode),
		logger.Str("auth_status", authEnabled),
	)

	var err error
	if s.config.SecurityMode == "tls" || s.config.SecurityMode == "ssl" {
		// Start with immediate TLS
		err = s.smtpServer.ListenAndServeTLS()
	} else {
		// Start with optional STARTTLS or no TLS
		err = s.smtpServer.ListenAndServe()
	}

	if err != nil && err != smtp.ErrServerClosed {
		return fmt.Errorf("SMTP server error: %w", err)
	}

	return nil
}

// Stop gracefully stops the SMTP server
func (s *Server) Stop() error {
	s.logger.Info("stopping SMTP server")

	// Cancel context to signal shutdown
	s.cancel()

	// Close the SMTP server (stops accepting new connections)
	if err := s.smtpServer.Close(); err != nil {
		s.logger.Error("error closing SMTP server", err)
	}

	// Wait for in-flight connections to complete (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("all connections closed gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Info("shutdown timeout reached, forcing close")
	}

	return nil
}

// smtpBackend implements the go-smtp Backend interface
type smtpBackend struct {
	server *Server
}

// NewSession creates a new SMTP session
func (b *smtpBackend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	// Try to acquire semaphore (limit concurrent connections)
	select {
	case b.server.semaphore <- struct{}{}:
		// Acquired semaphore
		b.server.wg.Add(1)
		return &smtpSession{
			server: b.server,
			conn:   c,
		}, nil
	default:
		// Too many concurrent connections
		return nil, &smtp.SMTPError{
			Code:    421,
			Message: "Too many concurrent connections",
		}
	}
}

// smtpSession implements the go-smtp Session interface
type smtpSession struct {
	server        *Server
	conn          *smtp.Conn
	from          string
	to            []string
	authenticated bool
}

// AuthMechanisms returns the list of supported authentication mechanisms
func (s *smtpSession) AuthMechanisms() []string {
	if s.server.authenticator == nil {
		return nil
	}
	return []string{"PLAIN", "LOGIN"}
}

// Auth handles authentication
func (s *smtpSession) Auth(mech string) (sasl.Server, error) {
	if s.server.authenticator == nil {
		return nil, smtp.ErrAuthUnsupported
	}

	return sasl.NewPlainServer(func(identity, username, password string) error {
		if s.server.authenticator.Authenticate(username, password) {
			s.authenticated = true
			s.server.logger.Info("authentication successful",
				logger.Str("username", username),
			)
			return nil
		}

		s.server.logger.Info("authentication failed",
			logger.Str("username", username),
		)
		return smtp.ErrAuthFailed
	}), nil
}

// Mail sets the sender address
func (s *smtpSession) Mail(from string, opts *smtp.MailOptions) error {
	// Check if authentication is required but not completed
	if s.server.authenticator != nil && !s.authenticated {
		return &smtp.SMTPError{
			Code:    530,
			Message: "Authentication required",
		}
	}

	s.from = from
	s.server.logger.Debug("MAIL FROM",
		logger.Str("from", from),
	)
	return nil
}

// Rcpt adds a recipient address
func (s *smtpSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	s.server.logger.Debug("RCPT TO",
		logger.Str("to", to),
	)
	return nil
}

// Data receives the email data
func (s *smtpSession) Data(r io.Reader) error {
	// Handle the message using the handler
	err := s.server.handler.Handle(s.server.ctx, s.from, s.to, r)
	if err != nil {
		// Check if it's an SMTPError
		if smtpErr, ok := err.(*SMTPError); ok {
			return &smtp.SMTPError{
				Code:    smtpErr.Code,
				Message: smtpErr.Message,
			}
		}
		// Generic error
		return &smtp.SMTPError{
			Code:    451,
			Message: "Temporary failure",
		}
	}

	return nil
}

// Reset resets the session state
func (s *smtpSession) Reset() {
	s.from = ""
	s.to = nil
}

// Logout closes the session
func (s *smtpSession) Logout() error {
	// Release semaphore
	<-s.server.semaphore
	s.server.wg.Done()
	return nil
}
