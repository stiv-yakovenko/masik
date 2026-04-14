package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	defaultTerminalCols = 120
	defaultTerminalRows = 32
	terminalCellWidthDp = 8
)

type terminalState struct {
	visible        bool
	height         int
	activeTab      int
	nextTabID      int
	toggleButton   widget.Clickable
	minimizeButton widget.Clickable
	addButton      widget.Clickable
	splitter       verticalPaneSplitter
	tabs           []terminalTabState
}

type terminalTabState struct {
	ID          int
	Title       string
	TabButton   widget.Clickable
	CloseButton widget.Clickable
	List        widget.List
	FocusTag    *terminalFocusTag
	StartDir    string
	Session     *terminalSession
	Buffer      terminalBuffer
	Debug       []string
	ScrollRows  int
	Stopping    bool
	FocusNext   bool
	LastCols    int
	LastRows    int
}

type terminalFocusTag struct{}

type terminalProcessEvent struct {
	TabID  int
	Data   []byte
	Closed bool
	Err    error
}

type terminalBuffer struct {
	lines       [][]rune
	cursorRow   int
	cursorCol   int
	savedRow    int
	savedCol    int
	utf8Partial []byte
	escMode     terminalEscapeMode
	escBuf      []byte
}

type terminalEscapeMode uint8

const (
	terminalEscapeNone terminalEscapeMode = iota
	terminalEscapeEsc
	terminalEscapeCSI
	terminalEscapeOSC
	terminalEscapeOSCMayEnd
)

func newTerminalState() terminalState {
	return terminalState{
		height:    220,
		activeTab: -1,
		nextTabID: 1,
	}
}

func (t *terminalState) normalize(totalHeight, splitterHeight int) {
	if !t.visible {
		return
	}
	maxTerminalHeight := totalHeight - splitterHeight - minMainHeightPx
	if maxTerminalHeight < minTerminalHeight {
		maxTerminalHeight = minTerminalHeight
	}
	t.height = clampInt(t.height, minTerminalHeight, maxTerminalHeight)
}

func (t *terminalState) show(currentDir string, win *app.Window, terminalEvents chan<- terminalProcessEvent) error {
	t.visible = true
	if len(t.tabs) == 0 {
		_, err := t.openTab(currentDir, win, terminalEvents)
		return err
	}
	if t.activeTab < 0 || t.activeTab >= len(t.tabs) {
		t.activeTab = len(t.tabs) - 1
	}
	tab := t.activeTabState()
	if tab == nil {
		return fmt.Errorf("no active terminal tab")
	}
	tab.FocusNext = true
	return t.ensureSession(tab, win, terminalEvents)
}

func (t *terminalState) collapse() {
	t.visible = false
}

func (t *terminalState) closeAll() {
	t.visible = false
	for i := range t.tabs {
		t.stopSession(&t.tabs[i])
	}
	t.tabs = nil
	t.activeTab = -1
}

func (t *terminalState) activeTabState() *terminalTabState {
	if t.activeTab < 0 || t.activeTab >= len(t.tabs) {
		return nil
	}
	return &t.tabs[t.activeTab]
}

func (t *terminalState) openTab(currentDir string, win *app.Window, terminalEvents chan<- terminalProcessEvent) (string, error) {
	tab := newTerminalTabState(t.nextTabID, currentDir)
	t.nextTabID++
	t.tabs = append(t.tabs, tab)
	t.activeTab = len(t.tabs) - 1
	t.visible = true
	t.tabs[t.activeTab].FocusNext = true
	if err := t.ensureSession(&t.tabs[t.activeTab], win, terminalEvents); err != nil {
		t.tabs = t.tabs[:len(t.tabs)-1]
		t.activeTab = len(t.tabs) - 1
		return "", err
	}
	return t.tabs[t.activeTab].Title, nil
}

func newTerminalTabState(id int, startDir string) terminalTabState {
	list := widget.List{}
	list.Axis = layout.Vertical
	list.ScrollToEnd = true
	title := "Terminal"
	if id > 1 {
		title = fmt.Sprintf("Terminal %d", id)
	}
	tab := terminalTabState{
		ID:       id,
		Title:    title,
		List:     list,
		FocusTag: &terminalFocusTag{},
		StartDir: startDir,
	}
	tab.Buffer.reset()
	return tab
}

