package tui

import (
	"context"

	"github.com/archik008/archie-tg/internal/application/dto"
	setupclient "github.com/archik008/archie-tg/internal/setup/tg_client"
	tea "github.com/charmbracelet/bubbletea"
)

type AuthClient interface {
	SessionExists() bool
	SessionDir() string
	BeginAuth(ctx context.Context, appID int, appHash, phone string) (setupclient.BeginAuthResult, error)
	ResendAuthCode(ctx context.Context) (setupclient.BeginAuthResult, error)
	SubmitCode(ctx context.Context, code string) (bool, error)
	SubmitPassword(ctx context.Context, password string) error
	ResetAuthFlow()
	WaitIdle(ctx context.Context) error
}

type WhiteListClient interface {
	AddToWhiteList(ctx context.Context, a dto.AccountDTO) error
}

type BotRunner interface {
	Start(ctx context.Context, logs chan<- string) error
	Stop()
	IsRunning() bool
	WaitStopped(ctx context.Context) error
}

func Run(ctx context.Context, authClient AuthClient, whitelistClient WhiteListClient, bot BotRunner) error {
	p := tea.NewProgram(
		newModel(ctx, authClient, whitelistClient, bot),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
