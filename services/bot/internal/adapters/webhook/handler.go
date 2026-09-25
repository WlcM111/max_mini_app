package webhook

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"vovremya/internal/platform/logging"
	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/tracectx"
	"vovremya/services/bot/internal/app"
)

// Path — маршрут webhook (spec §7).
const Path = "/max/webhook"

// Service — сценарий обработки события.
type Service interface {
	Handle(ctx context.Context, in app.InboundUpdate) (app.WebhookResult, error)
}

// Handler — обработчик POST /max/webhook.
type Handler struct {
	svc      Service
	secret   []byte
	maxBody  int64
	timeout  time.Duration
	log      *slog.Logger
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewHandler создаёт обработчик webhook.
func NewHandler(svc Service, secret string, maxBody int64, timeout time.Duration,
	log *slog.Logger, reg *metrics.Registry) *Handler {
	h := &Handler{
		svc: svc, secret: []byte(secret), maxBody: maxBody, timeout: timeout, log: log,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_http_requests_total",
			Help: "Запросы HTTP по маршруту и коду ответа.",
		}, []string{"route", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vovremya_http_request_duration_seconds",
			Help:    "Длительность обработки запросов HTTP.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 10},
		}, []string{"route"}),
	}
	if reg != nil {
		reg.MustRegister(h.requests, h.duration)
	}
	return h
}

// Routes возвращает обработчик с единственным маршрутом webhook.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(Path, h)
	return mux
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	status := http.StatusOK
	defer func() {
		h.requests.WithLabelValues(Path, strconv.Itoa(status)).Inc()
		h.duration.WithLabelValues(Path).Observe(time.Since(started).Seconds())
	}()

	ids := tracectx.New()
	requestID := r.Header.Get("X-Request-Id")
	if requestID == "" {
		requestID = ids.SpanID
	}
	ctx := logging.WithOperation(logging.WithCorrelation(r.Context(), requestID, ids.TraceID, ids.SpanID), "POST "+Path)
	log := logging.From(ctx, h.log)

	if r.Method != http.MethodPost {
		status = http.StatusMethodNotAllowed
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, status, `{"error":"method_not_allowed"}`)
		return
	}
	// Подлинность: секрет сравнивается за постоянное время (F-46).
	given := []byte(r.Header.Get("X-Max-Bot-Api-Secret"))
	if len(h.secret) == 0 || subtle.ConstantTimeCompare(given, h.secret) != 1 {
		status = http.StatusUnauthorized
		log.Warn("webhook request rejected: bad secret")
		writeJSON(w, status, `{"error":"unauthorized"}`)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.maxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
			log.Warn("webhook body too large", slog.Int64("limit_bytes", h.maxBody))
			writeJSON(w, status, `{"error":"payload_too_large"}`)
			return
		}
		status = http.StatusBadRequest
		log.Warn("webhook body read failed", slog.Any("error", err))
		writeJSON(w, status, `{"error":"bad_request"}`)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	in := ParseUpdate(body)
	res, err := h.svc.Handle(ctx, in)
	if err != nil {
		// MAX повторит событие: отвечаем 503 (F-45).
		status = http.StatusServiceUnavailable
		log.Error("webhook processing failed", slog.Any("error", err), slog.String("update_type", in.RawType))
		writeJSON(w, status, `{"error":"unavailable"}`)
		return
	}
	log.Info("webhook update processed",
		slog.String("update_type", in.RawType),
		slog.Bool("duplicate", res.Duplicate),
		slog.String("outcome", string(res.Outcome)),
		slog.Bool("reply", res.Reply != 0))
	writeJSON(w, status, `{}`)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