func (tab *terminalTabState) appendDebug(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[terminal %d] %s\n", tab.ID, message)
	tab.Debug = append(tab.Debug, message)
	if len(tab.Debug) > 80 {
		tab.Debug = append([]string(nil), tab.Debug[len(tab.Debug)-80:]...)
	}
}

func (t *terminalState) closeTab(index int) {
	if index < 0 || index >= len(t.tabs) {
		return
	}
	t.stopSession(&t.tabs[index])
	t.tabs = append(t.tabs[:index], t.tabs[index+1:]...)
	switch {
	case len(t.tabs) == 0:
		t.visible = false
		t.activeTab = -1
	case index < t.activeTab:
		t.activeTab--
	case index == t.activeTab:
		if index >= len(t.tabs) {
			index = len(t.tabs) - 1
		}
		t.activeTab = index
	}
	if tab := t.activeTabState(); tab != nil {
		tab.FocusNext = true
	}
}

func (t *terminalState) ensureSession(tab *terminalTabState, win *app.Window, terminalEvents chan<- terminalProcessEvent) error {
	if tab == nil {
		return fmt.Errorf("no active terminal tab")
	}
	if tab.Session != nil {
		return nil
	}
	session, err := startTerminalSession(tab.StartDir, defaultTerminalCols, defaultTerminalRows)
	if err != nil {
		tab.appendDebug("ensureSession error: " + err.Error())
		return err
	}
	tab.Session = session
	tab.Stopping = false
	tab.LastCols = defaultTerminalCols
	tab.LastRows = defaultTerminalRows
	tab.Buffer.reset()
	tab.appendDebug(fmt.Sprintf("session started dir=%q cols=%d rows=%d", tab.StartDir, defaultTerminalCols, defaultTerminalRows))
	go session.readOutput(tab.ID, terminalEvents, win)
	go session.wait(tab.ID, terminalEvents, win)
	return nil
}

func (t *terminalState) stopSession(tab *terminalTabState) {
	if tab == nil || tab.Session == nil {
		return
	}
	tab.Stopping = true
	tab.appendDebug("stopSession requested")
	tab.Session.Stop()
	tab.Session = nil
}

func (t *terminalState) applyProcessEvent(event terminalProcessEvent) (string, bool) {
	tab := t.findTabByID(event.TabID)
	if tab == nil {
		return "", false
	}
	if len(event.Data) > 0 {
		sample := string(event.Data)
		sample = strings.ReplaceAll(sample, "\r", "\\r")
		sample = strings.ReplaceAll(sample, "\n", "\\n")
		if len(sample) > 120 {
			sample = sample[:120] + "..."
		}
		tab.appendDebug(fmt.Sprintf("chunk %dB: %s", len(event.Data), sample))
		tab.Buffer.Process(event.Data)
	}
	if !event.Closed {
		return "", false
	}

	if tab.Session != nil {
		tab.Session.Close()
		tab.Session = nil
	}
	if tab.Stopping {
		tab.Stopping = false
		return "", false
	}
	if event.Err != nil {
		tab.appendDebug("closed with error: " + event.Err.Error())
		return "Terminal session ended with an error.", true
	}
	tab.appendDebug("closed without error")
	return "Terminal session ended.", false
}

func (t *terminalState) findTabByID(id int) *terminalTabState {
	for i := range t.tabs {
		if t.tabs[i].ID == id {
			return &t.tabs[i]
		}
	}
	return nil
}

