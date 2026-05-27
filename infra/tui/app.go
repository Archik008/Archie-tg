package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

type AuthClient interface {
	SessionExists() bool
	BeginAuth(ctx context.Context, appID int, appHash, phone string) error
	SubmitCode(ctx context.Context, code string) (bool, error)
	SubmitPassword(ctx context.Context, password string) error
	ResetAuthFlow()
}

func Run(ctx context.Context, authClient AuthClient) error {
	p := tea.NewProgram(
		newModel(ctx, authClient),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
