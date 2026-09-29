// Package gigachat — клиент GigaChat API (ADR-032). Используется сценариями
// core-service через порт ports.DraftAssistant: извлечение реквизитов документа
// из свободного текста и сопоставление описания бизнеса с кодами справочника.
//
// Особенности, заданные документацией Сбера:
//   - токен доступа выдаётся в обмен на ключ авторизации (Basic) запросом
//     POST /api/v2/oauth с обязательным заголовком RqUID и живёт 30 минут;
//   - генерация — POST /chat/completions с Bearer-токеном;
//   - домены Сбера требуют корневой сертификат НУЦ Минцифры; проверка TLS
//     никогда не отключается.
package gigachat

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"vovremya/services/core/internal/ports"
)

// Ошибки клиента.
var (
	ErrBudgetExceeded = errors.New("gigachat: исчерпан суточный бюджет токенов")
	ErrBadResponse    = errors.New("gigachat: ответ не соответствует схеме")
)

// ErrNoResult — модель ответила, но не по схеме или отказалась отвечать по содержанию
// (finish_reason = blacklist): текст или фото не подходят, сервис при этом исправен.
var ErrNoResult = fmt.Errorf("gigachat: модель не вернула реквизиты: %w", ports.ErrAssistantNoResult)

// finishBlacklist — ответ заменён ограничителем GigaChat: запрос не обрабатывается по содержанию.
const finishBlacklist = "blacklist"

// Config — параметры клиента.
type Config struct {
	AuthKey          string // Base64(Client ID:Client Secret)
	Scope            string // GIGACHAT_API_PERS | GIGACHAT_API_B2B | GIGACHAT_API_CORP
	Model            string
	BaseURL          string // https://gigachat.devices.sberbank.ru/api/v1
	OAuthURL         string // https://ngw.devices.sberbank.ru:9443/api/v2/oauth
	CAFile           string // бандл НУЦ Минцифры; пусто — только системные корни
	Timeout          time.Duration
	DailyTokenBudget int
	Now              func() time.Time
	OnUsage          func(operation string, tokens int)
	Log              *slog.Logger
}

// Client — клиент GigaChat API с кэшем токена и суточным бюджетом.
type Client struct {
	cfg  Config
	http *http.Client

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
	spentDay    string
	spentTokens int
}

// New создаёт клиент. Возвращает ошибку, если сертификат указан, но нечитаем.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.AuthKey) == "" {
		return nil, errors.New("gigachat: не задан ключ авторизации")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 8 * time.Second
	}
	if cfg.Scope == "" {
		cfg.Scope = "GIGACHAT_API_PERS"
	}
	if cfg.Model == "" {
		cfg.Model = "GigaChat-Pro"
	}
	pool, err := certPool(cfg.CAFile)
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: cfg.Timeout,
			Transport: &http.Transport{
				TLSClientConfig:     &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

// certPool собирает доверенные корни: системные плюс бандл НУЦ Минцифры.
func certPool(caFile string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if strings.TrimSpace(caFile) == "" {
		return pool, nil
	}
	pem, err := os.ReadFile(caFile)
	if errors.Is(err, os.ErrNotExist) {
		return pool, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gigachat: чтение %s: %w", caFile, err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("gigachat: файл %s не содержит сертификатов PEM", caFile)
	}
	return pool, nil
}

// accessToken возвращает действующий токен, обновляя его не позже чем за минуту
// до истечения (срок жизни токена — 30 минут).
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	token, expiry := c.token, c.tokenExpiry
	c.mu.Unlock()
	now := c.cfg.Now()
	if token != "" && now.Before(expiry.Add(-time.Minute)) {
		return token, nil
	}

	form := url.Values{"scope": {c.cfg.Scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.OAuthURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", newRqUID())
	req.Header.Set("Authorization", "Basic "+c.cfg.AuthKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gigachat: получение токена: статус %d", resp.StatusCode)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.AccessToken == "" {
		return "", ErrBadResponse
	}
	expires := now.Add(30 * time.Minute)
	if parsed.ExpiresAt > 0 {
		expires = time.UnixMilli(parsed.ExpiresAt).UTC()
	}
	c.mu.Lock()
	c.token, c.tokenExpiry = parsed.AccessToken, expires
	c.mu.Unlock()
	return parsed.AccessToken, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type   string         `json:"type"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	FunctionCall   string          `json:"function_call"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// complete выполняет запрос генерации и возвращает содержимое ответа модели.
// Одна повторная попытка выполняется при 429, 5xx и сетевых сбоях.
func (c *Client) complete(ctx context.Context, operation string, req chatRequest) (string, error) {
	if err := c.checkBudget(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		token, err := c.accessToken(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		content, usage, retryable, err := c.doComplete(ctx, token, payload)
		c.addUsage(operation, usage)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !retryable {
			return "", err
		}
	}
	return "", lastErr
}

func (c *Client) doComplete(ctx context.Context, token string, payload []byte) (string, int, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-ID", newRqUID())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		c.invalidateToken()
		return "", 0, true, fmt.Errorf("gigachat: генерация: статус %d", resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return "", 0, true, fmt.Errorf("gigachat: генерация: статус %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return "", 0, false, fmt.Errorf("gigachat: генерация: статус %d", resp.StatusCode)
	}
	content, usage, err := parseChatResponse(body)
	return content, usage, false, err
}

// parseChatResponse извлекает ответ модели. Сработавший ограничитель по содержанию
// (finish_reason = blacklist) — ErrNoResult: повтор того же запроса бесполезен.
func parseChatResponse(body []byte) (string, int, error) {
	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", 0, ErrBadResponse
	}
	if parsed.Choices[0].FinishReason == finishBlacklist {
		return "", parsed.Usage.TotalTokens, ErrNoResult
	}
	return parsed.Choices[0].Message.Content, parsed.Usage.TotalTokens, nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	c.token, c.tokenExpiry = "", time.Time{}
	c.mu.Unlock()
}

// checkBudget не даёт превысить суточный расход токенов.
func (c *Client) checkBudget() error {
	if c.cfg.DailyTokenBudget <= 0 {
		return nil
	}
	day := c.cfg.Now().UTC().Format("2006-01-02")
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.spentDay != day {
		c.spentDay, c.spentTokens = day, 0
	}
	if c.spentTokens >= c.cfg.DailyTokenBudget {
		return ErrBudgetExceeded
	}
	return nil
}

func (c *Client) addUsage(operation string, tokens int) {
	if tokens <= 0 {
		return
	}
	day := c.cfg.Now().UTC().Format("2006-01-02")
	c.mu.Lock()
	if c.spentDay != day {
		c.spentDay, c.spentTokens = day, 0
	}
	c.spentTokens += tokens
	c.mu.Unlock()
	if c.cfg.OnUsage != nil {
		c.cfg.OnUsage(operation, tokens)
	}
}

// SpentTokens возвращает израсходованные за текущие сутки токены (для проверок).
func (c *Client) SpentTokens() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.spentTokens
}

// newRqUID формирует идентификатор запроса в формате uuid4 без внешних библиотек.
func newRqUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		binary.BigEndian.PutUint64(buf[:8], uint64(time.Now().UnixNano()))
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
