package cmd

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateProgressMsg represents a progress update from the update process
type UpdateProgressMsg struct {
	Type        string // "status", "check", "download_start", "download_success", "error", "summary", "done"
	Message     string
	ProjectName string
	ProjectSlug string
	Version     string
	Color       int
}

// UpdateModel controls the UI for the update command
type UpdateModel struct {
	spinner      spinner.Model
	progressChan chan UpdateProgressMsg
	forceUpdate  bool

	// State
	status      string
	checking    []string
	downloading []string
	completed   []string
	errors      []string
	summary     string
	done        bool

	// Counters
	totalChecked int
	totalUpdated int
	totalErrors  int
}

func initialUpdateModel(forceUpdate bool) UpdateModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return UpdateModel{
		spinner:      s,
		progressChan: make(chan UpdateProgressMsg, 100), // Buffer slightly to avoid blocking
		forceUpdate:  forceUpdate,
		status:       "Initializing...",
		checking:     []string{},
		downloading:  []string{},
		completed:    []string{},
		errors:       []string{},
	}
}

func (m UpdateModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startUpdate(),
		m.waitForActivity(),
	)
}

func (m UpdateModel) startUpdate() tea.Cmd {
	return func() tea.Msg {
		// Run update in a separate goroutine
		go func() {
			defer close(m.progressChan)
			runUpdate(m.forceUpdate, m.progressChan)
		}()
		return nil
	}
}

func (m UpdateModel) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.progressChan
		if !ok {
			return UpdateProgressMsg{Type: "done"}
		}
		return msg
	}
}

func (m UpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// If done, allow any key to exit
		if m.done {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case UpdateProgressMsg:
		switch msg.Type {
		case "done":
			m.done = true
			m.status = "Finished"
			return m, tea.Quit

		case "status":
			m.status = msg.Message

		case "check":
			m.status = fmt.Sprintf("Checking %s...", msg.ProjectName)
			m.totalChecked++

		case "download_start":
			m.removeFromChecking(msg.ProjectName)
			m.downloading = append(m.downloading, fmt.Sprintf("%s (%s)", msg.ProjectName, msg.Version))

		case "download_success":
			m.removeFromDownloading(fmt.Sprintf("%s (%s)", msg.ProjectName, msg.Version))
			m.completed = append(m.completed, fmt.Sprintf("Updated %s to %s", msg.ProjectName, msg.Version))
			m.totalUpdated++

		case "error":
			m.errors = append(m.errors, fmt.Sprintf("%s: %s", msg.ProjectName, msg.Message))
			m.totalErrors++

		case "summary":
			m.summary = msg.Message
		}

		return m, m.waitForActivity()
	}

	return m, nil
}

func (m *UpdateModel) removeFromChecking(name string) {
	for i, v := range m.checking {
		if v == name {
			m.checking = append(m.checking[:i], m.checking[i+1:]...)
			return
		}
	}
}

func (m *UpdateModel) removeFromDownloading(name string) {
	for i, v := range m.downloading {
		if v == name {
			m.downloading = append(m.downloading[:i], m.downloading[i+1:]...)
			return
		}
	}
}

func (m UpdateModel) View() string {
	var symbol string
	if m.done {
		symbol = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓")
	} else {
		symbol = m.spinner.View()
	}

	s := fmt.Sprintf("\n %s %s\n\n", symbol, m.status)

	if len(m.downloading) > 0 {
		s += lipgloss.NewStyle().Bold(true).Render("Downloading:") + "\n"
		for _, d := range m.downloading {
			s += fmt.Sprintf("  • %s\n", d)
		}
		s += "\n"
	}

	if len(m.errors) > 0 {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Errors:") + "\n"
		for _, e := range m.errors {
			s += fmt.Sprintf("  • %s\n", e)
		}
		s += "\n"
	}

	// Show last few completed
	if len(m.completed) > 0 {
		s += lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("Completed:") + "\n"
		start := 0
		// If done, maybe show more? For now keep it consistent to avoid jumpiness,
		// but ensure we show the user something happened.
		if len(m.completed) > 5 && !m.done {
			start = len(m.completed) - 5
		}
		for i := start; i < len(m.completed); i++ {
			s += fmt.Sprintf("  • %s\n", m.completed[i])
		}
		s += "\n"
	}

	if m.done {
		s += lipgloss.NewStyle().Bold(true).Render(m.summary) + "\n"
	}

	return s
}
