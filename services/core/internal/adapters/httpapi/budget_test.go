package httpapi

import (
	"net/http"
	"testing"
	"time"
)

func TestRequestBudget(t *testing.T) {
	const base, assistant = 5 * time.Second, 8 * time.Second
	cases := []struct {
		method, path string
		want         time.Duration
	}{
		{http.MethodPost, "/api/v1/organizations/0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01/documents/draft-image", 29 * time.Second},
		{http.MethodPost, "/api/v1/organizations/0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01/documents/draft", 13 * time.Second},
		{http.MethodPost, "/api/v1/profile-match", 13 * time.Second},
		{http.MethodPost, "/api/v1/organizations/0b6f2c1e-5a3d-4c8e-9f21-7d4a6b8c9e01/documents", base},
		{http.MethodGet, "/api/v1/profile-match", base},
		{http.MethodGet, "/api/v1/me", base},
	}
	for _, c := range cases {
		if got := requestBudget(c.method, c.path, base, assistant); got != c.want {
			t.Errorf("%s %s: бюджет %v, ожидался %v", c.method, c.path, got, c.want)
		}
	}
	if got := requestBudget(http.MethodPost, "/api/v1/profile-match", base, 0); got != base {
		t.Errorf("без ассистента бюджет не расширяется: %v", got)
	}
	if got := requestBudget(http.MethodPost, "/api/v1/profile-match", 30*time.Second, assistant); got != 30*time.Second {
		t.Errorf("больший общий бюджет не уменьшается: %v", got)
	}
}