func (t *terminalState) handleInput(gtx layout.Context) {
	tab := t.activeTabState()
	if tab == nil {
		return
	}
	if tab.FocusNext {
		gtx.Execute(key.FocusCmd{Tag: tab.FocusTag})
		tab.FocusNext = false
	}
	filters := []event.Filter{
		pointer.Filter{Target: tab.FocusTag, Kinds: pointer.Press},
		key.FocusFilter{Target: tab.FocusTag},
		key.Filter{Focus: tab.FocusTag, Name: key.NameTab},
		key.Filter{Focus: tab.FocusTag, Name: ""},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch ev := ev.(type) {
		case pointer.Event:
			if ev.Kind == pointer.Press {
				tab.appendDebug("pointer press -> focus")
				gtx.Execute(key.FocusCmd{Tag: tab.FocusTag})
			}
		case key.FocusEvent:
			tab.appendDebug(fmt.Sprintf("focus event: %v", ev.Focus))
		case key.EditEvent:
			if tab.Session == nil || ev.Text == "" {
				tab.appendDebug(fmt.Sprintf("edit ignored session_nil=%v text=%q", tab.Session == nil, ev.Text))
				continue
			}
			tab.ScrollRows = 0
			tab.appendDebug(fmt.Sprintf("edit text=%q", ev.Text))
			n, err := tab.Session.Write([]byte(ev.Text))
			if err != nil {
				tab.appendDebug(fmt.Sprintf("edit write error n=%d err=%v", n, err))
			} else {
				tab.appendDebug(fmt.Sprintf("edit write ok n=%d", n))
			}
		case key.Event:
			if ev.State != key.Press || tab.Session == nil {
				tab.appendDebug(fmt.Sprintf("key ignored name=%q state=%v session_nil=%v", ev.Name, ev.State, tab.Session == nil))
				continue
			}
			tab.appendDebug(fmt.Sprintf("key name=%q mods=%v", ev.Name, ev.Modifiers))
			data, ok := terminalKeyBytes(ev)
			if !ok || len(data) == 0 {
				tab.appendDebug("key -> no bytes")
				continue
			}
			tab.ScrollRows = 0
			tab.appendDebug(fmt.Sprintf("key bytes=%q", string(data)))
			n, err := tab.Session.Write(data)
			if err != nil {
				tab.appendDebug(fmt.Sprintf("key write error n=%d err=%v", n, err))
			} else {
				tab.appendDebug(fmt.Sprintf("key write ok n=%d", n))
			}
		}
	}
}

func terminalKeyBytes(ev key.Event) ([]byte, bool) {
	if ev.Modifiers.Contain(key.ModCtrl) && len(ev.Name) == 1 {
		ch := ev.Name[0]
		if ch >= 'A' && ch <= 'Z' {
			return []byte{byte(ch - 'A' + 1)}, true
		}
	}
	switch ev.Name {
	case key.NameReturn, key.NameEnter:
		return []byte("\r"), true
	case key.NameTab:
		if ev.Modifiers.Contain(key.ModShift) {
			return []byte("\x1b[Z"), true
		}
		return []byte("\t"), true
	case key.NameDeleteBackward:
		return []byte{'\b'}, true
	case key.NameDeleteForward:
		return []byte("\x1b[3~"), true
	case key.NameLeftArrow:
		return []byte("\x1b[D"), true
	case key.NameRightArrow:
		return []byte("\x1b[C"), true
	case key.NameUpArrow:
		return []byte("\x1b[A"), true
	case key.NameDownArrow:
		return []byte("\x1b[B"), true
	case key.NameHome:
		return []byte("\x1b[H"), true
	case key.NameEnd:
		return []byte("\x1b[F"), true
	case key.NamePageUp:
		return []byte("\x1b[5~"), true
	case key.NamePageDown:
		return []byte("\x1b[6~"), true
	case key.NameEscape:
		return []byte{0x1b}, true
	default:
		return nil, false
	}
}

func (b *terminalBuffer) reset() {
	b.lines = [][]rune{make([]rune, 0, 64)}
	b.cursorRow = 0
	b.cursorCol = 0
	b.savedRow = 0
	b.savedCol = 0
	b.utf8Partial = nil
	b.escMode = terminalEscapeNone
	b.escBuf = nil
}

func (b *terminalBuffer) ensureCursorLine() {
	for len(b.lines) <= b.cursorRow {
		b.lines = append(b.lines, make([]rune, 0, 64))
	}
}

