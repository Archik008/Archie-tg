package tgclient

import (
	"testing"

	"github.com/archik008/archie-tg/internal/application/dto"
	"github.com/gotd/td/tg"
)

func TestAccountFromMessage_PrivateText(t *testing.T) {
	entities := tg.Entities{
		Users: map[int64]*tg.User{
			42: {ID: 42, AccessHash: 99, Username: "spammer"},
		},
	}

	acc, kind, ok := AccountFromMessage(&tg.Message{
		Out:     false,
		PeerID:  &tg.PeerUser{UserID: 42},
		Message: "hello",
	}, entities, 1)

	if !ok {
		t.Fatal("expected message to be handled")
	}
	if kind != "text" {
		t.Fatalf("kind = %q, want text", kind)
	}
	if acc != (dto.AccountDTO{UserID: 42, Username: "spammer", AccessHash: 99}) {
		t.Fatalf("account = %+v", acc)
	}
}

func TestAccountFromMessage_SkipsOutgoing(t *testing.T) {
	_, _, ok := AccountFromMessage(&tg.Message{
		Out:    true,
		PeerID: &tg.PeerUser{UserID: 42},
	}, tg.Entities{}, 1)
	if ok {
		t.Fatal("expected outgoing message to be skipped")
	}
}

func TestAccountFromMessage_ServiceMessage(t *testing.T) {
	entities := tg.Entities{
		Users: map[int64]*tg.User{
			77: {ID: 77, AccessHash: 11, Username: "caller"},
		},
	}

	acc, kind, ok := AccountFromMessage(&tg.MessageService{
		Out:    false,
		PeerID: &tg.PeerUser{UserID: 77},
		Action: &tg.MessageActionPhoneCall{},
	}, entities, 1)

	if !ok {
		t.Fatal("expected service message to be handled")
	}
	if kind != "service:*tg.MessageActionPhoneCall" {
		t.Fatalf("kind = %q", kind)
	}
	if acc.UserID != 77 {
		t.Fatalf("user id = %d", acc.UserID)
	}
}
