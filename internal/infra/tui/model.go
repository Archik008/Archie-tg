package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/archik008/archie-tg/internal/application/dto"
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
	"Авторизоваться",
	"Выход из приложения",
	"Добавить пользователя в whitelist",
	"Старт приложения",
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
	appID    string
	appHash  string
	phone    string
	code     string
	password string
	whiteUserID string

	status     string
	loadingFor string
	loadingIdx int

	logs  []string
	logCh chan string
}

type authBeginDoneMsg struct{ err error }
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
		status:          "Выбери действие",
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
		if typed.err != nil {
			m.status = "Ошибка инициализации: " + typed.err.Error()
			m.screen = screenMenu
			return m, nil
		}
		m.screen = screenAuthCode
		m.status = "Введите код подтверждения"
		m.input = newTextInput("", false)
		return m, textinput.Blink
	case authCodeDoneMsg:
		if typed.err != nil {
			m.status = "Ошибка кода: " + typed.err.Error()
			m.screen = screenAuthCode
			m.input = newTextInput("", false)
			return m, textinput.Blink
		}
		if typed.requires2FA {
			m.screen = screenAuthPassword
			m.status = "Введите 2FA пароль"
			m.input = newTextInput("", true)
			return m, textinput.Blink
		}
		m.screen = screenMenu
		m.status = "Авторизация завершена. Возврат в меню."
		return m, nil
	case authPasswordDoneMsg:
		if typed.err != nil {
			m.status = "Ошибка 2FA: " + typed.err.Error()
			m.screen = screenAuthPassword
			m.input = newTextInput("", true)
			return m, textinput.Blink
		}
		m.screen = screenMenu
		m.status = "Авторизация завершена. Возврат в меню."
		return m, nil
	case whitelistAddDoneMsg:
		m.screen = screenMenu
		if typed.err != nil {
			m.status = "Ошибка whitelist: " + typed.err.Error()
			return m, nil
		}
		m.status = "Пользователь добавлен в whitelist."
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
			m.status = "Бот остановлен"
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
		m.status = "Возврат в меню."
		return m, nil
	case "enter":
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.status = "Поле не должно быть пустым"
			return m, nil
		}
		return m.submitInput(value)
	}
	return m, nil
}

func (m model) submitInput(value string) (model, tea.Cmd) {
	switch m.screen {
	case screenAuthAppID:
		m.appID = value
		m.screen = screenAuthAppHash
		m.status = "Шаг 2/5: app_hash"
		m.input = newTextInput("app_hash", false)
		return m, textinput.Blink
	case screenAuthAppHash:
		m.appHash = value
		m.screen = screenAuthPhone
		m.status = "Шаг 3/5: phone"
		m.input = newTextInput("+77001234567", false)
		return m, textinput.Blink
	case screenAuthPhone:
		m.phone = value
		m.screen = screenAuthLoading
		m.loadingFor = "Инициализация клиента..."
		m.status = "Шаг 4/5: запрос кода"
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdBeginAuth(),
		)
	case screenAuthCode:
		m.code = value
		m.screen = screenAuthLoading
		m.loadingFor = "Проверка кода..."
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdSubmitCode(),
		)
	case screenAuthPassword:
		m.password = value
		m.screen = screenAuthLoading
		m.loadingFor = "Проверка 2FA..."
		return m, tea.Batch(
			tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return loadingTickMsg(t) }),
			m.cmdSubmitPassword(),
		)
	case screenWhitelistUserID:
		m.whiteUserID = value
		m.screen = screenWhitelistUsername
		m.status = "Введите username пользователя"
		m.input = newTextInput("username", false)
		return m, textinput.Blink
	case screenWhitelistUsername:
		m.screen = screenAuthLoading
		m.loadingFor = "Добавление в whitelist..."
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
		return m.viewInputScreen("Введите app_id")
	case screenAuthAppHash:
		return m.viewInputScreen("Введите app_hash")
	case screenAuthPhone:
		return m.viewInputScreen("Введите номер телефона (например +7700...)")
	case screenAuthLoading:
		return fmt.Sprintf("%s %s\n\n%s\n\nEsc: назад в меню", spinnerFrames[m.loadingIdx], m.loadingFor, m.status)
	case screenAuthCode:
		return m.viewInputScreen("Введите код подтверждения")
	case screenAuthPassword:
		return m.viewInputScreen("Введите 2FA пароль")
	case screenWhitelistUserID:
		return m.viewInputScreen("Введите user_id")
	case screenWhitelistUsername:
		return m.viewInputScreen("Введите username (без @)")
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
				m.status = "Шаг 1/5: app_id"
				m.appID, m.appHash, m.phone, m.code, m.password = "", "", "", "", ""
				m.input = newTextInput("123456", false)
				return m, textinput.Blink
			case menuExit:
				return m, tea.Quit
			case menuAddWhitelist:
				m.screen = screenWhitelistUserID
				m.whiteUserID = ""
				m.status = "Введите ID пользователя"
				m.input = newTextInput("123456789", false)
				return m, textinput.Blink
			case menuStart:
				if !m.authClient.SessionExists() {
					m.status = "Нет файла аккаунта. Сначала авторизуйся."
					return m, nil
				}
				if m.bot == nil {
					m.status = "Бот не инициализирован."
					return m, nil
				}
				if m.bot.IsRunning() {
					m.status = "Бот уже запущен."
					return m, nil
				}
				m.logCh = make(chan string, 128)
				if err := m.bot.Start(m.ctx, m.logCh); err != nil {
					m.status = "Не удалось запустить бота: " + err.Error()
					return m, nil
				}
				m.screen = screenLogs
				m.logs = []string{"Старт приложения...", "Ожидание событий Telegram..."}
				return m, waitLogLine(m.logCh)
			}
		}
		return m, nil
	case screenAuthLoading:
		if msg.String() == "esc" {
			m.authClient.ResetAuthFlow()
			m.screen = screenMenu
			m.status = "Авторизация отменена."
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
			m.status = "Возврат в главное меню."
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
			return authBeginDoneMsg{err: fmt.Errorf("app_id должен быть числом")}
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

func (m model) cmdAddToWhitelist() tea.Cmd {
	userIDRaw := strings.TrimSpace(m.whiteUserID)
	username := strings.TrimSpace(strings.TrimPrefix(m.input.Value(), "@"))

	return func() tea.Msg {
		userID, err := strconv.Atoi(userIDRaw)
		if err != nil {
			return whitelistAddDoneMsg{err: fmt.Errorf("user_id должен быть числом")}
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
	b.WriteString("Archie TG - Главное меню\n\n")
	for i, item := range menuItems {
		cursor := "  "
		if i == m.menuCursor {
			cursor = "> "
		}
		b.WriteString(cursor + item + "\n")
	}
	b.WriteString("\n")
	b.WriteString(m.status + "\n")
	b.WriteString("\n↑/↓ выбрать • Enter подтвердить • q выйти\n")
	return b.String()
}

func (m model) viewInputScreen(title string) string {
	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n\nEnter: далее • Esc: назад",
		title,
		m.input.View(),
		m.status,
	)
}

func (m model) viewLogs() string {
	var b strings.Builder
	b.WriteString("Режим логов (приложение запущено)\n\n")
	for _, line := range m.logs {
		b.WriteString(line + "\n")
	}
	b.WriteString("\nEsc: назад в меню • q: выход\n")
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