func (b *terminalBuffer) Process(data []byte) {
	if len(data) == 0 {
		return
	}
	if len(b.lines) == 0 {
		b.reset()
	}
	if len(b.utf8Partial) > 0 {
		combined := make([]byte, 0, len(b.utf8Partial)+len(data))
		combined = append(combined, b.utf8Partial...)
		combined = append(combined, data...)
		data = combined
		b.utf8Partial = nil
	}
	for len(data) > 0 {
		switch b.escMode {
		case terminalEscapeCSI:
			final := data[0]
			data = data[1:]
			b.escBuf = append(b.escBuf, final)
			if final >= 0x40 && final <= 0x7E {
				b.handleCSI(b.escBuf)
				b.escBuf = b.escBuf[:0]
				b.escMode = terminalEscapeNone
			}
			continue
		case terminalEscapeOSC:
			ch := data[0]
			data = data[1:]
			switch ch {
			case 0x07:
				b.escBuf = b.escBuf[:0]
				b.escMode = terminalEscapeNone
			case 0x1b:
				b.escMode = terminalEscapeOSCMayEnd
			default:
				b.escBuf = append(b.escBuf, ch)
			}
			continue
		case terminalEscapeOSCMayEnd:
			ch := data[0]
			data = data[1:]
			if ch == '\\' {
				b.escBuf = b.escBuf[:0]
				b.escMode = terminalEscapeNone
			} else {
				b.escBuf = append(b.escBuf, 0x1b, ch)
				b.escMode = terminalEscapeOSC
			}
			continue
		case terminalEscapeEsc:
			ch := data[0]
			data = data[1:]
			switch ch {
			case '[':
				b.escBuf = b.escBuf[:0]
				b.escMode = terminalEscapeCSI
			case ']':
				b.escBuf = b.escBuf[:0]
				b.escMode = terminalEscapeOSC
			case '7':
				b.savedRow = b.cursorRow
				b.savedCol = b.cursorCol
				b.escMode = terminalEscapeNone
			case '8':
				b.cursorRow = b.savedRow
				b.cursorCol = b.savedCol
				b.ensureCursorLine()
				b.escMode = terminalEscapeNone
			case 'c':
				b.reset()
				b.escMode = terminalEscapeNone
			default:
				b.escMode = terminalEscapeNone
			}
			continue
		}

		ch := data[0]
		if ch == 0x1b {
			data = data[1:]
			b.escMode = terminalEscapeEsc
			continue
		}
		if ch < 0x20 || ch == 0x7f {
			data = data[1:]
			b.handleControl(ch)
			continue
		}
		if ch < utf8.RuneSelf {
			data = data[1:]
			b.putRune(rune(ch))
			continue
		}
		if !utf8.FullRune(data) {
			b.utf8Partial = append(b.utf8Partial[:0], data...)
			break
		}
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			data = data[1:]
			continue
		}
		data = data[size:]
		b.putRune(r)
	}
}

func (b *terminalBuffer) handleControl(ch byte) {
	switch ch {
	case '\r':
		b.cursorCol = 0
	case '\n':
		b.cursorRow++
		b.cursorCol = 0
		b.ensureCursorLine()
	case '\b':
		if b.cursorCol > 0 {
			b.cursorCol--
		}
	case '\t':
		next := ((b.cursorCol / 8) + 1) * 8
		for b.cursorCol < next {
			b.putRune(' ')
		}
	case 0x00, 0x07:
		// Ignore NUL and BEL.
	default:
	}
}

func (b *terminalBuffer) putRune(r rune) {
	b.ensureCursorLine()
	line := b.lines[b.cursorRow]
	for len(line) < b.cursorCol {
		line = append(line, ' ')
	}
	if b.cursorCol < len(line) {
		line[b.cursorCol] = r
	} else {
		line = append(line, r)
	}
	b.lines[b.cursorRow] = line
	b.cursorCol++
}

