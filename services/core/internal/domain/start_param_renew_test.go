package domain

import "testing"

func TestParseStartTargetRenew(t *testing.T) {
	id := "3f2a9c1e-5b7d-4e8a-9c21-7d4e5f6a8b90"
	if got := ParseStartTarget("renew_" + id); got.Kind != StartRenew || got.DocumentID != id {
		t.Fatalf("renew_: получили %+v", got)
	}
	if got := ParseStartTarget("renew_bad"); got.Kind != StartNone {
		t.Fatalf("renew_ с неверным id: получили %+v", got)
	}
}
