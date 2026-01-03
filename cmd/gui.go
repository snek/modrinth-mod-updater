package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"modrinth-mod-updater/config"
	"modrinth-mod-updater/db"
	"modrinth-mod-updater/logger"
	"modrinth-mod-updater/modrinth"
)

// guiCmd represents the gui command
var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the graphical interface to manage mods",
	Long:  `Launch an interactive TUI to view and manage your followed Modrinth mods.`,
	Run: func(_ *cobra.Command, _ []string) {
		runGUI()
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}

// ModInfo represents information about a mod
type ModInfo struct {
	Title              string
	Slug               string
	InstalledVersion   string // Display version number
	InstalledVersionID string // Version ID hash for database
	AvailableVersion   string
	Status             string // "up-to-date", "update-available", "not-installed"
	Color              int
	ProjectType        string
	Selected           bool // Whether this mod is selected for download
	Selectable         bool // Whether this mod can be selected (not up-to-date)
}

// Model represents the state of the TUI
type Model struct {
	mods          []ModInfo
	selectedIndex int
	loading       bool
	downloading   bool
	error         string
	message       string
	client        *modrinth.Client
	cfg           config.Config
	width         int
	height        int
	loadingMods   int
	totalMods     int
	spinnerFrame  int
}

// Initialize the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadMods(),
		tickSpinner(),
	)
}

func tickSpinner() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case modsLoadedMsg:
		m.handleModsLoaded(msg)
	case modProgressMsg:
		m.loadingMods = msg.current
		m.totalMods = msg.total
		return m, tickSpinner()
	case spinnerTickMsg:
		return m.handleSpinnerTick()
	case errorMsg:
		m.error = string(msg)
		m.loading = false
		m.downloading = false
	case downloadCompleteMsg:
		return m.handleDownloadComplete(msg)
	case clearMessageMsg:
		m.message = ""
	}
	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "down", "j":
		if m.selectedIndex < len(m.mods)-1 {
			m.selectedIndex++
		}
	case " ":
		if len(m.mods) > 0 && m.mods[m.selectedIndex].Selectable {
			m.mods[m.selectedIndex].Selected = !m.mods[m.selectedIndex].Selected
		}
	case "ctrl+d":
		if !m.downloading {
			m.downloading = true
			return m, m.downloadSelectedMods()
		}
	}
	return m, nil
}

func (m *Model) handleModsLoaded(msg modsLoadedMsg) {
	m.mods = msg.mods
	m.loading = false
	sort.Slice(m.mods, func(i, j int) bool {
		return strings.ToLower(m.mods[i].Title) < strings.ToLower(m.mods[j].Title)
	})
	for i := range m.mods {
		m.mods[i].Selected = false
	}
}

func (m *Model) handleSpinnerTick() (tea.Model, tea.Cmd) {
	m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
	if m.loading {
		return m, tickSpinner()
	}
	return m, nil
}

func (m *Model) handleDownloadComplete(msg downloadCompleteMsg) (tea.Model, tea.Cmd) {
	m.downloading = false
	m.message = msg.message
	return m, tea.Batch(
		m.loadMods(),
		tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return clearMessageMsg{}
		}),
	)
}

// View renders the UI
func (m Model) View() string {
	if m.loading {
		return m.renderLoadingScreen()
	}

	if m.downloading {
		return m.renderDownloadingScreen()
	}

	if m.error != "" {
		return fmt.Sprintf("Error: %s\n", m.error)
	}

	if len(m.mods) == 0 {
		return "No mods found. Follow some mods on Modrinth!\n"
	}

	var output string
	output += renderHeader()
	output += "\n"

	for i, mod := range m.mods {
		output += m.renderModRow(i, mod)
		output += "\n"
	}

	output += "\n" + renderFooter()

	if m.message != "" {
		output += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.message)
	}

	return output
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func loadLogo() string {
	// Try to load logo from file
	logoPath := filepath.Join("ui", "logo.txt")
	logoBytes, err := os.ReadFile(logoPath)
	if err != nil {
		// Fallback if file not found
		return ""
	}
	return strings.TrimSpace(string(logoBytes))
}

