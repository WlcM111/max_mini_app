package gigachat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"vovremya/services/core/internal/ports"
)

// Распознавание фото дольше текстового запроса, поэтому свой тайм-аут.
const visionTimeout = 40 * time.Second

const visionSystemPrompt = "На фотографии — документ организации: лицензия, договор, сертификат, удостоверение и т. п. " +
	"Извлеки его реквизиты. Отвечай только объектом JSON с ключами title, number, issuer, valid_from, valid_until, " +
	"document_type_code, confidence — без пояснений и обрамления. Бери значения исключительно с изображения: " +
	"ничего не додумывай. Если значения нет или его не видно — верни пустую строку. Даты приводи к формату YYYY-MM-DD. " +
	"valid_from — дата начала действия или, если её нет, дата выдачи; valid_until — дата окончания срока действия, " +
	"не вычисляй её. Персональные данные людей (ФИО, паспорт, адрес) не переписывай. " +
	"Поле document_type_code выбирай только из перечисленных кодов и только если документ явно относится к этому типу. " +
	"Если на фото нет документа или реквизиты не читаются — верни пустые строки и confidence 0. " +
	"В поле confidence верни число от 0 до 1 — насколько уверенно реквизиты прочитаны."

type visionMessage struct {
	Role        string   `json:"role"`
	Content     string   `json:"content"`
	Attachments []string `json:"attachments,omitempty"`
}

type visionRequest struct {
	Model          string          `json:"model"`
	Messages       []visionMessage `json:"messages"`
	Temperature    float64         `json:"temperature"`
	FunctionCall   string          `json:"function_call"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

// DraftDocumentFromImage читает реквизиты документа с фотографии. Файл загружается в хранилище
// GigaChat, передаётся в запрос через attachments и сразу удаляется (персональные данные не копятся).
func (c *Client) DraftDocumentFromImage(ctx context.Context, image []byte, mimeType string,
	catalog ports.AssistantCatalog) (ports.DocumentDraft, error) {
	if err := c.checkBudget(); err != nil {
		return ports.DocumentDraft{}, err
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return ports.DocumentDraft{}, err
	}
	fileID, err := c.uploadFile(ctx, token, image, mimeType)
	if err != nil {
		return ports.DocumentDraft{}, err
	}
	defer c.deleteFile(context.WithoutCancel(ctx), token, fileID)

	payload, err := json.Marshal(visionRequest{
		Model:        c.cfg.Model,
		Temperature:  0,
		FunctionCall: "none",
		MaxTokens:    maxCompletionTokens,
		Messages: []visionMessage{
			{Role: "system", Content: visionSystemPrompt},
			{Role: "user", Content: "Типы документов справочника:\n" + optionsList(catalog.DocumentTypes) +
				"\n\nИзвлеки реквизиты документа с фотографии.", Attachments: []string{fileID}},
		},
		ResponseFormat: &responseFormat{Type: "json_schema", Schema: draftSchema(catalog), Strict: true},
	})
	if err != nil {
		return ports.DocumentDraft{}, err
	}
	content, usage, err := c.visionComplete(ctx, token, payload)
	c.addUsage("draft_document_image", usage)
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

// draftSchema — та же схема ответа, что у распознавания текста.
func draftSchema(catalog ports.AssistantCatalog) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"title", "confidence"},
		"properties": map[string]any{
			"title":       map[string]any{"type": "string", "description": "название документа"},
			"number":      map[string]any{"type": "string", "description": "номер документа или пустая строка"},
			"issuer":      map[string]any{"type": "string", "description": "кем выдан или пустая строка"},
			"valid_from":  map[string]any{"type": "string", "description": "дата начала действия, YYYY-MM-DD или пустая строка"},
			"valid_until": map[string]any{"type": "string", "description": "дата окончания действия, YYYY-MM-DD или пустая строка"},
			"document_type_code": map[string]any{"type": "string", "enum": append(codesOf(catalog.DocumentTypes), ""),
				"description": "код типа документа из справочника или пустая строка"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
		},
	}
}

func (c *Client) visionHTTP() *http.Client {
	return &http.Client{Timeout: visionTimeout, Transport: c.http.Transport}
}

func (c *Client) uploadFile(ctx context.Context, token string, data []byte, mimeType string) (string, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	ext := "jpg"
	if mimeType == "image/png" {
		ext = "png"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="document.%s"`, ext))
	header.Set("Content-Type", mimeType)
	part, err := form.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := form.WriteField("purpose", "general"); err != nil {
		return "", err
	}
	if err := form.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/files", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.visionHTTP().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			c.invalidateToken()
		}
		return "", fmt.Errorf("gigachat: загрузка файла: статус %d", resp.StatusCode)
	}
	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.ID == "" {
		return "", ErrBadResponse
	}
	return parsed.ID, nil
}

func (c *Client) deleteFile(ctx context.Context, token, fileID string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.cfg.BaseURL, "/")+"/files/"+url.PathEscape(fileID)+"/delete", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.visionHTTP().Do(req)
	if err != nil {
		c.cfg.Log.Warn("gigachat: не удалось удалить файл", "err", err)
		return
	}
	_ = resp.Body.Close()
}

func (c *Client) visionComplete(ctx context.Context, token string, payload []byte) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Request-ID", newRqUID())
	resp, err := c.visionHTTP().Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			c.invalidateToken()
		}
		return "", 0, fmt.Errorf("gigachat: распознавание фото: статус %d", resp.StatusCode)
	}
	return parseChatResponse(body)
}
