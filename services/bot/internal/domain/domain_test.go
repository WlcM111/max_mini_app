package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"vovremya/services/bot/internal/domain"
)

var now = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

func validRequest() domain.EnqueueRequest {
	return domain.EnqueueRequest{
		IdempotencyKey: "rem:6d1f0a73-2b8c-4e5f-9a10-3c7b5e2d8f04:9f3c7b21-4d6e-4a8b-b5c0-1e2f3a4b5c06:30",
		Message: domain.Message{
			Kind:               domain.KindReminder,
			RecipientMaxUserID: 1001,
			Text:               "Через 30 дней заканчивается срок: «Лицензия». Организация: Кафе. Срок до 21.10.2026.",
			Buttons:            []domain.Button{{Text: "Открыть документ", Action: domain.ActionOpenApp, Payload: "doc_3c9e7a52-1f4b-4d6a-8e2c-5b7f9a1d3e02"}},
		},
		NotAfter: now.Add(24 * time.Hour),
	}
}

func TestEnqueueRequestValidation(t *testing.T) {
	if err := validRequest().Validate(now); err != nil {
		t.Fatalf("корректный запрос (формат reminders-service) отвергнут: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(r *domain.EnqueueRequest)
		field  string
	}{
		{"ключ короче 8", func(r *domain.EnqueueRequest) { r.IdempotencyKey = "rem:1" }, "idempotency_key"},
		{"ключ с заглавными", func(r *domain.EnqueueRequest) { r.IdempotencyKey = "REM:ABCDEFGH" }, "idempotency_key"},
		{"ключ длиннее 200", func(r *domain.EnqueueRequest) { r.IdempotencyKey = strings.Repeat("a", 201) }, "idempotency_key"},
		{"ключ с пробелом", func(r *domain.EnqueueRequest) { r.IdempotencyKey = "rem:abc def" }, "idempotency_key"},
		{"вид не задан", func(r *domain.EnqueueRequest) { r.Kind = "" }, "kind"},
		{"получатель 0", func(r *domain.EnqueueRequest) { r.RecipientMaxUserID = 0 }, "recipient_max_user_id"},
		{"получатель отрицательный", func(r *domain.EnqueueRequest) { r.RecipientMaxUserID = -5 }, "recipient_max_user_id"},
		{"пустой текст", func(r *domain.EnqueueRequest) { r.Text = "" }, "text"},
		{"текст 4001 символ", func(r *domain.EnqueueRequest) { r.Text = strings.Repeat("я", 4001) }, "text"},
		{"текст с NUL", func(r *domain.EnqueueRequest) { r.Text = "a\x00b" }, "text"},
		{"четыре кнопки", func(r *domain.EnqueueRequest) {
			b := r.Buttons[0]
			r.Buttons = []domain.Button{b, b, b, b}
		}, "buttons"},
		{"пустой текст кнопки", func(r *domain.EnqueueRequest) { r.Buttons[0].Text = "" }, "buttons[0].text"},
		{"текст кнопки 65 символов", func(r *domain.EnqueueRequest) { r.Buttons[0].Text = strings.Repeat("ы", 65) }, "buttons[0].text"},
		{"payload с точкой", func(r *domain.EnqueueRequest) { r.Buttons[0].Payload = "doc.1" }, "buttons[0].open_app_payload"},
		{"payload 513 символов", func(r *domain.EnqueueRequest) { r.Buttons[0].Payload = strings.Repeat("a", 513) }, "buttons[0].open_app_payload"},
		{"url без https", func(r *domain.EnqueueRequest) {
			r.Buttons[0] = domain.Button{Text: "Ссылка", Action: domain.ActionURL, URL: "http://example.org"}
		}, "buttons[0].url"},
		{"url длиннее 2048", func(r *domain.EnqueueRequest) {
			r.Buttons[0] = domain.Button{Text: "Ссылка", Action: domain.ActionURL, URL: "https://" + strings.Repeat("a", 2041)}
		}, "buttons[0].url"},
		{"два действия", func(r *domain.EnqueueRequest) { r.Buttons[0].URL = "https://example.org" }, "buttons[0]"},
		{"нет действия", func(r *domain.EnqueueRequest) { r.Buttons[0].Action = "" }, "buttons[0]"},
		{"not_after не задан", func(r *domain.EnqueueRequest) { r.NotAfter = time.Time{} }, "not_after"},
		{"not_after в прошлом", func(r *domain.EnqueueRequest) { r.NotAfter = now.Add(-time.Second) }, "not_after"},
		{"not_after равен now", func(r *domain.EnqueueRequest) { r.NotAfter = now }, "not_after"},
		{"not_after позже 7 суток", func(r *domain.EnqueueRequest) { r.NotAfter = now.Add(7*24*time.Hour + time.Second) }, "not_after"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := validRequest()
			r.Buttons = append([]domain.Button(nil), r.Buttons...)
			c.mutate(&r)
			err := r.Validate(now)
			var ve *domain.ValidationError
			if !errors.As(err, &ve) || !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("ожидалась ошибка валидации, получено %v", err)
			}
			if ve.Field != c.field {
				t.Errorf("поле ошибки: ожидалось %q, получено %q", c.field, ve.Field)
			}
		})
	}

	t.Run("граничные корректные значения", func(t *testing.T) {
		r := validRequest()
		r.Text = strings.Repeat("я", 4000)
		r.NotAfter = now.Add(7 * 24 * time.Hour)
		r.Buttons = []domain.Button{
			{Text: strings.Repeat("ы", 64), Action: domain.ActionOpenApp, Payload: ""},
			{Text: "Ссылка", Action: domain.ActionURL, URL: "https://" + strings.Repeat("a", 2040)},
			{Text: "Ещё", Action: domain.ActionOpenApp, Payload: strings.Repeat("Z", 512)},
		}
		if err := r.Validate(now); err != nil {
			t.Fatalf("граничные значения отвергнуты: %v", err)
		}
	})
}

