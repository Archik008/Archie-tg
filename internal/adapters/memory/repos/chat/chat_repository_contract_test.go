package chat_test

import (
	"testing"

	memorychat "github.com/archik008/archie-tg/internal/adapters/memory/repos/chat"
	chatrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/chat"
	"github.com/archik008/archie-tg/internal/test/contracts"
)

func TestInMemoryChatRepositoryContract(t *testing.T) {
	contracts.RunChatRepositoryContractTests(t, func(t *testing.T) chatrepo.ChatRepositoryPort {
		t.Helper()
		return memorychat.NewInMemoryChatRepository()
	})
}
