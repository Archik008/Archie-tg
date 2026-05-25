package chat_test

import (
	"path/filepath"
	"testing"

	sqlitechat "github.com/archik008/archie-tg/internal/adapters/sqlite/chat"
	chatrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
	"github.com/archik008/archie-tg/internal/test/contracts"
)

func TestInSqliteChatRepositoryContract(t *testing.T) {
	contracts.RunChatRepositoryContractTests(t, func(t *testing.T) chatrepo.ChatRepositoryPort {
		t.Helper()

		repo := sqlitechat.NewInSqliteChatRepository(filepath.Join(t.TempDir(), "chat.db"))
		if err := repo.Connect(); err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
		t.Cleanup(func() {
			if err := repo.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})

		return repo
	})
}
