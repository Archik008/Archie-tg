package whitelist

import "github.com/archik008/archie-tg/internal/domain/ports/out/repository/whitelist"

func NewWhiteListService(repo whitelist.UserWhiteListRepositoryPort) *WhiteListService {
	return &WhiteListService{
		whiteListRepo: repo,
	}
}
