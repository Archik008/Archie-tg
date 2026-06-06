package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/archik008/archie-tg/internal/application/dto"
	setupclient "github.com/archik008/archie-tg/internal/setup/tg_client"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	screenMenu screen = iota
	screenAuthAppID
	screenAuthAppHash
	screenAuthPhone
	screenAuthLoading
	screenAuthCode
	screenAuthPassword
	screenWhitelistUserID
	screenWhitelistUsername
	screenLogs
)

type menuItem int

const (
	menuAuthorize menuItem = iota
	menuExit
	menuAddWhitelist
	menuStart
)

var menuItems = []string{
	"Authorize",
	"Exit",
	"Add user to whitelist",
	"Start application",
}

type model struct {
	ctx             context.Context
	authClient      AuthClient
	whitelistClient WhiteListClient
	bot             BotRunner

	screen     screen
	menuCursor int
	input      textinput.Model

	// Kept between steps of multi-screen flows.
	appID       string
	appHash     string
	phone       string
	code        string
	password    string
	whiteUserID string

	status     string
	loadingFor string
	loadingIdx int

	logs  []string
	logCh chan string
}

type authBeginDoneMsg struct {
	result setupclient.BeginAuthResult
	err    error
}
type authResendDoneMsg struct {
	result setupclient.BeginAuthResult
	err    error
}
type authCodeDoneMsg struct {
	requires2FA bool
	err         error
}
type authPasswordDoneMsg struct{ err error }
type whitelistAddDoneMsg struct{ err error }
type logLineMsg string
type logClosedMsg struct{}
type loadingTickMsg time.Time

func newModel(ctx context.Context, authClient AuthClient, whitelistClient WhiteListClient, bot BotRunner) model {
	return model{
		ctx:             ctx,
		authClient:      authClient,
		whitelistClient: whitelistClient,
		bot:             bot,
		screen:          screenMenu,
		status:          "Choose an action",
		logs:            make([]string, 0, 64),
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) isInputScreen() bool {
	switch m.screen {
	case screenAuthAppID, screenAuthAppHash, screenAuthPhone,
		screenAuthCode, screenAuthPassword,
		screenWhitelistUserID, screenWhitelistUsername:
		return true
	default:
		return false
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.isInputScreen() {
		m.input, cmd = m.input.Update(msg)
	}

	switch typed := msg.(type) {
	case tea.KeyMsg:
		if m.isInputScreen() {
			var submitCmd tea.Cmd
			m, submitCmd = m.handleInputKey(typed)
			return m, tea.Batch(cmd, submitCmd)
		}
		return m.onKey(typed)
	case authBeginDoneMsg:
		return m.onAuthCodeReady(typed.result, typed.err)
	case authResendDoneMsg:
		return m.onAuthCodeReady(typed.result, typed.err)
	case authCodeDoneMsg:
		if typed.err != nil {
			m.status = "Code error: " + typed.err.Error()
			m.screen = screenAuthCode
			m.input = newTextInput("", false)
			return m, textinput.Blink
		}
		if typed.requires2FA {
			m.screen = screenAuthPassword
			m.status = "Enter 2FA password"
			m.input = newTextInput("", true)
			return m, textinput.Blink
		}
		m.screen = screenMenu
		m.status = "Authorization complete. Back to menu."
		return m, nil
	case authPasswordDoneMsg:
		if typed.err != nil {
			m.status = "2FA error: " + typed.err.Error()
			m.screen = screenAuthPassword
			m.input = newTextInput("", true)
			return m, textinput.Blink
		}
		m.screen = screenMenu
		m.status = "Authorization complete. Back to menu."
		return m, nil
	case whitelistAddDoneMsg:
		m.screen = screenMenu
		if typed.err != nil {
			m.status = "Whitelist error: " + typed.err.Error()
			return m, nil
		}
		m.status = "User added to whitelist."
		return m, nil
	case logLineMsg:
		if m.screen != screenLogs {
			return m, nil
		}
		m.logs = append(m.logs, string(typed))
		if len(m.logs) > 18 {
			m.logs = m.logs[len(m.logs)-18:]
		}
		return m, waitLogLine(m.logCh)
	case logClosedMsg:
		if m.screen == screenLogs {
			m.status = "Bot stopped"
		}
		return m, nil
	case loadingTickMsg:
		if m.screen != screenAuthLoading {
			return m, nil
		}
		m.loadingIdx = (m.loadingIdx + 1) % len(spinnerFrames)
		return m, tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) })
	}

	return m, cmd
}

