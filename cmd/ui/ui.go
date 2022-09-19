package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/caarlos0/org-stats/orgstats"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v39/github"
)

type errMsg struct{ error }

// NewInitialModel creates a new InitialModel with required fields.
func NewInitialModel(
	client *github.Client,
	org string,
	userBlacklist, repoBlacklist []string,
	since time.Time,
	top int,
	includeReviewStats bool,
	csv io.Writer,
	jw io.Writer,
) InitialModel {
	var s = spinner.NewModel()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return InitialModel{
		client:             client,
		org:                org,
		userBlacklist:      userBlacklist,
		repoBlacklist:      repoBlacklist,
		since:              since,
		includeReviewStats: includeReviewStats,
		top:                top,
		spinner:            s,
		csv:                csv,
		json:               jw,
		loading:            true,
	}
}

// InitialModel is the UI when the CLI starts, basically loading the repos.
type InitialModel struct {
	err      error
	spinner  spinner.Model
	loading  bool
	quitting bool

	client             *github.Client
	org                string
	userBlacklist      []string
	repoBlacklist      []string
	since              time.Time
	includeReviewStats bool
	top                int
	csv                io.Writer
	json               io.Writer
}

func (m InitialModel) Init() tea.Cmd {
	return tea.Batch(
		getStats(
			m.client,
			m.org,
			m.userBlacklist,
			m.repoBlacklist,
			m.since,
			m.includeReviewStats,
		),
		spinner.Tick,
	)
}

func (m InitialModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.loading = false
		m.err = msg.error
		return m, nil
	case gotResults:
		log.Println("got results", len(msg.stats.Logins()), "logins")
		highlights := NewHighlightsModel(msg.stats, m.top, m.includeReviewStats)
		if m.json != nil {
			if _, err := m.json.Write([]byte(msg.stats.JSON())); err != nil {
				return m, tea.Quit
			}
		}
		return highlights, tea.Batch(highlights.Init())
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m InitialModel) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	str := fmt.Sprintf("\n\n   %s Gathering data for %s... press q to quit\n\n", m.spinner.View(), m.org)
	if m.quitting {
		return str + "\n"
	}
	return str
}

type gotResults struct {
	stats orgstats.Stats
}

func getStats(
	client *github.Client,
	org string,
	userBlacklist, repoBlacklist []string,
	since time.Time,
	includeReviews bool,
) tea.Cmd {
	return func() tea.Msg {
		stats, err := orgstats.Gather(
			context.Background(),
			client,
			org,
			userBlacklist,
			repoBlacklist,
			since,
			includeReviews,
		)
		if err != nil {
			return errMsg{err}
		}
		return gotResults{stats}
	}
}

func writeJSON(w io.Writer, stats orgstats.Stats) tea.Cmd {
	return func() tea.Msg {
		if _, err := w.Write([]byte(stats.JSON())); err != nil {
			return errMsg{err}
		}
		return tea.Quit
	}
}
