package coloreditor

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

// textIterator computes the bounding box of and paints text.
type textIterator struct {
	// viewport is the rectangle of document coordinates that the iterator is
	// trying to fill with text.
	viewport image.Rectangle
	// maxLines is the maximum number of text lines that should be displayed.
	maxLines int
	// material sets the paint material for the text glyphs.
	material op.CallOp
	// truncated tracks the count of truncated runes in the text.
	truncated int
	// linesSeen tracks the quantity of line endings this iterator has seen.
	linesSeen int
	// lineOff tracks the origin for the glyphs in the current line.
	lineOff f32.Point
	// padding is the space needed outside of the bounds of the text to ensure no
	// part of a glyph is clipped.
	padding image.Rectangle
	// bounds is the logical bounding box of the text.
	bounds image.Rectangle
	// visible tracks whether the most recently iterated glyph is visible within
	// the viewport.
	visible bool
	// first tracks whether the iterator has processed a glyph yet.
	first bool
	// baseline tracks the location of the first line of text's baseline.
	baseline int
}

// processGlyph checks whether the glyph is visible within the iterator's configured
// viewport and updates the iterator's text dimensions to include the glyph.
func (it *textIterator) processGlyph(g text.Glyph, ok bool) (visibleOrBefore bool) {
	if it.maxLines > 0 {
		if g.Flags&text.FlagTruncator != 0 && g.Flags&text.FlagClusterBreak != 0 {
			it.truncated = int(g.Runes)
		}
		if g.Flags&text.FlagLineBreak != 0 {
			it.linesSeen++
		}
		if it.linesSeen == it.maxLines && g.Flags&text.FlagParagraphBreak != 0 {
			return false
		}
	}
	if d := g.Bounds.Min.X.Floor(); d < it.padding.Min.X {
		it.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > it.padding.Max.X {
		it.padding.Max.X = d
	}
	if d := (g.Bounds.Min.Y + g.Ascent).Floor(); d < it.padding.Min.Y {
		it.padding.Min.Y = d
	}
	if d := (g.Bounds.Max.Y - g.Descent).Ceil(); d > it.padding.Max.Y {
		it.padding.Max.Y = d
	}
	logicalBounds := image.Rectangle{
		Min: image.Pt(g.X.Floor(), int(g.Y)-g.Ascent.Ceil()),
		Max: image.Pt((g.X + g.Advance).Ceil(), int(g.Y)+g.Descent.Ceil()),
	}
	if !it.first {
		it.first = true
		it.baseline = int(g.Y)
		it.bounds = logicalBounds
	}

	above := logicalBounds.Max.Y < it.viewport.Min.Y
	below := logicalBounds.Min.Y > it.viewport.Max.Y
	left := logicalBounds.Max.X < it.viewport.Min.X
	right := logicalBounds.Min.X > it.viewport.Max.X
	it.visible = !above && !below && !left && !right
	if it.visible {
		it.bounds.Min.X = min(it.bounds.Min.X, logicalBounds.Min.X)
		it.bounds.Min.Y = min(it.bounds.Min.Y, logicalBounds.Min.Y)
		it.bounds.Max.X = max(it.bounds.Max.X, logicalBounds.Max.X)
		it.bounds.Max.Y = max(it.bounds.Max.Y, logicalBounds.Max.Y)
	}
	return ok && !below
}

func fixedToFloat(i fixed.Int26_6) float32 {
	return float32(i) / 64.0
}

func (it *textIterator) flushLine(gtx layout.Context, shaper *text.Shaper, line []text.Glyph) {
	if len(line) == 0 {
		return
	}
	t := op.Affine(f32.AffineId().Offset(it.lineOff)).Push(gtx.Ops)
	path := shaper.Shape(line)
	outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
	it.material.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	outline.Pop()
	if call := shaper.Bitmaps(line); call != (op.CallOp{}) {
		call.Add(gtx.Ops)
	}
	t.Pop()
}

// paintGlyph buffers up and paints text glyphs.
func (it *textIterator) paintGlyph(gtx layout.Context, shaper *text.Shaper, glyph text.Glyph, line []text.Glyph) ([]text.Glyph, bool) {
	visibleOrBefore := it.processGlyph(glyph, true)
	if it.visible {
		if len(line) == 0 {
			it.lineOff = f32.Point{X: fixedToFloat(glyph.X), Y: float32(glyph.Y)}.Sub(layout.FPt(it.viewport.Min))
		}
		line = append(line, glyph)
	}
	if glyph.Flags&text.FlagLineBreak != 0 || cap(line)-len(line) == 0 || !visibleOrBefore {
		it.flushLine(gtx, shaper, line)
		line = line[:0]
	}
	return line, visibleOrBefore
}
