package app

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"unicode/utf8"

	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// imageTimeoutFactor — во сколько раз распознавание фотографии дольше разбора текста:
// файл загружается в хранилище GigaChat, затем модель читает изображение.
const imageTimeoutFactor = 3

// ImageTimeout — предел ожидания распознавания фотографии (ADR-033).
func ImageTimeout(textTimeout time.Duration) time.Duration { return imageTimeoutFactor * textTimeout }

// AssistantEnabled сообщает, подключён ли языковой ассистент (FR-21, FR-22).
func (a *App) AssistantEnabled() bool { return a.Assistant != nil }

// assistantCatalog собирает закрытые списки кодов для ассистента.
func (a *App) assistantCatalog() ports.AssistantCatalog {
	catalog := a.CatalogSnapshot()
	out := ports.AssistantCatalog{}
	if catalog == nil {
		return out
	}
	for _, t := range catalog.DocumentTypes {
		out.DocumentTypes = append(out.DocumentTypes, ports.AssistantOption{
			Code: t.Code, Title: t.Title, Hint: t.Description,
		})
	}
	for _, c := range catalog.Categories {
		out.Categories = append(out.Categories, ports.AssistantOption{Code: c.Code, Title: c.Title})
	}
	for _, f := range catalog.Features {
		out.Features = append(out.Features, ports.AssistantOption{Code: f.Code, Title: f.Question, Hint: f.Hint})
	}
	return out
}

// DraftDocument распознаёт реквизиты документа в свободном тексте (FR-21).
// Ничего не сохраняет: результат подставляется в форму, которую подтверждает
// пользователь. Даты с явными маркерами («до», «от», «выдана») дополнительно
// извлекаются из текста без модели. Сбой ассистента — ErrDependencyUnavailable,
// отсутствие реквизитов в тексте — ErrDocumentNotRecognized.
func (a *App) DraftDocument(ctx context.Context, actor Actor, orgPublicID, text string) (domain.AssistantDraft, error) {
	if _, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleEditor); err != nil {
		return domain.AssistantDraft{}, err
	}
	clean, err := domain.ValidateAssistantText("text", text, a.Settings.AssistantMaxInputChars)
	if err != nil {
		return domain.AssistantDraft{}, err
	}
	if a.Assistant == nil {
		return domain.AssistantDraft{}, domain.ErrDependencyUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, a.Settings.AssistantTimeout)
	defer cancel()
	raw, err := a.Assistant.DraftDocument(callCtx, clean, a.assistantCatalog())
	if err != nil && !errors.Is(err, ports.ErrAssistantNoResult) {
		a.assistantFailed("draft_document", clean, err)
		return domain.AssistantDraft{}, domain.ErrDependencyUnavailable
	}
	// При ErrAssistantNoResult черновик модели пуст: остаются даты с маркерами из текста.
	now := a.Clock.Now()
	draft := domain.SanitizeAssistantDraft(a.CatalogSnapshot(), raw.Title, raw.Number, raw.Issuer,
		raw.ValidFrom, raw.ValidUntil, raw.DocumentTypeCode, raw.Confidence, now)
	draft = domain.MergeTextDates(draft, domain.ExtractTextDates(clean, now))
	if draft.Empty() {
		a.assistantNoResult("draft_document", clean, err)
		return domain.AssistantDraft{}, domain.ErrDocumentNotRecognized
	}
	a.Metrics.Assistant.WithLabelValues("draft_document", "ok").Inc()
	return draft, nil
}

// ImageDrafter — необязательная возможность ассистента: чтение реквизитов с фотографии.
type ImageDrafter interface {
	DraftDocumentFromImage(ctx context.Context, image []byte, mimeType string, catalog ports.AssistantCatalog) (ports.DocumentDraft, error)
}

// MaxDraftImageBytes — предел размера фотографии документа.
const MaxDraftImageBytes = 5 << 20

