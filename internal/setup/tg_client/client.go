package tgclient

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	sessionDir  string
	sessionPath string

	mu        sync.Mutex
	pending   *pendingCreds
	activeRun *runHandle
	tgClient  *telegram.Client
	authAPI   *auth.Client
	api       *tg.Client
}

type runHandle struct {
	cancel context.CancelFunc
	done   chan struct{}
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
		sessionDir:  sessionDir,
		sessionPath: filepath.Join(sessionDir, "tg.session"),
	}, nil
}

// SessionDir returns the directory where session files are stored.
func (c *Client) SessionDir() string {
	return c.sessionDir
}

// SessionExists reports whether a saved session file already exists.
func (c *Client) SessionExists() bool {
	_, err := os.Stat(c.sessionPath)
	return err == nil
}

func (c *Client) pendingSessionPath() string {
	return c.sessionPath + ".pending"
}

// BeginAuth builds the Telegram client for the given credentials, connects
// to Telegram and sends a code to the phone number.
// Returns the normalized phone that was sent to Telegram API.
func (c *Client) BeginAuth(ctx context.Context, appID int, appHash, phone string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	appHash = strings.TrimSpace(appHash)
	phone = normalizePhone(phone)
	if appID <= 0 || appHash == "" || phone == "" {
		return "", ErrInvalidCredentials
	}

	c.stopActiveRunAndWait()

	pendingPath := c.pendingSessionPath()
	if err := c.removeSessionFiles(pendingPath); err != nil {
		return "", err
	}

	clientCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	c.mu.Lock()
	c.activeRun = &runHandle{cancel: cancel, done: done}
	c.mu.Unlock()

	tgClient := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: pendingPath},
	})

	apiReady := make(chan *tg.Client, 1)
	runErr := make(chan error, 1)

	go func() {
		defer close(done)
		runErr <- tgClient.Run(clientCtx, func(ctx context.Context) error {
			apiReady <- tgClient.API()
			<-ctx.Done()
			return nil
		})
	}()

	select {
	case err := <-runErr:
		c.finishRun(done)
		_ = c.removeSessionFiles(pendingPath)
		return "", err
	case api := <-apiReady:
		authClient := auth.NewClient(api, rand.Reader, appID, appHash)

		sendCtx, sendCancel := context.WithTimeout(context.Background(), 45*time.Second)
		sent, err := authClient.SendCode(sendCtx, phone, auth.SendCodeOptions{})
		sendCancel()
		if err != nil {
			c.stopActiveRunAndWait()
			_ = c.removeSessionFiles(pendingPath)
			return "", err
		}
		sentCode, ok := sent.(*tg.AuthSentCode)
		if !ok {
			c.stopActiveRunAndWait()
			_ = c.removeSessionFiles(pendingPath)
			return "", errors.New("unexpected SendCode response type")
		}

		c.mu.Lock()
		c.tgClient = tgClient
		c.authAPI = authClient
		c.api = api
		c.pending = &pendingCreds{
			appID:    appID,
			appHash:  appHash,
			phone:    phone,
			codeHash: sentCode.PhoneCodeHash,
		}
		c.mu.Unlock()

		return phone, nil

	case <-ctx.Done():
		c.stopActiveRunAndWait()
		_ = c.removeSessionFiles(pendingPath)
		return "", ctx.Err()
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

	return false, c.completeAuth(creds)
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
	creds := c.pending
	authAPI := c.authAPI
	c.mu.Unlock()

	if creds == nil || authAPI == nil {
		return ErrAuthNotStarted
	}

	if _, err := authAPI.Password(ctx, strings.TrimSpace(password)); err != nil {
		return err
	}

	return c.completeAuth(creds)
}

func (c *Client) completeAuth(creds *pendingCreds) error {
	c.stopActiveRunAndWait()

	if err := c.promotePendingSession(); err != nil {
		return err
	}
	if err := c.SaveSessionMeta(SessionMeta{
		AppID:   creds.appID,
		AppHash: creds.appHash,
		Phone:   creds.phone,
	}); err != nil {
		return err
	}

	c.mu.Lock()
	c.pending = nil
	c.mu.Unlock()
	return nil
}

// ResetAuthFlow cancels any in-progress auth flow and removes pending session files.
func (c *Client) ResetAuthFlow() {
	c.stopActiveRunAndWait()
	_ = c.removeSessionFiles(c.pendingSessionPath())

	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = nil
	c.tgClient = nil
	c.authAPI = nil
	c.api = nil
}

// WaitIdle blocks until the active Telegram connection is fully stopped.
func (c *Client) WaitIdle(ctx context.Context) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		c.mu.Lock()
		running := c.activeRun != nil
		c.mu.Unlock()
		if !running {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
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
	c.stopActiveRunAndWait()
}

// SessionHook is called after session is authorized and API is ready.
type SessionHook func(ctx context.Context, api *tg.Client) error

// RunSession connects with saved session and dispatches updates until ctx is canceled.
func (c *Client) RunSession(ctx context.Context, dispatcher tg.UpdateDispatcher, hook SessionHook) error {
	meta, err := c.LoadSessionMeta()
	if err != nil {
		return err
	}

	c.stopActiveRunAndWait()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	c.mu.Lock()
	c.activeRun = &runHandle{cancel: cancel, done: done}
	c.mu.Unlock()

	tgClient := telegram.NewClient(meta.AppID, meta.AppHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: c.sessionPath},
		UpdateHandler:  dispatcher,
	})

	var runErr error
	go func() {
		defer close(done)
		runErr = tgClient.Run(runCtx, func(ctx context.Context) error {
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
	}()

	<-done

	c.mu.Lock()
	c.activeRun = nil
	c.tgClient = nil
	c.authAPI = nil
	c.api = nil
	c.mu.Unlock()

	return runErr
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

func (c *Client) stopActiveRunAndWait() {
	c.mu.Lock()
	handle := c.activeRun
	c.mu.Unlock()
	if handle == nil {
		return
	}

	handle.cancel()
	<-handle.done

	c.mu.Lock()
	if c.activeRun == handle {
		c.activeRun = nil
	}
	c.mu.Unlock()
}

func (c *Client) finishRun(done chan struct{}) {
	<-done
	c.mu.Lock()
	c.activeRun = nil
	c.mu.Unlock()
}

func (c *Client) promotePendingSession() error {
	pending := c.pendingSessionPath()
	if err := c.removeSessionFiles(c.sessionPath); err != nil {
		return err
	}
	if err := os.Rename(pending, c.sessionPath); err != nil {
		return err
	}
	return c.removeSessionFiles(pending)
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return phone
	}
	if !strings.HasPrefix(phone, "+") {
		return "+" + phone
	}
	return phone
}

func (c *Client) removeSessionFiles(basePath string) error {
	paths := []string{
		basePath,
		basePath + "-journal",
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
