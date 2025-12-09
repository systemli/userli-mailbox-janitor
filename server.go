package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Server handles HTTP requests and webhook events
type Server struct {
	router        *chi.Mux
	webhookSecret string
	db            *Database
}

// NewServer creates a new HTTP server instance
func NewServer(webhookSecret string, db *Database) *Server {
	return &Server{
		router:        chi.NewRouter(),
		webhookSecret: webhookSecret,
		db:            db,
	}
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	s.RegisterRoutes()
	logger.Info("Starting HTTP server", zap.String("address", addr))
	return http.ListenAndServe(addr, s.router)
}

// RegisterRoutes registers all HTTP routes
func (s *Server) RegisterRoutes() {
	s.router.Get("/health", s.handleHealth)
	s.router.With(s.AuthMiddleware).Post("/userli", s.handleUserliEvent)
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleUserliEvent processes incoming webhook events from userli
func (s *Server) handleUserliEvent(w http.ResponseWriter, r *http.Request) {
	logger.Info("Userli event received")

	var event UserEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		logger.Error("Failed to decode event", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	switch event.Type {
	case EventTypeUserDeleted:
		s.handleUserDeleted(event)
	default:
		logger.Warn("Unknown event type received", zap.String("type", event.Type))
		http.Error(w, "Unknown event type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleUserDeleted processes user deletion events
func (s *Server) handleUserDeleted(event UserEvent) {
	email := event.Data.Email
	logger.Info("User deleted event received", zap.String("email", email))

	// Validate email before adding to database (defense in depth)
	if err := validateEmail(email); err != nil {
		logger.Error("Invalid email address rejected",
			zap.String("email", email),
			zap.Error(err))
		return
	}

	if err := s.db.AddMailbox(email); err != nil {
		logger.Error("Failed to add mailbox to database",
			zap.String("email", email),
			zap.Error(err))
		return
	}

	logger.Info("Mailbox added to purge queue", zap.String("email", email))
}

// AuthMiddleware verifies webhook signatures using HMAC SHA256
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature := r.Header.Get("X-Webhook-Signature")
		if signature == "" {
			logger.Warn("Missing webhook signature")
			http.Error(w, "Missing signature header", http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("Failed to read request body", zap.Error(err))
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// Restore body for next handler
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Compute expected signature
		mac := hmac.New(sha256.New, []byte(s.webhookSecret))
		mac.Write(body)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Compare signatures
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			logger.Warn("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
