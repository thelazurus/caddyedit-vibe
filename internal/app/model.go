// Package app wires the buffer, highlighter, outline, templates, and
// validate packages into an interactive Bubble Tea program.
package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thelazurus/caddyedit-vibe/internal/buffer"
	"github.com/thelazurus/caddyedit-vibe/internal/outline"
	"github.com/thelazurus/caddyedit-vibe/internal/templates"
	"github.com/thelazurus/caddyedit-vibe/internal/validate"
)

type mode int

const (
	modeEdit mode = iota
	modeOutline
	modeWizard
	modeHelp
)

type wizardStep int

const (
	wizStepTemplate wizardStep = iota
	wizStepDomain
	wizStepUpstream
	wizStepComment
)

var (
	styleHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1)
	styleStatus     = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styleStatusErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("160")).Bold(true)
	styleStatusOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("28")).Bold(true)
	styleGutter     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	styleCursor     = lipgloss.NewStyle().Reverse(true)
	styleSelected   = lipgloss.NewStyle().Reverse(true).Bold(true)
	styleDim        = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleTitle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	styleModalBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2)
	styleFieldLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
)

type Model struct {
	buf      *buffer.Buffer
	hostPath string
	target   validate.Target

	width, height int
	viewTop       int
	mode          mode

	outlineBlocks []outline.Block
	outlineIdx    int

	wizStep        wizardStep
	wizTemplateIdx int
	wizDomain      string
	wizUpstream    string
	wizComment     string
	wizInput       textinput.Model

	status      string
	statusIsErr bool
	statusIsOK  bool
	busy        bool

	quitConfirm bool
}

func New(hostPath string, content string, target validate.Target) Model {
	ti := textinput.New()
	ti.CharLimit = 200
	return Model{
		buf:      buffer.New(content),
		hostPath: hostPath,
		target:   target,
		wizInput: ti,
		status:   fmt.Sprintf("target: %s — ^S save  ^O outline  ^N new site  ^V validate  ^R reload  q quit", target.Description()),
	}
}

func (m Model) Init() tea.Cmd { return nil }

// --- messages ---

type validateResultMsg struct{ res validate.Result }
type reloadResultMsg struct{ res validate.Result }

func (m Model) doValidate() tea.Cmd {
	target := m.target
	hostPath := m.hostPath
	content := []byte(m.buf.String())
	return func() tea.Msg {
		res := validate.Validate(context.Background(), hostPath, content, target)
		return validateResultMsg{res}
	}
}

func (m Model) doReload() tea.Cmd {
	target := m.target
	return func() tea.Msg {
		res := validate.Reload(context.Background(), target)
		return reloadResultMsg{res}
	}
}

// --- update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case validateResultMsg:
		m.busy = false
		m.setStatus(fmt.Sprintf("[validate: %s] %s", msg.res.Source, msg.res.Message), !msg.res.OK, msg.res.OK)
		return m, nil

	case reloadResultMsg:
		m.busy = false
		m.setStatus(fmt.Sprintf("[reload: %s] %s", msg.res.Source, msg.res.Message), !msg.res.OK, msg.res.OK)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) setStatus(s string, isErr, isOK bool) {
	m.status = s
	m.statusIsErr = isErr
	m.statusIsOK = isOK
}

func (m *Model) clearStatusFlags() {
	m.statusIsErr = false
	m.statusIsOK = false
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeOutline:
		return m.handleOutlineKey(msg)
	case modeWizard:
		return m.handleWizardKey(msg)
	case modeHelp:
		m.mode = modeEdit
		return m, nil
	default:
		return m.handleEditKey(msg)
	}
}

func (m Model) handleEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.quitConfirm && key != "Q" {
		m.quitConfirm = false
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if m.buf.Dirty {
			m.quitConfirm = true
			m.setStatus("unsaved changes — ^S to save, or press Q (shift) to discard and quit", true, false)
			return m, nil
		}
		return m, tea.Quit
	case "Q":
		return m, tea.Quit
	case "ctrl+s":
		if err := m.save(); err != nil {
			m.setStatus("save failed: "+err.Error(), true, false)
		} else {
			m.setStatus("saved "+m.hostPath, false, true)
		}
		return m, nil
	case "ctrl+o":
		m.outlineBlocks = outline.Parse(m.buf.Lines)
		if len(m.outlineBlocks) == 0 {
			m.setStatus("no top-level blocks found", true, false)
			return m, nil
		}
		m.outlineIdx = m.nearestOutlineIndex()
		m.mode = modeOutline
		m.clearStatusFlags()
		return m, nil
	case "ctrl+n":
		m.mode = modeWizard
		m.wizStep = wizStepTemplate
		m.wizTemplateIdx = 0
		m.wizDomain, m.wizUpstream, m.wizComment = "", "", ""
		m.clearStatusFlags()
		return m, nil
	case "ctrl+v":
		if err := m.save(); err != nil {
			m.setStatus("save failed: "+err.Error(), true, false)
			return m, nil
		}
		m.busy = true
		m.setStatus("validating via "+m.target.Description()+"...", false, false)
		return m, m.doValidate()
	case "ctrl+r":
		m.busy = true
		m.setStatus("reloading via "+m.target.Description()+"...", false, false)
		return m, m.doReload()
	case "?":
		m.mode = modeHelp
		return m, nil
	case "up":
		m.buf.MoveUp()
	case "down":
		m.buf.MoveDown()
	case "left":
		m.buf.MoveLeft()
	case "right":
		m.buf.MoveRight()
	case "home":
		m.buf.MoveHome()
	case "end":
		m.buf.MoveEnd()
	case "ctrl+home":
		m.buf.MoveDocStart()
	case "ctrl+end":
		m.buf.MoveDocEnd()
	case "pgup":
		m.buf.PageUp(m.viewportHeight())
	case "pgdown":
		m.buf.PageDown(m.viewportHeight())
	case "enter":
		m.buf.InsertNewline()
	case "backspace":
		m.buf.Backspace()
	case "delete":
		m.buf.DeleteForward()
	case "tab":
		m.buf.InsertTab()
	default:
		if len(msg.Runes) > 0 {
			for _, r := range msg.Runes {
				m.buf.InsertRune(r)
			}
		}
	}
	m.scrollToCursor()
	return m, nil
}

