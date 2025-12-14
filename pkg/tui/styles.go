package tui

import "github.com/charmbracelet/lipgloss"

// colors - keeping it simple, mostly using ANSI 256 for terminal compat
var (
	colorPrimary   = lipgloss.Color("99")  // purple
	colorSecondary = lipgloss.Color("39")  // cyan
	colorSuccess   = lipgloss.Color("82")  // green
	colorWarning   = lipgloss.Color("214") // orange
	colorDanger    = lipgloss.Color("196") // red
	colorMuted     = lipgloss.Color("240") // gray
	colorWater     = lipgloss.Color("27")  // blue
	colorHit       = lipgloss.Color("208") // orange/fire
	colorMiss      = lipgloss.Color("245") // light gray
	colorShip      = lipgloss.Color("255") // white

	// company colors
	colorPlayer     = lipgloss.Color("250") // bright gray/white for player
	colorNetflix    = lipgloss.Color("196") // red
	colorAWS        = lipgloss.Color("208") // orange
	colorGoogle     = lipgloss.Color("39")  // blue
	colorMicrosoft  = lipgloss.Color("33")  // azure blue
	colorSteam      = lipgloss.Color("236") // dark gray
	colorOpenAI     = lipgloss.Color("48")  // teal/green
	colorIBM        = lipgloss.Color("27")  // IBM blue
	colorMeta       = lipgloss.Color("33")  // meta blue
	colorCloudflare = lipgloss.Color("208") // cloudflare orange
	colorSpotify    = lipgloss.Color("82")  // spotify green
)

// Styles holds all the lipgloss styles for the TUI
type Styles struct {
	// layout
	App       lipgloss.Style
	Header    lipgloss.Style
	Footer    lipgloss.Style
	Sidebar   lipgloss.Style
	BoardArea lipgloss.Style

	// board cells
	Water     lipgloss.Style
	Hit       lipgloss.Style
	Miss      lipgloss.Style
	Ship      lipgloss.Style
	Cursor    lipgloss.Style
	Destroyed lipgloss.Style

	// text
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Normal     lipgloss.Style
	Muted      lipgloss.Style
	Success    lipgloss.Style
	Warning    lipgloss.Style
	Error      lipgloss.Style
	Selected   lipgloss.Style
	Unselected lipgloss.Style

	// status indicators
	HealthGood lipgloss.Style
	HealthMid  lipgloss.Style
	HealthBad  lipgloss.Style

	// menu
	MenuItem         lipgloss.Style
	MenuItemSelected lipgloss.Style

	// panels
	Box lipgloss.Style

	// overlays
	InfoBox lipgloss.Style
}

// DefaultStyles returns the default style configuration
func DefaultStyles() *Styles {
	return &Styles{
		App: lipgloss.NewStyle().
			Padding(1, 2),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorMuted).
			PaddingBottom(1).
			MarginBottom(1),

		Footer: lipgloss.NewStyle().
			Foreground(colorMuted).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(colorMuted).
			PaddingTop(1).
			MarginTop(1),

		Sidebar: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(1).
			Width(35),

		BoardArea: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1),

		Water: lipgloss.NewStyle().
			Foreground(colorWater),

		Hit: lipgloss.NewStyle().
			Foreground(colorHit).
			Bold(true),

		Miss: lipgloss.NewStyle().
			Foreground(colorMiss),

		Ship: lipgloss.NewStyle().
			Foreground(colorShip).
			Bold(true),

		Cursor: lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(lipgloss.Color("0")),

		Destroyed: lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),

		Subtitle: lipgloss.NewStyle().
			Foreground(colorSecondary),

		Normal: lipgloss.NewStyle(),

		Muted: lipgloss.NewStyle().
			Foreground(colorMuted),

		Success: lipgloss.NewStyle().
			Foreground(colorSuccess),

		Warning: lipgloss.NewStyle().
			Foreground(colorWarning),

		Error: lipgloss.NewStyle().
			Foreground(colorDanger),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary),

		Unselected: lipgloss.NewStyle().
			Foreground(colorMuted),

		HealthGood: lipgloss.NewStyle().
			Foreground(colorSuccess),

		HealthMid: lipgloss.NewStyle().
			Foreground(colorWarning),

		HealthBad: lipgloss.NewStyle().
			Foreground(colorDanger),

		MenuItem: lipgloss.NewStyle().
			PaddingLeft(2),

		MenuItemSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			PaddingLeft(2).
			SetString("> "),

		Box: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorSecondary).
			Padding(1).
			MarginTop(1),

		InfoBox: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorSecondary).
			Padding(1, 2).
			Width(64).
			Background(lipgloss.Color("235")),
	}
}

// board symbols
const (
	SymWater     = "~"
	SymMiss      = "o"
	SymHit       = "X"
	SymShip      = "#"
	SymDestroyed = "!"
)

// CompanyColor returns the color for a company ID
func CompanyColor(companyID string) lipgloss.Color {
	switch companyID {
	case "player":
		return colorPlayer
	case "netflix":
		return colorNetflix
	case "aws":
		return colorAWS
	case "google":
		return colorGoogle
	case "microsoft":
		return colorMicrosoft
	case "steam":
		return colorSteam
	case "openai":
		return colorOpenAI
	case "ibm":
		return colorIBM
	case "meta":
		return colorMeta
	case "cloudflare":
		return colorCloudflare
	case "spotify":
		return colorSpotify
	default:
		return colorShip
	}
}

// CompanyStyle returns a styled ship display for a company
func CompanyStyle(companyID string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(CompanyColor(companyID)).Bold(true)
}
