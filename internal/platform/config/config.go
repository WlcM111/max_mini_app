// Package config загружает конфигурацию сервисов из переменных окружения.
// Секрет может быть задан файлом через <ИМЯ>_FILE; в APP_ENV=prod запрещены
// значения с префиксом devonly и известные dev-ключи (ADR-015).
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// InsecurePrefix — префикс значений, допустимых только в локальном режиме.
const InsecurePrefix = "devonly"

// devWebAppSecret — производный ключ проверки initData от dev-токена бота.
const devWebAppSecret = "e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663"

// ErrInsecureSecret возвращается, когда в prod обнаружено небезопасное значение.
var ErrInsecureSecret = errors.New("insecure default secret")

// ExpandFileRefs заменяет значение переменной X содержимым файла из X_FILE.
func ExpandFileRefs(names ...string) error {
	for _, name := range names {
		path := os.Getenv(name + "_FILE")
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s_FILE: %w", name, err)
		}
		if err := os.Setenv(name, strings.TrimSpace(string(data))); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}
	return nil
}

// VerifySecrets проверяет, что в prod не используются dev-значения.
func VerifySecrets(appEnv string, values map[string]string) error {
	if appEnv != "prod" {
		return nil
	}
	for name, value := range values {
		if value == "" {
			continue
		}
		if strings.Contains(strings.ToLower(value), InsecurePrefix) || value == devWebAppSecret {
			return fmt.Errorf("%w: %s", ErrInsecureSecret, name)
		}
	}
	return nil
}
