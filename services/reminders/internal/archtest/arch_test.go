// Package archtest проверяет направление зависимостей внутри reminders-service.
// Правила описаны в docs/architecture/backend-services.md.
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rule — запрет импортов для группы пакетов сервиса.
type rule struct {
	dir       string   // каталог относительно корня сервиса
	forbidden []string // подстроки запрещённых путей импорта
	reason    string
}

func TestLayerDependencies(t *testing.T) {
	root := serviceRoot(t)
	rules := []rule{
		{
			dir: "internal/domain",
			forbidden: []string{
				"vovremya/services/reminders/internal/adapters",
				"vovremya/services/reminders/internal/app",
				"vovremya/services/reminders/internal/ports",
				"vovremya/internal/platform",
				"vovremya/gen/",
				"github.com/jackc/pgx",
				"google.golang.org/grpc",
				"google.golang.org/protobuf",
				"net/http",
				"database/sql",
				"encoding/json",
			},
			reason: "домен не зависит от транспорта, SQL, генерируемого кода и инфраструктуры",
		},
		{
			dir: "internal/ports",
			forbidden: []string{
				"vovremya/services/reminders/internal/adapters",
				"vovremya/services/reminders/internal/app",
				"github.com/jackc/pgx",
				"google.golang.org/grpc",
				"net/http",
			},
			reason: "порты объявляют интерфейсы поверх домена и не знают реализаций",
		},
		{
			dir: "internal/app",
			forbidden: []string{
				"vovremya/services/reminders/internal/adapters",
				"vovremya/gen/",
				"github.com/jackc/pgx",
				"google.golang.org/grpc",
				"database/sql",
				"net/http",
			},
			reason: "сценарии работают через порты, а не через конкретные адаптеры",
		},
		{
			dir:       "internal/adapters/postgres",
			forbidden: []string{"google.golang.org/grpc", "vovremya/services/reminders/internal/adapters/grpcserver", "vovremya/services/reminders/internal/adapters/botgrpc"},
			reason:    "адаптер хранения не зависит от других адаптеров",
		},
		{
			dir:       "internal/adapters/grpcserver",
			forbidden: []string{"github.com/jackc/pgx", "vovremya/services/reminders/internal/adapters/postgres", "vovremya/services/reminders/internal/adapters/botgrpc"},
			reason:    "входящий адаптер не зависит от хранилища и исходящих адаптеров",
		},
		{
			dir:       "internal/adapters/botgrpc",
			forbidden: []string{"github.com/jackc/pgx", "vovremya/services/reminders/internal/app", "vovremya/services/reminders/internal/adapters/postgres"},
			reason:    "исходящий адаптер реализует порт и не знает сценариев",
		},
	}

	for _, r := range rules {
		t.Run(r.dir, func(t *testing.T) {
			imports := collectImports(t, filepath.Join(root, r.dir))
			if len(imports) == 0 {
				t.Fatalf("каталог %s не содержит исходников", r.dir)
			}
			for file, list := range imports {
				for _, imp := range list {
					for _, bad := range r.forbidden {
						if strings.Contains(imp, bad) {
							t.Errorf("%s импортирует %q: %s", file, imp, r.reason)
						}
					}
				}
			}
		})
	}
}

// TestPlatformDoesNotDependOnServices проверяет, что общий технический каркас
// не знает предметных сервисов (ADR-026).
func TestPlatformDoesNotDependOnServices(t *testing.T) {
	root := filepath.Join(serviceRoot(t), "..", "..", "internal", "platform")
	imports := collectImports(t, root)
	if len(imports) == 0 {
		t.Fatal("каталог internal/platform не содержит исходников")
	}
	for file, list := range imports {
		for _, imp := range list {
			if strings.Contains(imp, "vovremya/services/") {
				t.Errorf("%s импортирует %q: каркас не должен зависеть от сервисов", file, imp)
			}
		}
	}
}

func serviceRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Тест лежит в <service>/internal/archtest.
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func collectImports(t *testing.T, dir string) map[string][]string {
	t.Helper()
	out := make(map[string][]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		var list []string
		for _, imp := range f.Imports {
			list = append(list, strings.Trim(imp.Path.Value, `"`))
		}
		out[path] = list
		return nil
	})
	if err != nil {
		t.Fatalf("обход %s: %v", dir, err)
	}
	return out
}
