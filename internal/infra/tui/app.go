package tui

import (
	"context"

	"github.com/archik008/archie-tg/internal/application/dto"
	tea "github.com/charmbracelet/bubbletea"
)

type AuthClient interface {
	SessionExists() bool
	BeginAuth(ctx context.Context, appID int, appHash, phone string) error
	SubmitCode(ctx context.Context, code string) (bool, error)
	SubmitPassword(ctx context.Context, password string) error
	ResetAuthFlow()
}

type WhiteListClient interface {
	AddToWhiteList(ctx context.Context, a dto.AccountDTO) error
}

func Run(ctx context.Context, authClient AuthClient, whitelistClient WhiteListClient) error {
	p := tea.NewProgram(
		newModel(ctx, authClient, whitelistClient),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
