package main

import (
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
	"mycharm/coloreditor"
)

type preferencesDraft struct {
	TabWidthSpaces int
}

type preferencesPageState struct {
	ID          string
	GroupTitle  string
	Title       string
	Description string
	Button      widget.Clickable
}

type intPreferenceOption struct {
	Label  string
	Value  int
	Button widget.Clickable
}

type floatPreferenceOption struct {
	Label  string
	Value  float32
	Button widget.Clickable
}

type preferencesDialogState struct {
	open               bool
	search             widget.Editor
	menuList           widget.List
	selectedPage       string
	pages              []preferencesPageState
	tabWidthOptions    []intPreferenceOption
	lineHeightOptions  []floatPreferenceOption
	okButton           widget.Clickable
	cancelButton       widget.Clickable
	draft              preferencesDraft
	position           image.Point
	positioned         bool
	titleDrag          gesture.Drag
	dragging           bool
	dragOrigin         image.Point
	dragStartPos       image.Point
}

type preferenceGroupView struct {
	Title string
	Pages []*preferencesPageState
}

func newPreferencesDialogState() preferencesDialogState {
	search := widget.Editor{SingleLine: true}
	menuList := widget.List{}
	menuList.Axis = layout.Vertical

	return preferencesDialogState{
		search:       search,
		menuList:     menuList,
		selectedPage: "editor-indentation",
		pages: []preferencesPageState{
			{ID: "editor-indentation", GroupTitle: "Editor", Title: "Indentation", Description: "Tab width and indentation behavior."},
			{ID: "editor-appearance", GroupTitle: "Editor", Title: "Appearance", Description: "Line spacing and editor drawing options."},
			{ID: "files-general", GroupTitle: "Files", Title: "General", Description: "Global file opening behavior and future defaults."},
		},
		tabWidthOptions: []intPreferenceOption{
			{Label: "2 spaces", Value: 2},
			{Label: "4 spaces", Value: 4},
			{Label: "8 spaces", Value: 8},
		},
		lineHeightOptions: []floatPreferenceOption{
			{Label: "1.15x", Value: 1.15},
			{Label: "1.35x", Value: 1.35},
			{Label: "1.60x", Value: 1.60},
		},
	}
}

func (s *ideState) openPreferences() {
	s.preferences.open = true
	s.preferences.positioned = false
	s.preferences.search.SetText("")
	s.preferences.draft = preferencesDraft{
		TabWidthSpaces: s.tabWidthSpaces,
	}
}

func (s *ideState) handlePreferencesEvents(gtx layout.Context, editor *coloreditor.Editor) {
	if !s.preferences.open {
		return
	}

	s.preferences.handleDrag(gtx, preferencesDialogSize(gtx))

	for i := range s.preferences.pages {
		if s.preferences.pages[i].Button.Clicked(gtx) {
			s.preferences.selectedPage = s.preferences.pages[i].ID
		}
	}
	for i := range s.preferences.tabWidthOptions {
		if s.preferences.tabWidthOptions[i].Button.Clicked(gtx) {
			s.preferences.draft.TabWidthSpaces = s.preferences.tabWidthOptions[i].Value
		}
	}
	if s.preferences.cancelButton.Clicked(gtx) {
		s.preferences.open = false
		return
	}
	if s.preferences.okButton.Clicked(gtx) {
		s.tabWidthSpaces = s.preferences.draft.TabWidthSpaces
		s.applyEditorPreferences(editor)
		if err := s.saveUserSettings(); err != nil {
			s.setStatus("Preferences updated, but user config save failed: "+err.Error(), true)
		} else {
			s.setStatus("Preferences saved.", false)
		}
		s.preferences.open = false
	}
}

func (p *preferencesDialogState) handleDrag(gtx layout.Context, size image.Point) {
	for {
		ev, ok := p.titleDrag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			p.dragging = true
			p.dragOrigin = p.position
			p.dragStartPos = image.Pt(int(math.Round(float64(ev.Position.X))), int(math.Round(float64(ev.Position.Y))))
		case pointer.Drag:
			if p.dragging {
				deltaX := int(math.Round(float64(ev.Position.X))) - p.dragStartPos.X
				deltaY := int(math.Round(float64(ev.Position.Y))) - p.dragStartPos.Y
				p.position.X = p.dragOrigin.X + deltaX
				p.position.Y = p.dragOrigin.Y + deltaY
			}
		case pointer.Release, pointer.Cancel:
			p.dragging = false
		}
	}
	p.clampPosition(gtx.Constraints.Max, size)
}

