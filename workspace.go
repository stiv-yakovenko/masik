package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	activityBarWidthDp = 46
	splitterWidthDp    = 6
	minEditorWidthPx   = 220
	minMainHeightPx    = 180
	minSidePanelWidth  = 110
	minTerminalHeight  = 120
)

type workspaceState struct {
	explorerVisible bool
	chatVisible     bool
	explorerWidth   int
	chatWidth       int

	explorerButton widget.Clickable
	chatButton     widget.Clickable

	explorerSplitter paneSplitter
	chatSplitter     paneSplitter

	chatList     widget.List
	chatInput    widget.Editor
	chatSend     widget.Clickable
	chatMessages []string

	terminal terminalState
}

type paneSplitter struct {
	drag          gesture.Drag
	dragging      bool
	startSize     int
	startPointerX float32
	originX       float32
}

type verticalPaneSplitter struct {
	drag          gesture.Drag
	dragging      bool
	startSize     int
	startPointerY float32
	originY       float32
}

func newWorkspaceState() workspaceState {
	chatList := widget.List{}
	chatList.Axis = layout.Vertical
	chatInput := widget.Editor{SingleLine: true}

	return workspaceState{
		explorerVisible: true,
		chatVisible:     true,
		explorerWidth:   280,
		chatWidth:       320,
		chatList:        chatList,
		chatInput:       chatInput,
		terminal:        newTerminalState(),
		chatMessages: []string{
			"Chat panel is ready.",
			"We can wire real assistant conversations into this panel next.",
		},
	}
}

func (s *ideState) handleWorkspaceEvents(gtx layout.Context, w *app.Window, terminalEvents chan<- terminalProcessEvent) {
	if s.workspace.explorerButton.Clicked(gtx) {
		s.workspace.explorerVisible = !s.workspace.explorerVisible
	}
	if s.workspace.chatButton.Clicked(gtx) {
		s.workspace.chatVisible = !s.workspace.chatVisible
	}
	if s.workspace.terminal.toggleButton.Clicked(gtx) {
		if s.workspace.terminal.visible {
			s.workspace.terminal.collapse()
			s.setStatus("Terminal hidden.", false)
		} else if err := s.workspace.terminal.show(s.currentDir, w, terminalEvents); err != nil {
			s.setStatus("Failed to open terminal: "+err.Error(), true)
		} else {
			s.setStatus("Terminal opened.", false)
		}
	}
	if s.workspace.terminal.visible && s.workspace.terminal.minimizeButton.Clicked(gtx) {
		s.workspace.terminal.collapse()
		s.setStatus("Terminal minimized.", false)
	}
	if s.workspace.terminal.visible && s.workspace.terminal.addButton.Clicked(gtx) {
		label, err := s.workspace.terminal.openTab(s.currentDir, w, terminalEvents)
		if err != nil {
			s.setStatus("Failed to open terminal tab: "+err.Error(), true)
		} else {
			s.setStatus(label+" opened.", false)
		}
	}
	if s.workspace.terminal.visible {
		for i := range s.workspace.terminal.tabs {
			if s.workspace.terminal.tabs[i].CloseButton.Clicked(gtx) {
				closedTitle := s.workspace.terminal.tabs[i].Title
				s.workspace.terminal.closeTab(i)
				if len(s.workspace.terminal.tabs) == 0 {
					s.setStatus("Terminal closed.", false)
				} else {
					s.setStatus(closedTitle+" closed.", false)
				}
				s.workspace.handleSplitters(gtx)
				return
			}
		}
		for i := range s.workspace.terminal.tabs {
			if !s.workspace.terminal.tabs[i].TabButton.Clicked(gtx) || i == s.workspace.terminal.activeTab {
				continue
			}
			s.workspace.terminal.activeTab = i
			s.workspace.terminal.tabs[i].FocusNext = true
			s.setStatus("Switched to "+s.workspace.terminal.tabs[i].Title, false)
		}
	}
	if s.workspace.chatSend.Clicked(gtx) {
		msg := strings.TrimSpace(s.workspace.chatInput.Text())
		if msg != "" {
			s.workspace.chatMessages = append(s.workspace.chatMessages, "You: "+msg)
			s.workspace.chatInput.SetText("")
		}
	}
	s.workspace.handleSplitters(gtx)
}

