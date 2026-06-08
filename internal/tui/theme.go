package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Glass-cockpit palette: dark background with vivid phosphor cyan/green/amber/
// red accents, the way a modern flight deck PFD reads under night lighting.
var (
	colCyan  = lipgloss.Color("51")  // primary instruments (bright cyan)
	colGreen = lipgloss.Color("48")  // nominal (spring green)
	colAmber = lipgloss.Color("220") // caution (gold)
	colRed   = lipgloss.Color("196") // warning
	colDim   = lipgloss.Color("245") // chrome / labels
	colText  = lipgloss.Color("231") // readouts (bright white)

	// Per-engine (agent) colors, like separate engine instruments.
	agentColors = map[string]lipgloss.Color{
		"claude": lipgloss.Color("213"), // bright magenta
		"codex":  lipgloss.Color("51"),  // bright cyan
		"gemini": lipgloss.Color("75"),  // bright blue
	}
)

func agentColor(name string) lipgloss.Color {
	if c, ok := agentColors[name]; ok {
		return c
	}
	return colDim
}

var (
	labelStyle = lipgloss.NewStyle().Foreground(colDim)
	valueStyle = lipgloss.NewStyle().Foreground(colText).Bold(true)
	bigStyle   = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	titleStyle = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(colAmber).Bold(true)
	alertStyle = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	litStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(colAmber).Bold(true)
	alarmStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(colRed).Bold(true)
	darkLamp   = lipgloss.NewStyle().Foreground(colDim)
	gaugeEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	// Airspeed tape zones and needle.
	zoneGreen   = lipgloss.NewStyle().Foreground(colGreen)
	zoneAmber   = lipgloss.NewStyle().Foreground(colAmber)
	zoneRed     = lipgloss.NewStyle().Foreground(colRed)
	needleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Bold(true)

	// focusChip highlights the selected widget in the focus bar.
	focusChip = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(colCyan).Bold(true)
)

// renderCompact selects the lighter, airier compact look. It is set from the
// model at the start of each frame; Bubble Tea rendering is single-threaded, so
// this render-context flag is safe to keep package-level.
var renderCompact bool

// panel wraps body in a rounded, accented box with a header line. width is the
// total rendered width including borders.
func panel(title string, accent lipgloss.Color, width int, body string) string {
	padV := 0
	if renderCompact {
		padV = 1 // breathe vertically in compact mode
	}
	return boxed(title, accent, width, body, lipgloss.RoundedBorder(), padV)
}

// heroPanel is a heavier double-ruled box for top-level instrument clusters in
// expert mode; compact mode softens it to a rounded box for a lighter feel.
func heroPanel(title string, accent lipgloss.Color, width int, body string) string {
	border := lipgloss.DoubleBorder()
	padV := 0
	if renderCompact {
		border = lipgloss.RoundedBorder()
		padV = 1
	}
	return boxed(title, accent, width, body, border, padV)
}

func boxed(title string, accent lipgloss.Color, width int, body string, border lipgloss.Border, padV int) string {
	return boxedH(title, accent, width, body, border, padV, 0)
}

// boxedH is boxed with an optional fixed body height (in lines). When bodyLines
// > 0 the content area is padded to that height so sibling panels line up to the
// same box height instead of ending raggedly. bodyLines counts the body only;
// the header line is added on top.
func boxedH(title string, accent lipgloss.Color, width int, body string, border lipgloss.Border, padV, bodyLines int) string {
	inner := width - 4 // 2 border + 2 horizontal padding
	if inner < 1 {
		inner = 1
	}
	header := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(title)
	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	// Hard-clip every line to the text area so block-glyph content (gauges,
	// charts, heat cells) that cannot word-wrap never spills past the border.
	content = clipLines(content, inner-2)
	st := lipgloss.NewStyle().
		Border(border).
		BorderForeground(accent).
		Padding(padV, 1).
		Width(inner)
	if bodyLines > 0 {
		st = st.Height(bodyLines + 1) // +1 for the header line
	}
	return st.Render(content)
}