func (p *preferencesDialogState) filteredGroups() []preferenceGroupView {
	query := strings.TrimSpace(strings.ToLower(p.search.Text()))
	groups := make([]preferenceGroupView, 0, len(p.pages))

	for i := range p.pages {
		page := &p.pages[i]
		if query != "" {
			haystack := strings.ToLower(page.GroupTitle + " " + page.Title + " " + page.Description)
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		if len(groups) == 0 || groups[len(groups)-1].Title != page.GroupTitle {
			groups = append(groups, preferenceGroupView{Title: page.GroupTitle})
		}
		last := &groups[len(groups)-1]
		last.Pages = append(last.Pages, page)
	}

	return groups
}

func layoutPreferencesOverlay(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	if !state.preferences.open {
		return layout.Dimensions{}
	}

	fillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), color.NRGBA{A: 70})

	dialogSize := preferencesDialogSize(gtx)
	if !state.preferences.positioned {
		state.preferences.position = image.Pt(
			(gtx.Constraints.Max.X-dialogSize.X)/2,
			(gtx.Constraints.Max.Y-dialogSize.Y)/2,
		)
		state.preferences.positioned = true
	}
	state.preferences.clampPosition(gtx.Constraints.Max, dialogSize)

	return layout.Inset{
		Left: unit.Dp(float32(state.preferences.position.X) / gtx.Metric.PxPerDp),
		Top:  unit.Dp(float32(state.preferences.position.Y) / gtx.Metric.PxPerDp),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(dialogSize)
		return widget.Border{
			Color:        color.NRGBA{R: 0xC8, G: 0xC0, B: 0xB2, A: 0xFF},
			CornerRadius: unit.Dp(10),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF})
			return layout.UniformInset(unit.Dp(18)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutPreferencesTitleBar(gtx, th, state)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutPreferencesMenu(gtx, th, state)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layoutPreferencesContent(gtx, th, state)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layoutPreferencesButtons(gtx, th, state)
					}),
				)
			})
		})
	})
}

func preferencesDialogSize(gtx layout.Context) image.Point {
	dialogWidth := gtx.Dp(unit.Dp(860))
	dialogHeight := gtx.Dp(unit.Dp(580))
	if dialogWidth > gtx.Constraints.Max.X-32 {
		dialogWidth = gtx.Constraints.Max.X - 32
	}
	if dialogHeight > gtx.Constraints.Max.Y-32 {
		dialogHeight = gtx.Constraints.Max.Y - 32
	}
	if dialogWidth < 420 {
		dialogWidth = gtx.Constraints.Max.X
	}
	if dialogHeight < 320 {
		dialogHeight = gtx.Constraints.Max.Y
	}
	return image.Pt(dialogWidth, dialogHeight)
}

func (p *preferencesDialogState) clampPosition(viewport, size image.Point) {
	maxX := viewport.X - size.X
	maxY := viewport.Y - size.Y
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if p.position.X < 0 {
		p.position.X = 0
	}
	if p.position.Y < 0 {
		p.position.Y = 0
	}
	if p.position.X > maxX {
		p.position.X = maxX
	}
	if p.position.Y > maxY {
		p.position.Y = maxY
	}
}

func layoutPreferencesTitleBar(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	pointer.CursorGrab.Add(gtx.Ops)
	state.preferences.titleDrag.Add(gtx.Ops)
	return widget.Border{
		Color:        color.NRGBA{R: 0xE0, G: 0xD8, B: 0xCB, A: 0xFF},
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xF6, G: 0xF0, B: 0xE6, A: 0xFF})
		return layout.Inset{
			Top:    unit.Dp(10),
			Bottom: unit.Dp(10),
			Left:   unit.Dp(12),
			Right:  unit.Dp(12),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			title := material.H6(th, "Preferences")
			title.Color = color.NRGBA{R: 0x38, G: 0x40, B: 0x46, A: 0xFF}
			return title.Layout(gtx)
		})
	})
}

