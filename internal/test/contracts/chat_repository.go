package contracts

import (
	"errors"
	"reflect"
	"testing"

	chatagg "github.com/archik008/archie-tg/internal/domain/aggregate/chat"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	chatrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
)

type ChatRepositoryFactory func(t *testing.T) chatrepo.ChatRepositoryPort

func RunChatRepositoryContractTests(t *testing.T, newRepo ChatRepositoryFactory) {
	t.Helper()

	t.Run("Create stores chat and Get returns it", func(t *testing.T) {
		repo := newRepo(t)
		want := mustNewChat(t, 101)

		if err := repo.Create(want); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		got, err := repo.Get(want.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Get() chat mismatch: got %+v want %+v", got, want)
		}
	})

	t.Run("Create returns ErrChatExists for duplicate IDs", func(t *testing.T) {
		repo := newRepo(t)
		existing := mustNewChat(t, 102)

		if err := repo.Create(existing); err != nil {
			t.Fatalf("Create() first call error = %v", err)
		}

		err := repo.Create(existing)
		if !errors.Is(err, chatrepo.ErrChatExists) {
			t.Fatalf("Create() duplicate error = %v, want %v", err, chatrepo.ErrChatExists)
		}
	})

	t.Run("Get returns ErrChatNotFound for missing chat", func(t *testing.T) {
		repo := newRepo(t)

		_, err := repo.Get(999)
		if !errors.Is(err, chatrepo.ErrChatNotFound) {
			t.Fatalf("Get() missing error = %v, want %v", err, chatrepo.ErrChatNotFound)
		}
	})

	t.Run("Delete removes previously stored chat", func(t *testing.T) {
		repo := newRepo(t)
		existing := mustNewChat(t, 103)

		if err := repo.Create(existing); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if err := repo.Delete(existing); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err := repo.Get(existing.ID)
		if !errors.Is(err, chatrepo.ErrChatNotFound) {
			t.Fatalf("Get() after delete error = %v, want %v", err, chatrepo.ErrChatNotFound)
		}
	})

	t.Run("Delete returns ErrChatNotFound for missing chat", func(t *testing.T) {
		repo := newRepo(t)

		err := repo.Delete(mustNewChat(t, 104))
		if !errors.Is(err, chatrepo.ErrChatNotFound) {
			t.Fatalf("Delete() missing error = %v, want %v", err, chatrepo.ErrChatNotFound)
		}
	})
}

func mustNewChat(t *testing.T, id int) chatagg.Chat {
	t.Helper()

	userChat, err := chatagg.NewChat(
		account.NewAccount(1, "base"),
		account.NewAccount(id, "user"),
	)
	if err != nil {
		t.Fatalf("NewChat() error = %v", err)
	}

	return userChat.WithID(id)
}