func TestBackoff(t *testing.T) {
	b := domain.DefaultBackoff
	wantCeil := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second,
		80 * time.Second, 160 * time.Second, 320 * time.Second, 600 * time.Second, 600 * time.Second}
	for i, want := range wantCeil {
		if got := b.Ceiling(i + 1); got != want {
			t.Errorf("Ceiling(%d): ожидалось %s, получено %s", i+1, want, got)
		}
	}
	for n := 1; n <= 20; n++ {
		for _, rnd := range []float64{0, 0.5, 0.999, 1.5, -1} {
			d := b.Delay(n, rnd)
			if d < 0 || d >= b.Ceiling(n) && b.Ceiling(n) > 0 && rnd < 1 && rnd >= 0 {
				t.Fatalf("Delay(%d, %v) = %s вне [0; %s)", n, rnd, d, b.Ceiling(n))
			}
			if d > b.Max {
				t.Fatalf("Delay(%d) = %s больше предела", n, d)
			}
		}
	}
}

func TestRetryPolicy(t *testing.T) {
	p := domain.RetryPolicy{MaxAttempts: 8, Backoff: domain.DefaultBackoff}
	retry := domain.SendFailure{Code: domain.CodeMax5xx, Retryable: true}

	tr := p.AfterFailure(retry, 1, now, 0.5)
	if tr.Status != domain.StatusRetryWait || tr.ErrorCode != domain.CodeMax5xx {
		t.Fatalf("временная ошибка: %+v", tr)
	}
	if tr.NextAttemptAt.Before(now) || tr.NextAttemptAt.After(now.Add(5*time.Second)) {
		t.Errorf("первая выдержка вне [0; 5 с]: %s", tr.NextAttemptAt.Sub(now))
	}
	if tr := p.AfterFailure(retry, 8, now, 0.5); tr.Status != domain.StatusFailed || tr.ErrorCode != domain.CodeMax5xx {
		t.Errorf("после восьмой попытки ожидался failed: %+v", tr)
	}
	perm := domain.SendFailure{Code: domain.CodeMax4xx}
	if tr := p.AfterFailure(perm, 1, now, 0.1); tr.Status != domain.StatusFailed {
		t.Errorf("окончательная ошибка должна давать failed: %+v", tr)
	}
	limited := domain.SendFailure{Code: domain.CodeMax429, Retryable: true, RetryAfter: 90 * time.Second}
	if tr := p.AfterFailure(limited, 1, now, 0.1); !tr.NextAttemptAt.Equal(now.Add(90 * time.Second)) {
		t.Errorf("Retry-After должен учитываться: %s", tr.NextAttemptAt.Sub(now))
	}
}

