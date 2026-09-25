package app

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// ProfileStore хранит загруженный профиль бота.
type ProfileStore struct {
	mode    domain.Mode
	current atomic.Pointer[domain.Profile]
}

// NewProfileStore создаёт пустое хранилище профиля.
func NewProfileStore(mode domain.Mode) *ProfileStore { return &ProfileStore{mode: mode} }

// Get возвращает профиль; ok=false — профиль ещё не загружен.
func (s *ProfileStore) Get() (domain.Profile, bool) {
	p := s.current.Load()
	if p == nil {
		return domain.Profile{}, false
	}
	return *p, true
}

// Set сохраняет профиль.
func (s *ProfileStore) Set(p domain.Profile) { s.current.Store(&p) }

// Mode возвращает режим канала.
func (s *ProfileStore) Mode() domain.Mode { return s.mode }

// BotCommands — команды меню бота (handoff §8).
var BotCommands = []ports.BotCommand{
	{Name: "start", Description: "Открыть приложение"},
	{Name: "help", Description: "Как пользоваться"},
}

// ProfileLoader загружает профиль бота через GET /me и повторяет попытку
// каждые RetryInterval до успеха (spec §10); после загрузки задаёт команды.
type ProfileLoader struct {
	client        ports.MaxClient
	store         *ProfileStore
	log           *slog.Logger
	RetryInterval time.Duration
}

// NewProfileLoader создаёт загрузчик профиля.
func NewProfileLoader(client ports.MaxClient, store *ProfileStore, log *slog.Logger) *ProfileLoader {
	return &ProfileLoader{client: client, store: store, log: log, RetryInterval: 30 * time.Second}
}

// LoadOnce выполняет одну попытку загрузки профиля.
func (l *ProfileLoader) LoadOnce(ctx context.Context) error {
	p, err := l.client.GetMe(ctx)
	if err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	l.store.Set(p)
	l.log.Info("bot profile loaded", slog.String("username", p.Username), slog.String("mode", string(l.store.Mode())))
	if err := l.client.SetCommands(ctx, BotCommands); err != nil {
		// Команды меню не влияют на основной сценарий: только предупреждение.
		l.log.Warn("bot commands not updated", slog.Any("error", err))
	}
	return nil
}

// Run загружает профиль, повторяя попытки до успеха или остановки.
func (l *ProfileLoader) Run(ctx context.Context) error {
	for {
		err := l.LoadOnce(ctx)
		if err == nil {
			return nil
		}
		l.log.Warn("bot profile not loaded", slog.Any("error", err))
		t := time.NewTimer(l.RetryInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
	}
}
