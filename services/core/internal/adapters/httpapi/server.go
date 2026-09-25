package httpapi

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"vovremya/services/core/internal/app"
	"vovremya/services/core/internal/config"
	"vovremya/services/core/internal/domain"
)

type ctxKey int

const (
	ctxRequestID ctxKey = iota
	ctxActor
)

// Server — публичный HTTP API core-service.
type Server struct {
	app *app.App
	cfg config.Config
	log *slog.Logger

	inflight       chan struct{}
	accountLimiter *keyedLimiter
	sessionByUser  *windowLimiter
	sessionByIP    *windowLimiter
	inviteLimiter  *windowLimiter
	eventsLimiter  *windowLimiter
	trusted        []*net.IPNet
}

// New создаёт HTTP-адаптер.
func New(a *app.App, cfg config.Config, log *slog.Logger) *Server {
	s := &Server{
		app:            a,
		cfg:            cfg,
		log:            log,
		inflight:       make(chan struct{}, cfg.HTTPMaxInflight),
		accountLimiter: newKeyedLimiter(cfg.RateAccountRPS, cfg.RateAccountBurst),
		sessionByUser:  newWindowLimiter(cfg.RateSessionPerUser),
		sessionByIP:    newWindowLimiter(cfg.RateSessionPerIP),
		inviteLimiter:  newWindowLimiter(cfg.RateInvitePerMinute),
		eventsLimiter:  newWindowLimiter(cfg.RateEventsPerMinute),
	}
	for _, cidr := range cfg.TrustedProxyCIDRs {
		if _, network, err := net.ParseCIDR(strings.TrimSpace(cidr)); err == nil {
			s.trusted = append(s.trusted, network)
		}
	}
	return s
}

// Handler возвращает маршрутизатор с общими middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.routes(mux)
	return s.withRequestID(s.withInflight(s.withTimeout(mux)))
}

// withRequestID присваивает запросу идентификатор корреляции.
func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" || len(id) > 64 {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxRequestID, id)))
	})
}

// withInflight ограничивает число одновременно обрабатываемых запросов.
func (s *Server) withInflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.inflight <- struct{}{}:
			defer func() { <-s.inflight }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"type":"urn:vovremya:problem:overloaded","title":"Сервис перегружен",` +
				`"status":503,"code":"OVERLOADED","request_id":"` + requestIDFrom(r.Context()) + `"}`))
		}
	})
}

// withTimeout задаёт бюджет обработчика.
func (s *Server) withTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HandlerTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticated проверяет Bearer-токен и кладёт пользователя в контекст.
func (s *Server) authenticated(next func(http.ResponseWriter, *http.Request, app.Actor)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			s.fail(w, r, domain.ErrUnauthenticated)
			return
		}
		actor, err := s.app.Authenticate(r.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if !s.accountLimiter.Allow(actor.Account.PublicID, time.Now()) {
			s.fail(w, r, domain.ErrRateLimited)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxActor, actor)), actor)
	}
}

// clientIP определяет адрес клиента с учётом доверенных прокси.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	forwarded := r.Header.Get("X-Forwarded-For")
	if ip != nil && forwarded != "" {
		for _, network := range s.trusted {
			if network.Contains(ip) {
				parts := strings.Split(forwarded, ",")
				return strings.TrimSpace(parts[0])
			}
		}
	}
	return host
}

func requestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}