func (m Model) renderLoadingScreen() string {
	logo := loadLogo()
	spinner := spinnerFrames[m.spinnerFrame]

	var progressText string
	if m.totalMods > 0 {
		progressText = fmt.Sprintf(" %d/%d mods", m.loadingMods, m.totalMods)
	}

	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	loadingMsg := fmt.Sprintf("%s Loading mods%s...", spinner, progressText)

	var output string
	if logo != "" {
		logoStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("12"))
		output += logoStyle.Render(logo) + "\n\n"
	}
	output += loadingStyle.Render(loadingMsg) + "\n"

	return output
}

func renderHeader() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Padding(0, 1)

	return headerStyle.Render(fmt.Sprintf("%-40s %-20s %-20s %-15s", "Mod Name", "Installed", "Available", "Status"))
}

func renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	return footerStyle.Render("↑/k: up  ↓/j: down  space: select  ctrl+d: download  q: quit")
}

func (m Model) renderModRow(index int, mod ModInfo) string {
	var statusColor string
	switch mod.Status {
	case "update-available":
		statusColor = "11" // Yellow
	case "up-to-date":
		statusColor = "10" // Green
	case "not-installed":
		statusColor = "9" // Red
	default:
		statusColor = "7" // White
	}

	rowStyle := lipgloss.NewStyle().Padding(0, 1)
	isSelected := index == m.selectedIndex
	if isSelected {
		rowStyle = rowStyle.
			Background(lipgloss.Color("8")).
			Bold(true)
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor))

	// Add selection indicator
	selectionIndicator := " "
	if mod.Selected {
		selectionIndicator = "✓"
	} else if !mod.Selectable {
		selectionIndicator = "-"
	}

	// Pad status before applying color to maintain column alignment
	paddedStatus := fmt.Sprintf("%-15s", mod.Status)
	coloredStatus := statusStyle.Render(paddedStatus)

	row := fmt.Sprintf("%s %-39s %-20s %-20s %s",
		selectionIndicator,
		truncate(mod.Title, 37),
		truncate(mod.InstalledVersion, 18),
		truncate(mod.AvailableVersion, 18),
		coloredStatus,
	)

	return rowStyle.Render(row)
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// Message types
type modsLoadedMsg struct {
	mods []ModInfo
}

type errorMsg string

type modProgressMsg struct {
	current int
	total   int
}

type spinnerTickMsg struct{}

type downloadCompleteMsg struct {
	message string
}

type clearMessageMsg struct{}

// Load mods from the API and database
func (m Model) loadMods() tea.Cmd {
	return func() tea.Msg {
		mods, err := m.fetchModsWithProgress()
		if err != nil {
			logger.Log.Errorw("Failed to fetch mods", zap.Error(err))
			return errorMsg(fmt.Sprintf("Failed to fetch mods: %v", err))
		}
		return modsLoadedMsg{mods: mods}
	}
}

func (m Model) fetchModsWithProgress() ([]ModInfo, error) {
	// Get followed projects from Modrinth
	followedProjects, err := m.client.GetFollowedProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to get followed projects: %w", err)
	}

	var modInfos []ModInfo
	processedCount := 0

	for _, project := range followedProjects {
		// Skip non-mod/shader/resourcepack projects
		if project.ProjectType != "mod" && project.ProjectType != "shader" && project.ProjectType != "resourcepack" {
			continue
		}

		// Get latest version
		versions, err := m.client.GetProjectVersions(project.Slug, project.ProjectType, m.cfg.MinecraftVersion, m.cfg.MinecraftLoader)
		if err != nil || len(versions) == 0 {
			continue
		}

		latestVersion := versions[0]

		// Check if mod is installed
		var installedMod db.Mod
		result := db.DB.Where("project_slug = ?", project.Slug).First(&installedMod)

		var modInfo ModInfo
		modInfo.Title = project.Title
		modInfo.Slug = project.Slug
		modInfo.Color = project.Color
		modInfo.ProjectType = project.ProjectType
		modInfo.AvailableVersion = latestVersion.VersionNumber

		if result.Error == nil {
			// Mod is installed
			modInfo.InstalledVersion = installedMod.VersionNumber
			modInfo.InstalledVersionID = installedMod.VersionID
			if installedMod.VersionID == latestVersion.ID {
				modInfo.Status = "up-to-date"
				modInfo.Selectable = false // Can't select up-to-date mods
			} else {
				modInfo.Status = "update-available"
				modInfo.Selectable = true // Can select mods with updates
			}
		} else {
			// Mod is not installed
			modInfo.InstalledVersion = "Not installed"
			modInfo.Status = "not-installed"
			modInfo.Selectable = true // Can select not-installed mods
		}

		modInfos = append(modInfos, modInfo)
		processedCount++
	}

	return modInfos, nil
}

