package app

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/services/bot/internal/domain"
	"vovremya/services/bot/internal/ports"
)

// subscriptionsMax — клиент MAX с подписками; запоминает повторные подписки.
type subscriptionsMax struct {
	types      map[string][]string
	subscribed [][]string
}

func (f *subscriptionsMax) SendMessage(context.Context, domain.Message, domain.Profile) (ports.SendResult, error) {
	return ports.SendResult{}, nil
}

func (f *subscriptionsMax) GetMe(context.Context) (domain.Profile, error) {
	return domain.Profile{}, nil
}

func (f *subscriptionsMax) ListSubscriptions(context.Context) ([]string, error) {
	urls := make([]string, 0, len(f.types))
	for u := range f.types {
		urls = append(urls, u)
	}
	return urls, nil
}

func (f *subscriptionsMax) SubscriptionTypes(context.Context) (map[string][]string, error) {
	return f.types, nil
}

func (f *subscriptionsMax) Subscribe(_ context.Context, url string, updateTypes []string, _ string) error {
	f.subscribed = append(f.subscribed, updateTypes)
	f.types[url] = append([]string(nil), updateTypes...)
	return nil
}

func (f *subscriptionsMax) SetCommands(context.Context, []ports.BotCommand) error { return nil }

func TestSubscriptionKeeperAddsMissingTypes(t *testing.T) {
	const url = "https://vovremya.example/max/webhook"
	want := []string{"bot_started", "message_callback"}
	fake := &subscriptionsMax{types: map[string][]string{url: {"bot_started"}}}
	keeper := NewSubscriptionKeeper(fake, url, want, "secret", time.Minute,
		slog.New(slog.NewTextHandler(io.Discard, nil)), NewMetrics(metrics.New()))
	ctx := context.Background()
	if err := keeper.EnsureOnce(ctx); err != nil {
		t.Fatalf("проверка подписки: %v", err)
	}
	if len(fake.subscribed) != 1 || !reflect.DeepEqual(fake.subscribed[0], want) {
		t.Fatalf("подписка без message_callback не оформлена заново: %v", fake.subscribed)
	}
	if err := keeper.EnsureOnce(ctx); err != nil || len(fake.subscribed) != 1 {
		t.Fatalf("полная подписка не должна оформляться повторно: %v %v", fake.subscribed, err)
	}
}

func TestMissingTypes(t *testing.T) {
	want := []string{"bot_started", "message_callback"}
	if got := missingTypes(nil, want); len(got) != 0 {
		t.Errorf("пустой список MAX — подписка на все события: %v", got)
	}
	if got := missingTypes([]string{"bot_started"}, want); !reflect.DeepEqual(got, []string{"message_callback"}) {
		t.Errorf("недостающие типы: %v", got)
	}
}
