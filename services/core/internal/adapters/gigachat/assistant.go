package gigachat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"vovremya/services/core/internal/ports"
)

const (
	// Ключи ответа перечислены в самих инструкциях: разбор не зависит от того,
	// учитывает ли модель response_format.
	draftSystemPrompt = "Ты извлекаешь реквизиты документа из текста пользователя. " +
		"Отвечай только объектом JSON с ключами title, number, issuer, valid_from, valid_until, " +
		"document_type_code, confidence — без пояснений и обрамления. Бери значения исключительно из текста: " +
		"ничего не додумывай и не дополняй по смыслу. Если значения нет — верни пустую строку. " +
		"Даты приводи к формату YYYY-MM-DD. valid_from — дата начала действия; если отдельной даты начала нет, " +
		"это дата выдачи или заключения документа («от 12.03.2022», «выдана 14.03.2024»). " +
		"valid_until — дата окончания срока («до», «по», «действует до», «истекает»); не вычисляй её " +
		"и не подставляй вместо неё дату выдачи. Поле document_type_code выбирай только из " +
		"перечисленных кодов и только если документ явно относится к этому типу, иначе верни пустую строку. " +
		"В поле confidence верни число от 0 до 1 — насколько уверенно реквизиты найдены в тексте."

	profileSystemPrompt = "Ты сопоставляешь описание бизнеса с кодами справочника. " +
		"Отвечай только объектом JSON с ключами business_category_code, feature_codes, confidence — " +
		"без пояснений и обрамления. Выбирай коды исключительно из перечисленных списков. " +
		"В business_category_code верни один код вида деятельности или пустую строку, если ни один не подходит. " +
		"В feature_codes верни массив кодов признаков, которые прямо следуют из описания; не добавляй признаки по догадке. " +
		"В поле confidence верни число от 0 до 1."

	maxCompletionTokens = 512
)

// DraftDocument извлекает реквизиты документа из свободного текста (FR-21).
func (c *Client) DraftDocument(ctx context.Context, text string, catalog ports.AssistantCatalog) (ports.DocumentDraft, error) {
	typeCodes := codesOf(catalog.DocumentTypes)
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"title", "confidence"},
		"properties": map[string]any{
			"title":      map[string]any{"type": "string", "description": "название документа, как в тексте"},
			"number":     map[string]any{"type": "string", "description": "номер документа или пустая строка"},
			"issuer":     map[string]any{"type": "string", "description": "кем выдан или пустая строка"},
			"valid_from": map[string]any{"type": "string", "description": "дата начала действия, YYYY-MM-DD или пустая строка"},
			"valid_until": map[string]any{"type": "string",
				"description": "дата окончания действия, YYYY-MM-DD или пустая строка"},
			"document_type_code": map[string]any{"type": "string", "enum": append(typeCodes, ""),
				"description": "код типа документа из справочника или пустая строка"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
	}
	user := "Типы документов справочника:\n" + optionsList(catalog.DocumentTypes) + "\n\nТекст пользователя:\n" + text

	content, err := c.complete(ctx, "draft_document", chatRequest{
		Model:          c.cfg.Model,
		Temperature:    0,
		FunctionCall:   "none",
		MaxTokens:      maxCompletionTokens,
		Messages:       []chatMessage{{Role: "system", Content: draftSystemPrompt}, {Role: "user", Content: user}},
		ResponseFormat: &responseFormat{Type: "json_schema", Schema: schema, Strict: true},
	})
	if err != nil {
		return ports.DocumentDraft{}, err
	}
	var parsed struct {
		Title            string  `json:"title"`
		Number           string  `json:"number"`
		Issuer           string  `json:"issuer"`
		ValidFrom        string  `json:"valid_from"`
		ValidUntil       string  `json:"valid_until"`
		DocumentTypeCode string  `json:"document_type_code"`
		Confidence       float64 `json:"confidence"`
	}
	if err := decodeModelJSON(content, &parsed); err != nil {
		return ports.DocumentDraft{}, err
	}
	return ports.DocumentDraft{
		Title:            parsed.Title,
		Number:           parsed.Number,
		Issuer:           parsed.Issuer,
		ValidFrom:        parsed.ValidFrom,
		ValidUntil:       parsed.ValidUntil,
		DocumentTypeCode: parsed.DocumentTypeCode,
		Confidence:       parsed.Confidence,
	}, nil
}

// MatchProfile сопоставляет описание бизнеса с кодами справочника (FR-22).
func (c *Client) MatchProfile(ctx context.Context, description string, catalog ports.AssistantCatalog) (ports.ProfileMatch, error) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"business_category_code", "feature_codes", "confidence"},
		"properties": map[string]any{
			"business_category_code": map[string]any{"type": "string",
				"enum": append(codesOf(catalog.Categories), "")},
			"feature_codes": map[string]any{"type": "array", "maxItems": 20,
				"items": map[string]any{"type": "string", "enum": codesOf(catalog.Features)}},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
	}
	user := "Виды деятельности:\n" + optionsList(catalog.Categories) +
		"\n\nПризнаки организации:\n" + optionsList(catalog.Features) +
		"\n\nОписание бизнеса:\n" + description

	content, err := c.complete(ctx, "match_profile", chatRequest{
		Model:          c.cfg.Model,
		Temperature:    0,
		FunctionCall:   "none",
		MaxTokens:      maxCompletionTokens,
		Messages:       []chatMessage{{Role: "system", Content: profileSystemPrompt}, {Role: "user", Content: user}},
		ResponseFormat: &responseFormat{Type: "json_schema", Schema: schema, Strict: true},
	})
	if err != nil {
		return ports.ProfileMatch{}, err
	}
	var parsed struct {
		BusinessCategoryCode string   `json:"business_category_code"`
		FeatureCodes         []string `json:"feature_codes"`
		Confidence           float64  `json:"confidence"`
	}
	if err := decodeModelJSON(content, &parsed); err != nil {
		return ports.ProfileMatch{}, err
	}
	return ports.ProfileMatch{
		BusinessCategoryCode: parsed.BusinessCategoryCode,
		FeatureCodes:         parsed.FeatureCodes,
		Confidence:           parsed.Confidence,
	}, nil
}

// decodeModelJSON разбирает содержимое ответа модели, снимая обрамление ```json,
// если модель всё же его добавила. Пустой ответ или текст вместо JSON означает, что
// модель не нашла реквизитов или отказалась отвечать: ErrBadResponse и ErrNoResult.
func decodeModelJSON(content string, out any) error {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	if trimmed == "" {
		return fmt.Errorf("%w: %w: пустой ответ", ErrBadResponse, ErrNoResult)
	}
	if err := json.Unmarshal([]byte(trimmed), out); err != nil {
		return fmt.Errorf("%w: %w: %v", ErrBadResponse, ErrNoResult, err)
	}
	return nil
}

func codesOf(options []ports.AssistantOption) []string {
	codes := make([]string, 0, len(options))
	for _, o := range options {
		codes = append(codes, o.Code)
	}
	return codes
}

func optionsList(options []ports.AssistantOption) string {
	var b strings.Builder
	for _, o := range options {
		b.WriteString("- ")
		b.WriteString(o.Code)
		b.WriteString(": ")
		b.WriteString(o.Title)
		if o.Hint != "" {
			b.WriteString(" (")
			b.WriteString(o.Hint)
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	return b.String()
}