func (m model) handleInputKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.authClient.ResetAuthFlow()
		m.screen = screenMenu
		m.status = "Back to menu."
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.status = "Field must not be empty"
			return m, nil
		}
		return m.submitInput(value)
	case "r":
		if m.screen == screenAuthCode {
			m.screen = screenAuthLoading
			m.loadingFor = "Resending code..."
			return m, tea.Batch(
				tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
				m.cmdResendAuthCode(),
			)
		}
	}
	return m, nil
}

func (m model) onAuthCodeReady(result setupclient.BeginAuthResult, err error) (model, tea.Cmd) {
	if err != nil {
		if m.screen == screenAuthLoading {
			m.screen = screenAuthCode
			m.status = "Auth error: " + err.Error()
			m.input = newTextInput("", false)
			return m, textinput.Blink
		}
		m.status = "Initialization error: " + err.Error()
		m.screen = screenMenu
		return m, nil
	}

	m.screen = screenAuthCode
	status := fmt.Sprintf(
		"API %d / %s | %s | delivery: %s.",
		result.AppID,
		result.AppHashPrefix,
		result.Phone,
		result.CodeDelivery,
	)
	if result.CodeHint != "" {
		status += " " + result.CodeHint
	}
	status += " Press r to resend. Enter code below."
	m.status = status
	m.input = newTextInput("", false)
	return m, textinput.Blink
}

func (m model) submitInput(value string) (model, tea.Cmd) {
	switch m.screen {
	case screenAuthAppID:
		m.appID = value
		m.screen = screenAuthAppHash
		m.status = "Step 2/5: app_hash"
		m.input = newTextInput("app_hash", false)
		return m, textinput.Blink
	case screenAuthAppHash:
		m.appHash = value
		m.screen = screenAuthPhone
		m.status = "Step 3/5: phone"
		m.input = newTextInput("+79991234567", false)
		return m, textinput.Blink
	case screenAuthPhone:
		m.phone = value
		m.screen = screenAuthLoading
		m.loadingFor = "Initializing client..."
		m.status = "Step 4/5: requesting code"
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdBeginAuth(),
		)
	case screenAuthCode:
		m.code = value
		m.screen = screenAuthLoading
		m.loadingFor = "Verifying code..."
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdSubmitCode(),
		)
	case screenAuthPassword:
		m.password = value
		m.screen = screenAuthLoading
		m.loadingFor = "Verifying 2FA..."
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdSubmitPassword(),
		)
	case screenWhitelistUserID:
		m.whiteUserID = value
		m.screen = screenWhitelistUsername
		m.status = "Enter user username"
		m.input = newTextInput("username", false)
		return m, textinput.Blink
	case screenWhitelistUsername:
		m.screen = screenAuthLoading
		m.loadingFor = "Adding to whitelist..."
		username := strings.TrimPrefix(value, "@")
		m.input.SetValue(username)
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdAddToWhitelist(),
		)
	}
	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenMenu:
		return m.viewMenu()
	case screenAuthAppID:
		return m.viewInputScreen("Enter app_id")
	case screenAuthAppHash:
		return m.viewInputScreen("Enter app_hash")
	case screenAuthPhone:
		return m.viewInputScreen("Enter phone (+79991234567 or 89991234567 for Russia)")
	case screenAuthLoading:
		return fmt.Sprintf("%s %s\n\n%s\n\nEsc: back to menu", spinnerFrames[m.loadingIdx], m.loadingFor, m.status)
	case screenAuthCode:
		return m.viewInputScreen("Enter confirmation code (check API/phone in status below)")
	case screenAuthPassword:
		return m.viewInputScreen("Enter 2FA password")
	case screenWhitelistUserID:
		return m.viewInputScreen("Enter user_id")
	case screenWhitelistUsername:
		return m.viewInputScreen("Enter username (without @)")
	case screenLogs:
		return m.viewLogs()
	default:
		return "unknown screen"
	}
}