func (b *terminalBuffer) handleCSI(seq []byte) {
	if len(seq) == 0 {
		return
	}
	final := seq[len(seq)-1]
	params := string(seq[:len(seq)-1])
	private := strings.HasPrefix(params, "?")
	if private {
		params = strings.TrimPrefix(params, "?")
	}
	values := parseTerminalCSIParams(params)
	switch final {
	case 'A':
		b.cursorRow -= terminalCSIParam(values, 0, 1)
		if b.cursorRow < 0 {
			b.cursorRow = 0
		}
	case 'B':
		b.cursorRow += terminalCSIParam(values, 0, 1)
		b.ensureCursorLine()
	case 'C':
		b.cursorCol += terminalCSIParam(values, 0, 1)
	case 'D':
		b.cursorCol -= terminalCSIParam(values, 0, 1)
		if b.cursorCol < 0 {
			b.cursorCol = 0
		}
	case 'G':
		b.cursorCol = max(0, terminalCSIParam(values, 0, 1)-1)
	case 'H', 'f':
		row := max(0, terminalCSIParam(values, 0, 1)-1)
		col := max(0, terminalCSIParam(values, 1, 1)-1)
		b.cursorRow = row
		b.cursorCol = col
		b.ensureCursorLine()
	case 'J':
		mode := terminalCSIParam(values, 0, 0)
		if mode == 2 {
			b.reset()
		}
	case 'K':
		b.clearLine(terminalCSIParam(values, 0, 0))
	case 'P':
		b.deleteChars(terminalCSIParam(values, 0, 1))
	case '@':
		b.insertSpaces(terminalCSIParam(values, 0, 1))
	case 'm':
		// Ignore styling for now.
	case 's':
		b.savedRow = b.cursorRow
		b.savedCol = b.cursorCol
	case 'u':
		b.cursorRow = b.savedRow
		b.cursorCol = b.savedCol
		b.ensureCursorLine()
	default:
		if private {
			return
		}
	}
}

func parseTerminalCSIParams(params string) []int {
	if params == "" {
		return nil
	}
	parts := strings.Split(params, ";")
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			values = append(values, 0)
			continue
		}
		var value int
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				value = 0
				break
			}
			value = value*10 + int(ch-'0')
		}
		values = append(values, value)
	}
	return values
}

func terminalCSIParam(values []int, index, fallback int) int {
	if index >= len(values) || values[index] == 0 {
		return fallback
	}
	return values[index]
}

func (b *terminalBuffer) clearLine(mode int) {
	b.ensureCursorLine()
	line := b.lines[b.cursorRow]
	switch mode {
	case 1:
		limit := min(len(line), b.cursorCol+1)
		for i := 0; i < limit; i++ {
			line[i] = ' '
		}
	case 2:
		b.lines[b.cursorRow] = line[:0]
		b.cursorCol = 0
	default:
		if b.cursorCol < len(line) {
			line = line[:b.cursorCol]
		}
		b.lines[b.cursorRow] = line
	}
}

func (b *terminalBuffer) deleteChars(count int) {
	if count <= 0 {
		return
	}
	b.ensureCursorLine()
	line := b.lines[b.cursorRow]
	if b.cursorCol >= len(line) {
		return
	}
	end := min(len(line), b.cursorCol+count)
	copy(line[b.cursorCol:], line[end:])
	line = line[:len(line)-(end-b.cursorCol)]
	b.lines[b.cursorRow] = line
}

func (b *terminalBuffer) insertSpaces(count int) {
	if count <= 0 {
		return
	}
	b.ensureCursorLine()
	line := b.lines[b.cursorRow]
	for len(line) < b.cursorCol {
		line = append(line, ' ')
	}
	padding := make([]rune, count)
	for i := range padding {
		padding[i] = ' '
	}
	line = append(line[:b.cursorCol], append(padding, line[b.cursorCol:]...)...)
	b.lines[b.cursorRow] = line
}

func (b *terminalBuffer) lineCount() int {
	if len(b.lines) == 0 {
		return 1
	}
	return len(b.lines)
}

func (b *terminalBuffer) lineText(index int, showCursor bool) string {
	if index < 0 || index >= len(b.lines) {
		return ""
	}
	line := append([]rune(nil), b.lines[index]...)
	if showCursor && index == b.cursorRow {
		for len(line) < b.cursorCol {
			line = append(line, ' ')
		}
		if b.cursorCol < len(line) {
			if line[b.cursorCol] == ' ' {
				line[b.cursorCol] = '█'
			} else {
				line = append(line[:b.cursorCol], append([]rune{'█'}, line[b.cursorCol:]...)...)
			}
		} else {
			line = append(line, '█')
		}
	}
	return strings.TrimRight(string(line), " ")
}