func TestPreSendCheck(t *testing.T) {
	if tr, stop := domain.PreSendCheck(now.Add(-time.Second), domain.RecipientActive, now); !stop || tr.Status != domain.StatusExpired || tr.ErrorCode != domain.CodeExpired {
		t.Errorf("просроченное сообщение: %+v %t", tr, stop)
	}
	if tr, stop := domain.PreSendCheck(now.Add(time.Hour), domain.RecipientStopped, now); !stop || tr.Status != domain.StatusFailed || tr.ErrorCode != domain.CodeRecipientStopped {
		t.Errorf("остановленный получатель: %+v %t", tr, stop)
	}
	for _, st := range []domain.RecipientState{domain.RecipientUnknown, domain.RecipientActive, domain.RecipientMuted, domain.RecipientUnreachable} {
		if _, stop := domain.PreSendCheck(now.Add(time.Hour), st, now); stop {
			t.Errorf("состояние %s не должно блокировать отправку", st)
		}
	}
}

func TestRecipientTransitions(t *testing.T) {
	type row map[domain.RecipientState]domain.RecipientState
	U, A, M, S, R := domain.RecipientUnknown, domain.RecipientActive, domain.RecipientMuted, domain.RecipientStopped, domain.RecipientUnreachable
	table := map[domain.UpdateType]row{
		domain.UpdateBotStarted:     {U: A, A: A, M: A, S: A, R: A},
		domain.UpdateBotStopped:     {U: S, A: S, M: S, S: S, R: S},
		domain.UpdateDialogRemoved:  {U: S, A: S, M: S, S: S, R: S},
		domain.UpdateDialogMuted:    {U: M, A: M, M: M, S: S, R: M},
		domain.UpdateDialogUnmuted:  {U: A, A: A, M: A, S: S, R: A},
		domain.UpdateMessageCreated: {U: A, A: A, M: M, S: S, R: A},
	}
	for ev, r := range table {
		for from, want := range r {
			got, affects := domain.StateAfterEvent(from, ev)
			if !affects || got != want {
				t.Errorf("%s из %s: ожидалось %s, получено %s (affects=%t)", ev, from, want, got, affects)
			}
		}
	}
	if _, affects := domain.StateAfterEvent(A, "message_edited"); affects {
		t.Error("message_edited не должен менять состояние")
	}

	base := domain.Recipient{MaxUserID: 1, State: A, StateChangedAt: now}
	t1 := now.Add(time.Minute)
	muted, ok := base.ApplyEvent(domain.UpdateDialogMuted, t1)
	if !ok || muted.State != M || !muted.StateChangedAt.Equal(t1) || !muted.LastEventTime.Equal(t1) {
		t.Fatalf("dialog_muted: %+v ok=%t", muted, ok)
	}
	older, ok := muted.ApplyEvent(domain.UpdateDialogUnmuted, now)
	if ok || older.State != M {
		t.Errorf("событие старее последнего не должно применяться: %+v ok=%t", older, ok)
	}
	same, ok := muted.ApplyEvent(domain.UpdateDialogUnmuted, t1)
	if !ok || same.State != A {
		t.Errorf("событие с тем же временем должно применяться: %+v ok=%t", same, ok)
	}
	unreach, changed := base.MarkUnreachable(now)
	if !changed || unreach.State != R {
		t.Errorf("403/404 должен переводить в unreachable: %+v", unreach)
	}
	if _, changed := (domain.Recipient{State: S}).MarkUnreachable(now); changed {
		t.Error("остановленный получатель остаётся stopped")
	}
}

