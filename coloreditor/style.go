package coloreditor

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type Style struct {
	Font font.Font
	// LineHeight controls the distance between the baselines of lines of text.
	LineHeight unit.Sp
	// LineHeightScale applies a scaling factor to the LineHeight.
	LineHeightScale float32
	TextSize        unit.Sp
	Color           color.NRGBA
	Hint            string
	HintColor       color.NRGBA
	SelectionColor  color.NRGBA
	Editor          *Editor

	shaper *text.Shaper
}

func NewStyle(th *material.Theme, editor *Editor, hint string) Style {
	return Style{
		Editor: editor,
		Font: font.Font{
			Typeface: th.Face,
		},
		TextSize:       th.TextSize,
		Color:          th.Palette.Fg,
		shaper:         th.Shaper,
		Hint:           hint,
		HintColor:      mulAlpha(th.Palette.Fg, 0xbb),
		SelectionColor: mulAlpha(th.Palette.ContrastBg, 0x60),
	}
}

func (e Style) Layout(gtx layout.Context) layout.Dimensions {
	textColorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: e.Color}.Add(gtx.Ops)
	textColor := textColorMacro.Stop()
	hintColorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: e.HintColor}.Add(gtx.Ops)
	hintColor := hintColorMacro.Stop()
	selectionColorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: blendDisabledColor(!gtx.Enabled(), e.SelectionColor)}.Add(gtx.Ops)
	selectionColor := selectionColorMacro.Stop()

	var maxlines int
	if e.Editor.SingleLine {
		maxlines = 1
	}

	macro := op.Record(gtx.Ops)
	tl := widget.Label{
		Alignment:       e.Editor.Alignment,
		MaxLines:        maxlines,
		LineHeight:      e.LineHeight,
		LineHeightScale: e.LineHeightScale,
	}
	dims := tl.Layout(gtx, e.shaper, e.Font, e.TextSize, e.Hint, hintColor)
	call := macro.Stop()

	if w := dims.Size.X; gtx.Constraints.Min.X < w {
		gtx.Constraints.Min.X = w
	}
	if h := dims.Size.Y; gtx.Constraints.Min.Y < h {
		gtx.Constraints.Min.Y = h
	}
	e.Editor.LineHeight = e.LineHeight
	e.Editor.LineHeightScale = e.LineHeightScale
	dims = e.Editor.Layout(gtx, e.shaper, e.Font, e.TextSize, textColor, selectionColor)
	if e.Editor.Len() == 0 {
		call.Add(gtx.Ops)
	}
	return dims
}

func blendDisabledColor(disabled bool, c color.NRGBA) color.NRGBA {
	if disabled {
		return color.NRGBA{
			R: uint8((uint16(c.R) + 0xFF) / 2),
			G: uint8((uint16(c.G) + 0xFF) / 2),
			B: uint8((uint16(c.B) + 0xFF) / 2),
			A: c.A,
		}
	}
	return c
}

func mulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	return color.NRGBA{
		R: uint8(uint16(c.R) * uint16(alpha) / 0xFF),
		G: uint8(uint16(c.G) * uint16(alpha) / 0xFF),
		B: uint8(uint16(c.B) * uint16(alpha) / 0xFF),
		A: uint8(uint16(c.A) * uint16(alpha) / 0xFF),
	}
}
