package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"mycharm/coloreditor"
)

type openDocumentState struct {
	Path        string
	Content     string
	TabButton   widget.Clickable
	CloseButton widget.Clickable
}

func (s *ideState) handleCentralContentEvents(gtx layout.Context, editor *coloreditor.Editor) {
	for i := range s.openDocuments {
		if s.openDocuments[i].CloseButton.Clicked(gtx) {
			s.closeDocument(editor, i)
			return
		}
	}
	for i := range s.openDocuments {
		if !s.openDocuments[i].TabButton.Clicked(gtx) {
			continue
		}
		if i == s.activeDocument {
			continue
		}
		s.activateDocument(editor, i)
		s.setStatus("Switched to "+s.selectedFileLabel(), false)
	}
}

func layoutCentralContent(gtx layout.Context, th *material.Theme, editor *coloreditor.Editor, state *ideState) layout.Dimensions {
	return layoutWithSideBorders(gtx,
		color.NRGBA{R: 0xFE, G: 0xFC, B: 0xF8, A: 0xFF},
		color.NRGBA{R: 0xCF, G: 0xD6, B: 0xDD, A: 0xFF},
		func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutDocumentTabsBar(gtx, th, state)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layoutEditor(gtx, th, editor, state.appearance.editorMetrics())
			}),
		)
	})
}

func layoutDocumentTabsBar(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	height := gtx.Dp(unit.Dp(40))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height
	fill(gtx, color.NRGBA{R: 0xF1, G: 0xEB, B: 0xE0, A: 0xFF})
	fillRect(gtx, image.Rect(0, height-1, gtx.Constraints.Max.X, height), color.NRGBA{R: 0xD9, G: 0xD1, B: 0xC5, A: 0xFF})

	if len(state.openDocuments) == 0 {
		return layout.Inset{
			Left:  unit.Dp(12),
			Right: unit.Dp(12),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Caption(th, "No documents open")
					label.Color = color.NRGBA{R: 0x77, G: 0x70, B: 0x66, A: 0xFF}
					return label.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{}
				}),
			)
		})
	}

	children := make([]layout.FlexChild, 0, len(state.openDocuments)*2+1)
	for i := range state.openDocuments {
		doc := &state.openDocuments[i]
		active := i == state.activeDocument
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutDocumentTab(gtx, th, active, doc, state.documentTabLabel(doc.Path))
		}))
		if i < len(state.openDocuments)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(2)}.Layout))
		}
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{}
	}))

	return layout.Inset{
		Left:  unit.Dp(8),
		Top:   unit.Dp(6),
		Right: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx, children...)
	})
}

func layoutDocumentTab(gtx layout.Context, th *material.Theme, active bool, doc *openDocumentState, labelText string) layout.Dimensions {
	height := gtx.Dp(unit.Dp(34))
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height

	background := color.NRGBA{R: 0xEE, G: 0xE6, B: 0xDA, A: 0xFF}
	border := color.NRGBA{R: 0xD6, G: 0xCC, B: 0xBF, A: 0xFF}
	textColor := color.NRGBA{R: 0x4A, G: 0x52, B: 0x57, A: 0xFF}
	closeColor := color.NRGBA{R: 0x6A, G: 0x72, B: 0x77, A: 0xFF}
	if active {
		background = color.NRGBA{R: 0xFE, G: 0xFC, B: 0xF8, A: 0xFF}
		border = color.NRGBA{R: 0xC7, G: 0xD2, B: 0xDE, A: 0xFF}
		textColor = color.NRGBA{R: 0x34, G: 0x3D, B: 0x43, A: 0xFF}
		closeColor = color.NRGBA{R: 0x50, G: 0x59, B: 0x5E, A: 0xFF}
	} else if doc.TabButton.Hovered() || doc.CloseButton.Hovered() {
		background = color.NRGBA{R: 0xF7, G: 0xF0, B: 0xE4, A: 0xFF}
	}

	// Record content layout first to obtain tab width, then draw
	// background and three borders (top, left, right — no bottom border)
	// underneath by replaying the recorded ops on top.
	macro := op.Record(gtx.Ops)
	dims := layout.Inset{
		Top:    unit.Dp(3),
		Bottom: unit.Dp(3),
		Left:   unit.Dp(7),
		Right:  unit.Dp(7),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return doc.TabButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(5),
						Bottom: unit.Dp(5),
						Left:   unit.Dp(8),
						Right:  unit.Dp(10),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(th, labelText)
						label.Color = textColor
						return label.Layout(gtx)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return doc.CloseButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := gtx.Dp(unit.Dp(22))
					gtx.Constraints.Min = image.Pt(size, size)
					gtx.Constraints.Max = image.Pt(size, size)
					if doc.CloseButton.Hovered() {
						fill(gtx, color.NRGBA{R: 0xE7, G: 0xDD, B: 0xD0, A: 0xFF})
					}
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Body2(th, "x")
						label.Color = closeColor
						return label.Layout(gtx)
					})
				})
			}),
		)
	})
	call := macro.Stop()

	w := dims.Size.X
	h := dims.Size.Y
	bw := gtx.Dp(unit.Dp(1))

	// Background
	fillRect(gtx, image.Rect(0, 0, w, h), background)
	// Top border
	fillRect(gtx, image.Rect(0, 0, w, bw), border)
	// Left border
	fillRect(gtx, image.Rect(0, 0, bw, h), border)
	// Right border
	fillRect(gtx, image.Rect(w-bw, 0, w, h), border)
	// No bottom border

	call.Add(gtx.Ops)
	return dims
}

func layoutCurrentDocumentPath(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layout.Inset{
		Top:    unit.Dp(10),
		Bottom: unit.Dp(10),
		Left:   unit.Dp(12),
		Right:  unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		labelText := "No file selected"
		if state.selectedFile != "" {
			labelText = state.selectedFileLabel()
		}
		label := material.Body2(th, labelText)
		label.Color = color.NRGBA{R: 0x5C, G: 0x64, B: 0x69, A: 0xFF}
		return label.Layout(gtx)
	})
}
