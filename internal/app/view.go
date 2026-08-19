package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/thelazurus/caddyedit-vibe/internal/highlight"
	"github.com/thelazurus/caddyedit-vibe/internal/templates"
)

func (m Model) View() string {
	if m.width == 0 {
		return "starting..."
	}

	header := m.renderHeader()
	status := m.renderStatus()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(status)
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body string
	switch m.mode {
	case modeOutline:
		body = m.renderOutline(bodyHeight)
	case modeWizard:
		body = m.renderWizard(bodyHeight)
	case modeHelp:
		body = m.renderHelp(bodyHeight)
	default:
		body = m.renderEditor(bodyHeight)
	}

	return header + "\n" + body + "\n" + status
}

func (m Model) renderHeader() string {
	dirty := ""
	if m.buf.Dirty {
		dirty = " [modified]"
	}
	title := fmt.Sprintf(" caddyedit — %s%s ", m.hostPath, dirty)
	for lipgloss.Width(title) < m.width {
		title += " "
	}
	return styleHeader.Width(m.width).Render(title)
}

func (m Model) renderStatus() string {
	style := styleStatus
	if m.statusIsErr {
		style = styleStatusErr
	} else if m.statusIsOK {
		style = styleStatusOK
	}
	s := m.status
	if m.busy {
		s = "⏳ " + s
	}
	return style.Width(m.width).Render(truncate(s, m.width))
}

func (m Model) renderEditor(h int) string {
	gutterW := len(fmt.Sprintf("%d", m.buf.LineCount())) + 1
	contentW := m.width - gutterW - 3
	if contentW < 10 {
		contentW = 10
	}

	var lines []string
	for i := 0; i < h; i++ {
		row := m.viewTop + i
		if row >= m.buf.LineCount() {
			lines = append(lines, styleGutter.Render(strings.Repeat(" ", gutterW)+" ~"))
			continue
		}
		raw := m.buf.Line(row)
		display := expandTabs(raw)
		truncated := truncateRunes(display, contentW)

		gutter := styleGutter.Render(fmt.Sprintf("%*d │", gutterW, row+1))
		var content string
		if row == m.buf.Row {
			content = renderLineWithCursor(truncated, m.buf.Col)
		} else {
			content = highlight.Render(truncated)
		}
		lines = append(lines, gutter+" "+content)
	}
	return strings.Join(lines, "\n")
}

func renderLineWithCursor(line string, col int) string {
	toks := highlight.Tokenize(line)
	var b strings.Builder
	remaining := col
	placed := false

	for _, t := range toks {
		style := highlight.StyleFor(t.Kind)
		tr := []rune(t.Text)
		if placed {
			b.WriteString(style.Render(t.Text))
			continue
		}
		if remaining > len(tr) {
			remaining -= len(tr)
			b.WriteString(style.Render(t.Text))
			continue
		}
		before := string(tr[:remaining])
		after := ""
		cur := " "
		if remaining < len(tr) {
			cur = string(tr[remaining])
			after = string(tr[remaining+1:])
		}
		b.WriteString(style.Render(before))
		b.WriteString(styleCursor.Render(cur))
		b.WriteString(style.Render(after))
		placed = true
	}
	if !placed {
		b.WriteString(styleCursor.Render(" "))
	}
	return b.String()
}

func (m Model) renderOutline(h int) string {
	title := styleTitle.Render("Outline") + styleDim.Render("  (↑/↓ move, enter jump, esc close)")
	var rows []string
	for i, b := range m.outlineBlocks {
		label := b.Header
		if b.IsSnippet {
			label = "snippet " + label
		}
		line := fmt.Sprintf("%4d  %s", b.StartLine+1, label)
		if b.Comment != "" {
			line += styleDim.Render("  — " + b.Comment)
		}
		if i == m.outlineIdx {
			line = styleSelected.Render(fmt.Sprintf("%4d  %s", b.StartLine+1, label))
			if b.Comment != "" {
				line += styleSelected.Render("  — " + b.Comment)
			}
		}
		rows = append(rows, line)
	}
	content := title + "\n\n" + strings.Join(rows, "\n")
	return lipgloss.NewStyle().Height(h).Render(content)
}

func (m Model) renderWizard(h int) string {
	tpl := templates.All[m.wizTemplateIdx]

	var box strings.Builder
	box.WriteString(styleTitle.Render("New Site") + "\n\n")

	switch m.wizStep {
	case wizStepTemplate:
		box.WriteString("Pick a template:\n\n")
		for i, t := range templates.All {
			line := fmt.Sprintf("%s — %s", t.Name, t.Description)
			if i == m.wizTemplateIdx {
				line = styleSelected.Render("› " + line)
			} else {
				line = "  " + line
			}
			box.WriteString(line + "\n")
		}
		box.WriteString("\n" + styleDim.Render("↑/↓ choose  enter next  esc cancel"))

	case wizStepDomain:
		box.WriteString(styleDim.Render("Template: "+tpl.Name) + "\n\n")
		box.WriteString(styleFieldLabel.Render("Domain (site address):") + "\n")
		box.WriteString(m.wizInput.View() + "\n\n")
		box.WriteString(styleDim.Render("enter next  esc cancel"))

	case wizStepUpstream:
		box.WriteString(styleDim.Render("Template: "+tpl.Name) + "\n")
		box.WriteString(styleDim.Render("Domain: "+m.wizDomain) + "\n\n")
		box.WriteString(styleFieldLabel.Render("Upstream (e.g. http://192.168.0.171:8080):") + "\n")
		box.WriteString(m.wizInput.View() + "\n\n")
		box.WriteString(styleDim.Render("enter next  esc cancel"))

	case wizStepComment:
		box.WriteString(styleDim.Render("Template: "+tpl.Name) + "\n")
		box.WriteString(styleDim.Render("Domain: "+m.wizDomain) + "\n")
		if tpl.NeedsUpstream {
			box.WriteString(styleDim.Render("Upstream: "+m.wizUpstream) + "\n")
		}
		box.WriteString("\n" + styleFieldLabel.Render("Comment (optional header line):") + "\n")
		box.WriteString(m.wizInput.View() + "\n\n")
		box.WriteString(styleDim.Render("enter insert block  esc cancel"))
	}

	modal := styleModalBox.Render(box.String())
	return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, modal)
}

func (m Model) renderHelp(h int) string {
	text := `caddyedit — keybindings

  ^S   save (writes a .bak backup of the previous version)
  ^O   toggle outline (jump to a site/snippet block)
  ^N   new site wizard (insert a template block)
  ^V   save + validate (via docker exec / local caddy binary / bundled parser)
  ^R   reload the running Caddy instance
  q    quit (press Q to discard unsaved changes and quit)
  ?    this help screen, any other key closes it

  arrows / home / end / pgup / pgdn   navigate
  enter / backspace / delete / tab    edit
`
	return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, styleModalBox.Render(text))
}

func expandTabs(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		return r
	}, s)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

func truncate(s string, max int) string {
	if lipgloss.Width(s) <= max {
		return s
	}
	return truncateRunes(s, max)
}