func (m model) onKey(msg tea.KeyMsg) (model, tea.Cmd) {
	switch m.screen {
	case screenMenu:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < len(menuItems)-1 {
				m.menuCursor++
			}
		case "enter":
			switch menuItem(m.menuCursor) {
			case menuAuthorize:
				if m.bot != nil && m.bot.IsRunning() {
					m.bot.Stop()
					waitCtx, cancel := context.WithTimeout(m.ctx, 15*time.Second)
					_ = m.bot.WaitStopped(waitCtx)
					cancel()
				}
				m.authClient.ResetAuthFlow()
				_ = m.authClient.WaitIdle(m.ctx)

				m.screen = screenAuthAppID
				sessionNote := ""
				if m.authClient.SessionExists() {
					sessionNote = " Existing session stays until login succeeds."
				}
				m.status = fmt.Sprintf("Step 1/5: app_id.%s Data: %s", sessionNote, m.authClient.SessionDir())
				m.appID, m.appHash, m.phone, m.code, m.password = "", "", "", "", ""
				m.input = newTextInput("123456", false)
				return m, textinput.Blink
			case menuExit:
				return m, tea.Quit
			case menuAddWhitelist:
				m.screen = screenWhitelistUserID
				m.whiteUserID = ""
				m.status = "Enter user ID"
				m.input = newTextInput("123456789", false)
				return m, textinput.Blink
			case menuStart:
				if !m.authClient.SessionExists() {
					m.status = "No account session. Authorize first."
					return m, nil
				}
				if m.bot == nil {
					m.status = "Bot is not initialized."
					return m, nil
				}
				if m.bot.IsRunning() {
					m.status = "Bot is already running."
					return m, nil
				}
				m.logCh = make(chan string, 128)
				if err := m.bot.Start(m.ctx, m.logCh); err != nil {
					m.status = "Failed to start bot: " + err.Error()
					return m, nil
				}
				m.screen = screenLogs
				m.logs = []string{"Starting application...", "Waiting for Telegram events..."}
				return m, waitLogLine(m.logCh)
			}
		}
		return m, nil
	case screenAuthLoading:
		if msg.String() == "esc" {
			m.authClient.ResetAuthFlow()
			m.screen = screenMenu
			m.status = "Authorization cancelled."
		}
		return m, nil
	case screenLogs:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.bot != nil {
				m.bot.Stop()
			}
			m.screen = screenMenu
			m.status = "Back to main menu."
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m model) cmdBeginAuth() tea.Cmd {
	appIDRaw := strings.TrimSpace(m.appID)
	appHash := strings.TrimSpace(m.appHash)
	phone := strings.TrimSpace(m.phone)

	return func() tea.Msg {
		appID, err := strconv.Atoi(appIDRaw)
		if err != nil {
			return authBeginDoneMsg{err: fmt.Errorf("app_id must be a number")}
		}
		sent, err := m.authClient.BeginAuth(m.ctx, appID, appHash, phone)
		return authBeginDoneMsg{result: sent, err: err}
	}
}

func (m model) cmdResendAuthCode() tea.Cmd {
	return func() tea.Msg {
		result, err := m.authClient.ResendAuthCode(m.ctx)
		return authResendDoneMsg{result: result, err: err}
	}
}

func (m model) cmdSubmitCode() tea.Cmd {
	code := strings.TrimSpace(m.code)
	return func() tea.Msg {
		need2FA, err := m.authClient.SubmitCode(m.ctx, code)
		return authCodeDoneMsg{requires2FA: need2FA, err: err}
	}
}

func (m model) cmdSubmitPassword() tea.Cmd {
	password := strings.TrimSpace(m.password)
	return func() tea.Msg {
		return authPasswordDoneMsg{err: m.authClient.SubmitPassword(m.ctx, password)}
	}
}

func (m model) cmdAddToWhitelist() tea.Cmd {
	userIDRaw := strings.TrimSpace(m.whiteUserID)
	username := strings.TrimSpace(strings.TrimPrefix(m.input.Value(), "@"))

	return func() tea.Msg {
		userID, err := strconv.Atoi(userIDRaw)
		if err != nil {
			return whitelistAddDoneMsg{err: fmt.Errorf("user_id must be a number")}
		}
		err = m.whitelistClient.AddToWhiteList(m.ctx, dto.AccountDTO{
			UserID:   userID,
			Username: username,
		})
		return whitelistAddDoneMsg{err: err}
	}
}

func (m model) viewMenu() string {
	var b strings.Builder
	b.WriteString("Archie TG - Main Menu\n\n")
	for i, item := range menuItems {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "> "
		}
		b.WriteString(cursor + item + "\n")
	}
	b.WriteString("\n")
	b.WriteString(m.status + "\n")
	b.WriteString("\n↑/↓ select • Enter confirm • q quit\n")
	return b.String()
}

func (m model) viewInputScreen(title string) string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\nEnter: next • Esc: back",
		title,
		m.input.View(),
		m.status,
	)
}

func (m model) viewLogs() string {
	var b strings.Builder
	b.WriteString("Log mode (application running)\n\n")
	for _, line := range m.logs {
		b.WriteString(line + "\n")
	}
	b.WriteString("\nEsc: back to menu • q: quit\n")
	return b.String()
}

func newTextInput(placeholder string, password bool) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 128
	ti.Width = 50
	if password {
		ti.EchoMode = textinput.EchoPassword
		ti.EchoCharacter = '*'
	}
	return ti
}

func waitLogLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return logClosedMsg{}
		}
		line, ok := <-ch
		if !ok {
			return logClosedMsg{}
		}
		return logLineMsg(line)
	}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
