package config_test

import (
	"testing"
	"time"

	"vovremya/services/reminders/internal/config"
)

// TestBotRPCTimeoutMatchesContract закрепляет исправление A-1: нормативный
// gRPC-контракт (docs/contracts/grpc-contract.md §1) требует deadline 2 с.
func TestBotRPCTimeoutMatchesContract(t *testing.T) {
	t.Setenv("REMINDERS_DATABASE_URL", "postgres://reminders_app:devonly_reminders_app@127.0.0.1:5432/vovremya")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("конфигурация по умолчанию отвергнута: %v", err)
	}
	if cfg.BotRPCTimeout != 2*time.Second {
		t.Errorf("REMINDERS_BOT_RPC_TIMEOUT по умолчанию: ожидалось 2s, получено %s", cfg.BotRPCTimeout)
	}
	if cfg.SchedulerLease < cfg.BotRPCTimeout {
		t.Errorf("аренда планировщика меньше срока вызова bot: %s < %s", cfg.SchedulerLease, cfg.BotRPCTimeout)
	}
}
