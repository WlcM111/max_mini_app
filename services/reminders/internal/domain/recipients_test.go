package domain

import "testing"

func TestRecipientsFor(t *testing.T) {
	a := Member{AccountID: "a", Kind: "max", MaxUserID: 1, NotifyEnabled: true}
	b := Member{AccountID: "b", Kind: "max", MaxUserID: 2, NotifyEnabled: true}
	all := []Member{a, b}
	if got := RecipientsFor(Document{}, all); len(got) != 2 {
		t.Fatalf("без ответственного: %d получателей", len(got))
	}
	if got := RecipientsFor(Document{ResponsibleAccountID: "b"}, all); len(got) != 1 || got[0].AccountID != "b" {
		t.Fatalf("с ответственным: %+v", got)
	}
	if got := RecipientsFor(Document{ResponsibleAccountID: "z"}, all); len(got) != 2 {
		t.Fatalf("ответственный не участник: %d получателей", len(got))
	}
}
