package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

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
	minSidePanelWidth  = 110
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
}

type paneSplitter struct {
	drag          gesture.Drag
	dragging      bool
	startSize     int
	startPointerX float32
	originX       float32
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
		chatMessages: []string{
			"Chat panel is ready.",
			"We can wire real assistant conversations into this panel next.",
		},
	}
}

func (s *ideState) handleWorkspaceEvents(gtx layout.Context) {
	if s.workspace.explorerButton.Clicked(gtx) {
		s.workspace.explorerVisible = !s.workspace.explorerVisible
	}
	if s.workspace.chatButton.Clicked(gtx) {
		s.workspace.chatVisible = !s.workspace.chatVisible
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

func (w *workspaceState) normalize(totalWidth, splitterWidth, activityWidth int) {
	splitterCount := 0
	if w.explorerVisible {
		splitterCount++
	}
	if w.chatVisible {
		splitterCount++
	}

	available := totalWidth - activityWidth - splitterCount*splitterWidth - minEditorWidthPx
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
	if state.workspace.explorerVisible {
		state.workspace.explorerSplitter.originX = float32(activityWidth + state.workspace.explorerWidth)
	}
	if state.workspace.chatVisible {
		state.workspace.chatSplitter.originX = float32(gtx.Constraints.Max.X - state.workspace.chatWidth - splitterWidth)
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = activityWidth
			gtx.Constraints.Max.X = activityWidth
			return layoutActivityBar(gtx, th, state)
		}),
	}

	if state.workspace.explorerVisible {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = state.workspace.explorerWidth
			gtx.Constraints.Max.X = state.workspace.explorerWidth
			return layoutExplorerPanel(gtx, th, state)
		}))
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPaneSplitter(gtx, &state.workspace.explorerSplitter)
		}))
	}

	children = append(children, layout.Flexed(1, editor))

	if state.workspace.chatVisible {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPaneSplitter(gtx, &state.workspace.chatSplitter)
		}))
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = state.workspace.chatWidth
			gtx.Constraints.Max.X = state.workspace.chatWidth
			return layoutChatPanel(gtx, th, state)
		}))
	}

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func layoutActivityBar(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return widget.Border{
		Color:        color.NRGBA{R: 0xD0, G: 0xD7, B: 0xDE, A: 0xFF},
		CornerRadius: unit.Dp(10),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xF2, G: 0xEC, B: 0xE2, A: 0xFF})
		return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutActivityButton(gtx, th, &state.workspace.explorerButton, state.workspace.explorerVisible, "E")
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutActivityButton(gtx, th, &state.workspace.chatButton, state.workspace.chatVisible, "C")
				}),
			)
		})
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

func layoutChatPanel(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return widget.Border{
		Color:        color.NRGBA{R: 0xD0, G: 0xD7, B: 0xDE, A: 0xFF},
		CornerRadius: unit.Dp(10),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF})
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

	left := "Project: " + folderDisplayName(state.currentDir)
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