func TestUpdateReply(t *testing.T) {
	cases := []struct {
		u    domain.Update
		want domain.Reply
	}{
		{domain.Update{Type: domain.UpdateBotStarted}, domain.ReplyWelcome},
		{domain.Update{Type: domain.UpdateMessageCreated, Text: "/start"}, domain.ReplyWelcome},
		{domain.Update{Type: domain.UpdateMessageCreated, Text: "  /start  abc"}, domain.ReplyWelcome},
		{domain.Update{Type: domain.UpdateMessageCreated, Text: "/help"}, domain.ReplyHelp},
		{domain.Update{Type: domain.UpdateMessageCreated, Text: "/HELP"}, domain.ReplyHelp},
		{domain.Update{Type: domain.UpdateMessageCreated, Text: "привет"}, domain.ReplyHint},
		{domain.Update{Type: domain.UpdateMessageCreated, Text: ""}, domain.ReplyHint},
		{domain.Update{Type: domain.UpdateBotStopped}, domain.ReplyNone},
		{domain.Update{Type: domain.UpdateDialogMuted}, domain.ReplyNone},
	}
	for _, c := range cases {
		if got := c.u.Reply(); got != c.want {
			t.Errorf("%s %q: ожидалось %d, получено %d", c.u.Type, c.u.Text, c.want, got)
		}
	}
	for _, typ := range []domain.UpdateType{domain.UpdateBotStarted, domain.UpdateBotStopped, domain.UpdateDialogRemoved,
		domain.UpdateDialogMuted, domain.UpdateDialogUnmuted, domain.UpdateMessageCreated} {
		if !typ.AffectsRecipient() {
			t.Errorf("%s должен влиять на состояние получателя", typ)
		}
	}
	if domain.UpdateType("message_edited").AffectsRecipient() {
		t.Error("message_edited не влияет на состояние получателя")
	}
}

func TestReplyEnqueueRequest(t *testing.T) {
	key := domain.NewDedupeKey([]byte(`{"update_type":"bot_started"}`))
	for _, r := range []domain.Reply{domain.ReplyWelcome, domain.ReplyHelp, domain.ReplyHint} {
		req, ok := domain.ReplyEnqueueRequest(r, 42, key, now)
		if !ok {
			t.Fatalf("ответ %d не сформирован", r)
		}
		if err := req.Validate(now); err != nil {
			t.Fatalf("ответ %d не проходит контракт очереди: %v", r, err)
		}
		if req.IdempotencyKey != "reply:"+key.Hex() || !req.NotAfter.Equal(now.Add(time.Hour)) {
			t.Errorf("ключ или срок ответа: %s %s", req.IdempotencyKey, req.NotAfter)
		}
		if len(req.Buttons) != 1 || req.Buttons[0].Text != domain.ButtonOpenApp || req.Buttons[0].Payload != "" {
			t.Errorf("кнопка ответа: %+v", req.Buttons)
		}
	}
	if req, _ := domain.ReplyEnqueueRequest(domain.ReplyHint, 42, key, now); req.Kind != domain.KindHelp || req.Text != domain.TextHint {
		t.Errorf("подсказка должна храниться с видом help: %+v", req)
	}
	if _, ok := domain.ReplyEnqueueRequest(domain.ReplyNone, 42, key, now); ok {
		t.Error("для ReplyNone ответ не формируется")
	}
}

func TestProfileLinks(t *testing.T) {
	p := domain.Profile{Username: "vovremya_bot", DisplayName: "Вовремя"}
	if err := p.Validate(); err != nil {
		t.Fatalf("корректный профиль отвергнут: %v", err)
	}
	if p.ChatURL() != "https://max.ru/vovremya_bot" {
		t.Errorf("chat_url: %s", p.ChatURL())
	}
	if p.OpenAppLinkTemplate() != "https://max.ru/vovremya_bot?startapp={payload}" {
		t.Errorf("шаблон: %s", p.OpenAppLinkTemplate())
	}
	if p.OpenAppLink("doc_1") != "https://max.ru/vovremya_bot?startapp=doc_1" || p.OpenAppLink("") != "https://max.ru/vovremya_bot?startapp" {
		t.Errorf("диплинки: %s | %s", p.OpenAppLink("doc_1"), p.OpenAppLink(""))
	}
	for _, bad := range []string{"", "bad name", "a/b", "x?y", strings.Repeat("a", 65)} {
		if err := (domain.Profile{Username: bad}).Validate(); err == nil {
			t.Errorf("ник %q должен отвергаться", bad)
		}
	}
}
