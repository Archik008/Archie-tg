package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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
	ctx        context.Context
	authClient AuthClient

	screen     screen
	menuCursor int

	appID    string
	appHash  string
	phone    string
	code     string
	password string

	status     string
	loadingFor string
	loadingIdx int

	logs []string
}

type authBeginDoneMsg struct{ err error }
type authCodeDoneMsg struct {
	requires2FA bool
	err         error
}
type authPasswordDoneMsg struct{ err error }
type logTickMsg time.Time
type loadingTickMsg time.Time

func newModel(ctx context.Context, authClient AuthClient) model {
	return model{
		ctx:        ctx,
		authClient: authClient,
		screen:     screenMenu,
		status:     "Choose an action",
		logs:       make([]string, 0, 64),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		return m.onKey(typed)
	case authBeginDoneMsg:
		if typed.err != nil {
			m.status = "Initialization error: " + typed.err.Error()
			m.screen = screenMenu
			return m, nil
		}
		m.screen = screenAuthCode
		m.code = ""
		m.status = "Enter confirmation code"
		return m, nil
	case authCodeDoneMsg:
		if typed.err != nil {
			m.status = "Code error: " + typed.err.Error()
			m.screen = screenAuthCode
			return m, nil
		}
		if typed.requires2FA {
			m.screen = screenAuthPassword
			m.password = ""
			m.status = "Enter 2FA password"
			return m, nil
		}
		m.screen = screenMenu
		m.status = "Authorization complete. Back to menu."
		return m, nil
	case authPasswordDoneMsg:
		if typed.err != nil {
			m.status = "2FA error: " + typed.err.Error()
			m.screen = screenAuthPassword
			return m, nil
		}
		m.screen = screenMenu
		m.status = "Authorization complete. Back to menu."
		return m, nil
	case logTickMsg:
		if m.screen != screenLogs {
			return m, nil
		}
		m.logs = append(m.logs, fmt.Sprintf("%s | polling updates...", time.Time(typed).Format("15:04:05")))
		if len(m.logs) > 18 {
			m.logs = m.logs[len(m.logs)-18:]
		}
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return logTickMsg(t) })
	case loadingTickMsg:
		if m.screen != screenAuthLoading {
			return m, nil
		}
		m.loadingIdx = (m.loadingIdx + 1) % len(spinnerFrames)
		return m, tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) })
	}

	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenMenu:
		return m.viewMenu()
	case screenAuthAppID:
		return m.viewPrompt("Enter app_id", m.appID, false)
	case screenAuthAppHash:
		return m.viewPrompt("Enter app_hash", m.appHash, false)
	case screenAuthPhone:
		return m.viewPrompt("Enter phone number (e.g. +1234567890)", m.phone, false)
	case screenAuthLoading:
		return fmt.Sprintf("%s %s\n\n%s\n\nEsc: back to menu", spinnerFrames[m.loadingIdx], m.loadingFor, m.status)
	case screenAuthCode:
		return m.viewPrompt("Enter confirmation code", m.code, false)
	case screenAuthPassword:
		return m.viewPrompt("Enter 2FA password", m.password, true)
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
				m.screen = screenAuthAppID
				m.status = "Step 1/5: app_id"
				m.appID, m.appHash, m.phone, m.code, m.password = "", "", "", "", ""
			case menuExit:
				return m, tea.Quit
			case menuAddWhitelist:
				m.status = "Whitelist menu is not wired in this TUI build."
			case menuStart:
				if !m.authClient.SessionExists() {
					m.status = "No account session. Authorize first."
					return m, nil
				}
				m.screen = screenLogs
				m.logs = []string{"Starting application...", "Anti-spam bot activated."}
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return logTickMsg(t) })
			}
		}
		return m, nil
	case screenAuthAppID:
		return m.updateInput(msg, &m.appID, func() (model, tea.Cmd) {
			m.screen = screenAuthAppHash
			m.status = "Step 2/5: app_hash"
			return m, nil
		})
	case screenAuthAppHash:
		return m.updateInput(msg, &m.appHash, func() (model, tea.Cmd) {
			m.screen = screenAuthPhone
			m.status = "Step 3/5: phone"
			return m, nil
		})
	case screenAuthPhone:
		return m.updateInput(msg, &m.phone, func() (model, tea.Cmd) {
			m.screen = screenAuthLoading
			m.loadingFor = "Initializing client..."
			m.status = "Step 4/5: requesting code"
			return m, tea.Batch(
				tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
				m.cmdBeginAuth(),
			)
		})
	case screenAuthCode:
		return m.updateInput(msg, &m.code, func() (model, tea.Cmd) {
			m.screen = screenAuthLoading
			m.loadingFor = "Verifying code..."
			return m, tea.Batch(
				tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
				m.cmdSubmitCode(),
			)
		})
	case screenAuthPassword:
		return m.updateInput(msg, &m.password, func() (model, tea.Cmd) {
			m.screen = screenAuthLoading
			m.loadingFor = "Verifying 2FA..."
			return m, tea.Batch(
				tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
				m.cmdSubmitPassword(),
			)
		})
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
			m.screen = screenMenu
			m.status = "Back to main menu."
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m model) updateInput(msg tea.KeyMsg, target *string, onSubmit func() (model, tea.Cmd)) (model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.authClient.ResetAuthFlow()
		m.screen = screenMenu
		m.status = "Back to menu."
		return m, nil
	case tea.KeyEnter:
		if strings.TrimSpace(*target) == "" {
			m.status = "Field must not be empty"
			return m, nil
		}
		return onSubmit()
	case tea.KeyBackspace, tea.KeyDelete:
		if len(*target) > 0 {
			*target = (*target)[:len(*target)-1]
		}
		return m, nil
	case tea.KeySpace:
		*target += " "
		return m, nil
	case tea.KeyRunes:
		*target += string(msg.Runes)
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
		return authBeginDoneMsg{err: m.authClient.BeginAuth(m.ctx, appID, appHash, phone)}
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

func (m model) viewPrompt(title, value string, mask bool) string {
	display := value
	if mask {
		display = strings.Repeat("*", len(value))
	}
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\nEnter: next • Esc: back",
		title,
		display,
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

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