func (w *workspaceState) handleSplitters(gtx layout.Context) {
	w.explorerSplitter.update(gtx, &w.explorerWidth, 1)
	w.chatSplitter.update(gtx, &w.chatWidth, -1)
	w.terminal.splitter.update(gtx, &w.terminal.height)
}

func (d *paneSplitter) update(gtx layout.Context, size *int, direction int) {
	for {
		ev, ok := d.drag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			d.dragging = true
			d.startSize = *size
			d.startPointerX = d.originX + ev.Position.X
		case pointer.Drag:
			if d.dragging {
				pointerX := d.originX + ev.Position.X
				delta := int(math.Round(float64(pointerX - d.startPointerX)))
				*size = d.startSize + direction*delta
			}
		case pointer.Release, pointer.Cancel:
			d.dragging = false
		}
	}
}

func (d *verticalPaneSplitter) update(gtx layout.Context, size *int) {
	for {
		ev, ok := d.drag.Update(gtx.Metric, gtx.Source, gesture.Vertical)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			d.dragging = true
			d.startSize = *size
			d.startPointerY = d.originY + ev.Position.Y
		case pointer.Drag:
			if d.dragging {
				pointerY := d.originY + ev.Position.Y
				delta := int(math.Round(float64(pointerY - d.startPointerY)))
				*size = d.startSize - delta
			}
		case pointer.Release, pointer.Cancel:
			d.dragging = false
		}
	}
}

func (w *workspaceState) normalize(totalWidth, splitterWidth, activityWidth int) {
	splitterCount := 0
	if w.explorerVisible {
		splitterCount++
	}
	if w.chatVisible {
		splitterCount++
	}

	available := totalWidth - activityWidth*2 - splitterCount*splitterWidth - minEditorWidthPx
	if available < 0 {
		available = 0
	}

	if w.explorerVisible && !w.chatVisible {
		w.explorerWidth = clampInt(w.explorerWidth, minSidePanelWidth, max(minSidePanelWidth, available))
	}
	if !w.explorerVisible && w.chatVisible {
		w.chatWidth = clampInt(w.chatWidth, minSidePanelWidth, max(minSidePanelWidth, available))
	}
	if w.explorerVisible && w.chatVisible {
		maxPerPanel := max(minSidePanelWidth, available-minSidePanelWidth)
		w.explorerWidth = clampInt(w.explorerWidth, minSidePanelWidth, maxPerPanel)
		remaining := available - w.explorerWidth
		if remaining < minSidePanelWidth {
			deficit := minSidePanelWidth - remaining
			w.explorerWidth -= deficit
			if w.explorerWidth < minSidePanelWidth {
				w.explorerWidth = minSidePanelWidth
			}
		}
		w.chatWidth = clampInt(w.chatWidth, minSidePanelWidth, max(minSidePanelWidth, available-w.explorerWidth))
		sum := w.explorerWidth + w.chatWidth
		if sum > available {
			overflow := sum - available
			if w.chatWidth > w.explorerWidth {
				w.chatWidth -= overflow
			} else {
				w.explorerWidth -= overflow
			}
		}
		if w.explorerWidth < minSidePanelWidth {
			w.explorerWidth = minSidePanelWidth
		}
		if w.chatWidth < minSidePanelWidth {
			w.chatWidth = minSidePanelWidth
		}
	}
}

