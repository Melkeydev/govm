package model

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/melkeydev/govm/internal/styles"
	"github.com/melkeydev/govm/internal/utils"
)

type Model struct {
	List              list.Model
	Versions          []utils.GoVersion
	Err               error
	Loading           bool
	Spinner           spinner.Model
	HomeDir           string
	GoVersionsDir     string
	CurrentTab        int
	DownloadProgress  float64
	InstallingVersion string
	Message           string
	MessageType       string // "success" or "error"
	InstalledTable    table.Model
	ConfirmingDelete  bool
	DeleteVersion     string
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		utils.FetchGoVersions,
		m.Spinner.Tick,
	)
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			// Switch between tabs
			m.CurrentTab = (m.CurrentTab + 1) % 2
			return m, nil
		case "i":
			if m.CurrentTab == 0 {
				selectedItem := m.List.SelectedItem().(styles.Item)
				for _, v := range m.Versions {
					if v.Version == selectedItem.Name && !v.Installed {
						m.Loading = true
						m.InstallingVersion = v.Version
						m.Message = ""
						return m, utils.DownloadAndInstall(v)
					}
				}
			}
		case "u":
			if m.CurrentTab == 0 {
				selectedItem := m.List.SelectedItem().(styles.Item)
				for _, v := range m.Versions {
					if v.Version == selectedItem.Name && v.Installed {
						m.Loading = true
						m.Message = fmt.Sprintf("Switching to Go %s...", v.Version)
						return m, utils.SwitchVersion(v)
					}
				}
				m.Message = "You need to install this version first. Press 'i' to install."
				m.MessageType = "error"
			}
		case "r":
			m.Loading = true
			m.Message = ""
			return m, utils.FetchGoVersions
		case "d":
			if m.CurrentTab == 0 || m.CurrentTab == 1 {
				selectedItem := m.List.SelectedItem().(styles.Item)
				for _, v := range m.Versions {
					if v.Version == selectedItem.Name && v.Installed {
						if v.Active {
							m.Message = "Cannot delete active version. Switch to another version first."
							m.MessageType = "error"
							return m, nil
						}

						m.ConfirmingDelete = true
						m.DeleteVersion = v.Version
						m.Message = fmt.Sprintf("Are you sure you want to delete Go %s? Press Y to confirm, N to cancel.", v.Version)
						m.MessageType = "warning"
						return m, nil
					}
				}

				if m.CurrentTab == 0 {
					m.Message = "This version is not installed."
					m.MessageType = "error"
				}
			}
		case "y", "Y":
			if m.ConfirmingDelete {
				m.ConfirmingDelete = false
				m.Loading = true
				m.Message = fmt.Sprintf("Deleting Go %s...", m.DeleteVersion)
				m.MessageType = "info"

				var versionToDelete utils.GoVersion
				for _, v := range m.Versions {
					if v.Version == m.DeleteVersion {
						versionToDelete = v
						break
					}
				}

				return m, utils.DeleteVersion(versionToDelete)
			}
		case "n", "N":
			if m.ConfirmingDelete {
				m.ConfirmingDelete = false
				m.DeleteVersion = ""
				m.Message = "Delete operation canceled."
				m.MessageType = "info"
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		h, v := styles.DocStyle.GetFrameSize()
		m.List.SetSize(msg.Width-h, msg.Height-v-6)
		m.InstalledTable.SetWidth(msg.Width - h)
		m.InstalledTable.SetHeight(msg.Height - v - 10)
		return m, nil
	case utils.ErrMsg:
		m.Err = msg
		m.Loading = false
		m.Message = msg.Error()
		m.MessageType = "error"
		return m, nil
	case utils.VersionsMsg:
		m.Versions = msg
		items := make([]list.Item, len(m.Versions))
		for i, v := range m.Versions {
			items[i] = styles.Item{
				Name:            v.Version,
				DescriptionText: "go" + v.Version + " " + v.Filename,
				Installed:       v.Installed,
				Active:          v.Active,
			}
		}
		m.List.SetItems(items)
		m.Loading = false
		m.updateInstalledTable()
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case utils.DownloadCompleteMsg:
		m.Loading = false
		m.InstallingVersion = ""
		for i, v := range m.Versions {
			if v.Version == msg.Version {
				m.Versions[i].Installed = true
				m.Versions[i].Path = msg.Path
				break
			}
		}
		items := m.List.Items()
		for i, it := range items {
			if it.(styles.Item).Name == msg.Version {
				updatedItem := it.(styles.Item)
				updatedItem.Installed = true
				items[i] = updatedItem
			}
		}
		m.List.SetItems(items)
		m.updateInstalledTable()
		m.Message = fmt.Sprintf("Successfully installed Go %s", msg.Version)
		m.MessageType = "success"
		return m, nil
	case utils.SwitchCompletedMsg:
		m.Loading = false
		for i := range m.Versions {
			m.Versions[i].Active = (m.Versions[i].Version == msg.Version)
		}
		items := m.List.Items()
		for i, it := range items {
			updatedItem := it.(styles.Item)
			updatedItem.Active = (updatedItem.Name == msg.Version)
			items[i] = updatedItem
		}
		m.List.SetItems(items)
		m.updateInstalledTable()
		if msg.ShimInPath {
			m.Message = fmt.Sprintf("Switched to Go %s! Run 'go version' to verify.", msg.Version)
		} else {
			m.Message = fmt.Sprintf("Switched to Go %s!\n\n%s",
				msg.Version, utils.GetShimPathInstructions())
		}
		m.MessageType = "success"
		return m, nil
	case utils.DeleteCompleteMsg:
		m.Loading = false

		for i, v := range m.Versions {
			if v.Version == msg.Version {
				m.Versions[i].Installed = false
				m.Versions[i].Path = ""
				break
			}
		}

		items := m.List.Items()
		for i, it := range items {
			if it.(styles.Item).Name == msg.Version {
				updatedItem := it.(styles.Item)
				updatedItem.Installed = false
				items[i] = updatedItem
			}
		}
		m.List.SetItems(items)

		m.updateInstalledTable()

		m.Message = fmt.Sprintf("Successfully deleted Go %s", msg.Version)
		m.MessageType = "success"
		return m, nil
	}
	newListModel, cmd := m.List.Update(msg)
	m.List = newListModel
	cmds = append(cmds, cmd)
	newTableModel, tableCmd := m.InstalledTable.Update(msg)
	m.InstalledTable = newTableModel
	cmds = append(cmds, tableCmd)
	return m, tea.Batch(cmds...)
}
func (m *Model) updateInstalledTable() {
	rows := []table.Row{}
	for _, v := range m.Versions {
		if v.Installed {
			status := ""
			if v.Active {
				status = "active"
			}
			rows = append(rows, table.Row{v.Version, v.Path, status})
		}
	}
	m.InstalledTable.SetRows(rows)
}
func (m Model) View() string {
	if m.Err != nil {
		return fmt.Sprintf("Error: %s\n\nPress any key to quit.", m.Err)
	}
	// Build components
	components := []string{}
	header := styles.TitleStyle.Render("GoVM - Go Version Manager")
	components = append(components, header)
	if !utils.IsShimInPath() {
		warningStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#FFCC00")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(1, 2).
			Align(lipgloss.Left)
		instructions := utils.GetShimPathInstructions()
		warningBanner := warningStyle.Render("⚠️  GoVM is not in your PATH  ⚠️\n\n" + instructions)
		components = append(components, warningBanner)
	}
	tabs := []string{"Available Versions", "Installed Versions"}
	tabContent := ""
	for i, tab := range tabs {
		if i == m.CurrentTab {
			tabContent += styles.HighlightStyle.Render("[ "+tab+" ]") + " "
		} else {
			tabContent += fmt.Sprintf("[ %s ]", tab) + " "
		}
	}
	components = append(components, tabContent)
	if m.CurrentTab == 0 {
		listView := m.List.View()
		components = append(components, listView)
		if m.Loading {
			spinnerDisplay := ""
			if m.InstallingVersion != "" {
				progressBar := fmt.Sprintf("[downloading Go %s]", m.InstallingVersion)
				spinnerDisplay = fmt.Sprintf("%s %s", m.Spinner.View(), progressBar)
			} else {
				spinnerDisplay = fmt.Sprintf("%s Loading versions...", m.Spinner.View())
			}
			components = append(components, spinnerDisplay)
		}
	} else {
		tableView := m.InstalledTable.View()
		components = append(components, tableView)
	}
	if m.Message != "" {
		if m.MessageType == "success" {
			components = append(components, styles.SuccessStyle.Render(m.Message))
		} else {
			components = append(components, styles.ErrorStyle.Render(m.Message))
		}
	}
	if m.CurrentTab == 0 {
		components = append(components, styles.HelpStyle("\nPress 'i' to install, 'u' to use/switch, 'd' to delete, 'r' to refresh, 'tab' to switch tabs, 'q' to quit"))
	} else {
		components = append(components, styles.HelpStyle("\nPress 'u' to use/switch, 'd' to delete, 'tab' to switch tabs, 'q' to quit"))
	}
	return styles.AppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, components...))
}

