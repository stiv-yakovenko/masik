package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

const (
	preferenceKeyExplorerFontSize   = "explorer_font_size"
	preferenceKeyExplorerLineHeight = "explorer_line_height"
	preferenceKeyTerminalFontSize   = "terminal_font_size"
	preferenceKeyTerminalLineHeight = "terminal_line_height"
	preferenceKeyEditorFontSize     = "editor_font_size"
	preferenceKeyEditorLineHeight   = "editor_line_height"

	defaultExplorerFontSize   = 14.0
	defaultExplorerLineHeight = 20.0
	defaultTerminalFontSize   = 14.0
	defaultTerminalLineHeight = 20.0
	defaultEditorFontSize     = 16.0
	defaultEditorLineHeight   = 22.0

	minPreferenceFontSize   = 8.0
	maxPreferenceFontSize   = 40.0
	minPreferenceLineHeight = 8.0
	maxPreferenceLineHeight = 72.0
	preferenceValueStep     = 0.5
)

type textMetrics struct {
	FontSize   float32
	LineHeight float32
}

type appearanceSettings struct {
	ExplorerFontSize   float32 `json:"explorer_font_size"`
	ExplorerLineHeight float32 `json:"explorer_line_height"`
	TerminalFontSize   float32 `json:"terminal_font_size"`
	TerminalLineHeight float32 `json:"terminal_line_height"`
	EditorFontSize     float32 `json:"editor_font_size"`
	EditorLineHeight   float32 `json:"editor_line_height"`
}

type appearancePreferenceDescriptor struct {
	Key         string
	Description string
}

func defaultAppearanceSettings() appearanceSettings {
	return appearanceSettings{
		ExplorerFontSize:   defaultExplorerFontSize,
		ExplorerLineHeight: defaultExplorerLineHeight,
		TerminalFontSize:   defaultTerminalFontSize,
		TerminalLineHeight: defaultTerminalLineHeight,
		EditorFontSize:     defaultEditorFontSize,
		EditorLineHeight:   defaultEditorLineHeight,
	}
}

func defaultAppearancePreferenceDescriptors() []appearancePreferenceDescriptor {
	return []appearancePreferenceDescriptor{
		{Key: preferenceKeyExplorerFontSize, Description: "Explorer text size."},
		{Key: preferenceKeyExplorerLineHeight, Description: "Explorer row spacing."},
		{Key: preferenceKeyTerminalFontSize, Description: "Terminal text size."},
		{Key: preferenceKeyTerminalLineHeight, Description: "Terminal row spacing."},
		{Key: preferenceKeyEditorFontSize, Description: "Editor text size."},
		{Key: preferenceKeyEditorLineHeight, Description: "Editor line spacing."},
	}
}

func normalizeAppearanceSettings(settings appearanceSettings) appearanceSettings {
	defaults := defaultAppearanceSettings()

	if settings.ExplorerFontSize <= 0 {
		settings.ExplorerFontSize = defaults.ExplorerFontSize
	}
	if settings.ExplorerLineHeight <= 0 {
		settings.ExplorerLineHeight = defaults.ExplorerLineHeight
	}
	if settings.TerminalFontSize <= 0 {
		settings.TerminalFontSize = defaults.TerminalFontSize
	}
	if settings.TerminalLineHeight <= 0 {
		settings.TerminalLineHeight = defaults.TerminalLineHeight
	}
	if settings.EditorFontSize <= 0 {
		settings.EditorFontSize = defaults.EditorFontSize
	}
	if settings.EditorLineHeight <= 0 {
		settings.EditorLineHeight = defaults.EditorLineHeight
	}

	settings.ExplorerFontSize = roundPreferenceValue(clampFloat32(settings.ExplorerFontSize, minPreferenceFontSize, maxPreferenceFontSize))
	settings.TerminalFontSize = roundPreferenceValue(clampFloat32(settings.TerminalFontSize, minPreferenceFontSize, maxPreferenceFontSize))
	settings.EditorFontSize = roundPreferenceValue(clampFloat32(settings.EditorFontSize, minPreferenceFontSize, maxPreferenceFontSize))

	settings.ExplorerLineHeight = roundPreferenceValue(clampFloat32(settings.ExplorerLineHeight, max(settings.ExplorerFontSize, minPreferenceLineHeight), maxPreferenceLineHeight))
	settings.TerminalLineHeight = roundPreferenceValue(clampFloat32(settings.TerminalLineHeight, max(settings.TerminalFontSize, minPreferenceLineHeight), maxPreferenceLineHeight))
	settings.EditorLineHeight = roundPreferenceValue(clampFloat32(settings.EditorLineHeight, max(settings.EditorFontSize, minPreferenceLineHeight), maxPreferenceLineHeight))

	return settings
}