func layoutWorkspace(gtx layout.Context, th *material.Theme, state *ideState, editor layout.Widget) layout.Dimensions {
	activityWidth := gtx.Dp(unit.Dp(activityBarWidthDp))
	splitterWidth := gtx.Dp(unit.Dp(splitterWidthDp))
	state.workspace.normalize(gtx.Constraints.Max.X, splitterWidth, activityWidth)
	state.workspace.terminal.normalize(gtx.Constraints.Max.Y, splitterWidth)
	if state.workspace.explorerVisible {
		state.workspace.explorerSplitter.originX = float32(activityWidth + state.workspace.explorerWidth)
	}
	if state.workspace.chatVisible {
		state.workspace.chatSplitter.originX = float32(gtx.Constraints.Max.X - activityWidth - state.workspace.chatWidth - splitterWidth)
	}
	if state.workspace.terminal.visible {
		state.workspace.terminal.splitter.originY = float32(gtx.Constraints.Max.Y - state.workspace.terminal.height - splitterWidth)
	}

	rowChildren := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = activityWidth
			gtx.Constraints.Max.X = activityWidth
			return layoutActivityBar(gtx, th, []activityBarItem{
				{Button: &state.workspace.explorerButton, Active: state.workspace.explorerVisible, Label: "E"},
				{Button: &state.workspace.terminal.toggleButton, Active: state.workspace.terminal.visible, Label: "T"},
			})
		}),
	}

	if state.workspace.explorerVisible {
		rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = state.workspace.explorerWidth
			gtx.Constraints.Max.X = state.workspace.explorerWidth
			return layoutExplorerPanel(gtx, th, state)
		}))
		rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPaneSplitter(gtx, &state.workspace.explorerSplitter)
		}))
	}

	rowChildren = append(rowChildren, layout.Flexed(1, editor))

	if state.workspace.chatVisible {
		rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPaneSplitter(gtx, &state.workspace.chatSplitter)
		}))
		rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = state.workspace.chatWidth
			gtx.Constraints.Max.X = state.workspace.chatWidth
			return layoutChatPanel(gtx, th, state)
		}))
	}

	rowChildren = append(rowChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = activityWidth
		gtx.Constraints.Max.X = activityWidth
		return layoutActivityBar(gtx, th, []activityBarItem{
			{Button: &state.workspace.chatButton, Active: state.workspace.chatVisible, Label: "C"},
		})
	}))

	mainRow := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, rowChildren...)
	}

	if !state.workspace.terminal.visible {
		return mainRow(gtx)
	}

	topHeight := gtx.Constraints.Max.Y - splitterWidth - state.workspace.terminal.height
	if topHeight < minMainHeightPx {
		topHeight = minMainHeightPx
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = topHeight
			gtx.Constraints.Max.Y = topHeight
			return mainRow(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutHorizontalSplitter(gtx, &state.workspace.terminal.splitter)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = state.workspace.terminal.height
			gtx.Constraints.Max.Y = state.workspace.terminal.height
			return layoutTerminalPanel(gtx, th, state)
		}),
	)
}

type activityBarItem struct {
	Button *widget.Clickable
	Active bool
	Label  string
}

func layoutActivityBar(gtx layout.Context, th *material.Theme, items []activityBarItem) layout.Dimensions {
	fill(gtx, color.NRGBA{R: 0xF2, G: 0xEC, B: 0xE2, A: 0xFF})
	return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(items)*2)
			for i := range items {
				item := items[i]
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutActivityButton(gtx, th, item.Button, item.Active, item.Label)
				}))
				if i < len(items)-1 {
					children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout))
				}
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{}
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func layoutActivityButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, active bool, labelText string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Dp(unit.Dp(32))
		gtx.Constraints.Min = image.Pt(size, size)
		gtx.Constraints.Max = image.Pt(size, size)
		background := color.NRGBA{R: 0xF5, G: 0xEF, B: 0xE4, A: 0xFF}
		border := color.NRGBA{R: 0xDB, G: 0xD1, B: 0xC5, A: 0xFF}
		if active {
			background = color.NRGBA{R: 0xDF, G: 0xE9, B: 0xF4, A: 0xFF}
			border = color.NRGBA{R: 0xBA, G: 0xC8, B: 0xD8, A: 0xFF}
		} else if btn.Hovered() {
			background = color.NRGBA{R: 0xFB, G: 0xF6, B: 0xEC, A: 0xFF}
		}
		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(7),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, background)
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(th, labelText)
				label.Color = color.NRGBA{R: 0x3A, G: 0x43, B: 0x48, A: 0xFF}
				return label.Layout(gtx)
			})
		})
	})
}

