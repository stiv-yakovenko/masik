package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
)

type windowCaptionDecorator func(layout.Context, layout.Widget) layout.Dimensions

type WindowCaption struct {
	Height         unit.Dp
	Background     color.NRGBA
	BottomBorder   color.NRGBA
	ContentInset   layout.Inset
	Icon           layout.Widget
	IconDecorator  windowCaptionDecorator
	IconGap        unit.Dp
	Menus          []layout.Widget
	MenuSpacing    unit.Dp
	GapAfterMenus  unit.Dp
	Title          layout.Widget
	TitleDecorator windowCaptionDecorator
	RightButtons   []layout.Widget
	RightSpacing   unit.Dp
}

func layoutWindowCaption(gtx layout.Context, caption WindowCaption) layout.Dimensions {
	height := gtx.Dp(caption.Height)
	if height <= 0 {
		height = gtx.Dp(unit.Dp(46))
	}
	gtx.Constraints.Min.Y = height
	gtx.Constraints.Max.Y = height

	fill(gtx, caption.Background)
	if caption.BottomBorder.A != 0 {
		fillRect(gtx, image.Rect(0, height-1, gtx.Constraints.Max.X, height), caption.BottomBorder)
	}

	return caption.ContentInset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(caption.Menus)*2+len(caption.RightButtons)*2+4)

		if caption.Icon != nil {
			iconWidget := caption.Icon
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return applyWindowCaptionDecorator(gtx, caption.IconDecorator, iconWidget)
			}))
			if hasWindowCaptionContent(caption.Menus, caption.Title, caption.RightButtons) {
				children = append(children, layout.Rigid(layout.Spacer{Width: caption.IconGap}.Layout))
			}
		}

		for i, menu := range caption.Menus {
			menuWidget := menu
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return menuWidget(gtx)
			}))
			if i < len(caption.Menus)-1 {
				children = append(children, layout.Rigid(layout.Spacer{Width: caption.MenuSpacing}.Layout))
			}
		}
		if len(caption.Menus) > 0 && (caption.Title != nil || len(caption.RightButtons) > 0) {
			children = append(children, layout.Rigid(layout.Spacer{Width: caption.GapAfterMenus}.Layout))
		}

		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if caption.Title == nil {
				return layout.Dimensions{}
			}
			return applyWindowCaptionDecorator(gtx, caption.TitleDecorator, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return caption.Title(gtx)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{}
					}),
				)
			})
		}))

		for i, button := range caption.RightButtons {
			if i > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: caption.RightSpacing}.Layout))
			}
			buttonWidget := button
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return buttonWidget(gtx)
			}))
		}

		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func applyWindowCaptionDecorator(gtx layout.Context, decorator windowCaptionDecorator, w layout.Widget) layout.Dimensions {
	if decorator == nil {
		return w(gtx)
	}
	return decorator(gtx, w)
}

func hasWindowCaptionContent(menus []layout.Widget, title layout.Widget, rightButtons []layout.Widget) bool {
	return len(menus) > 0 || title != nil || len(rightButtons) > 0
}
