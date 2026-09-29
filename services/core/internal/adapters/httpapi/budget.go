package httpapi

import (
	"net/http"
	"strings"
	"time"

	"vovremya/services/core/internal/app"
)

// assistantBudgetMargin — запас сверх ожидания ассистента: чтение тела, проверка сессии, запись ответа.
const assistantBudgetMargin = 5 * time.Second

// requestBudget возвращает бюджет обработчика. Операции ассистента ждут внешний сервис
// (для фото — app.ImageTimeout), поэтому общий CORE_HANDLER_TIMEOUT их не обрывает.
func requestBudget(method, path string, base, assistant time.Duration) time.Duration {
	if method != http.MethodPost || assistant <= 0 {
		return base
	}
	var need time.Duration
	switch {
	case strings.HasSuffix(path, "/documents/draft-image"):
		need = app.ImageTimeout(assistant) + assistantBudgetMargin
	case strings.HasSuffix(path, "/documents/draft"), path == "/api/v1/profile-match":
		need = assistant + assistantBudgetMargin
	}
	if need > base {
		return need
	}
	return base
}
