// Package httpapi — публичный HTTP API мини-приложения (openapi.yaml 1.3.0).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"vovremya/services/core/internal/domain"
)

// Problem — тело ошибки RFC 9457.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id"`
	Errors    []fieldError `json:"errors,omitempty"`
}

type fieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type problemSpec struct {
	status     int
	code       string
	title      string
	retryAfter int
}

// problemFor сопоставляет доменную ошибку коду ответа (handoff core-service §14).
func problemFor(err error) problemSpec {
	switch {
	case errors.Is(err, domain.ErrValidation):
		return problemSpec{http.StatusBadRequest, "VALIDATION_FAILED", "Некорректный запрос", 0}
	case errors.Is(err, domain.ErrUnauthenticated):
		return problemSpec{http.StatusUnauthorized, "UNAUTHENTICATED", "Нужна действующая сессия", 0}
	case errors.Is(err, domain.ErrLaunchInvalid):
		return problemSpec{http.StatusUnauthorized, "LAUNCH_DATA_INVALID", "Данные запуска недействительны", 0}
	case errors.Is(err, domain.ErrLaunchExpired):
		return problemSpec{http.StatusUnauthorized, "LAUNCH_DATA_EXPIRED", "Данные запуска просрочены", 0}
	case errors.Is(err, domain.ErrForbidden):
		return problemSpec{http.StatusForbidden, "FORBIDDEN", "Недостаточно прав", 0}
	case errors.Is(err, domain.ErrNotFound):
		return problemSpec{http.StatusNotFound, "NOT_FOUND", "Объект не найден", 0}
	case errors.Is(err, domain.ErrConflictVersion):
		return problemSpec{http.StatusConflict, "CONFLICT_VERSION", "Объект изменён другим пользователем", 0}
	case errors.Is(err, domain.ErrConflictIDReused):
		return problemSpec{http.StatusConflict, "CONFLICT_ID_REUSED", "Идентификатор уже использован", 0}
	case errors.Is(err, domain.ErrQuotaExceeded):
		return problemSpec{http.StatusConflict, "QUOTA_EXCEEDED", "Превышено ограничение", 0}
	case errors.Is(err, domain.ErrInviteExpired):
		return problemSpec{http.StatusConflict, "INVITE_EXPIRED", "Срок приглашения истёк", 0}
	case errors.Is(err, domain.ErrAlreadyMember):
		return problemSpec{http.StatusConflict, "ALREADY_MEMBER", "Вы уже участник организации", 0}
	case errors.Is(err, domain.ErrInviteInvalid):
		return problemSpec{http.StatusNotFound, "INVITE_INVALID", "Приглашение недействительно", 0}
	case errors.Is(err, domain.ErrLinkGone):
		return problemSpec{http.StatusGone, "LINK_GONE", "Ссылка истекла или исчерпана", 0}
	case errors.Is(err, domain.ErrRateLimited):
		return problemSpec{http.StatusTooManyRequests, "RATE_LIMITED", "Слишком много запросов", 60}
	case errors.Is(err, domain.ErrDependencyUnavailable):
		return problemSpec{http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Сервис временно недоступен", 5}
	case errors.Is(err, domain.ErrDocumentNotRecognized):
		return problemSpec{http.StatusUnprocessableEntity, "DOCUMENT_NOT_RECOGNIZED", "Реквизиты документа не распознаны", 0}
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return problemSpec{http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Сервис временно недоступен", 1}
	default:
		return problemSpec{http.StatusInternalServerError, "INTERNAL", "Внутренняя ошибка", 0}
	}
}

// fail отправляет ошибку клиенту; подробности внутренних ошибок остаются в логе.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	spec := problemFor(err)
	if spec.code == "INVITE_INVALID" && r.Method == http.MethodDelete {
		// Принятое приглашение отозвать нельзя (OpenAPI 1.1.0: 409 у revokeInvite).
		spec.status = http.StatusConflict
	}
	problem := Problem{
		Type:      "urn:vovremya:problem:" + strings.ToLower(strings.ReplaceAll(spec.code, "_", "-")),
		Title:     spec.title,
		Status:    spec.status,
		Code:      spec.code,
		RequestID: requestIDFrom(r.Context()),
	}
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		for _, f := range ve.Fields {
			problem.Errors = append(problem.Errors, fieldError{Field: f.Field, Code: f.Code, Message: f.Message})
		}
		problem.Detail = "Проверьте значения полей запроса"
	}
	if spec.status >= 500 {
		s.log.Error("request failed",
			slog.String("request_id", problem.RequestID),
			slog.String("path", r.URL.Path),
			slog.Any("error", err))
		s.app.Metrics.AppErrors.WithLabelValues(spec.code).Inc()
	} else if spec.status >= 400 {
		s.app.Metrics.AppErrors.WithLabelValues(spec.code).Inc()
	}
	if spec.retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(spec.retryAfter))
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(spec.status)
	_ = json.NewEncoder(w).Encode(problem)
}

// writeJSON отправляет успешный ответ.
func (s *Server) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