func layoutPreferencesMenu(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	width := gtx.Dp(unit.Dp(250))
	gtx.Constraints.Min.X = width
	gtx.Constraints.Max.X = width

	return widget.Border{
		Color:        color.NRGBA{R: 0xD7, G: 0xCF, B: 0xC2, A: 0xFF},
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xF7, G: 0xF2, B: 0xE8, A: 0xFF})
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					search := material.Editor(th, &state.preferences.search, "Search settings")
					search.Color = color.NRGBA{R: 0x34, G: 0x3C, B: 0x42, A: 0xFF}
					search.HintColor = color.NRGBA{R: 0x84, G: 0x8B, B: 0x90, A: 0xFF}
					return search.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					groups := state.preferences.filteredGroups()
					if len(groups) == 0 {
						label := material.Body2(th, "No matching settings.")
						label.Color = color.NRGBA{R: 0x6B, G: 0x73, B: 0x78, A: 0xFF}
						return label.Layout(gtx)
					}

					rowCount := 0
					for _, group := range groups {
						rowCount += 1 + len(group.Pages)
					}

					rows := make([]interface{}, 0, rowCount)
					for _, group := range groups {
						rows = append(rows, group.Title)
						for _, page := range group.Pages {
							rows = append(rows, page)
						}
					}

					list := material.List(th, &state.preferences.menuList)
					return list.Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
						switch item := rows[index].(type) {
						case string:
							label := material.Caption(th, item)
							label.Color = color.NRGBA{R: 0x7D, G: 0x76, B: 0x69, A: 0xFF}
							return layout.Inset{
								Top:    unit.Dp(8),
								Bottom: unit.Dp(4),
								Left:   unit.Dp(2),
							}.Layout(gtx, label.Layout)
						case *preferencesPageState:
							return layoutPreferencesMenuItem(gtx, th, state.preferences.selectedPage == item.ID, &item.Button, item.Title)
						default:
							return layout.Dimensions{}
						}
					})
				}),
			)
		})
	})
}

func layoutPreferencesMenuItem(gtx layout.Context, th *material.Theme, selected bool, btn *widget.Clickable, labelText string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		height := gtx.Dp(unit.Dp(32))
		gtx.Constraints.Min.Y = height
		gtx.Constraints.Max.Y = height

		background := color.NRGBA{A: 0}
		border := color.NRGBA{A: 0}
		if selected {
			background = color.NRGBA{R: 0xE4, G: 0xEC, B: 0xF3, A: 0xFF}
			border = color.NRGBA{R: 0xBE, G: 0xCC, B: 0xD8, A: 0xFF}
		} else if btn.Hovered() {
			background = color.NRGBA{R: 0xEF, G: 0xE8, B: 0xDB, A: 0xFF}
			border = color.NRGBA{R: 0xE0, G: 0xD7, B: 0xCA, A: 0xFF}
		}

		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(6),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if background.A != 0 {
				fillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), background)
			}
			return layout.Inset{
				Top:    unit.Dp(6),
				Bottom: unit.Dp(6),
				Left:   unit.Dp(14),
				Right:  unit.Dp(10),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(th, labelText)
				label.Color = color.NRGBA{R: 0x3E, G: 0x47, B: 0x4C, A: 0xFF}
				return label.Layout(gtx)
			})
		})
	})
}

func layoutPreferencesContent(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return widget.Border{
		Color:        color.NRGBA{R: 0xD7, G: 0xCF, B: 0xC2, A: 0xFF},
		CornerRadius: unit.Dp(8),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xFF, G: 0xFC, B: 0xF7, A: 0xFF})
		return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			switch state.preferences.selectedPage {
			case "editor-indentation":
				return layoutPreferencesIndentationPage(gtx, th, state)
			case "editor-appearance":
				return layoutPreferencesAppearancePage(gtx, th, state)
			case "files-general":
				return layoutPreferencesFilesPage(gtx, th, state)
			default:
				return material.Body1(th, "Choose a preferences section on the left.").Layout(gtx)
			}
		})
	})
}

