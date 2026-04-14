package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gioui.org/unit"
	"mycharm/coloreditor"
)

const defaultTabWidthSpaces = 4

const defaultEditorText = "Open a folder from the File menu and choose a file in the explorer."

type userConfig struct {
	LastProjectDir string `json:"lastProjectDir"`
	TabWidthSpaces int    `json:"tabWidthSpaces"`
}

type projectConfig struct {
	SelectedFile string `json:"selectedFile"`
}

type ideState struct {
	appRoot              string
	userConfigPath       string
	legacyUserConfigPath string
	fontsConfigPath      string
	projectConfigPath    string
	currentDir           string
	selectedFile         string
	openDocuments        []openDocumentState
	activeDocument       int
	tabWidthSpaces       int
	appearance           appearanceSettings
	statusMessage        string
	statusError          bool
	explorer             explorerState
	workspace            workspaceState
	preferences          preferencesDialogState
}

func newIDEState(appRoot string, editor *coloreditor.Editor) (*ideState, error) {
	state := newDefaultIDEState(appRoot, editor)

	cfg, err := loadUserConfig(state.userConfigPath, state.legacyUserConfigPath)
	if err != nil {
		return state, err
	}
	if cfg.TabWidthSpaces > 0 {
		state.tabWidthSpaces = cfg.TabWidthSpaces
	}
	fonts, err := loadFontsConfig(state.fontsConfigPath)
	if err != nil {
		return state, err
	}
	state.appearance = fonts
	state.applyEditorPreferences(editor)

	currentDir := absoluteCleanPath(cfg.LastProjectDir)
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
	appRoot = absoluteCleanPath(appRoot)
	state := &ideState{
		appRoot:              appRoot,
		userConfigPath:       defaultUserConfigPath(appRoot),
		legacyUserConfigPath: legacyUserConfigPath(appRoot),
		fontsConfigPath:      defaultFontsConfigPath(appRoot),
		currentDir:           appRoot,
		activeDocument:       -1,
		tabWidthSpaces:       defaultTabWidthSpaces,
		appearance:           defaultAppearanceSettings(),
	}
	state.explorer = newExplorerState()
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
			LastProjectDir: "",
			TabWidthSpaces: defaultTabWidthSpaces,
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
		LastProjectDir: filepath.Clean(s.currentDir),
		TabWidthSpaces: s.tabWidthSpaces,
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
	dir = absoluteCleanPath(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	s.currentDir = dir
	s.projectConfigPath = projectConfigPath(s.currentDir)
	s.resetOpenDocuments(editor)
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
	s.syncActiveDocument(editor)
	path = filepath.Clean(path)
	if index := s.findOpenDocument(path); index >= 0 {
		s.activateDocument(editor, index)
		s.setStatus(fmt.Sprintf("Switched to %s", s.selectedFileLabel()), false)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return fmt.Errorf("binary files are not supported in the editor yet")
	}

	s.openDocuments = append(s.openDocuments, openDocumentState{
		Path:    path,
		Content: string(data),
	})
	s.activeDocument = len(s.openDocuments) - 1
	editor.SetText(s.openDocuments[s.activeDocument].Content)
	s.selectedFile = path
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
	editor.LineHeight = unit.Sp(s.appearance.EditorLineHeight)
	editor.LineHeightScale = 1
}

func (s *ideState) resetOpenDocuments(editor *coloreditor.Editor) {
	s.openDocuments = nil
	s.activeDocument = -1
	s.selectedFile = ""
	s.applyEditorPreferences(editor)
	editor.SetText(defaultEditorText)
}

func (s *ideState) syncActiveDocument(editor *coloreditor.Editor) {
	if s.activeDocument < 0 || s.activeDocument >= len(s.openDocuments) {
		s.selectedFile = ""
		return
	}
	s.openDocuments[s.activeDocument].Content = editor.Text()
	s.selectedFile = s.openDocuments[s.activeDocument].Path
}

func (s *ideState) findOpenDocument(path string) int {
	path = filepath.Clean(path)
	for i := range s.openDocuments {
		if samePath(s.openDocuments[i].Path, path) {
			return i
		}
	}
	return -1
}

func (s *ideState) activateDocument(editor *coloreditor.Editor, index int) {
	if index < 0 || index >= len(s.openDocuments) {
		return
	}
	if index == s.activeDocument {
		s.selectedFile = s.openDocuments[index].Path
		return
	}
	s.syncActiveDocument(editor)
	s.activeDocument = index
	editor.SetText(s.openDocuments[index].Content)
	s.selectedFile = s.openDocuments[index].Path
	s.expandToPath(s.selectedFile)
	if err := s.saveProjectSettings(); err != nil {
		s.setStatus(fmt.Sprintf("Document selected, but project config save failed: %v", err), true)
	}
}

func (s *ideState) closeDocument(editor *coloreditor.Editor, index int) {
	if index < 0 || index >= len(s.openDocuments) {
		return
	}

	closedPath := s.openDocuments[index].Path
	s.syncActiveDocument(editor)

	s.openDocuments = append(s.openDocuments[:index], s.openDocuments[index+1:]...)

	switch {
	case len(s.openDocuments) == 0:
		s.activeDocument = -1
		s.selectedFile = ""
		editor.SetText(defaultEditorText)
	case index == s.activeDocument:
		if index >= len(s.openDocuments) {
			index = len(s.openDocuments) - 1
		}
		s.activeDocument = index
		editor.SetText(s.openDocuments[s.activeDocument].Content)
		s.selectedFile = s.openDocuments[s.activeDocument].Path
		s.expandToPath(s.selectedFile)
	case index < s.activeDocument:
		s.activeDocument--
		s.selectedFile = s.openDocuments[s.activeDocument].Path
	default:
		s.selectedFile = s.openDocuments[s.activeDocument].Path
	}

	if err := s.saveProjectSettings(); err != nil {
		s.setStatus(fmt.Sprintf("Closed %s, but project config save failed: %v", s.relativePathLabel(closedPath), err), true)
		return
	}
	s.setStatus(fmt.Sprintf("Closed %s", s.relativePathLabel(closedPath)), false)
}

func (s *ideState) setStatus(message string, isError bool) {
	s.statusMessage = message
	s.statusError = isError
}

func (s *ideState) selectedFileLabel() string {
	if s.selectedFile == "" {
		return "No file selected"
	}
	return s.relativePathLabel(s.selectedFile)
}

func (s *ideState) relativePathLabel(path string) string {
	rel, err := filepath.Rel(s.currentDir, path)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func (s *ideState) documentTabLabel(path string) string {
	base := filepath.Base(path)
	duplicates := 0
	for i := range s.openDocuments {
		if filepath.Base(s.openDocuments[i].Path) == base {
			duplicates++
		}
	}
	if duplicates > 1 {
		return s.relativePathLabel(path)
	}
	return base
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

func absoluteCleanPath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}
