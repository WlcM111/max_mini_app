package maxapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"vovremya/internal/platform/metrics"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// Config — параметры клиента Bot API.
type Config struct {
	BaseURL        string
	Token          string
	Timeout        time.Duration
	ButtonKind     ButtonKind
	ExtraCAFile    string // сертификаты Минцифры (F-40)
	RequireExtraCA bool   // в режиме live отсутствие сертификатов — ошибка запуска
}

// Client — клиент MAX Bot API поверх net/http (ADR-008).
type Client struct {
	base       *url.URL
	token      string
	http       *http.Client
	buttonKind ButtonKind
	log        *slog.Logger
	requests   *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

var _ ports.MaxClient = (*Client)(nil)

// New создаёт клиента и пул доверия: системные корни плюс сертификаты из файла.
func New(cfg Config, log *slog.Logger, reg *metrics.Registry) (*Client, error) {
	base, err := url.Parse(strings.TrimSuffix(cfg.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("некорректный BOT_MAX_API_BASE_URL: %q", cfg.BaseURL)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if cfg.ExtraCAFile != "" {
		pem, err := os.ReadFile(cfg.ExtraCAFile)
		switch {
		case err != nil && cfg.RequireExtraCA:
			return nil, fmt.Errorf("read %s: %w", cfg.ExtraCAFile, err)
		case err != nil:
			log.Warn("extra CA file is not available", slog.String("file", cfg.ExtraCAFile), slog.Any("error", err))
		case !pool.AppendCertsFromPEM(pem):
			if cfg.RequireExtraCA {
				return nil, fmt.Errorf("файл %s не содержит сертификатов PEM", cfg.ExtraCAFile)
			}
			log.Warn("extra CA file has no certificates", slog.String("file", cfg.ExtraCAFile))
		}
	}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	c := &Client{
		base:       base,
		token:      cfg.Token,
		buttonKind: cfg.ButtonKind,
		log:        log,
		http: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
			// Перенаправления не выполняются: токен не должен уходить на другой хост.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vovremya_max_api_requests_total",
			Help: "Вызовы MAX Bot API по методу и коду ответа.",
		}, []string{"method", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vovremya_max_api_duration_seconds",
			Help:    "Длительность вызовов MAX Bot API.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method"}),
	}
	if reg != nil {
		reg.MustRegister(c.requests, c.duration)
	}
	return c, nil
}

// response — разобранный ответ MAX.
type response struct {
	status int
	header http.Header
	body   []byte
}

const maxResponseBytes = 1 << 20

func (c *Client) do(ctx context.Context, method, name, path string, query url.Values, payload any) (response, error) {
	started := time.Now()
	u := *c.base
	u.Path = c.base.Path + path
	u.RawQuery = query.Encode()

	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return response{}, fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return response{}, fmt.Errorf("build request: %w", err)
	}
	// Токен передаётся только заголовком Authorization (F-40, PLAT-06).
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		c.requests.WithLabelValues(name, "transport_error").Inc()
		c.duration.WithLabelValues(name).Observe(time.Since(started).Seconds())
		return response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	c.requests.WithLabelValues(name, strconv.Itoa(resp.StatusCode)).Inc()
	c.duration.WithLabelValues(name).Observe(time.Since(started).Seconds())
	if readErr != nil {
		return response{status: resp.StatusCode, header: resp.Header}, readErr
	}
	return response{status: resp.StatusCode, header: resp.Header, body: body}, nil
}

// SendMessage отправляет сообщение пользователю MAX.
func (c *Client) SendMessage(ctx context.Context, msg domain.Message, profile domain.Profile) (ports.SendResult, error) {
	body, err := renderSendBody(msg, profile, c.buttonKind)
	if err != nil {
		// Сформировать запрос невозможно: повтор не поможет.
		return ports.SendResult{}, &ports.SendError{
			Failure: domain.SendFailure{Code: domain.CodeMax4xx}, Err: err,
		}
	}
	query := url.Values{"user_id": {strconv.FormatInt(msg.RecipientMaxUserID, 10)}}
	resp, err := c.do(ctx, http.MethodPost, "send_message", "/messages", query, body)
	if err != nil {
		return ports.SendResult{}, &ports.SendError{Failure: classifyTransport(err), Err: err}
	}
	if resp.status/100 == 2 {
		var parsed sendMessageResult
		if err := json.Unmarshal(resp.body, &parsed); err != nil {
			// Сообщение принято; неизвестный формат ответа не отменяет доставку.
			c.log.Warn("max send response is not recognized", slog.Any("error", err))
		}
		return ports.SendResult{MessageID: parsed.Message.Body.Mid}, nil
	}
	return ports.SendResult{}, &ports.SendError{
		Failure:    classifyStatus(resp.status, resp.header),
		HTTPStatus: resp.status,
		Err:        fmt.Errorf("MAX ответил %d", resp.status),
	}
}

