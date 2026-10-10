package ui

import (
	"fmt"

	"github.com/Gu1llaum-3/sshm/internal/credentials"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// passwordFormModel lets the user save or remove a host password in the OS credential manager.
type passwordFormModel struct {
	hostName string
	input    textinput.Model
	saved    bool
	err      error
	width    int
	height   int
	styles   Styles
}

type passwordFormCancelMsg struct{}

// NewPasswordForm creates a form to store the password of hostName.
func NewPasswordForm(hostName string, styles Styles, width, height int) *passwordFormModel {
	ti := textinput.New()
	ti.Placeholder = "password"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 256
	ti.Focus()
	return &passwordFormModel{
		hostName: hostName,
		input:    ti,
		saved:    credentials.Has(hostName),
		width:    width,
		height:   height,
		styles:   styles,
	}
}

func (m *passwordFormModel) Init() tea.Cmd { return textinput.Blink }

// Update handles key input: enter saves, ctrl+d removes, esc cancels.
func (m *passwordFormModel) Update(msg tea.Msg) (*passwordFormModel, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = ws.Width, ws.Height
		m.styles = NewStyles(m.width)
		return m, nil
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "ctrl+c":
			return m, func() tea.Msg { return passwordFormCancelMsg{} }
		case "ctrl+d":
			if err := credentials.Delete(m.hostName); err != nil {
				m.err = err
				return m, nil
			}
			return m, func() tea.Msg { return passwordFormCancelMsg{} }
		case "enter":
			if m.input.Value() == "" {
				return m, nil
			}
			if err := credentials.Set(m.hostName, m.input.Value()); err != nil {
				m.err = err
				return m, nil
			}
			return m, func() tea.Msg { return passwordFormCancelMsg{} }
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the form.
func (m *passwordFormModel) View() string {
	status := "no password saved"
	if m.saved {
		status = "a password is already saved (entering a new one replaces it)"
	}
	out := lipgloss.JoinVertical(lipgloss.Left,
		m.styles.Header.Render(fmt.Sprintf("🔑 Saved password for '%s'", m.hostName)),
		"",
		m.styles.HelpText.Render("Stored in the OS credential manager; "+status),
		"",
		m.input.View(),
	)
	if m.err != nil {
		out += "\n\n" + m.styles.Error.Render(m.err.Error())
	}
	out += "\n\n" + m.styles.HelpText.Render("Enter save • Ctrl+D remove saved password • Esc cancel")
	return m.styles.App.Render(out)
}
