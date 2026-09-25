package maxlaunch_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"vovremya/services/core/internal/adapters/maxlaunch"
	"vovremya/services/core/internal/domain"
)

// Тест-векторы TV-1…TV-6 — docs/max/test-vectors.md.
const (
	secretHex = "e45f53166d1da778700f29abbf89f6ac247864cb97356f927707e56f0066d663"
	tv1       = "auth_date=1789900000&chat=%7B%22id%22%3A1001%2C%22type%22%3A%22DIALOG%22%7D&" +
		"query_id=7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10&" +
		"user=%7B%22id%22%3A1001%2C%22first_name%22%3A%22%D0%A2%D0%B5%D1%81%D1%82%22%2C%22last_name%22%3Anull%2C" +
		"%22username%22%3Anull%2C%22language_code%22%3A%22ru%22%2C%22photo_url%22%3Anull%7D&" +
		"start_param=doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02&" +
		"hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc"
	tv2 = "auth_date=1789900000&chat=%7B%22id%22%3A1001%2C%22type%22%3A%22DIALOG%22%7D&" +
		"query_id=7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10&" +
		"user=%7B%22id%22%3A1002%2C%22first_name%22%3A%22%D0%A2%D0%B5%D1%81%D1%82%22%2C%22last_name%22%3Anull%2C" +
		"%22username%22%3Anull%2C%22language_code%22%3A%22ru%22%2C%22photo_url%22%3Anull%7D&" +
		"start_param=doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02&" +
		"hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc"
)

func newVerifier(t *testing.T) *maxlaunch.Verifier {
	t.Helper()
	v, err := maxlaunch.New(secretHex, time.Hour, 60*time.Second)
	if err != nil {
		t.Fatalf("создание проверяющего: %v", err)
	}
	return v
}

func TestTV1ValidLaunchData(t *testing.T) {
	now := time.Unix(1789900600, 0).UTC()
	identity, err := newVerifier(t).Verify(tv1, now)
	if err != nil {
		t.Fatalf("TV-1 должен проходить проверку: %v", err)
	}
	if identity.MaxUserID != 1001 {
		t.Fatalf("user.id = %d", identity.MaxUserID)
	}
	if identity.FirstName != "Тест" {
		t.Fatalf("first_name = %q", identity.FirstName)
	}
	if identity.QueryID != "7a1c5e1e-2f63-4f3a-9d2b-0c8e5b1a4f10" {
		t.Fatalf("query_id = %q", identity.QueryID)
	}
	start := domain.ParseStartTarget(identity.StartParam)
	if start.Kind != domain.StartDocument || start.DocumentID != "3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02" {
		t.Fatalf("start = %+v", start)
	}
}

func TestTV2TamperedUserID(t *testing.T) {
	now := time.Unix(1789900600, 0).UTC()
	if _, err := newVerifier(t).Verify(tv2, now); !errors.Is(err, domain.ErrLaunchInvalid) {
		t.Fatalf("подмена user.id должна отклоняться: %v", err)
	}
}

func TestTV3DuplicateHashParameter(t *testing.T) {
	now := time.Unix(1789900600, 0).UTC()
	doubled := tv1 + "&hash=f7ba0b22030ffefaca9ef98ac6753c08558d723cc75a099dd0b183f4ed6caefc"
	if _, err := newVerifier(t).Verify(doubled, now); !errors.Is(err, domain.ErrLaunchInvalid) {
		t.Fatalf("повтор hash должен отклоняться: %v", err)
	}
}

func TestTV4Expired(t *testing.T) {
	now := time.Unix(1789903601, 0).UTC()
	if _, err := newVerifier(t).Verify(tv1, now); !errors.Is(err, domain.ErrLaunchExpired) {
		t.Fatalf("просроченные данные запуска: %v", err)
	}
}

func TestTV5FutureAuthDate(t *testing.T) {
	now := time.Unix(1789899939, 0).UTC()
	if _, err := newVerifier(t).Verify(tv1, now); !errors.Is(err, domain.ErrLaunchInvalid) {
		t.Fatalf("auth_date из будущего: %v", err)
	}
}

func TestTV6MalformedInputs(t *testing.T) {
	now := time.Unix(1789900600, 0).UTC()
	v := newVerifier(t)
	cases := map[string]string{
		"пустая строка":        "",
		"без hash":             "auth_date=1789900000&user=%7B%22id%22%3A1%7D",
		"короткий hash":        "auth_date=1789900000&hash=abc",
		"часть без знака =":    "auth_date=1789900000&broken&hash=" + strings.Repeat("a", 64),
		"нешестнадцатеричный":  "auth_date=1789900000&hash=" + strings.Repeat("z", 64),
		"повтор ключа":         "auth_date=1&auth_date=2&hash=" + strings.Repeat("a", 64),
		"слишком длинная":      strings.Repeat("a", 5000),
		"дубликат start_param": tv1 + "&start_param=org_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(value, now); err == nil {
				t.Fatal("ожидалась ошибка проверки")
			}
		})
	}
}

func TestPlusSignPreservedInUsername(t *testing.T) {
	// F-14: плюс в значении остаётся плюсом (PathUnescape, не QueryUnescape).
	v := newVerifier(t)
	if _, err := v.Verify("user=%7B%22id%22%3A1%2C%22first_name%22%3A%22a+b%22%7D&auth_date=1789900000&hash="+
		strings.Repeat("0", 64), time.Unix(1789900600, 0).UTC()); !errors.Is(err, domain.ErrLaunchInvalid) {
		t.Fatalf("некорректная подпись должна отклоняться: %v", err)
	}
}

func TestSecretValidation(t *testing.T) {
	if _, err := maxlaunch.New("короткий", time.Hour, time.Minute); err == nil {
		t.Fatal("ожидалась ошибка длины ключа")
	}
}
