package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ServerTestSuite struct {
	suite.Suite
	server *Server
	db     *Database
	logger *zap.Logger
}

func (s *ServerTestSuite) SetupTest() {
	// Create test logger
	s.logger = zap.NewNop()

	// Create test database
	dbPath := ":memory:"
	var err error
	s.db, err = NewDatabase(dbPath, s.logger)
	s.Require().NoError(err)

	// Create server
	s.server = NewServer("test-secret", s.db, s.logger)
}

func (s *ServerTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *ServerTestSuite) TestHandleUserliEvent_InvalidBody() {
	req := httptest.NewRequest("POST", "/userli", bytes.NewBuffer([]byte("invalid")))
	w := httptest.NewRecorder()

	s.server.handleUserliEvent(w, req)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *ServerTestSuite) TestHandleUserliEvent_UnknownEventType() {
	event := UserEvent{
		Type: "unknown.event",
	}
	jsonData, err := json.Marshal(event)
	s.NoError(err)

	req := httptest.NewRequest("POST", "/userli", bytes.NewBuffer(jsonData))
	w := httptest.NewRecorder()

	s.server.handleUserliEvent(w, req)
	s.Equal(http.StatusBadRequest, w.Code)
}

func (s *ServerTestSuite) TestHandleUserliEvent_UserDeleted() {
	event := UserEvent{
		Type: EventTypeUserDeleted,
	}
	event.Data.Email = "test@example.com"
	jsonData, err := json.Marshal(event)
	s.NoError(err)

	req := httptest.NewRequest("POST", "/userli", bytes.NewBuffer(jsonData))
	w := httptest.NewRecorder()

	s.server.handleUserliEvent(w, req)
	s.Equal(http.StatusOK, w.Code)

	// Verify mailbox was added to database
	mailboxes, err := s.db.GetDueMailboxes(0)
	s.NoError(err)
	s.Len(mailboxes, 1)
	s.Equal("test@example.com", mailboxes[0].Email)
}

func (s *ServerTestSuite) TestAuthMiddleware_ValidSignature() {
	payload := []byte(`{"type":"user.deleted","data":{"email":"test@example.com"}}`)
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/userli", bytes.NewBuffer(payload))
	req.Header.Set("X-Webhook-Signature", signature)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	s.server.AuthMiddleware(handler).ServeHTTP(rr, req)

	s.Equal(http.StatusOK, rr.Code)
}

func (s *ServerTestSuite) TestAuthMiddleware_InvalidSignature() {
	payload := []byte(`{"type":"user.deleted","data":{"email":"test@example.com"}}`)

	req := httptest.NewRequest("POST", "/userli", bytes.NewBuffer(payload))
	req.Header.Set("X-Webhook-Signature", "invalid-signature")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	s.server.AuthMiddleware(handler).ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func (s *ServerTestSuite) TestAuthMiddleware_MissingSignature() {
	payload := []byte(`{"type":"user.deleted","data":{"email":"test@example.com"}}`)

	req := httptest.NewRequest("POST", "/userli", bytes.NewBuffer(payload))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	s.server.AuthMiddleware(handler).ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}
