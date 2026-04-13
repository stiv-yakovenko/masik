package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"mycharm/coloreditor"
)

const defaultLineHeightScale = 1.35
const defaultTabWidthSpaces = 4

const defaultEditorText = "Open a folder from the File menu and choose a file in the explorer."

type userConfig struct {
	LastProjectDir        string  `json:"lastProjectDir"`
	EditorLineHeightScale float32 `json:"editorLineHeightScale"`
	TabWidthSpaces        int     `json:"tabWidthSpaces"`
}

type projectConfig struct {
	SelectedFile string `json:"selectedFile"`
}

type ideState struct {
	appRoot               string
	userConfigPath        string
	legacyUserConfigPath  string
	projectConfigPath     string
	currentDir            string
	selectedFile          string
	editorLineHeightScale float32
	tabWidthSpaces        int
	statusMessage         string
	statusError           bool
	explorer              explorerState
	workspace             workspaceState
	preferences           preferencesDialogState
}

type explorerState struct {
	list widget.List
	root *explorerNode
}

type explorerNode struct {
	Name     string
	Path     string
	IsDir    bool
	Expanded bool
	Loaded   bool
	Depth    int
	Children []*explorerNode
	Row      widget.Clickable
}

func newIDEState(appRoot string, editor *coloreditor.Editor) (*ideState, error) {
	state := newDefaultIDEState(appRoot, editor)

	cfg, err := loadUserConfig(state.userConfigPath, state.legacyUserConfigPath)
	if err != nil {
		return state, err
	}
	if cfg.EditorLineHeightScale > 0 {
		state.editorLineHeightScale = cfg.EditorLineHeightScale
	}
	if cfg.TabWidthSpaces > 0 {
		state.tabWidthSpaces = cfg.TabWidthSpaces
	}
	state.applyEditorPreferences(editor)

	currentDir := filepath.Clean(cfg.LastProjectDir)
	if currentDir == "" {
		currentDir = appRoot
	}
	if info, err := os.Stat(currentDir); err != nil || !info.IsDir() {
		currentDir = appRoot
	}
	if err := state.setCurrentDir(currentDir, editor); err != nil {
		return state, err
	}

	return state, nil
}

func newDefaultIDEState(appRoot string, editor *coloreditor.Editor) *ideState {
	appRoot = filepath.Clean(appRoot)
	state := &ideState{
		appRoot:               appRoot,
		userConfigPath:        defaultUserConfigPath(appRoot),
		legacyUserConfigPath:  legacyUserConfigPath(appRoot),
		currentDir:            appRoot,
		editorLineHeightScale: defaultLineHeightScale,
		tabWidthSpaces:        defaultTabWidthSpaces,
	}
	state.explorer.list.Axis = layout.Vertical
	state.workspace = newWorkspaceState()
	state.preferences = newPreferencesDialogState()
	state.applyEditorPreferences(editor)
	editor.SetText(defaultEditorText)
	state.projectConfigPath = projectConfigPath(state.currentDir)
	if err := state.reloadExplorer(); err != nil {
		state.setStatus(fmt.Sprintf("Failed to load explorer: %v", err), true)
	} else {
		state.setStatus("Choose a file in the explorer to open it in the editor.", false)
	}
	return state
}

func defaultUserConfigPath(appRoot string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = appRoot
	}
	return filepath.Join(homeDir, ".masic", "config.json")
}

func legacyUserConfigPath(appRoot string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = appRoot
	}
	return filepath.Join(homeDir, ".masik", "config.json")
}

func projectConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".masik", "config.json")
}

func loadUserConfig(path, legacyPath string) (userConfig, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return userConfig{}, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if legacyPath != "" {
			if legacyCfg, legacyErr := readUserConfigFile(legacyPath); legacyErr == nil {
				if err := saveUserConfig(path, legacyCfg); err != nil {
					return userConfig{}, err
				}
				return normalizeUserConfig(legacyCfg), nil
			}
		}
		cfg := userConfig{
			LastProjectDir:        "",
			EditorLineHeightScale: defaultLineHeightScale,
			TabWidthSpaces:        defaultTabWidthSpaces,
		}
		if err := saveUserConfig(path, cfg); err != nil {
			return userConfig{}, err
		}
		return normalizeUserConfig(cfg), nil
	} else if err != nil {
		return userConfig{}, err
	}

	cfg, err := readUserConfigFile(path)
	if err != nil {
		return userConfig{}, err
	}
	return normalizeUserConfig(cfg), nil
}

func readUserConfigFile(path string) (userConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return userConfig{}, err
	}
	var cfg userConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return userConfig{}, err
	}
	return cfg, nil
}

func normalizeUserConfig(cfg userConfig) userConfig {
	if cfg.EditorLineHeightScale <= 0 {
		cfg.EditorLineHeightScale = defaultLineHeightScale
	}
	if cfg.TabWidthSpaces <= 0 {
		cfg.TabWidthSpaces = defaultTabWidthSpaces
	}
	return cfg
}