func layoutTerminalPanel(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	terminal := &state.workspace.terminal
	metrics := state.appearance.terminalMetrics()
	return widget.Border{
		Color: color.NRGBA{R: 0xD0, G: 0xD7, B: 0xDE, A: 0xFF},
		Width: unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF})
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutTerminalTabsBar(gtx, th, terminal, metrics)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layoutTerminalOutput(gtx, th, terminal, metrics)
			}),
		)
	})
}

func layoutTerminalTabsBar(gtx layout.Context, th *material.Theme, terminal *terminalState, metrics textMetrics) layout.Dimensions {
	height := textBlockHeightPx(gtx, metrics, 16, 36)
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	fill(gtx, color.NRGBA{R: 0xF1, G: 0xEB, B: 0xE0, A: 0xFF})
	fillRect(gtx, image.Rect(0, height-1, gtx.Constraints.Max.X, height), color.NRGBA{R: 0xD9, G: 0xD1, B: 0xC5, A: 0xFF})

	leftChildren := make([]layout.FlexChild, 0, len(terminal.tabs)*2+1)
	for i := range terminal.tabs {
		tab := &terminal.tabs[i]
		active := i == terminal.activeTab
		leftChildren = append(leftChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTerminalTab(gtx, th, tab, active, metrics)
		}))
		if i < len(terminal.tabs)-1 {
			leftChildren = append(leftChildren, layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout))
		}
	}
	leftChildren = append(leftChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layoutTerminalTabActionButton(gtx, th, &terminal.addButton, "+", metrics)
	}))

	return layout.Inset{
		Left: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top: unit.Dp(4),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx, leftChildren...)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutTerminalTabActionButton(gtx, th, &terminal.minimizeButton, "_", metrics)
			}),
		)
	})
}