func layoutPaneSplitter(gtx layout.Context, splitter *paneSplitter) layout.Dimensions {
	width := gtx.Dp(unit.Dp(splitterWidthDp))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	pointer.CursorColResize.Add(gtx.Ops)
	splitter.drag.Add(gtx.Ops)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fill(gtx, color.NRGBA{R: 0xEE, G: 0xE7, B: 0xDA, A: 0xFF})
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lineW := max(2, width/3)
				gtx.Constraints.Min = image.Pt(lineW, gtx.Constraints.Max.Y-12)
				gtx.Constraints.Max = gtx.Constraints.Min
				fill(gtx, color.NRGBA{R: 0xD0, G: 0xC6, B: 0xB6, A: 0xFF})
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		}),
	)
}

func layoutHorizontalSplitter(gtx layout.Context, splitter *verticalPaneSplitter) layout.Dimensions {
	height := gtx.Dp(unit.Dp(splitterWidthDp))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	pointer.CursorRowResize.Add(gtx.Ops)
	splitter.drag.Add(gtx.Ops)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fill(gtx, color.NRGBA{R: 0xEE, G: 0xE7, B: 0xDA, A: 0xFF})
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lineH := max(2, height/3)
				gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X-12, lineH)
				gtx.Constraints.Max = gtx.Constraints.Min
				fill(gtx, color.NRGBA{R: 0xD0, G: 0xC6, B: 0xB6, A: 0xFF})
				return layout.Dimensions{Size: gtx.Constraints.Min}
			})
		}),
	)
}

func layoutChatPanel(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layoutWithSideBorders(gtx,
		color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF},
		color.NRGBA{R: 0xD0, G: 0xD7, B: 0xDE, A: 0xFF},
		func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(th, "Chat")
					label.Color = color.NRGBA{R: 0x5A, G: 0x63, B: 0x68, A: 0xFF}
					return label.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					list := material.List(th, &state.workspace.chatList)
					return list.Layout(gtx, len(state.workspace.chatMessages), func(gtx layout.Context, index int) layout.Dimensions {
						return layoutChatMessage(gtx, th, state.workspace.chatMessages[index])
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutChatComposer(gtx, th, state)
				}),
			)
		})
	})
}

func layoutChatMessage(gtx layout.Context, th *material.Theme, msg string) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{
			Color:        color.NRGBA{A: 0},
			CornerRadius: unit.Dp(7),
			Width:        unit.Dp(0),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, color.NRGBA{R: 0xF1, G: 0xEB, B: 0xE0, A: 0xFF})
			return layout.Inset{
				Top:    unit.Dp(10),
				Bottom: unit.Dp(10),
				Left:   unit.Dp(12),
				Right:  unit.Dp(12),
			}.Layout(gtx, material.Body2(th, msg).Layout)
		})
	})
}

func layoutChatComposer(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			editor := material.Editor(th, &state.workspace.chatInput, "Message")
			editor.Color = color.NRGBA{R: 0x34, G: 0x3C, B: 0x42, A: 0xFF}
			editor.HintColor = color.NRGBA{R: 0x84, G: 0x8B, B: 0x90, A: 0xFF}
			return editor.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutDialogButton(gtx, th, &state.workspace.chatSend, "Send", false)
		}),
	)
}

func layoutStatusBar(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	height := gtx.Dp(unit.Dp(30))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	fill(gtx, color.NRGBA{R: 0xF0, G: 0xEA, B: 0xDE, A: 0xFF})
	fillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, 1), color.NRGBA{R: 0xD9, G: 0xD2, B: 0xC7, A: 0xFF})

	left := "Project: " + state.currentDir
	center := "File: " + state.selectedFileLabel()
	right := fmt.Sprintf("Explorer %dpx | Chat %dpx", state.workspace.explorerWidth, state.workspace.chatWidth)
	return layout.Inset{
		Left:  unit.Dp(10),
		Right: unit.Dp(10),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, left)
				lbl.Color = color.NRGBA{R: 0x4E, G: 0x56, B: 0x5B, A: 0xFF}
				return lbl.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, center)
					lbl.Color = color.NRGBA{R: 0x4E, G: 0x56, B: 0x5B, A: 0xFF}
					return lbl.Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, right)
				lbl.Color = color.NRGBA{R: 0x4E, G: 0x56, B: 0x5B, A: 0xFF}
				return lbl.Layout(gtx)
			}),
		)
	})
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
