package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	width       int
	height      int
	shimPath    string
	shellConfig string
	done        bool
	keyPrompt   string
}

func New() Model {
	homeDir, _ := os.UserHomeDir()
	shimPath := filepath.Join(homeDir, ".govm", "shim")

	shellConfig := "~/.bashrc"
	if runtime.GOOS == "windows" {
		shellConfig = "PATH environment variable"
	} else {
		shell := os.Getenv("SHELL")
		if strings.Contains(shell, "zsh") {
			shellConfig = "~/.zshrc"
		}
	}

	keyPrompt := "Press Enter to continue..."
	if runtime.GOOS == "windows" {
		keyPrompt = "Press Enter to continue..."
	}

	return Model{
		shimPath:    shimPath,
		shellConfig: shellConfig,
		keyPrompt:   keyPrompt,
		width:       80,
		height:      24,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			m.done = true
			return m, tea.Quit
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#3c71a8")).
		MarginBottom(1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("#3c71a8")).
		PaddingBottom(1)

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3c71a8")).
		Padding(1, 2).
		Width(min(m.width-4, 80))

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3c71a8")).
		Bold(true)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1)

	title := titleStyle.Render("GoVM First-Time Setup")

	var setupInstructions string
	if runtime.GOOS == "windows" {
		setupInstructions = fmt.Sprintf(`To use GoVM, you need to add this directory to your PATH:

%s

You can do this by running this command in Command Prompt:

%s

After adding to PATH, restart your terminal.`,
			highlightStyle.Render(m.shimPath),
			highlightStyle.Render(fmt.Sprintf("setx PATH \"%%PATH%%;%s\"", m.shimPath)))
	} else {
		shellConfigFile := "~/.bashrc"

		if strings.Contains(os.Getenv("SHELL"), "zsh") {
			shellConfigFile = "~/.zshrc"
		}

		setupInstructions = fmt.Sprintf(`To use GoVM, you need to add this directory to your PATH:

%s

Option 1: Run this command to add it automatically:

%s

Option 2: Or manually add this line to your %s:

%s

After adding to PATH, restart your terminal or run:

%s`,
			highlightStyle.Render(m.shimPath),
			highlightStyle.Render(fmt.Sprintf("echo 'export PATH=\"$HOME/.govm/shim:$PATH\"' >> %s", shellConfigFile)),
			shellConfigFile,
			highlightStyle.Render("export PATH=\"$HOME/.govm/shim:$PATH\""),
			highlightStyle.Render(fmt.Sprintf("source %s", shellConfigFile)))
	}

	box := boxStyle.Render(setupInstructions)
	footer := footerStyle.Render(m.keyPrompt)

	paddingTop := max(0, (m.height-lipgloss.Height(title)-lipgloss.Height(box)-lipgloss.Height(footer)-4)/2)
	padTopStr := strings.Repeat("\n", paddingTop)

	return padTopStr + lipgloss.JoinVertical(lipgloss.Center,
		title,
		box,
		footer,
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func IsShimInPath() bool {
	homeDir, _ := os.UserHomeDir()
	shimDir := filepath.Join(homeDir, ".govm", "shim")

	currentPath := os.Getenv("PATH")
	pathSeparator := string(os.PathListSeparator)
	pathEntries := strings.Split(currentPath, pathSeparator)

	for _, entry := range pathEntries {
		if entry == shimDir {
			return true
		}
	}

	return false
}
