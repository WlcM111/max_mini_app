// Package archtest проверяет архитектурные границы core-service (ADR-025):
// домен не знает транспорта и хранилища, сценарии не знают адаптеров,
// адаптеры не зависят друг от друга, сервисы не делят предметный код.
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const root = "../../../.." // корень репозитория

type rule struct {
	dir       string
	forbidden []string
	allowed   []string
}

func TestLayerDependencies(t *testing.T) {
	rules := []rule{
		{
			dir: "services/core/internal/domain",
			forbidden: []string{
				"vovremya/services/core/internal/app", "vovremya/services/core/internal/ports",
				"vovremya/services/core/internal/adapters", "vovremya/gen/", "vovremya/internal/platform",
				"github.com/jackc/", "google.golang.org/grpc", "net/http", "database/sql",
				"github.com/prometheus/", "github.com/caarlos0/",
			},
		},
		{
			dir: "services/core/internal/ports",
			forbidden: []string{
				"vovremya/services/core/internal/app", "vovremya/services/core/internal/adapters",
				"vovremya/gen/", "github.com/jackc/", "google.golang.org/grpc", "net/http", "database/sql",
			},
		},
		{
			dir: "services/core/internal/app",
			forbidden: []string{
				"vovremya/services/core/internal/adapters", "vovremya/gen/",
				"github.com/jackc/", "google.golang.org/grpc", "net/http",
			},
			allowed: []string{"vovremya/internal/platform/metrics"},
		},
		{
			dir:       "services/core/internal/adapters/postgres",
			forbidden: []string{"net/http", "google.golang.org/grpc", "vovremya/services/core/internal/adapters/httpapi"},
		},
		{
			dir:       "services/core/internal/adapters/httpapi",
			forbidden: []string{"github.com/jackc/", "google.golang.org/grpc", "vovremya/services/core/internal/adapters/postgres"},
		},
		{
			dir:       "services/core/internal/adapters/maxlaunch",
			forbidden: []string{"net/http", "github.com/jackc/", "google.golang.org/grpc"},
		},
	}
	for _, r := range rules {
		t.Run(r.dir, func(t *testing.T) {
			checkImports(t, r)
		})
	}
}

func TestServicesDoNotShareDomainCode(t *testing.T) {
	for _, dir := range []string{"services/core"} {
		filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			for _, imp := range importsOf(t, path) {
				if strings.HasPrefix(imp, "vovremya/services/bot/") || strings.HasPrefix(imp, "vovremya/services/reminders/") {
					t.Errorf("%s импортирует предметный код другого сервиса: %s", path, imp)
				}
			}
			return nil
		})
	}
}

func checkImports(t *testing.T, r rule) {
	t.Helper()
	dir := filepath.Join(root, r.dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("чтение каталога %s: %v", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		for _, imp := range importsOf(t, path) {
			if allowedImport(imp, r.allowed) {
				continue
			}
			for _, bad := range r.forbidden {
				if strings.HasPrefix(imp, bad) {
					t.Errorf("%s: запрещённый импорт %s", path, imp)
				}
			}
		}
	}
}

func allowedImport(imp string, allowed []string) bool {
	for _, a := range allowed {
		if strings.HasPrefix(imp, a) {
			return true
		}
	}
	return false
}

func importsOf(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("разбор %s: %v", path, err)
	}
	out := make([]string, 0, len(file.Imports))
	for _, imp := range file.Imports {
		out = append(out, strings.Trim(imp.Path.Value, `"`))
	}
	return out
}
