package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"vovremya/services/core/test/testutil"
)

type client struct {
	t     *testing.T
	h     *testutil.Harness
	token string
}

func newClient(t *testing.T, h *testutil.Harness, userID int64, name string) *client {
	t.Helper()
	c := &client{t: t, h: h}
	initData := testutil.LaunchData(t, userID, name, "", h.Clock.Now())
	var session struct {
		Token string `json:"token"`
	}
	status := c.do(http.MethodPost, "/api/v1/sessions", map[string]any{"init_data": initData}, &session)
	if status != http.StatusCreated {
		t.Fatalf("создание сессии: статус %d", status)
	}
	c.token = session.Token
	return c
}

// do выполняет запрос к публичному API и разбирает ответ.
func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("сериализация тела: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.h.Server.URL+path, reader)
	if err != nil {
		c.t.Fatalf("запрос: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := testutil.NoRedirectClient().Do(req)
	if err != nil {
		c.t.Fatalf("выполнение запроса: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("разбор ответа %s %s (%d): %v: %s", method, path, resp.StatusCode, err, raw)
		}
	}
	return resp.StatusCode
}

// raw выполняет запрос и возвращает статус, заголовки и тело как есть.
func (c *client) raw(method, path string, headers map[string]string) (int, http.Header, string) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.h.Server.URL+path, nil)
	if err != nil {
		c.t.Fatalf("запрос: %v", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := testutil.NoRedirectClient().Do(req)
	if err != nil {
		c.t.Fatalf("выполнение запроса: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(raw)
}

type organization struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	MyRole  string `json:"my_role"`
	Stats   struct {
		Total          int     `json:"total"`
		Expired        int     `json:"expired"`
		Expiring       int     `json:"expiring"`
		Valid          int     `json:"valid"`
		NoExpiry       int     `json:"no_expiry"`
		NextValidUntil *string `json:"next_valid_until"`
	} `json:"stats"`
	FeatureCodes []string `json:"feature_codes"`
	Timezone     string   `json:"timezone"`
}

type document struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organization_id"`
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	DaysLeft       *int     `json:"days_left"`
	Version        int      `json:"version"`
	CanEdit        bool     `json:"can_edit"`
	RemindersState string   `json:"reminders_state"`
	NextReminderAt *string  `json:"next_reminder_at"`
	Offsets        []int    `json:"reminder_offsets_days"`
	RenewalSteps   []string `json:"renewal_steps"`
	CurrentPeriod  struct {
		ID         string  `json:"id"`
		ValidFrom  *string `json:"valid_from"`
		ValidUntil *string `json:"valid_until"`
		IsCurrent  bool    `json:"is_current"`
	} `json:"current_period"`
	Periods []struct {
		ID        string `json:"id"`
		IsCurrent bool   `json:"is_current"`
	} `json:"periods"`
}

type problem struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

// createOrganization создаёт организацию общепита в Петербурге.
func (c *client) createOrganization(id, name string) organization {
	c.t.Helper()
	var org organization
	status := c.do(http.MethodPost, "/api/v1/organizations", map[string]any{
		"id": id, "name": name, "business_category_code": "food_service",
		"region_code": "RU-SPE", "timezone": "Europe/Moscow",
		"feature_codes": []string{"has_premises", "sells_alcohol"},
	}, &org)
	if status != http.StatusCreated {
		c.t.Fatalf("создание организации: статус %d", status)
	}
	return org
}

// createDocument добавляет документ со сроком через указанное число дней.
func (c *client) createDocument(orgID, docID, title string, daysUntil int) document {
	c.t.Helper()
	until := c.h.Clock.Now().AddDate(0, 0, daysUntil).Format("2006-01-02")
	var doc document
	status := c.do(http.MethodPost, "/api/v1/organizations/"+orgID+"/documents", map[string]any{
		"id": docID, "title": title, "valid_until": until,
	}, &doc)
	if status != http.StatusCreated {
		c.t.Fatalf("создание документа: статус %d", status)
	}
	return doc
}

func uuidFor(n int) string {
	return fmt.Sprintf("%08x-1111-4222-8333-%012d", n, n)
}

var _ = time.Now