// panelH is panel with a fixed body height, so panels that share a row (or a
// column) line up to the same box height.
func panelH(title string, accent lipgloss.Color, width, bodyLines int, body string) string {
	padV := 0
	if renderCompact {
		padV = 1
	}
	return boxedH(title, accent, width, body, lipgloss.RoundedBorder(), padV, bodyLines)
}

// panelSpec describes one panel for the row/column equalizers.
type panelSpec struct {
	title  string
	accent lipgloss.Color
	width  int
	body   string
}

// lineCount returns the number of display lines in s ("" is zero lines).
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func maxBodyLines(specs []panelSpec) int {
	h := 0
	for _, s := range specs {
		if n := lineCount(s.body); n > h {
			h = n
		}
	}
	return h
}

// panelsRow lays out panels side by side at a uniform box height (the tallest
// body sets the height). When stack is true it falls back to a natural-height
// vertical stack for narrow terminals.
func panelsRow(stack bool, gap int, specs ...panelSpec) string {
	if stack {
		out := make([]string, len(specs))
		for i, s := range specs {
			out[i] = panel(s.title, s.accent, s.width, s.body)
		}
		return arrangePanels(true, gap, out...)
	}
	h := maxBodyLines(specs)
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = panelH(s.title, s.accent, s.width, h, s.body)
	}
	return arrangePanels(false, gap, out...)
}

// panelsCol stacks panels vertically at a uniform box height, so a column of
// instrument cards reads as an even grid rather than ragged blocks.
func panelsCol(specs ...panelSpec) string {
	h := maxBodyLines(specs)
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = panelH(s.title, s.accent, s.width, h, s.body)
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}

// vstack joins panels vertically, inserting blank spacer rows in compact mode
// for a more spacious layout.
func vstack(parts ...string) string {
	if !renderCompact {
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
	spaced := make([]string, 0, len(parts)*2)
	for i, p := range parts {
		if i > 0 {
			spaced = append(spaced, "")
		}
		spaced = append(spaced, p)
	}
	return lipgloss.JoinVertical(lipgloss.Left, spaced...)
}

// clipLines truncates each line to w display cells, ANSI-aware.
func clipLines(s string, w int) string {
	if w < 1 {
		w = 1
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = ansi.Truncate(ln, w, "")
	}
	return strings.Join(lines, "\n")
}

// arrangePanels lays out pre-rendered panels in a row, or stacks them
// vertically when stack is true (narrow terminal fallback).
func arrangePanels(stack bool, gap int, panels ...string) string {
	if stack {
		return lipgloss.JoinVertical(lipgloss.Left, panels...)
	}
	out := make([]string, 0, len(panels)*2)
	sep := strings.Repeat(" ", gap)
	for i, p := range panels {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, p)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}

// gauge renders a colored horizontal bar like a fuel/quantity indicator.
// ratio is clamped to [0,1]. Load/share bars use a single calm fill: bar length
// already conveys magnitude, so colour is reserved for genuine warnings
// (annunciator lamps, the airspeed redline) rather than "this is the biggest".
func gauge(ratio float64, width int) string {
	return gaugeColored(ratio, width, colCyan)
}

func gaugeColored(ratio float64, width int, fill lipgloss.Color) string {
	if width < 1 {
		width = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	bar := lipgloss.NewStyle().Foreground(fill).Render(repeat("█", filled))
	return bar + gaugeEmpty.Render(repeat("░", width-filled))
}

// truncate shortens s to at most max runes (never splitting a multibyte rune),
// appending an ellipsis when it cuts. Byte-slicing would corrupt UTF-8 names
// like Korean project directories.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		if max < 1 {
			return ""
		}
		return string(r[:1])
	}
	return string(r[:max-1]) + "…"
}

// gridWidths splits total into n columns of width >= min. If a column would be
// narrower than min, it returns (total, true) signalling the caller to stack the
// cells vertically at full width instead of overflowing horizontally.
func gridWidths(total, gap, n, min int) (cellW int, stack bool) {
	if n <= 1 {
		return total, false
	}
	cellW = (total - gap*(n-1)) / n
	if cellW < min {
		return total, true
	}
	return cellW, false
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
