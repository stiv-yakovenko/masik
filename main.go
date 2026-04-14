package main

import (
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"mycharm/coloreditor"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Gio Multiline Editor"),
			app.Size(unit.Dp(900), unit.Dp(640)),
			app.MinSize(unit.Dp(520), unit.Dp(380)),
			app.Decorated(false),
		)
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()

	app.Main()
}

func run(w *app.Window) error {
	var ops op.Ops
	th := material.NewTheme()

	var editor coloreditor.Editor
	var fileMenu widget.Clickable
	var editMenu widget.Clickable
	var viewMenu widget.Clickable
	var openFolderItem widget.Clickable
	var preferencesItem widget.Clickable
	var minBtn widget.Clickable
	var maxBtn widget.Clickable
	var closeBtn widget.Clickable
	var maximized bool
	var fileMenuOpen bool
	var browsingFolder bool
	appRoot, err := os.Getwd()
	if err != nil {
		appRoot = "."
	}
	state, err := newIDEState(appRoot, &editor)
	if err != nil {
		log.Printf("load IDE config failed: %v", err)
		state = newDefaultIDEState(appRoot, &editor)
	}

	w.Option(app.Title("Gio Multiline Editor - " + state.currentDir))
	folderPickResults := make(chan folderPickResult, 1)
	terminalEvents := make(chan terminalProcessEvent, 8)

	editor.SingleLine = false
	editor.WrapPolicy = text.WrapWords
	editor.LineColor = visualLineColor

	for {
		select {
		case result := <-folderPickResults:
			browsingFolder = false
			if result.err != nil {
				log.Printf("choose folder failed: %v", result.err)
				state.setStatus(fmt.Sprintf("Failed to open folder chooser: %v", result.err), true)
			} else if result.dir != "" {
				if err := state.setCurrentDir(result.dir, &editor); err == nil {
					w.Option(app.Title("Gio Multiline Editor - " + state.currentDir))
				} else {
					log.Printf("change dir failed: %v", err)
				}
			}
		case event := <-terminalEvents:
			if _, repaint := state.workspace.terminal.applyProcessEvent(event); repaint {
				w.Invalidate()
			}
		default:
		}

		switch e := w.Event().(type) {
		case app.ConfigEvent:
			maximized = e.Config.Mode == app.Maximized

		case app.DestroyEvent:
			return e.Err

		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			layoutUI(gtx, th, w, state, &editor, maximized, &fileMenuOpen, &browsingFolder, folderPickResults, terminalEvents, &fileMenu, &editMenu, &viewMenu, &openFolderItem, &preferencesItem, &minBtn, &maxBtn, &closeBtn)
			e.Frame(gtx.Ops)
		}
	}
}

