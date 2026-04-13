package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

//go:embed logo.png
var logoPNG []byte

var (
	logoOp    paint.ImageOp
	logoReady bool
)

func init() {
	img, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return
	}
	logoOp = paint.NewImageOp(removeWhiteLogoBackground(img))
	logoReady = true
}

func layoutAppLogo(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(unit.Dp(28))
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	if !logoReady {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	return widget.Image{
		Src:      logoOp,
		Fit:      widget.Contain,
		Position: layout.Center,
		Scale:    1.0 / gtx.Metric.PxPerDp,
	}.Layout(gtx)
}

func removeWhiteLogoBackground(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	visited := make([]bool, bounds.Dx()*bounds.Dy())
	queue := make([]image.Point, 0, bounds.Dx()*2+bounds.Dy()*2)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			dst.SetNRGBA(x, y, c)
		}
	}

	push := func(x, y int) {
		if !image.Pt(x, y).In(bounds) {
			return
		}
		idx := (y-bounds.Min.Y)*bounds.Dx() + (x - bounds.Min.X)
		if visited[idx] {
			return
		}
		c := dst.NRGBAAt(x, y)
		if !isBackgroundWhite(c) {
			return
		}
		visited[idx] = true
		queue = append(queue, image.Pt(x, y))
	}

	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		push(x, bounds.Min.Y)
		push(x, bounds.Max.Y-1)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		push(bounds.Min.X, y)
		push(bounds.Max.X-1, y)
	}

	for head := 0; head < len(queue); head++ {
		p := queue[head]
		c := dst.NRGBAAt(p.X, p.Y)
		dst.SetNRGBA(p.X, p.Y, knockOutWhite(c))

		push(p.X-1, p.Y)
		push(p.X+1, p.Y)
		push(p.X, p.Y-1)
		push(p.X, p.Y+1)
	}

	return dst
}

func isBackgroundWhite(c color.NRGBA) bool {
	if c.A == 0 {
		return true
	}
	return whitenessDeviation(c) <= 56
}

func knockOutWhite(c color.NRGBA) color.NRGBA {
	if c.A == 0 {
		return c
	}

	deviation := whitenessDeviation(c)
	switch {
	case deviation <= 12:
		return color.NRGBA{}
	case deviation <= 56:
		alpha := uint8((deviation - 12) * 255 / (56 - 12))
		return unblendFromWhite(c, alpha)
	default:
		return c
	}
}

func whitenessDeviation(c color.NRGBA) int {
	return max3(255-int(c.R), 255-int(c.G), 255-int(c.B))
}

func unblendFromWhite(c color.NRGBA, alpha uint8) color.NRGBA {
	if alpha == 0 {
		return color.NRGBA{}
	}
	if alpha == 0xFF {
		c.A = 0xFF
		return c
	}

	return color.NRGBA{
		R: recoverChannelFromWhite(c.R, alpha),
		G: recoverChannelFromWhite(c.G, alpha),
		B: recoverChannelFromWhite(c.B, alpha),
		A: alpha,
	}
}

func recoverChannelFromWhite(v, alpha uint8) uint8 {
	a := float64(alpha) / 255.0
	if a <= 0 {
		return 0
	}
	out := (float64(v)/255.0 - (1.0 - a)) / a
	if out < 0 {
		out = 0
	}
	if out > 1 {
		out = 1
	}
	return uint8(out*255 + 0.5)
}

func max3(a, b, c int) int {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}
