package tgclient

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

var (
	ErrInvalidCredentials = errors.New("invalid telegram credentials")
	ErrCodeRequired       = errors.New("confirmation code is required")
	ErrPasswordRequired   = errors.New("2fa password is required")
	ErrAuthNotStarted     = errors.New("auth flow is not started")
	ErrClientNotRunning   = errors.New("telegram client is not running")
)

// Client wraps a gotd Telegram userbot client.
// It is safe to call from multiple goroutines.
type Client struct {
	sessionPath string

	mu       sync.Mutex
	pending  *pendingCreds
	tgClient *telegram.Client
	authAPI  *auth.Client
	api      *tg.Client

	// runCancel stops the background client goroutine.
	runCancel context.CancelFunc
}

type pendingCreds struct {
	appID    int
	appHash  string
	phone    string
	codeHash string // hash returned by SendCode, required for SignIn
}

// NewClient creates a Client that stores its session files in sessionDir.
func NewClient(sessionDir string) (*Client, error) {
	if strings.TrimSpace(sessionDir) == "" {
		return nil, errors.New("session dir is required")
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, err
	}
	return &Client{
		sessionPath: filepath.Join(sessionDir, "tg.session"),
	}, nil
}

// SessionExists reports whether a saved session file already exists.
func (c *Client) SessionExists() bool {
	_, err := os.Stat(c.sessionPath)
	return err == nil
}

// BeginAuth builds the Telegram client for the given credentials, connects
// to Telegram and sends an SMS code to the phone number.
// After this returns without error the user should call SubmitCode.
func (c *Client) BeginAuth(ctx context.Context, appID int, appHash, phone string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if appID <= 0 || strings.TrimSpace(appHash) == "" || strings.TrimSpace(phone) == "" {
		return ErrInvalidCredentials
	}

	c.mu.Lock()
	if c.runCancel != nil {
		c.runCancel()
	}
	c.mu.Unlock()

	clientCtx, cancel := context.WithCancel(context.Background())

	tgClient := telegram.NewClient(appID, strings.TrimSpace(appHash), telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: c.sessionPath},
	})

	apiReady := make(chan *tg.Client, 1)
	runErr := make(chan error, 1)

	go func() {
		runErr <- tgClient.Run(clientCtx, func(ctx context.Context) error {
			apiReady <- tgClient.API()
			<-ctx.Done()
			return nil
		})
	}()

	select {
	case err := <-runErr:
		cancel()
		return err
	case api := <-apiReady:
		authClient := auth.NewClient(api, rand.Reader, appID, strings.TrimSpace(appHash))

		// Send the code immediately so the user can enter it next.
		sent, err := authClient.SendCode(ctx, strings.TrimSpace(phone), auth.SendCodeOptions{})
		if err != nil {
			cancel()
			return err
		}
		sentCode, ok := sent.(*tg.AuthSentCode)
		if !ok {
			cancel()
			return errors.New("unexpected SendCode response type")
		}

		c.mu.Lock()
		c.tgClient = tgClient
		c.authAPI = authClient
		c.api = api
		c.runCancel = cancel
		c.pending = &pendingCreds{
			appID:    appID,
			appHash:  strings.TrimSpace(appHash),
			phone:    strings.TrimSpace(phone),
			codeHash: sentCode.PhoneCodeHash,
		}
		c.mu.Unlock()

		return nil

	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}

// SubmitCode passes the received SMS/app code to Telegram.
// Returns requires2FA=true if the account has a cloud password enabled.
func (c *Client) SubmitCode(ctx context.Context, code string) (requires2FA bool, err error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if strings.TrimSpace(code) == "" {
		return false, ErrCodeRequired
	}

	c.mu.Lock()
	creds := c.pending
	authAPI := c.authAPI
	c.mu.Unlock()

	if creds == nil || authAPI == nil {
		return false, ErrAuthNotStarted
	}

	_, err = authAPI.SignIn(ctx, creds.phone, strings.TrimSpace(code), creds.codeHash)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordAuthNeeded) {
			return true, nil
		}
		return false, err
	}

	c.mu.Lock()
	if err := c.savePendingMetaLocked(); err != nil {
		c.mu.Unlock()
		return false, err
	}
	c.pending = nil
	c.mu.Unlock()

	return false, nil
}

// SubmitPassword completes the 2FA step with the cloud password.
func (c *Client) SubmitPassword(ctx context.Context, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(password) == "" {
		return ErrPasswordRequired
	}

	c.mu.Lock()
	authAPI := c.authAPI
	c.mu.Unlock()

	if authAPI == nil {
		return ErrAuthNotStarted
	}

	if _, err := authAPI.Password(ctx, strings.TrimSpace(password)); err != nil {
		return err
	}

	c.mu.Lock()
	if err := c.savePendingMetaLocked(); err != nil {
		c.mu.Unlock()
		return err
	}
	c.pending = nil
	c.mu.Unlock()

	return nil
}

// ResetAuthFlow cancels any in-progress auth flow and shuts down the
// background client connection if one was started.
func (c *Client) ResetAuthFlow() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.runCancel != nil {
		c.runCancel()
		c.runCancel = nil
	}
	c.pending = nil
	c.tgClient = nil
	c.authAPI = nil
	c.api = nil
}

// API returns the low-level Telegram API client for use by adapters
// (ChatDeleter, UserBlocker). Returns nil if not yet connected.
func (c *Client) API() *tg.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.api
}

// Close cleanly shuts down the background Telegram connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runCancel != nil {
		c.runCancel()
		c.runCancel = nil
	}
}

// SessionHook is called after session is authorized and API is ready.
type SessionHook func(ctx context.Context, api *tg.Client) error

// RunSession connects with saved session and dispatches updates until ctx is canceled.
func (c *Client) RunSession(ctx context.Context, dispatcher tg.UpdateDispatcher, hook SessionHook) error {
	meta, err := c.LoadSessionMeta()
	if err != nil {
		return err
	}

	tgClient := telegram.NewClient(meta.AppID, meta.AppHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: c.sessionPath},
		UpdateHandler:  dispatcher,
	})

	return tgClient.Run(ctx, func(ctx context.Context) error {
		api := tgClient.API()

		c.mu.Lock()
		c.tgClient = tgClient
		c.api = api
		c.mu.Unlock()

		status, err := tgClient.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return errors.New("telegram session is not authorized")
		}

		if hook == nil {
			<-ctx.Done()
			return ctx.Err()
		}
		return hook(ctx, api)
	})
}

// EnsureSelfUserID resolves and persists current Telegram account id.
func (c *Client) EnsureSelfUserID(ctx context.Context, api *tg.Client) (int64, error) {
	meta, err := c.LoadSessionMeta()
	if err != nil {
		return 0, err
	}
	if meta.SelfUserID != 0 {
		return meta.SelfUserID, nil
	}

	users, err := api.UsersGetUsers(ctx, []tg.InputUserClass{&tg.InputUserSelf{}})
	if err != nil {
		return 0, err
	}
	user, ok := users[0].(*tg.User)
	if !ok {
		return 0, errors.New("cannot resolve self user")
	}

	meta.SelfUserID = user.ID
	if err := c.SaveSessionMeta(meta); err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (c *Client) savePendingMetaLocked() error {
	if c.pending == nil {
		return ErrAuthNotStarted
	}
	return c.SaveSessionMeta(SessionMeta{
		AppID:   c.pending.appID,
		AppHash: c.pending.appHash,
		Phone:   c.pending.phone,
	})
}