func layoutUI(gtx layout.Context, th *material.Theme, w *app.Window, state *ideState, editor *coloreditor.Editor, maximized bool, fileMenuOpen *bool, browsingFolder *bool, folderPickResults chan<- folderPickResult, terminalEvents chan<- terminalProcessEvent, fileMenu, editMenu, viewMenu, openFolderItem, preferencesItem, minBtn, maxBtn, closeBtn *widget.Clickable) layout.Dimensions {
	state.handlePreferencesEvents(gtx, editor)
	state.handleCentralContentEvents(gtx, editor)
	state.handleWorkspaceEvents(gtx, w, terminalEvents)
	if !state.preferences.open {
		state.handleExplorerClicks(gtx, editor)
	}

	if !state.preferences.open && fileMenu.Clicked(gtx) {
		*fileMenuOpen = !*fileMenuOpen
	}
	if !state.preferences.open && (editMenu.Clicked(gtx) || viewMenu.Clicked(gtx)) {
		*fileMenuOpen = false
	}
	if !state.preferences.open && openFolderItem.Clicked(gtx) {
		*fileMenuOpen = false
		if !*browsingFolder {
			*browsingFolder = true
			startDir := state.currentDir
			go func() {
				dir, err := chooseFolder(startDir)
				folderPickResults <- folderPickResult{dir: dir, err: err}
				w.Invalidate()
			}()
		}
	}
	if !state.preferences.open && preferencesItem.Clicked(gtx) {
		*fileMenuOpen = false
		state.openPreferences()
	}
	if minBtn.Clicked(gtx) {
		*fileMenuOpen = false
		w.Perform(system.ActionMinimize)
	}
	if maxBtn.Clicked(gtx) {
		*fileMenuOpen = false
		if maximized {
			w.Perform(system.ActionUnmaximize)
		} else {
			w.Perform(system.ActionMaximize)
		}
	}
	if closeBtn.Clicked(gtx) {
		*fileMenuOpen = false
		w.Perform(system.ActionClose)
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutTitleMenuBar(gtx, th, maximized, state.currentDir, fileMenu, editMenu, viewMenu, minBtn, maxBtn, closeBtn)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layoutWorkspace(gtx, th, state, func(gtx layout.Context) layout.Dimensions {
						return layoutCentralContent(gtx, th, editor, state)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutStatusBar(gtx, th, state)
				}),
			)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !*fileMenuOpen {
				return layout.Dimensions{}
			}
			return layout.Inset{
				Top:  unit.Dp(42),
				Left: unit.Dp(44),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutFileMenuPopup(gtx, th, openFolderItem, preferencesItem)
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layoutPreferencesOverlay(gtx, th, state)
		}),
	)
}

func layoutTitleMenuBar(gtx layout.Context, th *material.Theme, maximized bool, currentFolder string, fileMenu, editMenu, viewMenu, minBtn, maxBtn, closeBtn *widget.Clickable) layout.Dimensions {
	barHeight := gtx.Dp(unit.Dp(46))
	gtx.Constraints.Min.Y = barHeight
	gtx.Constraints.Max.Y = barHeight

	fill(gtx, color.NRGBA{R: 0xF5, G: 0xF1, B: 0xE8, A: 0xFF})
	fillRect(gtx, image.Rect(0, barHeight-1, gtx.Constraints.Max.X, barHeight), color.NRGBA{R: 0xD9, G: 0xD2, B: 0xC7, A: 0xFF})

	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutMoveArea(gtx, func(gtx layout.Context) layout.Dimensions {
					return layoutAppLogo(gtx)
				})
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutMenuButton(gtx, th, fileMenu, "File")
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutMenuButton(gtx, th, editMenu, "Edit")
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutMenuButton(gtx, th, viewMenu, "View")
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutFolderBadge(gtx, th, currentFolder)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layoutMoveArea(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: gtx.Constraints.Min}
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutWindowButton(gtx, minBtn, windowButtonMinimize, color.NRGBA{R: 0xE5, G: 0xDD, B: 0xD0, A: 0xFF}, color.NRGBA{R: 0x46, G: 0x4E, B: 0x54, A: 0xFF})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				kind := windowButtonMaximize
				if maximized {
					kind = windowButtonRestore
				}
				return layoutWindowButton(gtx, maxBtn, kind, color.NRGBA{R: 0xE5, G: 0xDD, B: 0xD0, A: 0xFF}, color.NRGBA{R: 0x46, G: 0x4E, B: 0x54, A: 0xFF})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutWindowButton(gtx, closeBtn, windowButtonClose, color.NRGBA{R: 0xD9, G: 0x93, B: 0x7B, A: 0xFF}, color.NRGBA{R: 0x6D, G: 0x24, B: 0x24, A: 0xFF})
			}),
		)
	})
}

func layoutFileMenuPopup(gtx layout.Context, th *material.Theme, openFolderItem, preferencesItem *widget.Clickable) layout.Dimensions {
	width := gtx.Dp(unit.Dp(220))
	height := gtx.Dp(unit.Dp(106))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height

	return widget.Border{
		Color:        color.NRGBA{R: 0xD9, G: 0xD2, B: 0xC7, A: 0xFF},
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fillRect(gtx, image.Rect(0, 0, width, height), color.NRGBA{R: 0xF5, G: 0xF1, B: 0xE8, A: 0xFF})
		return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutFileMenuItem(gtx, th, openFolderItem, "Open folder...")
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutFileMenuItem(gtx, th, preferencesItem, "Preferences...")
				}),
			)
		})
	})
}