func (m Model) renderDownloadingScreen() string {
	spinner := spinnerFrames[m.spinnerFrame]
	downloadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	downloadingMsg := fmt.Sprintf("%s Downloading selected mods...", spinner)
	return downloadingStyle.Render(downloadingMsg) + "\n"
}

func (m Model) downloadSelectedMods() tea.Cmd {
	return func() tea.Msg {
		var selectedMods []ModInfo
		for _, mod := range m.mods {
			if mod.Selected {
				selectedMods = append(selectedMods, mod)
			}
		}

		if len(selectedMods) == 0 {
			return downloadCompleteMsg{message: "No mods selected for download"}
		}

		successCount := 0
		for _, mod := range selectedMods {
			if err := m.downloadAndRecordMod(mod); err != nil {
				logger.Log.Warnw("Failed to download mod", zap.String("slug", mod.Slug), zap.Error(err))
				continue
			}
			successCount++
		}

		message := fmt.Sprintf("Downloaded %d/%d selected mods", successCount, len(selectedMods))
		return downloadCompleteMsg{message: message}
	}
}

func (m Model) downloadAndRecordMod(mod ModInfo) error {
	versions, err := m.client.GetProjectVersions(mod.Slug, mod.ProjectType, m.cfg.MinecraftVersion, m.cfg.MinecraftLoader)
	if err != nil || len(versions) == 0 {
		return fmt.Errorf("failed to get versions: %w", err)
	}

	latestVersion := versions[0]
	primaryFile := findPrimaryFile(latestVersion)
	if primaryFile == nil {
		return fmt.Errorf("no files found for version %s", latestVersion.ID)
	}

	targetSubDir := getTargetSubDir(mod.ProjectType)
	projectBaseDir := filepath.Join(m.cfg.MinecraftDir, targetSubDir)
	downloadPath := filepath.Join(projectBaseDir, primaryFile.Filename)

	// Before downloading, archive the old version if it exists in the database
	var existingMod db.Mod
	result := db.DB.Where("project_slug = ?", mod.Slug).First(&existingMod)
	if result.Error == nil {
		archiveAndCleanupOld(existingMod, projectBaseDir, &m.cfg, logger.Log)
	}

	if err := m.client.DownloadModFile(logger.Log, downloadPath, primaryFile.URL); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return m.updateModDatabase(mod, latestVersion, primaryFile, downloadPath)
}

func (m Model) updateModDatabase(mod ModInfo, latestVersion modrinth.Version, primaryFile *modrinth.File, downloadPath string) error {
	var existingMod db.Mod
	result := db.DB.Where("project_slug = ?", mod.Slug).First(&existingMod)

	if result.Error == nil {
		existingMod.VersionID = latestVersion.ID
		existingMod.VersionNumber = latestVersion.VersionNumber
		existingMod.FileName = primaryFile.Filename
		existingMod.InstallPath = downloadPath
		return db.DB.Save(&existingMod).Error
	}

	newMod := db.Mod{
		ProjectSlug:   mod.Slug,
		ProjectID:     mod.Slug,
		Title:         mod.Title,
		Color:         mod.Color,
		Updated:       time.Now(),
		VersionID:     latestVersion.ID,
		VersionNumber: latestVersion.VersionNumber,
		FileName:      primaryFile.Filename,
		InstallPath:   downloadPath,
	}
	return db.DB.Create(&newMod).Error
}

func runGUI() {
	cfg, client := bootstrap(".")

	// Create and run the model
	m := Model{
		selectedIndex: 0,
		loading:       true,
		client:        client,
		cfg:           cfg,
		width:         80,
		height:        24,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Log.Fatalw("Failed to run GUI", zap.Error(err))
	}
}