func saveUserConfig(path string, cfg userConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func loadProjectConfig(projectDir string) (projectConfig, error) {
	path := projectConfigPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return projectConfig{}, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := projectConfig{SelectedFile: ""}
		if err := saveProjectConfig(projectDir, cfg); err != nil {
			return projectConfig{}, err
		}
		return cfg, nil
	} else if err != nil {
		return projectConfig{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return projectConfig{}, err
	}

	var cfg projectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return projectConfig{}, err
	}
	return cfg, nil
}

func saveProjectConfig(projectDir string, cfg projectConfig) error {
	path := projectConfigPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (s *ideState) saveUserSettings() error {
	cfg := userConfig{
		LastProjectDir:        filepath.Clean(s.currentDir),
		EditorLineHeightScale: s.editorLineHeightScale,
		TabWidthSpaces:        s.tabWidthSpaces,
	}
	return saveUserConfig(s.userConfigPath, cfg)
}

func (s *ideState) saveProjectSettings() error {
	cfg := projectConfig{
		SelectedFile: encodeConfigPath(s.currentDir, s.selectedFile),
	}
	return saveProjectConfig(s.currentDir, cfg)
}

func (s *ideState) setCurrentDir(dir string, editor *coloreditor.Editor) error {
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	s.currentDir = dir
	s.projectConfigPath = projectConfigPath(s.currentDir)
	s.selectedFile = ""
	s.applyEditorPreferences(editor)
	editor.SetText(defaultEditorText)
	if err := s.reloadExplorer(); err != nil {
		return err
	}
	userSaveErr := s.saveUserSettings()
	if err := s.restoreProjectSettings(editor); err != nil {
		s.setStatus(fmt.Sprintf("Project opened, but project config restore failed: %v", err), true)
		return nil
	}
	if userSaveErr != nil {
		s.setStatus(fmt.Sprintf("Project opened, but user config save failed: %v", userSaveErr), true)
		return nil
	}
	if s.selectedFile == "" {
		s.setStatus(fmt.Sprintf("Browsing %s", s.currentDir), false)
	}
	return nil
}

func (s *ideState) openFile(editor *coloreditor.Editor, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return fmt.Errorf("binary files are not supported in the editor yet")
	}

	editor.SetText(string(data))
	s.selectedFile = filepath.Clean(path)
	s.setStatus(fmt.Sprintf("Opened %s", s.selectedFileLabel()), false)
	s.expandToPath(s.selectedFile)
	if err := s.saveProjectSettings(); err != nil {
		s.setStatus(fmt.Sprintf("File opened, but project config save failed: %v", err), true)
	}
	return nil
}

func (s *ideState) restoreProjectSettings(editor *coloreditor.Editor) error {
	cfg, err := loadProjectConfig(s.currentDir)
	if err != nil {
		return err
	}

	selectedFile := resolveConfigPath(s.currentDir, cfg.SelectedFile)
	if selectedFile == "" || !isWithinRoot(s.currentDir, selectedFile) {
		return nil
	}

	info, err := os.Stat(selectedFile)
	if err != nil || info.IsDir() {
		return nil
	}

	return s.openFile(editor, selectedFile)
}

func (s *ideState) applyEditorPreferences(editor *coloreditor.Editor) {
	editor.TabWidth = s.tabWidthSpaces
}

func (s *ideState) setStatus(message string, isError bool) {
	s.statusMessage = message
	s.statusError = isError
}

func (s *ideState) selectedFileLabel() string {
	if s.selectedFile == "" {
		return "No file selected"
	}
	rel, err := filepath.Rel(s.currentDir, s.selectedFile)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return s.selectedFile
}

func (s *ideState) reloadExplorer() error {
	root := &explorerNode{
		Name:     folderDisplayName(s.currentDir),
		Path:     s.currentDir,
		IsDir:    true,
		Expanded: true,
		Loaded:   true,
	}
	children, err := readExplorerChildren(root.Path, 0)
	if err != nil {
		return err
	}
	root.Children = children
	s.explorer.root = root
	return nil
}

func (s *ideState) visibleExplorerNodes() []*explorerNode {
	if s.explorer.root == nil {
		return nil
	}
	nodes := make([]*explorerNode, 0, len(s.explorer.root.Children))
	var walk func(items []*explorerNode)
	walk = func(items []*explorerNode) {
		for _, item := range items {
			nodes = append(nodes, item)
			if item.IsDir && item.Expanded {
				walk(item.Children)
			}
		}
	}
	walk(s.explorer.root.Children)
	return nodes
}

