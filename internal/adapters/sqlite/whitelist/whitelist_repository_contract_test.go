package whitelist_test

import (
	"path/filepath"
	"testing"

	sqlitewhitelist "github.com/archik008/archie-tg/internal/adapters/sqlite/whitelist"
	whitelistrepo "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"
	"github.com/archik008/archie-tg/internal/test/contracts"
)

func TestInSqliteWhiteListRepositoryContract(t *testing.T) {
	contracts.RunWhiteListRepositoryContractTests(t, func(t *testing.T) whitelistrepo.UserWhiteListRepositoryPort {
		t.Helper()

		repo := sqlitewhitelist.NewInSqliteUserWhiteListRepository(filepath.Join(t.TempDir(), "whitelist.db"))
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
