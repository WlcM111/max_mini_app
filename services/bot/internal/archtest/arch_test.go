// Package archtest проверяет архитектурные границы bot-service (ADR-025):
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
	allowed   []string // исключения из запрета
}

func TestLayerDependencies(t *testing.T) {
	rules := []rule{
		{
			dir: "services/bot/internal/domain",
			forbidden: []string{
				"vovremya/services/bot/internal/app", "vovremya/services/bot/internal/ports",
				"vovremya/services/bot/internal/adapters", "vovremya/gen/", "vovremya/internal/platform",
				"github.com/jackc/", "google.golang.org/grpc", "net/http", "database/sql",
				"github.com/prometheus/", "github.com/caarlos0/",
			},
		},
		{
			dir: "services/bot/internal/ports",
			forbidden: []string{
				"vovremya/services/bot/internal/app", "vovremya/services/bot/internal/adapters",
				"vovremya/gen/", "github.com/jackc/", "google.golang.org/grpc", "net/http", "database/sql",
			},
		},
		{
			dir: "services/bot/internal/app",
			forbidden: []string{
				"vovremya/services/bot/internal/adapters", "vovremya/gen/", "github.com/jackc/",
				"google.golang.org/grpc", "net/http", "database/sql",
			},
		},
		{
			dir:       "services/bot/internal/adapters/postgres",
			forbidden: []string{"vovremya/services/bot/internal/adapters/", "vovremya/gen/", "net/http"},
		},
		{
			dir:       "services/bot/internal/adapters/grpcserver",
			forbidden: []string{"vovremya/services/bot/internal/adapters/", "github.com/jackc/", "net/http"},
		},
		{
			dir:       "services/bot/internal/adapters/maxapi",
			forbidden: []string{"vovremya/services/bot/internal/adapters/", "github.com/jackc/", "google.golang.org/grpc"},
		},
		{
			dir:       "services/bot/internal/adapters/webhook",
			forbidden: []string{"vovremya/services/bot/internal/adapters/", "github.com/jackc/", "google.golang.org/grpc"},
		},
	}
	for _, r := range rules {
		t.Run(r.dir, func(t *testing.T) {
			for file, imports := range packageImports(t, filepath.Join(root, r.dir)) {
				for _, imp := range imports {
					for _, bad := range r.forbidden {
						if strings.HasPrefix(imp, bad) && !allowed(imp, r.allowed) {
							t.Errorf("%s импортирует %q: запрещено правилом %q", file, imp, bad)
						}
					}
				}
			}
		})
	}
}

// TestNoSharedDomainBetweenServices — микросервисы не делят предметный код
// и не обращаются к пакетам друг друга (ADR-017, ADR-019).
func TestNoSharedDomainBetweenServices(t *testing.T) {
	checks := []struct{ dir, forbidden string }{
		{"services/bot", "vovremya/services/reminders/"},
		{"services/reminders", "vovremya/services/bot/"},
		{"internal/platform", "vovremya/services/"},
	}
	for _, c := range checks {
		t.Run(c.dir, func(t *testing.T) {
			for file, imports := range treeImports(t, filepath.Join(root, c.dir)) {
				for _, imp := range imports {
					if strings.HasPrefix(imp, c.forbidden) {
						t.Errorf("%s импортирует %q: пакеты другого сервиса недоступны", file, imp)
					}
				}
			}
		})
	}
}

// TestPlatformHasNoServiceMetrics — общий каркас не содержит предметных метрик
// сервисов (ADR-026): иначе метрики одного сервиса попадают в /metrics другого.
func TestPlatformHasNoServiceMetrics(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root, "internal/platform/metrics/metrics.go"))
	if err != nil {
		t.Fatalf("чтение каркаса метрик: %v", err)
	}
	for _, name := range []string{"vovremya_reminders_", "vovremya_ingest_", "vovremya_outbound_",
		"vovremya_max_", "vovremya_projection_"} {
		if strings.Contains(string(data), name) {
			t.Errorf("каркас метрик содержит предметную метрику %q", name)
		}
	}
}

func allowed(imp string, list []string) bool {
	for _, a := range list {
		if strings.HasPrefix(imp, a) {
			return true
		}
	}
	return false
}

// packageImports возвращает импорты файлов одного каталога.
func packageImports(t *testing.T, dir string) map[string][]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("чтение каталога %s: %v", dir, err)
	}
	out := make(map[string][]string)
	for _, e := range entries {
		// Правила границ относятся к production-коду: внешние тестовые файлы
		// пакета (package X_test) импортируют сам X, и это не нарушение.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		out[path] = fileImports(t, path)
	}
	if len(out) == 0 {
		t.Fatalf("в каталоге %s не найдено файлов Go", dir)
	}
	return out
}

// treeImports возвращает импорты всех файлов поддерева.
func treeImports(t *testing.T, dir string) map[string][]string {
	t.Helper()
	out := make(map[string][]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out[path] = fileImports(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("обход %s: %v", dir, err)
	}
	return out
}

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("разбор %s: %v", path, err)
	}
	imports := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	return imports
}
