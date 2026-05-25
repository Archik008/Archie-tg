package whitelist_test

import (
	"testing"

	memorywhitelist "github.com/archik008/archie-tg/internal/adapters/memory/repos/whitelist"
	whitelistrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
	"github.com/archik008/archie-tg/internal/test/contracts"
)

func TestInMemoryWhiteListRepositoryContract(t *testing.T) {
	contracts.RunWhiteListRepositoryContractTests(t, func(t *testing.T) whitelistrepo.UserWhiteListRepositoryPort {
		t.Helper()
		return memorywhitelist.NewInMemoryUserWhiteListRepository()
	})
}