func layoutFileMenuItem(gtx layout.Context, th *material.Theme, item *widget.Clickable, labelText string) layout.Dimensions {
	return item.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		background := color.NRGBA{A: 0}
		if item.Hovered() {
			background = color.NRGBA{R: 0xEB, G: 0xE5, B: 0xD8, A: 0xFF}
		}

		return widget.Border{
			Color:        color.NRGBA{R: 0xE2, G: 0xDC, B: 0xD1, A: 0x00},
			CornerRadius: unit.Dp(6),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if background.A != 0 {
				fillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), background)
			}
			return layout.Inset{
				Top:    unit.Dp(9),
				Bottom: unit.Dp(9),
				Left:   unit.Dp(12),
				Right:  unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body1(th, labelText)
				label.Color = color.NRGBA{R: 0x37, G: 0x40, B: 0x46, A: 0xFF}
				return label.Layout(gtx)
			})
		})
	})
}

func layoutFolderBadge(gtx layout.Context, th *material.Theme, folder string) layout.Dimensions {
	return widget.Border{
		Color:        color.NRGBA{R: 0xDB, G: 0xD3, B: 0xC7, A: 0xFF},
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF})
		return layout.Inset{
			Top:    unit.Dp(5),
			Bottom: unit.Dp(5),
			Left:   unit.Dp(12),
			Right:  unit.Dp(12),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := material.Body2(th, folder)
			label.Color = color.NRGBA{R: 0x4A, G: 0x53, B: 0x58, A: 0xFF}
			return label.Layout(gtx)
		})
	})
}

func layoutMoveArea(gtx layout.Context, w layout.Widget) layout.Dimensions {
	dims := w(gtx)
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
	return dims
}

func layoutMenuButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, label string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		height := gtx.Dp(unit.Dp(30))
		gtx.Constraints.Min.Y = height
		gtx.Constraints.Max.Y = height

		border := color.NRGBA{R: 0xD9, G: 0xD2, B: 0xC7, A: 0x00}
		background := color.NRGBA{R: 0xF5, G: 0xF1, B: 0xE8, A: 0x00}
		if btn.Hovered() {
			border = color.NRGBA{R: 0xD7, G: 0xCF, B: 0xC2, A: 0xFF}
			background = color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF1, A: 0xFF}
		}

		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(8),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if background.A != 0 {
				fill(gtx, background)
			}
			return layout.Inset{
				Top:    unit.Dp(5),
				Bottom: unit.Dp(5),
				Left:   unit.Dp(12),
				Right:  unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, label)
				lbl.Color = color.NRGBA{R: 0x39, G: 0x42, B: 0x48, A: 0xFF}
				return lbl.Layout(gtx)
			})
		})
	})
}

type windowButtonKind uint8

const (
	windowButtonMinimize windowButtonKind = iota
	windowButtonMaximize
	windowButtonRestore
	windowButtonClose
)

func layoutWindowButton(gtx layout.Context, btn *widget.Clickable, kind windowButtonKind, bg color.NRGBA, fg color.NRGBA) layout.Dimensions {
	buttonSize := layout.Exact(image.Pt(gtx.Dp(unit.Dp(44)), gtx.Dp(unit.Dp(30))))
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = buttonSize
		background := bg
		border := shiftColor(bg, -14)
		if btn.Hovered() {
			background = shiftColor(bg, 14)
			border = shiftColor(bg, -24)
		}
		if kind == windowButtonClose {
			if btn.Hovered() {
				background = shiftColor(bg, 10)
				border = shiftColor(bg, -28)
			}
		}
		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(5),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, background)
			drawWindowIcon(gtx, kind, fg)
			return layout.Dimensions{Size: gtx.Constraints.Min}
		})
	})
}