func layoutTerminalOutput(gtx layout.Context, th *material.Theme, terminal *terminalState, metrics textMetrics) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	tab := terminal.activeTabState()
	if tab == nil {
		return layout.Dimensions{}
	}

	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tab.FocusTag)
	key.InputHintOp{Tag: tab.FocusTag, Hint: key.HintText}.Add(gtx.Ops)
	pointer.CursorText.Add(gtx.Ops)
	terminal.handleInput(gtx)

	cellWidth := terminalCellWidthPx(gtx, metrics)
	cellHeight := max(1, gtx.Sp(unit.Sp(metrics.LineHeight)))
	cols := max(20, (gtx.Constraints.Max.X-24)/cellWidth)
	rows := max(4, (gtx.Constraints.Max.Y-12)/cellHeight)
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: tab.FocusTag,
			Kinds:  pointer.Scroll,
			ScrollY: pointer.ScrollRange{
				Min: -gtx.Constraints.Max.Y,
				Max: gtx.Constraints.Max.Y,
			},
		})
		if !ok {
			break
		}
		if pe, ok := ev.(pointer.Event); ok {
			step := int(pe.Scroll.Y / float32(cellHeight))
			if step == 0 {
				if pe.Scroll.Y < 0 {
					step = -1
				} else if pe.Scroll.Y > 0 {
					step = 1
				}
			}
			tab.ScrollRows -= step
		}
	}
	if tab.Session != nil && (cols != tab.LastCols || rows != tab.LastRows) {
		if err := tab.Session.Resize(cols, rows); err == nil {
			tab.LastCols = cols
			tab.LastRows = rows
		}
	}

	lineCount := tab.Buffer.lineCount()
	maxScrollRows := max(0, lineCount-rows)
	tab.ScrollRows = clampInt(tab.ScrollRows, 0, maxScrollRows)
	start := max(0, lineCount-rows-tab.ScrollRows)
	if lineCount <= rows {
		tab.ScrollRows = 0
	}

	children := make([]layout.FlexChild, 0, rows)
	for i := 0; i < rows; i++ {
		lineIndex := start + i
		lineText := ""
		if lineIndex < lineCount {
			lineText = tab.Buffer.lineText(lineIndex, true)
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			gtx.Constraints.Min.Y = cellHeight
			gtx.Constraints.Max.Y = cellHeight
			return layout.Inset{
				Left:  unit.Dp(12),
				Right: unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := monoLabelWithMetrics(th, lineText, metrics)
				label.Color = color.NRGBA{R: 0x3D, G: 0x45, B: 0x4B, A: 0xFF}
				return label.Layout(gtx)
			})
		}))
	}

	return layout.Inset{
		Top:    unit.Dp(6),
		Bottom: unit.Dp(6),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func layoutTerminalTab(gtx layout.Context, th *material.Theme, tab *terminalTabState, active bool, metrics textMetrics) layout.Dimensions {
	background := color.NRGBA{R: 0xEE, G: 0xE6, B: 0xDA, A: 0xFF}
	border := color.NRGBA{R: 0xD6, G: 0xCC, B: 0xBF, A: 0xFF}
	textColor := color.NRGBA{R: 0x4A, G: 0x52, B: 0x57, A: 0xFF}
	closeColor := color.NRGBA{R: 0x6A, G: 0x72, B: 0x77, A: 0xFF}
	if active {
		background = color.NRGBA{R: 0xFE, G: 0xFC, B: 0xF8, A: 0xFF}
		border = color.NRGBA{R: 0xC7, G: 0xD2, B: 0xDE, A: 0xFF}
		textColor = color.NRGBA{R: 0x34, G: 0x3D, B: 0x43, A: 0xFF}
		closeColor = color.NRGBA{R: 0x50, G: 0x59, B: 0x5E, A: 0xFF}
	} else if tab.TabButton.Hovered() || tab.CloseButton.Hovered() {
		background = color.NRGBA{R: 0xF7, G: 0xF0, B: 0xE4, A: 0xFF}
	}

	return widget.Border{
		Color: border,
		Width: unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, background)
		return layout.Inset{
			Top:    unit.Dp(3),
			Bottom: unit.Dp(3),
			Left:   unit.Dp(6),
			Right:  unit.Dp(6),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return tab.TabButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{
							Top:    unit.Dp(5),
							Bottom: unit.Dp(5),
							Left:   unit.Dp(8),
							Right:  unit.Dp(10),
						}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := monoLabelWithMetrics(th, tab.Title, metrics)
							label.Color = textColor
							return label.Layout(gtx)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return tab.CloseButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(unit.Dp(22))
						gtx.Constraints.Min = image.Pt(size, size)
						gtx.Constraints.Max = image.Pt(size, size)
						if tab.CloseButton.Hovered() {
							fill(gtx, color.NRGBA{R: 0xE7, G: 0xDD, B: 0xD0, A: 0xFF})
						}
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							label := monoLabelWithMetrics(th, "x", metrics)
							label.Color = closeColor
							return label.Layout(gtx)
						})
					})
				}),
			)
		})
	})
}

func layoutTerminalTabActionButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, labelText string, metrics textMetrics) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := max(gtx.Dp(unit.Dp(20)), gtx.Sp(unit.Sp(metrics.FontSize))+gtx.Dp(unit.Dp(8)))
		gtx.Constraints.Min = image.Pt(size, size)
		gtx.Constraints.Max = image.Pt(size, size)

		background := color.NRGBA{A: 0}
		border := color.NRGBA{A: 0}
		if btn.Hovered() {
			background = color.NRGBA{R: 0xE7, G: 0xDD, B: 0xD0, A: 0xFF}
			border = color.NRGBA{R: 0xD3, G: 0xC8, B: 0xBA, A: 0xFF}
		}

		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(4),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if background.A != 0 {
				fill(gtx, background)
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := monoLabelWithMetrics(th, labelText, metrics)
				label.Color = color.NRGBA{R: 0x5A, G: 0x63, B: 0x68, A: 0xFF}
				return label.Layout(gtx)
			})
		})
	})
}

func terminalCellWidthPx(gtx layout.Context, metrics textMetrics) int {
	base := float64(gtx.Dp(unit.Dp(terminalCellWidthDp)))
	scale := float64(metrics.FontSize / defaultTerminalFontSize)
	if scale <= 0 {
		scale = 1
	}
	return max(1, int(math.Round(base*scale)))
}