func (s *ideState) handleExplorerClicks(gtx layout.Context, editor *coloreditor.Editor) {
	for _, node := range s.visibleExplorerNodes() {
		if !node.Row.Clicked(gtx) {
			continue
		}
		if node.IsDir {
			if !node.Loaded {
				children, err := readExplorerChildren(node.Path, node.Depth)
				if err != nil {
					s.setStatus(fmt.Sprintf("Failed to read %s: %v", node.Path, err), true)
					continue
				}
				node.Children = children
				node.Loaded = true
			}
			node.Expanded = !node.Expanded
			continue
		}
		if err := s.openFile(editor, node.Path); err != nil {
			s.setStatus(fmt.Sprintf("Failed to open %s: %v", filepath.Base(node.Path), err), true)
		}
	}
}

func readExplorerChildren(dir string, depth int) ([]*explorerNode, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	nodes := make([]*explorerNode, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)
		nodes = append(nodes, &explorerNode{
			Name:  name,
			Path:  fullPath,
			IsDir: entry.IsDir(),
			Depth: depth + 1,
		})
	}
	return nodes, nil
}

func (s *ideState) expandToPath(target string) {
	if s.explorer.root == nil || target == "" {
		return
	}
	rel, err := filepath.Rel(s.currentDir, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) <= 1 {
		return
	}

	node := s.explorer.root
	for _, part := range parts[:len(parts)-1] {
		var match *explorerNode
		for _, child := range node.Children {
			if child.IsDir && child.Name == part {
				match = child
				break
			}
		}
		if match == nil {
			return
		}
		if !match.Loaded {
			children, err := readExplorerChildren(match.Path, match.Depth)
			if err != nil {
				return
			}
			match.Children = children
			match.Loaded = true
		}
		match.Expanded = true
		node = match
	}
}

func layoutExplorerPanel(gtx layout.Context, th *material.Theme, state *ideState) layout.Dimensions {
	panelWidth := gtx.Dp(unit.Dp(280))
	gtx.Constraints.Min.X = panelWidth
	gtx.Constraints.Max.X = panelWidth

	return widget.Border{
		Color:        color.NRGBA{R: 0xD0, G: 0xD7, B: 0xDE, A: 0xFF},
		CornerRadius: unit.Dp(10),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill(gtx, color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF})
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := material.Body2(th, "Explorer")
					label.Color = color.NRGBA{R: 0x5A, G: 0x63, B: 0x68, A: 0xFF}
					return label.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					nodes := state.visibleExplorerNodes()
					list := material.List(th, &state.explorer.list)
					return list.Layout(gtx, len(nodes), func(gtx layout.Context, index int) layout.Dimensions {
						return layoutExplorerRow(gtx, th, state, nodes[index])
					})
				}),
			)
		})
	})
}

func layoutExplorerRow(gtx layout.Context, th *material.Theme, state *ideState, node *explorerNode) layout.Dimensions {
	return node.Row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		height := gtx.Dp(unit.Dp(30))
		gtx.Constraints.Min.Y = height
		gtx.Constraints.Max.Y = height

		isSelected := !node.IsDir && filepath.Clean(node.Path) == state.selectedFile
		background := color.NRGBA{A: 0}
		border := color.NRGBA{A: 0}
		if isSelected {
			background = color.NRGBA{R: 0xE3, G: 0xEC, B: 0xF6, A: 0xFF}
			border = color.NRGBA{R: 0xB7, G: 0xC9, B: 0xDB, A: 0xFF}
		} else if node.Row.Hovered() {
			background = color.NRGBA{R: 0xF0, G: 0xEA, B: 0xDE, A: 0xFF}
			border = color.NRGBA{R: 0xE1, G: 0xD9, B: 0xCC, A: 0xFF}
		}

		return widget.Border{
			Color:        border,
			CornerRadius: unit.Dp(6),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if background.A != 0 {
				fillRect(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), background)
			}

			leftInset := unit.Dp(float32((node.Depth - 1) * 16))
			return layout.Inset{
				Top:    unit.Dp(5),
				Bottom: unit.Dp(5),
				Left:   leftInset + unit.Dp(10),
				Right:  unit.Dp(10),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := material.Body2(th, explorerLabel(node))
				if node.IsDir {
					label.Color = color.NRGBA{R: 0x38, G: 0x42, B: 0x48, A: 0xFF}
				} else {
					label.Color = color.NRGBA{R: 0x52, G: 0x5B, B: 0x61, A: 0xFF}
				}
				return label.Layout(gtx)
			})
		})
	})
}

func explorerLabel(node *explorerNode) string {
	if !node.IsDir {
		return node.Name
	}
	if node.Expanded {
		return "[-] " + node.Name
	}
	return "[+] " + node.Name
}

func resolveConfigPath(base, value string) string {
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}

func encodeConfigPath(base, value string) string {
	if value == "" {
		return ""
	}
	rel, err := filepath.Rel(base, value)
	if err == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return rel
	}
	if samePath(base, value) {
		return "."
	}
	return filepath.Clean(value)
}

func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "..")
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
