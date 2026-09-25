// Package maxlaunch проверяет подпись данных запуска мини-приложения MAX
// (max-integration-spec §4). Обращений к сети нет: проверка локальная.
package maxlaunch

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// Verifier проверяет init_data производным ключом мини-приложения.
type Verifier struct {
	secret     []byte
	maxAge     time.Duration
	futureSkew time.Duration
}

var _ ports.LaunchVerifier = (*Verifier)(nil)

// New создаёт проверяющего по ключу CORE_MAX_WEBAPP_SECRET_HEX (32 байта).
func New(secretHex string, maxAge, futureSkew time.Duration) (*Verifier, error) {
	secret, err := hex.DecodeString(strings.TrimSpace(secretHex))
	if err != nil || len(secret) != 32 {
		return nil, fmt.Errorf("CORE_MAX_WEBAPP_SECRET_HEX: ожидается 64 шестнадцатеричных символа")
	}
	return &Verifier{secret: secret, maxAge: maxAge, futureSkew: futureSkew}, nil
}

type launchUser struct {
	ID           int64   `json:"id"`
	FirstName    string  `json:"first_name"`
	LastName     *string `json:"last_name"`
	Username     *string `json:"username"`
	LanguageCode *string `json:"language_code"`
}

// Verify проверяет подпись и возвращает данные пользователя.
func (v *Verifier) Verify(initData string, now time.Time) (ports.LaunchIdentity, error) {
	if initData == "" || utf8.RuneCountInString(initData) > 4096 {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}
	pairs := make(map[string]string, 8)
	var hashValue string
	for _, part := range strings.Split(initData, "&") {
		key, rawValue, found := strings.Cut(part, "=")
		if !found || key == "" {
			return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
		}
		if key == "hash" {
			if hashValue != "" {
				return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
			}
			hashValue = rawValue
			continue
		}
		if _, dup := pairs[key]; dup {
			return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
		}
		value, err := url.PathUnescape(rawValue)
		if err != nil {
			return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
		}
		pairs[key] = value
	}
	if len(hashValue) != 64 {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}
	given, err := hex.DecodeString(hashValue)
	if err != nil {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}

	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+pairs[k])
	}
	mac := hmac.New(sha256.New, v.secret)
	_, _ = mac.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal(mac.Sum(nil), given) {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}

	authRaw, ok := pairs["auth_date"]
	if !ok {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}
	var authSeconds int64
	if _, err := fmt.Sscanf(authRaw, "%d", &authSeconds); err != nil || authSeconds <= 0 {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}
	authDate := time.Unix(authSeconds, 0).UTC()
	if now.Sub(authDate) > v.maxAge {
		return ports.LaunchIdentity{}, domain.ErrLaunchExpired
	}
	if authDate.Sub(now) > v.futureSkew {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}

	userRaw, ok := pairs["user"]
	if !ok {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}
	var user launchUser
	if err := json.Unmarshal([]byte(userRaw), &user); err != nil || user.ID <= 0 || user.FirstName == "" {
		return ports.LaunchIdentity{}, domain.ErrLaunchInvalid
	}
	identity := ports.LaunchIdentity{
		MaxUserID:  user.ID,
		FirstName:  truncate(user.FirstName, 128),
		AuthDate:   authDate,
		QueryID:    truncate(pairs["query_id"], 64),
		StartParam: pairs["start_param"],
	}
	if user.LastName != nil {
		identity.LastName = truncate(*user.LastName, 128)
	}
	if user.Username != nil {
		identity.Username = truncate(*user.Username, 64)
	}
	if user.LanguageCode != nil {
		identity.LanguageCode = truncate(*user.LanguageCode, 16)
	}
	return identity, nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
