package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/archik008/archie-tg/config"
	sqlitechat "github.com/archik008/archie-tg/internal/adapters/sqlite/chat"
	sqlitewhitelist "github.com/archik008/archie-tg/internal/adapters/sqlite/whitelist"
	tguserbot "github.com/archik008/archie-tg/internal/adapters/tg_userbot"
	spamchecker "github.com/archik008/archie-tg/internal/application/spam_checker"
	whitelistapp "github.com/archik008/archie-tg/internal/application/white_list"
	"github.com/archik008/archie-tg/internal/domain/entity/account"
	"github.com/archik008/archie-tg/internal/domain/ports/in"
	infratgclient "github.com/archik008/archie-tg/internal/infra/tg_client"
	"github.com/archik008/archie-tg/internal/infra/tui"
	"github.com/archik008/archie-tg/internal/queue"
	setupclient "github.com/archik008/archie-tg/internal/setup/tg_client"
	"github.com/gotd/td/tg"
)

func main() {
	sessionDir := os.Getenv("ARCHIE_SESSION_DIR")
	if sessionDir == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot determine executable path: %v\n", err)
			os.Exit(1)
		}
		sessionDir = filepath.Join(filepath.Dir(exe), ".archie")
	}

	client, err := setupclient.NewClient(sessionDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init tg client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dbPath := filepath.Join(sessionDir, "archie.db")
	chatRepo := sqlitechat.NewInSqliteChatRepository(dbPath)
	if err := chatRepo.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect sqlite chat repo: %v\n", err)
		os.Exit(1)
	}
	defer chatRepo.Close()

	whiteListRepo := sqlitewhitelist.NewInSqliteUserWhiteListRepository(dbPath)
	if err := whiteListRepo.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect sqlite whitelist repo: %v\n", err)
		os.Exit(1)
	}
	defer whiteListRepo.Close()

	whiteListService := whitelistapp.NewWhiteListService(whiteListRepo)

	botRuntime := infratgclient.NewRuntime(client, func(api *tg.Client, selfUserID int64) in.UserSpamCheckerPort {
		msgQueue := queue.NewInMemoryMsgQueue(ctx, 2*time.Second)
		tgCfg := loadTelegramCfgFromSession(client, api)
		chatDeleter := tguserbot.NewTgUserBotChatDeleter(api, tgCfg, msgQueue)
		userBlocker := tguserbot.NewTgUserBlocker(api, tgCfg, msgQueue)

		return spamchecker.NewSpamCheckerService(
			chatRepo,
			whiteListRepo,
			chatDeleter,
			userBlocker,
			account.NewAccount(int(selfUserID), "self"),
		)
	})

	if err := tui.Run(ctx, client, whiteListService, botRuntime); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func loadTelegramCfgFromSession(client *setupclient.Client, api *tg.Client) config.TelegramUserCfg {
	meta, err := client.LoadSessionMeta()
	if err != nil {
		return config.TelegramUserCfg{}
	}

	cfg := config.TelegramUserCfg{
		APP_ID:   meta.AppID,
		APP_HASH: meta.AppHash,
		TG_PHONE: meta.Phone,
	}

	lookupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	users, err := api.UsersGetUsers(lookupCtx, []tg.InputUserClass{&tg.InputUserSelf{}})
	if err != nil || len(users) == 0 {
		return cfg
	}

	if self, ok := users[0].(*tg.User); ok {
		cfg.USER_ACCESS_HASH = self.AccessHash
	}

	return cfg
}
