package tgclient

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
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

	mu           sync.Mutex
	pending      *pendingCreds
	activeRun    *runHandle
	authMemStore *session.StorageMemory
	tgClient     *telegram.Client
	authAPI      *auth.Client
	api          *tg.Client
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

// BeginAuth builds the Telegram client for the given credentials, connects
// to Telegram and sends a code to the phone number.
func (c *Client) BeginAuth(ctx context.Context, appID int, appHash, phone string) (BeginAuthResult, error) {
	result := BeginAuthResult{
		AppID:         appID,
		AppHashPrefix: maskAppHash(appHash),
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}

	appHash = strings.TrimSpace(appHash)
	phone = normalizePhone(phone)
	result.Phone = phone
	result.AppHashPrefix = maskAppHash(appHash)

	if err := validatePhone(phone); err != nil {
		c.writeAuthAudit(authAuditRecord{
			AppID:         appID,
			AppHashPrefix: result.AppHashPrefix,
			Phone:         phone,
			Error:         err.Error(),
		})
		return result, err
	}

	if err := validateAppCredentials(appID, appHash); err != nil {
		c.writeAuthAudit(authAuditRecord{
			AppID:         appID,
			AppHashPrefix: result.AppHashPrefix,
			Phone:         phone,
			Error:         err.Error(),
		})
		return result, err
	}

	c.stopActiveRunAndWait()
	c.clearAuthMemoryStore()

	memStore := &session.StorageMemory{}
	clientCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	c.mu.Lock()
	c.activeRun = &runHandle{cancel: cancel, done: done}
	c.authMemStore = memStore
	c.mu.Unlock()

	tgClient := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: memStore,
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

	fail := func(err error) (BeginAuthResult, error) {
		c.stopActiveRunAndWait()
		c.clearAuthMemoryStore()
		err = formatTelegramAuthError(err)
		c.writeAuthAudit(authAuditRecord{
			AppID:         appID,
			AppHashPrefix: result.AppHashPrefix,
			Phone:         phone,
			Error:         err.Error(),
		})
		return result, err
	}

	select {
	case err := <-runErr:
		c.finishRun(done)
		return fail(err)
	case api := <-apiReady:
		authClient := auth.NewClient(api, rand.Reader, appID, appHash)

		sendCtx, sendCancel := context.WithTimeout(context.Background(), 45*time.Second)
		sent, err := authClient.SendCode(sendCtx, phone, auth.SendCodeOptions{})
		sendCancel()
		if err != nil {
			return fail(err)
		}

		delivery := codeDeliveryMessage(sent)
		result.CodeDelivery = delivery
		result.CodeHint = codeDeliveryHint(sent)

		switch sent.(type) {
		case *tg.AuthSentCodeSuccess:
			err := errors.New("telegram did not send a new code because this auth key is already authorized")
			return fail(err)
		}

		sentCode, ok := sent.(*tg.AuthSentCode)
		if !ok {
			return fail(errors.New("unexpected SendCode response type"))
		}

		if timeout, ok := sentCode.GetTimeout(); ok {
			result.ResendAfter = timeout
		}
		if next, ok := sentCode.GetNextType(); ok {
			if nextMsg := nextDeliveryMessage(next); nextMsg != "" {
				result.CodeHint += fmt.Sprintf(" If nothing arrives in %d sec, next try: %s.", result.ResendAfter, nextMsg)
			}
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

		c.writeAuthAudit(authAuditRecord{
			AppID:         appID,
			AppHashPrefix: result.AppHashPrefix,
			Phone:         phone,
			CodeDelivery:  delivery,
		})

		return result, nil

	case <-ctx.Done():
		c.stopActiveRunAndWait()
		c.clearAuthMemoryStore()
		return result, ctx.Err()
	}
}

// ResendAuthCode asks Telegram to resend the login code using the next delivery method.
func (c *Client) ResendAuthCode(ctx context.Context) (BeginAuthResult, error) {
	c.mu.Lock()
	creds := c.pending
	authAPI := c.authAPI
	c.mu.Unlock()

	result := BeginAuthResult{}
	if creds == nil || authAPI == nil {
		return result, ErrAuthNotStarted
	}

	result.Phone = creds.phone
	result.AppID = creds.appID
	result.AppHashPrefix = maskAppHash(creds.appHash)

	sent, err := authAPI.ResendCode(ctx, creds.phone, creds.codeHash)
	if err != nil {
		return result, formatTelegramAuthError(err)
	}

	result.CodeDelivery = codeDeliveryMessage(sent)
	result.CodeHint = codeDeliveryHint(sent)

	sentCode, ok := sent.(*tg.AuthSentCode)
	if !ok {
		return result, errors.New("unexpected ResendCode response type")
	}

	c.mu.Lock()
	c.pending.codeHash = sentCode.PhoneCodeHash
	c.mu.Unlock()

	if timeout, ok := sentCode.GetTimeout(); ok {
		result.ResendAfter = timeout
	}
	if next, ok := sentCode.GetNextType(); ok {
		if nextMsg := nextDeliveryMessage(next); nextMsg != "" {
			result.CodeHint += fmt.Sprintf(" If nothing arrives in %d sec, next try: %s.", result.ResendAfter, nextMsg)
		}
	}

	return result, nil
}

func formatTelegramAuthError(err error) error {
	if err == nil {
		return nil
	}
	if wait, ok := tgerr.AsFloodWait(err); ok {
		return fmt.Errorf("telegram rate limit: wait %ds before trying again", int(wait.Seconds()))
	}
	if tgErr, ok := tgerr.As(err); ok {
		switch tgErr.Type {
		case "PHONE_NUMBER_INVALID":
			return fmt.Errorf("invalid phone number format, use +79991234567 for Russia")
		case "PHONE_NUMBER_BANNED":
			return fmt.Errorf("this phone number is banned in Telegram")
		case "PHONE_NUMBER_FLOOD":
			return fmt.Errorf("too many attempts for this phone number, try later")
		case "PHONE_CODE_EXPIRED":
			return fmt.Errorf("code expired, start authorization again")
		case "API_ID_INVALID", "API_ID_PUBLISHED_FLOOD":
			return fmt.Errorf("invalid app_id/app_hash pair from my.telegram.org")
		}
	}
	return err
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

	if err := c.persistAuthMemoryStore(); err != nil {
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

func (c *Client) persistAuthMemoryStore() error {
	c.mu.Lock()
	memStore := c.authMemStore
	c.mu.Unlock()

	if memStore == nil {
		return errors.New("auth session memory store is empty")
	}
	if err := c.removeSessionFiles(c.sessionPath); err != nil {
		return err
	}
	if err := memStore.WriteFile(c.sessionPath, 0o600); err != nil {
		return err
	}
	c.clearAuthMemoryStore()
	return nil
}

func (c *Client) clearAuthMemoryStore() {
	c.mu.Lock()
	c.authMemStore = nil
	c.mu.Unlock()
	_ = c.removeSessionFiles(c.sessionPath + ".pending")
}

// ResetAuthFlow cancels any in-progress auth flow.
func (c *Client) ResetAuthFlow() {
	c.stopActiveRunAndWait()
	c.clearAuthMemoryStore()

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