// DraftDocumentFromImage распознаёт реквизиты по фотографии документа (FR-23); изображение
// не сохраняется. Если на фото нет реквизитов или модель отказалась его читать —
// ErrDocumentNotRecognized, при сбое сервиса — ErrDependencyUnavailable.
func (a *App) DraftDocumentFromImage(ctx context.Context, actor Actor, orgPublicID string, image []byte,
	mimeType string) (domain.AssistantDraft, error) {
	if _, _, err := a.authorize(ctx, actor, orgPublicID, domain.RoleEditor); err != nil {
		return domain.AssistantDraft{}, err
	}
	if len(image) == 0 {
		return domain.AssistantDraft{}, domain.ValidationFor("image", domain.CodeRequired, "нужна фотография документа")
	}
	if len(image) > MaxDraftImageBytes {
		return domain.AssistantDraft{}, domain.ValidationFor("image", domain.CodeTooLong, "фотография больше 5 МБ")
	}
	jpeg := mimeType == "image/jpeg" && len(image) > 3 && image[0] == 0xFF && image[1] == 0xD8
	png := mimeType == "image/png" && len(image) > 8 && string(image[1:4]) == "PNG"
	if !jpeg && !png {
		return domain.AssistantDraft{}, domain.ValidationFor("image", domain.CodeInvalidFormat, "поддерживаются JPEG и PNG")
	}
	drafter, ok := a.Assistant.(ImageDrafter)
	if a.Assistant == nil || !ok {
		return domain.AssistantDraft{}, domain.ErrDependencyUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, ImageTimeout(a.Settings.AssistantTimeout))
	defer cancel()
	raw, err := drafter.DraftDocumentFromImage(callCtx, image, mimeType, a.assistantCatalog())
	if err != nil {
		if errors.Is(err, ports.ErrAssistantNoResult) {
			a.assistantNoResult("draft_document_image", "", err)
			return domain.AssistantDraft{}, domain.ErrDocumentNotRecognized
		}
		a.assistantFailed("draft_document_image", "", err)
		return domain.AssistantDraft{}, domain.ErrDependencyUnavailable
	}
	draft := domain.SanitizeAssistantDraft(a.CatalogSnapshot(), raw.Title, raw.Number, raw.Issuer,
		raw.ValidFrom, raw.ValidUntil, raw.DocumentTypeCode, raw.Confidence, a.Clock.Now())
	if draft.Empty() {
		a.assistantNoResult("draft_document_image", "", nil)
		return domain.AssistantDraft{}, domain.ErrDocumentNotRecognized
	}
	a.Metrics.Assistant.WithLabelValues("draft_document_image", "ok").Inc()
	return draft, nil
}

// MatchProfile сопоставляет свободное описание бизнеса с кодами справочника (FR-22).
// Организация ещё не создана, поэтому нужна только действующая сессия. Если модель
// не подобрала коды, возвращается пустой профиль: форма заполняется вручную.
func (a *App) MatchProfile(ctx context.Context, _ Actor, description string) (domain.AssistantProfile, error) {
	clean, err := domain.ValidateAssistantText("description", description, a.Settings.AssistantMaxInputChars)
	if err != nil {
		return domain.AssistantProfile{}, err
	}
	if a.Assistant == nil {
		return domain.AssistantProfile{}, domain.ErrDependencyUnavailable
	}
	callCtx, cancel := context.WithTimeout(ctx, a.Settings.AssistantTimeout)
	defer cancel()
	raw, err := a.Assistant.MatchProfile(callCtx, clean, a.assistantCatalog())
	if err != nil && !errors.Is(err, ports.ErrAssistantNoResult) {
		a.assistantFailed("match_profile", clean, err)
		return domain.AssistantProfile{}, domain.ErrDependencyUnavailable
	}
	profile := domain.SanitizeAssistantProfile(a.CatalogSnapshot(), raw.BusinessCategoryCode,
		raw.FeatureCodes, raw.Confidence)
	if err != nil {
		a.assistantNoResult("match_profile", clean, err)
		return profile, nil
	}
	a.Metrics.Assistant.WithLabelValues("match_profile", "ok").Inc()
	return profile, nil
}

// assistantFailed пишет отказ ассистента: в журнал попадает длина текста и
// причина, но никогда не сам текст пользователя (ADR-032, раздел приватности).
func (a *App) assistantFailed(operation, text string, err error) {
	result := "error"
	if errors.Is(err, context.DeadlineExceeded) {
		result = "timeout"
	}
	a.Metrics.Assistant.WithLabelValues(operation, result).Inc()
	a.Metrics.AppErrors.WithLabelValues("assistant_" + result).Inc()
	a.Log.Warn("ассистент недоступен",
		slog.String("operation", operation),
		slog.Int("input_chars", utf8.RuneCountInString(text)),
		slog.String("result", result),
		slog.Any("error", err))
}

// assistantNoResult учитывает ответ без реквизитов: входные данные не подошли,
// сервис исправен. Содержимое текста в журнал не пишется.
func (a *App) assistantNoResult(operation, text string, cause error) {
	a.Metrics.Assistant.WithLabelValues(operation, "no_result").Inc()
	attrs := []any{slog.String("operation", operation), slog.Int("input_chars", utf8.RuneCountInString(text))}
	if cause != nil {
		attrs = append(attrs, slog.Any("reason", cause))
	}
	a.Log.Info("ассистент не нашёл реквизитов", attrs...)
}
