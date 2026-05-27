package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	whitelistrepo "github.com/archik008/archie-tg/internal/adapters/memory/repos/whitelist"
	whitelistapp "github.com/archik008/archie-tg/internal/application/white_list"
	"github.com/archik008/archie-tg/internal/infra/tui"
	tgclient "github.com/archik008/archie-tg/internal/setup/tg_client"
)

func main() {
	// Session files live next to the binary by default;
	// override via ARCHIE_SESSION_DIR env.
	sessionDir := os.Getenv("ARCHIE_SESSION_DIR")
	if sessionDir == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot determine executable path: %v\n", err)
			os.Exit(1)
		}
		sessionDir = filepath.Join(filepath.Dir(exe), ".archie")
	}

	client, err := tgclient.NewClient(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init tg client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	whiteListService := whitelistapp.NewWhiteListService(whitelistrepo.NewInMemoryUserWhiteListRepository())

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := tui.Run(ctx, client, whiteListService); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}