func (a appearanceSettings) explorerMetrics() textMetrics {
	return textMetrics{FontSize: a.ExplorerFontSize, LineHeight: a.ExplorerLineHeight}
}

func (a appearanceSettings) terminalMetrics() textMetrics {
	return textMetrics{FontSize: a.TerminalFontSize, LineHeight: a.TerminalLineHeight}
}

func (a appearanceSettings) editorMetrics() textMetrics {
	return textMetrics{FontSize: a.EditorFontSize, LineHeight: a.EditorLineHeight}
}

func (a appearanceSettings) valueForKey(key string) float32 {
	switch key {
	case preferenceKeyExplorerFontSize:
		return a.ExplorerFontSize
	case preferenceKeyExplorerLineHeight:
		return a.ExplorerLineHeight
	case preferenceKeyTerminalFontSize:
		return a.TerminalFontSize
	case preferenceKeyTerminalLineHeight:
		return a.TerminalLineHeight
	case preferenceKeyEditorFontSize:
		return a.EditorFontSize
	case preferenceKeyEditorLineHeight:
		return a.EditorLineHeight
	default:
		return 0
	}
}

func (a *appearanceSettings) setValueForKey(key string, value float32) bool {
	switch key {
	case preferenceKeyExplorerFontSize:
		a.ExplorerFontSize = value
	case preferenceKeyExplorerLineHeight:
		a.ExplorerLineHeight = value
	case preferenceKeyTerminalFontSize:
		a.TerminalFontSize = value
	case preferenceKeyTerminalLineHeight:
		a.TerminalLineHeight = value
	case preferenceKeyEditorFontSize:
		a.EditorFontSize = value
	case preferenceKeyEditorLineHeight:
		a.EditorLineHeight = value
	default:
		return false
	}
	return true
}

func formatPreferenceValue(value float32) string {
	return fmt.Sprintf("%.1f", roundPreferenceValue(value))
}

func parsePreferenceValue(text string) (float32, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(text, ",", "."))
	if normalized == "" {
		return 0, fmt.Errorf("value cannot be empty")
	}
	value, err := strconv.ParseFloat(normalized, 32)
	if err != nil {
		return 0, err
	}
	return roundPreferenceValue(float32(value)), nil
}

func roundPreferenceValue(value float32) float32 {
	return float32(math.Round(float64(value/preferenceValueStep))) * preferenceValueStep
}

func clampFloat32(value, lo, hi float32) float32 {
	if hi < lo {
		hi = lo
	}
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}

func labelWithMetrics(th *material.Theme, text string, metrics textMetrics) material.LabelStyle {
	label := material.Label(th, unit.Sp(metrics.FontSize), text)
	label.LineHeight = unit.Sp(metrics.LineHeight)
	label.LineHeightScale = 1
	return label
}

func monoLabelWithMetrics(th *material.Theme, text string, metrics textMetrics) material.LabelStyle {
	label := labelWithMetrics(th, text, metrics)
	label.Font.Typeface = "monospace"
	return label
}

func textBlockHeightPx(gtx layout.Context, metrics textMetrics, paddingDp float32, minHeightDp float32) int {
	contentHeight := max(gtx.Sp(unit.Sp(metrics.FontSize)), gtx.Sp(unit.Sp(metrics.LineHeight)))
	return max(gtx.Dp(unit.Dp(minHeightDp)), contentHeight+gtx.Dp(unit.Dp(paddingDp)))
}