func layoutEditor(gtx layout.Context, th *material.Theme, editor *coloreditor.Editor, metrics textMetrics) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		field := coloreditor.NewStyle(th, editor, "Type here...")
		field.TextSize = unit.Sp(metrics.FontSize)
		field.LineHeightScale = 1
		return field.Layout(gtx)
	})
}

func visualLineColor(index int) color.NRGBA {
	h := fnv.New32a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d", index)))
	sum := h.Sum32()

	return color.NRGBA{
		R: 0x60 + uint8(sum&0x5F),
		G: 0x60 + uint8((sum>>8)&0x5F),
		B: 0x60 + uint8((sum>>16)&0x5F),
		A: 0xFF,
	}
}

func layoutWithSideBorders(gtx layout.Context, bgColor, borderColor color.NRGBA, w layout.Widget) layout.Dimensions {
	maxPt := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, bgColor, clip.Rect{Max: maxPt}.Op())
	bw := gtx.Dp(unit.Dp(1))
	fillRect(gtx, image.Rect(0, 0, bw, maxPt.Y), borderColor)
	fillRect(gtx, image.Rect(maxPt.X-bw, 0, maxPt.X, maxPt.Y), borderColor)
	return layout.Inset{Left: unit.Dp(1), Right: unit.Dp(1)}.Layout(gtx, w)
}

func fill(gtx layout.Context, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Min}.Op())
}

func fillRect(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Rect(r).Op())
}

func drawWindowIcon(gtx layout.Context, kind windowButtonKind, c color.NRGBA) {
	size := gtx.Constraints.Min
	cx := size.X / 2
	cy := size.Y / 2
	switch kind {
	case windowButtonMinimize:
		w := 12
		h := 2
		x := (size.X - w) / 2
		y := cy + 6
		fillRect(gtx, image.Rect(x, y, x+w, y+h), c)
	case windowButtonMaximize:
		drawRectOutline(gtx, image.Rect(cx-5, cy-5, cx+5, cy+5), 2, c)
	case windowButtonRestore:
		back := image.Rect(cx-3, cy-7, cx+7, cy+3)
		front := image.Rect(cx-7, cy-3, cx+3, cy+7)
		drawRectOutline(gtx, back, 2, c)
		fillRect(gtx, image.Rect(front.Min.X+1, front.Min.Y, front.Max.X+1, front.Min.Y+2), color.NRGBA{R: 0xF5, G: 0xF1, B: 0xE8, A: 0xFF})
		drawRectOutline(gtx, front, 2, c)
	case windowButtonClose:
		drawCloseIcon(gtx, image.Pt(cx, cy), 5, c)
	}
}

func drawRectOutline(gtx layout.Context, r image.Rectangle, stroke int, c color.NRGBA) {
	fillRect(gtx, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+stroke), c)
	fillRect(gtx, image.Rect(r.Min.X, r.Max.Y-stroke, r.Max.X, r.Max.Y), c)
	fillRect(gtx, image.Rect(r.Min.X, r.Min.Y, r.Min.X+stroke, r.Max.Y), c)
	fillRect(gtx, image.Rect(r.Max.X-stroke, r.Min.Y, r.Max.X, r.Max.Y), c)
}

func drawLine(gtx layout.Context, from, to f32.Point, width float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(from)
	p.LineTo(to.Sub(from))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

func drawCloseIcon(gtx layout.Context, center image.Point, radius int, c color.NRGBA) {
	for i := -radius; i <= radius; i++ {
		fillRect(gtx, image.Rect(center.X+i-1, center.Y+i, center.X+i+1, center.Y+i+2), c)
		fillRect(gtx, image.Rect(center.X+i-1, center.Y-i, center.X+i+1, center.Y-i+2), c)
	}
}

func shiftColor(c color.NRGBA, delta int) color.NRGBA {
	return color.NRGBA{
		R: clampColor(int(c.R) + delta),
		G: clampColor(int(c.G) + delta),
		B: clampColor(int(c.B) + delta),
		A: c.A,
	}
}

func clampColor(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