func layoutPreferencesIndentationPage(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, "Editor / Indentation").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(material.Body2(th, "Tabs are shown as spaces in the editor. Choose how many spaces one tab should use.").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutPreferenceOptionRow(gtx, th, state.preferences.draft.TabWidthSpaces, state.preferences.tabWidthOptions)
		}),
	)
}

func layoutPreferencesAppearancePage(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, "Editor / Appearance").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(material.Body2(th, "Global editor appearance settings for every project on this machine.").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(18)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			pathLabel := material.Caption(th, state.userConfigPath)
			pathLabel.Color = color.NRGBA{R: 0x7A, G: 0x73, B: 0x68, A: 0xFF}
			return pathLabel.Layout(gtx)
		}),
	)
}

func layoutPreferencesFilesPage(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.H6(th, "Files / General").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(material.Body2(th, "This page is ready for future file-system preferences.").Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(material.Body2(th, "Project-specific state stays in the local .masik folder, while global editor settings stay in the user .masic folder.").Layout),
	)
}

func layoutPreferenceOptionRow(gtx layout.Context, th *material.Theme, selected int, options []intPreferenceOption) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(options)*2)
	for i := range options {
		option := &options[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutIntPreferenceOption(gtx, th, selected == option.Value, &option.Button, option.Label)
		}))
		if i < len(options)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func layoutFloatPreferenceOptionRow(gtx layout.Context, th *material.Theme, selected float32, options []floatPreferenceOption) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(options)*2)
	for i := range options {
		option := &options[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutIntPreferenceOption(gtx, th, selected == option.Value, &option.Button, option.Label)
		}))
		if i < len(options)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func layoutIntPreferenceOption(gtx layout.Context, th *material.Theme, selected bool, btn *widget.Clickable, labelText string) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		background := color.NRGBA{R: 0xF2, G: 0xEB, B: 0xDF, A: 0xFF}
		border := color.NRGBA{R: 0xD8, G: 0xD0, B: 0xC3, A: 0xFF}
		if selected {
			background = color.NRGBA{R: 0xDF, G: 0xE9, B: 0xF4, A: 0xFF}
			border = color.NRGBA{R: 0xB8, G: 0xC8, B: 0xD9, A: 0xFF}
		} else if btn.Hovered() {
			background = color.NRGBA{R: 0xF6, G: 0xF0, B: 0xE5, A: 0xFF}
		}

		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(7),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, background)
			return layout.Inset{
				Top:    unit.Dp(9),
				Bottom: unit.Dp(9),
				Left:   unit.Dp(14),
				Right:  unit.Dp(14),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(th, labelText)
				label.Color = color.NRGBA{R: 0x3D, G: 0x45, B: 0x4B, A: 0xFF}
				return label.Layout(gtx)
			})
		})
	})
}

func layoutPreferencesButtons(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutDialogButton(gtx, th, &state.preferences.cancelButton, "Cancel", false)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutDialogButton(gtx, th, &state.preferences.okButton, "OK", true)
		}),
	)
}

func layoutDialogButton(gtx layout.Context, th *material.Theme, btn *widget.Clickable, labelText string, primary bool) layout.Dimensions {
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		background := color.NRGBA{R: 0xF3, G: 0xEC, B: 0xE0, A: 0xFF}
		border := color.NRGBA{R: 0xD8, G: 0xCF, B: 0xC2, A: 0xFF}
		textColor := color.NRGBA{R: 0x3C, G: 0x44, B: 0x49, A: 0xFF}
		if primary {
			background = color.NRGBA{R: 0xD9, G: 0x93, B: 0x7B, A: 0xFF}
			border = color.NRGBA{R: 0xB8, G: 0x73, B: 0x5E, A: 0xFF}
			textColor = color.NRGBA{R: 0x33, G: 0x1C, B: 0x18, A: 0xFF}
		}
		if btn.Hovered() {
			background = shiftColor(background, 8)
		}

		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(7),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill(gtx, background)
			return layout.Inset{
				Top:    unit.Dp(9),
				Bottom: unit.Dp(9),
				Left:   unit.Dp(18),
				Right:  unit.Dp(18),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(th, labelText)
				label.Color = textColor
				return label.Layout(gtx)
			})
		})
	})
}