func (m Model) handleOutlineKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.outlineIdx > 0 {
			m.outlineIdx--
		}
	case "down", "j":
		if m.outlineIdx < len(m.outlineBlocks)-1 {
			m.outlineIdx++
		}
	case "enter":
		b := m.outlineBlocks[m.outlineIdx]
		m.buf.GotoLine(b.StartLine)
		m.mode = modeEdit
		m.scrollToCursor()
	case "esc", "ctrl+o", "q":
		m.mode = modeEdit
	}
	return m, nil
}

func (m Model) handleWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" {
		m.mode = modeEdit
		m.setStatus("new-site wizard cancelled", false, false)
		return m, nil
	}

	switch m.wizStep {
	case wizStepTemplate:
		switch key {
		case "up", "k":
			if m.wizTemplateIdx > 0 {
				m.wizTemplateIdx--
			}
		case "down", "j":
			if m.wizTemplateIdx < len(templates.All)-1 {
				m.wizTemplateIdx++
			}
		case "enter":
			m.wizStep = wizStepDomain
			m.wizInput.SetValue("")
			m.wizInput.Placeholder = "myapp.lazurus.me"
			m.wizInput.Focus()
		}
		return m, nil

	case wizStepDomain:
		if key == "enter" {
			v := strings.TrimSpace(m.wizInput.Value())
			if v == "" {
				m.setStatus("domain cannot be empty", true, false)
				return m, nil
			}
			m.wizDomain = v
			tpl := templates.All[m.wizTemplateIdx]
			if tpl.NeedsUpstream {
				m.wizStep = wizStepUpstream
				m.wizInput.SetValue("")
				m.wizInput.Placeholder = "http://192.168.0.171:8080"
			} else {
				m.wizStep = wizStepComment
				m.wizInput.SetValue("")
				m.wizInput.Placeholder = "optional comment"
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.wizInput, cmd = m.wizInput.Update(msg)
		return m, cmd

	case wizStepUpstream:
		if key == "enter" {
			v := strings.TrimSpace(m.wizInput.Value())
			if v == "" {
				m.setStatus("upstream cannot be empty", true, false)
				return m, nil
			}
			m.wizUpstream = v
			m.wizStep = wizStepComment
			m.wizInput.SetValue("")
			m.wizInput.Placeholder = "optional comment"
			return m, nil
		}
		var cmd tea.Cmd
		m.wizInput, cmd = m.wizInput.Update(msg)
		return m, cmd

	case wizStepComment:
		if key == "enter" {
			m.wizComment = strings.TrimSpace(m.wizInput.Value())
			tpl := templates.All[m.wizTemplateIdx]
			lines := tpl.Build(m.wizDomain, m.wizUpstream, m.wizComment)
			m.insertBlockAtEnd(lines)
			m.mode = modeEdit
			m.setStatus(fmt.Sprintf("inserted %s block for %s (unsaved — ^S to save)", tpl.Name, m.wizDomain), false, false)
			return m, nil
		}
		var cmd tea.Cmd
		m.wizInput, cmd = m.wizInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) insertBlockAtEnd(lines []string) {
	if len(m.buf.Lines) > 0 && strings.TrimSpace(m.buf.Lines[len(m.buf.Lines)-1]) != "" {
		m.buf.Lines = append(m.buf.Lines, "")
	}
	insertAt := len(m.buf.Lines)
	m.buf.Lines = append(m.buf.Lines, lines...)
	m.buf.Row = insertAt
	m.buf.Col = 0
	m.buf.Dirty = true
	m.scrollToCursor()
}

func (m *Model) save() error {
	content := m.buf.String()
	if existing, err := os.ReadFile(m.hostPath); err == nil {
		_ = os.WriteFile(m.hostPath+".bak", existing, 0644)
	}
	if err := os.WriteFile(m.hostPath, []byte(content), 0644); err != nil {
		return err
	}
	m.buf.Dirty = false
	return nil
}

func (m *Model) viewportHeight() int {
	h := m.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

func (m *Model) scrollToCursor() {
	vh := m.viewportHeight()
	if m.buf.Row < m.viewTop {
		m.viewTop = m.buf.Row
	}
	if m.buf.Row >= m.viewTop+vh {
		m.viewTop = m.buf.Row - vh + 1
	}
	if m.viewTop < 0 {
		m.viewTop = 0
	}
}

// nearestOutlineIndex finds the block the cursor currently sits inside (or
// nearest one after it), so opening the outline starts near where you are.
func (m Model) nearestOutlineIndex() int {
	for i, b := range m.outlineBlocks {
		if m.buf.Row >= b.StartLine && m.buf.Row <= b.EndLine {
			return i
		}
	}
	for i, b := range m.outlineBlocks {
		if b.StartLine >= m.buf.Row {
			return i
		}
	}
	return 0
}