// classifyStatus переводит код ответа MAX в исход доставки (spec §8).
func classifyStatus(status int, header http.Header) domain.SendFailure {
	switch {
	case status == http.StatusUnauthorized:
		return domain.SendFailure{Code: domain.CodeMax401, Retryable: true}
	case status == http.StatusForbidden || status == http.StatusNotFound:
		return domain.SendFailure{Code: domain.CodeRecipientUnreachable, RecipientUnreachable: true}
	case status == http.StatusTooManyRequests:
		return domain.SendFailure{Code: domain.CodeMax429, Retryable: true, RetryAfter: retryAfter(header)}
	case status >= 500:
		return domain.SendFailure{Code: domain.CodeMax5xx, Retryable: true}
	case status >= 400:
		return domain.SendFailure{Code: domain.CodeMax4xx}
	default:
		// Неожиданный ответ (1xx, 3xx): повторяем как временную неисправность.
		return domain.SendFailure{Code: domain.CodeMax5xx, Retryable: true}
	}
}

// classifyTransport различает таймаут и прочие сетевые ошибки.
func classifyTransport(err error) domain.SendFailure {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return domain.SendFailure{Code: domain.CodeTimeout, Retryable: true}
	}
	return domain.SendFailure{Code: domain.CodeNetwork, Retryable: true}
}

// retryAfter разбирает заголовок Retry-After (секунды или дата).
func retryAfter(header http.Header) time.Duration {
	v := strings.TrimSpace(header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return clampRetryAfter(time.Duration(secs) * time.Second)
	}
	if t, err := http.ParseTime(v); err == nil {
		return clampRetryAfter(time.Until(t))
	}
	return 0
}

func clampRetryAfter(d time.Duration) time.Duration {
	switch {
	case d < 0:
		return 0
	case d > time.Hour:
		return time.Hour
	default:
		return d
	}
}

// GetMe возвращает профиль бота.
func (c *Client) GetMe(ctx context.Context) (domain.Profile, error) {
	resp, err := c.do(ctx, http.MethodGet, "get_me", "/me", nil, nil)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("GET /me: %w", err)
	}
	if resp.status/100 != 2 {
		return domain.Profile{}, fmt.Errorf("GET /me: MAX ответил %d", resp.status)
	}
	var parsed meResult
	if err := json.Unmarshal(resp.body, &parsed); err != nil {
		return domain.Profile{}, fmt.Errorf("GET /me: разбор ответа: %w", err)
	}
	return domain.Profile{UserID: parsed.UserID, Username: parsed.Username, DisplayName: parsed.Name}, nil
}

// ListSubscriptions возвращает URL активных подписок webhook.
func (c *Client) ListSubscriptions(ctx context.Context) ([]string, error) {
	resp, err := c.do(ctx, http.MethodGet, "get_subscriptions", "/subscriptions", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("GET /subscriptions: %w", err)
	}
	if resp.status/100 != 2 {
		return nil, fmt.Errorf("GET /subscriptions: MAX ответил %d", resp.status)
	}
	var parsed subscriptionList
	if err := json.Unmarshal(resp.body, &parsed); err != nil {
		return nil, fmt.Errorf("GET /subscriptions: разбор ответа: %w", err)
	}
	out := make([]string, 0, len(parsed.Subscriptions))
	for _, s := range parsed.Subscriptions {
		out = append(out, s.URL)
	}
	return out, nil
}

// Subscribe создаёт подписку webhook.
func (c *Client) Subscribe(ctx context.Context, webhookURL string, updateTypes []string, secret string) error {
	resp, err := c.do(ctx, http.MethodPost, "subscribe", "/subscriptions", nil,
		subscribeBody{URL: webhookURL, UpdateTypes: updateTypes, Secret: secret})
	if err != nil {
		return fmt.Errorf("POST /subscriptions: %w", err)
	}
	if resp.status/100 != 2 {
		return fmt.Errorf("POST /subscriptions: MAX ответил %d", resp.status)
	}
	var parsed simpleResult
	if err := json.Unmarshal(resp.body, &parsed); err != nil {
		return fmt.Errorf("POST /subscriptions: разбор ответа: %w", err)
	}
	if !parsed.Success {
		return fmt.Errorf("POST /subscriptions: подписка не создана: %s", parsed.Message)
	}
	return nil
}

// SetCommands задаёт команды меню бота.
func (c *Client) SetCommands(ctx context.Context, cmds []ports.BotCommand) error {
	payload := make([]botCommand, 0, len(cmds))
	for _, c := range cmds {
		payload = append(payload, botCommand{Name: c.Name, Description: c.Description})
	}
	resp, err := c.do(ctx, http.MethodPatch, "set_commands", "/me/commands", nil, commandsRequest{Commands: payload})
	if err != nil {
		return fmt.Errorf("PATCH /me/commands: %w", err)
	}
	if resp.status/100 != 2 {
		return fmt.Errorf("PATCH /me/commands: MAX ответил %d", resp.status)
	}
	return nil
}
