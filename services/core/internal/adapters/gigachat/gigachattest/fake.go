// Package gigachattest — управляемый двойник GigaChat API для тестов.
// Настоящих обращений к сервису Сбера в тестах не выполняется.
package gigachattest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Server — двойник: отвечает на POST /api/v2/oauth и POST /api/v1/chat/completions.
type Server struct {
	*httptest.Server

	mu          sync.Mutex
	oauthCalls  int
	chatCalls   int
	content     string
	status      int
	failFirst   int
	delay       time.Duration
	tokenTTL    time.Duration
	lastRqUID   string
	lastRequest map[string]any
	lastAuth    string
	totalTokens int
}

// New поднимает двойник с ответом по умолчанию.
func New(t *testing.T) *Server {
	t.Helper()
	s := &Server{content: `{"title":"","confidence":0}`, status: http.StatusOK,
		tokenTTL: 30 * time.Minute, totalTokens: 100}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v2/oauth", s.handleOAuth)
	mux.HandleFunc("POST /api/v1/chat/completions", s.handleChat)
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Server.Close)
	return s
}

// OAuthURL — адрес выдачи токена для конфигурации клиента.
func (s *Server) OAuthURL() string { return s.Server.URL + "/api/v2/oauth" }

// BaseURL — базовый адрес API для конфигурации клиента.
func (s *Server) BaseURL() string { return s.Server.URL + "/api/v1" }

// SetContent задаёт содержимое ответа модели.
func (s *Server) SetContent(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.content = content
}

// SetStatus задаёт статус ответа генерации.
func (s *Server) SetStatus(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

// FailFirst заставляет первые n запросов генерации отвечать кодом 500.
func (s *Server) FailFirst(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failFirst = n
}

// SetDelay добавляет задержку ответа генерации.
func (s *Server) SetDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delay = d
}

// SetTokenTTL задаёт срок жизни выдаваемого токена.
func (s *Server) SetTokenTTL(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokenTTL = d
}

// SetTotalTokens задаёт расход токенов в ответе генерации.
func (s *Server) SetTotalTokens(v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalTokens = v
}

// OAuthCalls — число обращений за токеном.
func (s *Server) OAuthCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oauthCalls
}

// ChatCalls — число обращений к генерации.
func (s *Server) ChatCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chatCalls
}

// LastRqUID — заголовок RqUID последнего запроса токена.
func (s *Server) LastRqUID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRqUID
}

// LastAuthorization — заголовок Authorization последнего запроса токена.
func (s *Server) LastAuthorization() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAuth
}

// LastRequest — разобранное тело последнего запроса генерации.
func (s *Server) LastRequest() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRequest
}

func (s *Server) handleOAuth(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.oauthCalls++
	s.lastRqUID = r.Header.Get("RqUID")
	s.lastAuth = r.Header.Get("Authorization")
	ttl := s.tokenTTL
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "test-access-token",
		"expires_at":   time.Now().Add(ttl).UnixMilli(),
	})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	body := map[string]any{}
	_ = json.Unmarshal(raw, &body)

	s.mu.Lock()
	s.chatCalls++
	s.lastRequest = body
	delay, status, content, tokens := s.delay, s.status, s.content, s.totalTokens
	if s.failFirst > 0 {
		s.failFirst--
		status = http.StatusInternalServerError
	}
	s.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []any{map[string]any{
			"message":       map[string]any{"role": "assistant", "content": content},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"total_tokens": tokens},
	})
}
