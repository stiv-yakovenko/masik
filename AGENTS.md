# AGENTS.md — masik codebase map

masik is a desktop IDE built with [Gio UI](https://gioui.org/) (Go, immediate-mode GUI).
Custom window title bar, file explorer, line-colored editor, embedded terminal (ConPTY, Windows only), chat panel (stub).

---

## Root files (`package main`)

### [main.go](main.go)
Entry point and main window rendering.

| Function | Does |
|---|---|
| `main()` | Starts goroutine with `app.Window`, calls `run()`, then `app.Main()` |
| `run(w)` | Main event loop: handles `FrameEvent`, `DestroyEvent`, `ConfigEvent`, channels `folderPickResults` / `terminalEvents` |
| `layoutUI(...)` | Root layout: title → workspace → status bar; overlays File menu and Preferences modal |
| `layoutTitleMenuBar(...)` | Renders title bar: logo, File/Edit/View buttons, folder badge, window controls |
| `layoutFileMenuPopup(...)` | File dropdown: "Open folder" and "Preferences" |
| `layoutFileMenuItem(...)` | One clickable menu item |
| `layoutFolderBadge(...)` | Current folder path badge in title bar |
| `layoutMoveArea(...)` | Wraps widget in window drag zone (`system.ActionMove`) |
| `layoutMenuButton(...)` | Menu button with hover effect |
| `layoutWindowButton(...)` | Window control button (min/max/restore/close) with icon |
| `layoutEditor(...)` | Places `coloreditor.Editor` with font metrics |
| `visualLineColor(index)` | Deterministic line color via FNV hash of line index |
| `layoutWithSideBorders(...)` | Background fill + vertical side borders, then child widget |
| `fill(gtx, c)` | Fills `gtx.Constraints.Min` with color |
| `fillRect(gtx, r, c)` | Fills rect `r` with color `c` |
| `drawWindowIcon(...)` | Draws window control icon (─ / □ / ⧉ / ×) |
| `drawRectOutline(...)` | Draws rect outline with given thickness |
| `drawLine(...)` | Draws line segment via `clip.Path` |
| `drawCloseIcon(...)` | Draws × from pixels |
| `shiftColor(c, delta)` | Shifts RGB channels by `delta` (lighten/darken) |
| `clampColor(v)` | Clamps int to [0,255] → `uint8` |

---

### [ide.go](ide.go)
Core IDE state, config, explorer, file opening.

| Type | Holds |
|---|---|
| `userConfig` | User settings: `LastProjectDir`, `TabWidthSpaces` (JSON) |
| `projectConfig` | Project settings: `SelectedFile` (JSON, stored in `.masik/config.json`) |
| `ideState` | Main IDE state: config paths, current folder, open documents, active doc, tab width, appearance, status bar, explorer, workspace, preferences |
| `explorerState` | Explorer panel: scrollable `widget.List` + root tree node |
| `explorerNode` | File tree node: name, path, isDir, expanded/loaded, depth, children, row button |

| Function | Does |
|---|---|
| `newIDEState(appRoot, editor)` | Loads configs, restores last folder, returns `*ideState` |
| `newDefaultIDEState(appRoot, editor)` | Creates `ideState` with defaults, no file reads |
| `defaultUserConfigPath(appRoot)` | Path `~/.masic/config.json` |
| `legacyUserConfigPath(appRoot)` | Path `~/.masik/config.json` (old name) |
| `projectConfigPath(projectDir)` | Path `<project>/.masik/config.json` |
| `loadUserConfig(path, legacyPath)` | Reads user config, migrates from legacy path if new missing |
| `readUserConfigFile(path)` | Reads and parses JSON into `userConfig` |
| `normalizeUserConfig(cfg)` | Fills zero fields with defaults |
| `saveUserConfig(path, cfg)` | Serializes and writes `userConfig` to JSON |
| `loadProjectConfig(projectDir)` | Reads project config, creates if missing |
| `saveProjectConfig(projectDir, cfg)` | Writes `projectConfig` to JSON |
| `(s) saveUserSettings()` | Saves `currentDir` and `tabWidth` to user config |
| `(s) saveProjectSettings()` | Saves active file path to project config |
| `(s) setCurrentDir(dir, editor)` | Changes working folder: reloads explorer, restores project settings, saves user config |
| `(s) openFile(editor, path)` | Opens file in buffer; activates existing tab if already open; rejects binary files |
| `(s) restoreProjectSettings(editor)` | On folder change, opens file from project config |
| `(s) applyEditorPreferences(editor)` | Applies `TabWidth` and `LineHeight` to editor |
| `(s) resetOpenDocuments(editor)` | Closes all tabs, resets editor to welcome text |
| `(s) syncActiveDocument(editor)` | Syncs editor text → active `openDocumentState` |
| `(s) findOpenDocument(path)` | Finds open document by path, returns index or -1 |
| `(s) activateDocument(editor, index)` | Switches to tab `index`, syncs buffers |
| `(s) closeDocument(editor, index)` | Closes tab, switches to neighbor |
| `(s) setStatus(message, isError)` | Sets status bar text and error flag |
| `(s) selectedFileLabel()` | Returns display name of selected file (relative path) |
| `(s) relativePathLabel(path)` | Returns path relative to `currentDir` or absolute |
| `(s) documentTabLabel(path)` | Returns `basename`; relative path on collision |
| `(s) reloadExplorer()` | Rebuilds explorer tree for `currentDir` |
| `(s) visibleExplorerNodes()` | Returns flat list of visible (expanded) nodes |
| `(s) handleExplorerClicks(gtx, editor)` | Handles clicks: folder → expand/collapse, file → `openFile` |
| `readExplorerChildren(dir, depth)` | Reads dir, sorts (dirs first, alphabetically), returns nodes |
| `(s) expandToPath(target)` | Expands explorer to file `target` |
| `layoutExplorerPanel(gtx, th, state)` | Renders Explorer panel with header and scroll |
| `layoutExplorerRow(gtx, th, state, node)` | Renders one explorer row with indent, hover, selected state |
| `explorerLabel(node)` | Builds row label: `[+]`/`[-]` for dirs, name for files |
| `resolveConfigPath(base, value)` | Converts relative/absolute config path to absolute |
| `encodeConfigPath(base, value)` | Encodes path for config: makes relative if inside project |
| `isWithinRoot(root, target)` | Checks if `target` is inside `root` |
| `samePath(a, b)` | Compares two paths after `filepath.Clean` |
| `absoluteCleanPath(path)` | `filepath.Abs` + `filepath.Clean`, no panic on error |

---

### [workspace.go](workspace.go)
Workspace: side panels, splitters, activity bar, status bar, chat.

| Type | Holds |
|---|---|
| `workspaceState` | Explorer/chat visibility & width, buttons, splitters, chat state, terminal |
| `paneSplitter` | Horizontal splitter: drag gesture, position, size on press |
| `verticalPaneSplitter` | Vertical splitter (for terminal) |
| `activityBarItem` | Activity bar element: button, active flag, label |

| Function | Does |
|---|---|
| `newWorkspaceState()` | Inits workspace with default panel visibility |
| `(s) handleWorkspaceEvents(gtx, w, terminalEvents)` | Handles activity bar clicks, terminal buttons/tabs, chat send, updates splitters |
| `(w) handleSplitters(gtx)` | Calls `update` on all three splitters |
| `(d *paneSplitter) update(gtx, size, direction)` | Reads drag events, recalculates panel width |
| `(d *verticalPaneSplitter) update(gtx, size)` | Reads drag events, recalculates terminal height |
| `(w) normalize(totalWidth, splitterWidth, activityWidth)` | Clamps panel widths, prevents going below minimum |
| `layoutWorkspace(gtx, th, state, editor)` | Lays out horizontal row (activity + explorer + editor + chat + activity) and optional terminal |
| `layoutActivityBar(gtx, th, items)` | Renders vertical icon button strip |
| `layoutActivityButton(gtx, th, btn, active, label)` | One activity bar button with active/hover styles |
| `layoutPaneSplitter(gtx, splitter)` | Vertical divider strip with `ColResize` cursor |
| `layoutHorizontalSplitter(gtx, splitter)` | Horizontal divider strip with `RowResize` cursor |
| `layoutChatPanel(gtx, th, state)` | Chat panel: header, message list, input field + Send button |
| `layoutChatMessage(gtx, th, msg)` | One chat message in a frame |
| `layoutChatComposer(gtx, th, state)` | Chat input row with Send button |
| `layoutStatusBar(gtx, th, state)` | Bottom status bar: project, file, panel sizes |
| `clampInt(v, lo, hi)` | Clamps `int` to `[lo, hi]` |

---

### [central_content.go](central_content.go)
Central area with document tabs and editor.

| Type | Holds |
|---|---|
| `openDocumentState` | Open document: path, content, tab button, close button |

| Function | Does |
|---|---|
| `(s) handleCentralContentEvents(gtx, editor)` | Handles `CloseButton` and `TabButton` clicks per tab |
| `layoutCentralContent(gtx, th, editor, state)` | Renders tab bar + editor |
| `layoutDocumentTabsBar(gtx, th, state)` | Tab strip; placeholder if no documents |
| `layoutDocumentTab(gtx, th, active, doc, label)` | One tab: clickable name + × close button, active/hover styles |
| `layoutCurrentDocumentPath(gtx, th, state)` | (helper) Shows selected file path |

---

### [preferences.go](preferences.go)
Settings dialog (floating overlay, draggable).

| Type | Holds |
|---|---|
| `preferencesDraft` | Draft values before OK: `TabWidthSpaces` |
| `preferencesPageState` | One settings page: ID, group, title, description, select button |
| `intPreferenceOption` | Int choice option (e.g. tab width): label, value, button |
| `floatPreferenceOption` | Float choice option (e.g. line height): label, value, button |
| `preferencesDialogState` | Full dialog state: open flag, search, menu list, current page, options, draft, position, drag |
| `preferenceGroupView` | Page group for display in menu (render-only) |

| Function | Does |
|---|---|
| `newPreferencesDialogState()` | Inits dialog with three pages and options |
| `(s) openPreferences()` | Opens dialog, resets position and draft |
| `(s) handlePreferencesEvents(gtx, editor)` | Handles page/option clicks, OK/Cancel; applies and saves settings |
| `(p) handleDrag(gtx, size)` | Reads drag events from dialog title bar and moves position |
| `(p) filteredGroups()` | Returns page groups filtered by search query |
| `layoutPreferencesOverlay(gtx, th, state)` | Renders semi-transparent overlay + dialog |
| `preferencesDialogSize(gtx)` | Calculates dialog size based on window size |
| `(p) clampPosition(viewport, size)` | Keeps dialog on screen |
| `layoutPreferencesTitleBar(gtx, th, state)` | Dialog title bar with drag zone |
| `layoutPreferencesMenu(gtx, th, state)` | Left column: search + page list |
| `layoutPreferencesMenuItem(gtx, th, selected, btn, label)` | One settings menu item |
| `layoutPreferencesContent(gtx, th, state)` | Right area — shows current page |
| `layoutPreferencesIndentationPage(...)` | "Editor / Indentation" page: tab width selector |
| `layoutPreferencesAppearancePage(...)` | "Editor / Appearance" page: shows config path |
| `layoutPreferencesFilesPage(...)` | "Files / General" page: info text |
| `layoutPreferenceOptionRow(...)` | Horizontal row of option buttons for `int` settings |
| `layoutFloatPreferenceOptionRow(...)` | Horizontal row of option buttons for `float` settings |
| `layoutIntPreferenceOption(...)` | One option button with selected/hover styles |
| `layoutPreferencesButtons(gtx, th, state)` | Cancel and OK buttons in footer |
| `layoutDialogButton(gtx, th, btn, label, primary)` | Generic dialog button (primary = orange) |

---

### [appearance.go](appearance.go)
Display settings (font sizes and spacing) and label helpers.

| Type | Holds |
|---|---|
| `textMetrics` | Font size and line height for one widget |
| `appearanceSettings` | Six font-size/line-height pairs for explorer, terminal, editor (JSON) |
| `appearancePreferenceDescriptor` | Key and description of one appearance setting |

| Function | Does |
|---|---|
| `defaultAppearanceSettings()` | Returns default values for all metrics |
| `defaultAppearancePreferenceDescriptors()` | Returns list of keys and descriptions for all settings |
| `normalizeAppearanceSettings(settings)` | Fills zero fields with defaults, clamps to valid ranges, rounds to step 0.5 |
| `(a) explorerMetrics()` | Returns `textMetrics` for explorer |
| `(a) terminalMetrics()` | Returns `textMetrics` for terminal |
| `(a) editorMetrics()` | Returns `textMetrics` for editor |
| `(a) valueForKey(key)` | Returns setting value by string key |
| `(a) setValueForKey(key, value)` | Sets setting value by key; returns `false` if key unknown |
| `formatPreferenceValue(value)` | Formats float as `"XX.X"` string |
| `parsePreferenceValue(text)` | Parses string to float, normalizes separator |
| `roundPreferenceValue(value)` | Rounds to nearest 0.5 step |
| `clampFloat32(value, lo, hi)` | Clamps `float32` to range |
| `labelWithMetrics(th, text, metrics)` | Creates `material.LabelStyle` with given font size and line height |
| `monoLabelWithMetrics(th, text, metrics)` | Same but monospace font |
| `textBlockHeightPx(gtx, metrics, paddingDp, minHeightDp)` | Calculates text block height in pixels |

---

### [terminal.go](terminal.go)
Embedded terminal: state, tabs, VT emulator, keyboard input, rendering.

| Type | Holds |
|---|---|
| `terminalState` | Visibility, height, active tab, ID counter, buttons (toggle/minimize/add), splitter, tab array |
| `terminalTabState` | One tab: ID, title, buttons, scroll list, focus tag, start dir, session, buffer, debug log, scroll, flags |
| `terminalFocusTag` | Empty struct — unique tag for keyboard focus |
| `terminalProcessEvent` | Event from background process: TabID, data, close flag, error |
| `terminalBuffer` | VT buffer: rune matrix, cursor pos, saved pos, partial UTF-8, escape mode |
| `terminalEscapeMode` | Enum of escape parser modes (None/Esc/CSI/OSC/OSCMayEnd) |

| Function | Does |
|---|---|
| `newTerminalState()` | Initial terminal state (height=220, activeTab=-1) |
| `(t) normalize(totalHeight, splitterHeight)` | Clamps terminal height to valid range |
| `(t) show(currentDir, win, terminalEvents)` | Shows terminal, opens first tab or restores active |
| `(t) collapse()` | Hides terminal (does not close session) |
| `(t) closeAll()` | Hides and stops all tabs |
| `(t) activeTabState()` | Returns `*terminalTabState` of active tab or nil |
| `(t) openTab(currentDir, win, terminalEvents)` | Creates new tab and starts session in it |
| `newTerminalTabState(id, startDir)` | Inits `terminalTabState` with reset buffer |
| `(tab) appendDebug(message)` | Appends line to tab debug log (and stderr), capped at 80 entries |
| `(t) closeTab(index)` | Stops session, removes tab, updates activeTab |
| `(t) ensureSession(tab, win, terminalEvents)` | Starts `terminalSession` if missing, starts read/wait goroutines |
| `(t) stopSession(tab)` | Sends `Stop()` to session, clears pointer |
| `(t) applyProcessEvent(event)` | Applies background process data to buffer; handles close |
| `(t) findTabByID(id)` | Finds tab by ID |
| `(t) handleInput(gtx)` | Reads keyboard and pointer events, sends bytes to session |
| `terminalKeyBytes(ev)` | Converts `key.Event` to terminal escape sequence |
| `(b) reset()` | Resets VT buffer to initial state |
| `(b) ensureCursorLine()` | Adds lines up to cursor position |
| `(b) Process(data)` | Main VT parser: UTF-8 + escape (CSI/OSC/ESC) → calls `putRune`/`handleControl`/`handleCSI` |
| `(b) handleControl(ch)` | Handles control chars: CR, LF, BS, TAB, NUL, BEL |
| `(b) putRune(r)` | Inserts rune at cursor position, advances cursor |
| `(b) handleCSI(seq)` | Parses CSI sequences: cursor movement, erase, colors, etc. |

_(also contains `layoutTerminalPanel` and terminal rendering helpers)_

---

### [terminal_conpty_windows.go](terminal_conpty_windows.go)
Windows terminal session via ConPTY API. Build tag: `GOOS=windows`.

| Type | Holds |
|---|---|
| `terminalSession` | ConPTY handle (`hpc`), process, thread, I/O files, mutex, `sync.Once` |

| Function | Does |
|---|---|
| `startTerminalSession(currentDir, cols, rows)` | Creates ConPTY, launches `cmd.exe /Q /D` in start dir, returns session |
| `makeTerminalPipePair()` | Creates pair of Windows pipes as `*os.File` |
| `(s) Write(data)` | Writes bytes to input pipe under mutex |
| `(s) Resize(cols, rows)` | Calls `ResizePseudoConsole` to resize |
| `(s) Stop()` | Sends `exit\r` to shell and calls `TerminateProcess` |
| `(s) readOutput(tabID, terminalEvents, win)` | Goroutine: reads ConPTY output, sends events to channel |
| `(s) wait(tabID, terminalEvents, win)` | Goroutine: waits for process exit, closes session, sends event |
| `(s) Close()` | Closes all handles exactly once via `sync.Once` |

---

### [fonts_config.go](fonts_config.go)
Load/save `~/.masik/fonts.json` with font/spacing settings.

| Function | Does |
|---|---|
| `defaultFontsConfigPath(appRoot)` | Path `~/.masik/fonts.json` |
| `loadFontsConfig(path)` | Reads file, creates with defaults if missing, normalizes |
| `saveFontsConfig(path, cfg)` | Normalizes and writes `appearanceSettings` to JSON |

---

### [folderdialog.go](folderdialog.go)
Opens system folder picker via PowerShell (Windows).

| Type | Holds |
|---|---|
| `folderPickResult` | Folder pick result: `dir` path and `err` |

| Function | Does |
|---|---|
| `chooseFolder(initialDir)` | Runs PowerShell with `OpenFileDialog`, returns chosen path |
| `escapePowerShellString(s)` | Escapes single quotes for PowerShell string |
| `folderDisplayName(path)` | Returns `basename` of path for display in title |

---

### [window_caption.go](window_caption.go)
Reusable window caption component (not used in current render, but available).

| Type | Holds |
|---|---|
| `windowCaptionDecorator` | `func(layout.Context, layout.Widget) layout.Dimensions` — widget decorator |
| `WindowCaption` | Caption config: height, background, border, icon, menu, title, buttons |

| Function | Does |
|---|---|
| `layoutWindowCaption(gtx, caption)` | Renders caption: icon → menu → flexible title zone → right buttons |
| `applyWindowCaptionDecorator(gtx, decorator, w)` | Applies decorator or renders widget directly |
| `hasWindowCaptionContent(menus, title, rightButtons)` | Checks if there is anything besides icon |

---

### [resources.go](resources.go)
App logo embedded via `go:embed`.

| Function | Does |
|---|---|
| `init()` | Decodes `logo.png`, removes white background, creates `paint.ImageOp` |
| `layoutAppLogo(gtx)` | Renders logo 28×28 dp |
| `removeWhiteLogoBackground(src)` | BFS from edges: makes white pixels transparent |
| `isBackgroundWhite(c)` | Pixel is "white" if deviation from white ≤ 56 |
| `knockOutWhite(c)` | Makes pixel transparent or semi-transparent based on proximity to white |
| `whitenessDeviation(c)` | Max channel deviation from 255 |
| `unblendFromWhite(c, alpha)` | Recovers original color of pixel blended with white |
| `recoverChannelFromWhite(v, alpha)` | Reverse alpha-blend for one channel |
| `max3(a, b, c)` | Max of three ints |

---

## Package `coloreditor`

Custom multi-line editor on Gio with per-line coloring. Fork of standard `widget.Editor`.

### [coloreditor/buffer.go](coloreditor/buffer.go)
Gap buffer for text storage and editing.

| Type | Holds |
|---|---|
| `editBuffer` | Gap buffer: byte array `text`, positions `gapstart`/`gapend`, changed flag |

| Method | Does |
|---|---|
| `Changed()` | Returns and resets changed flag |
| `deleteRunes(caret, count)` | Deletes `count` runes forward (>0) or backward (<0) from `caret` |
| `moveGap(caret, space)` | Moves gap to `caret`, grows if not enough space |
| `Size()` | Returns byte count of text without gap |
| `gapLen()` | Returns gap length in bytes |
| `ReadAt(p, offset)` | Implements `io.ReaderAt`: reads around gap |
| `ReplaceRunes(byteOffset, runeCount, s)` | Deletes `runeCount` runes and inserts string `s` |
| `prepend(caret, s)` | Inserts string `s` at position `caret` |

---

### [coloreditor/editor.go](coloreditor/editor.go)
Main editor widget: events, rendering, undo/redo, clipboard.

| Type | Holds |
|---|---|
| `Editor` | Public widget: buffer, metrics, wrap policy, tab width, `LineColor`, undo history, IME, drag/scroll/click gestures |
| `offEntry` | Cache entry: rune index → byte offset |
| `imeState` | IME state: composition range, snippet |
| `maskReader` | `io.Reader` wrapper replacing runes with mask (password field) |
| `EditorEvent` | Editor events interface |
| `ChangeEvent` | Text changed |
| `SubmitEvent` | Enter pressed (Submit mode) |
| `SelectEvent` | Selection changed |
| `modification` | Undo record: position, deleted and inserted text |

| Method | Does |
|---|---|
| `processEvents(gtx)` | Dispatcher: calls pointer and key handlers |
| `processPointer(gtx)` | Registers pointer/click filters, calls `processPointerEvent` |
| `processPointerEvent(gtx, ev)` | Handles click/drag: positions caret, selection, focus |
| `processKey(gtx)` | Registers key filters, handles IME, calls `command` |
| `command(gtx, k)` | Key dispatcher: arrows, Home/End, Ctrl+C/V/X/Z/Y, etc. |
| `initBuffer()` | Lazy init of `editBuffer` and `textView` |
| `Update(gtx)` | Updates state without rendering, returns event |
| `Layout(gtx, lt, font, size, textMaterial, selectMaterial)` | Full render: scroll, selection, text, caret |
| `updateSnippet(gtx, start, end)` | Updates IME snippet |
| `layout(gtx, textMaterial, selectMaterial)` | Internal layout with clip, scroll, and coloring |
| `paintSelection(gtx, material)` | Draws selection |
| `paintText(gtx, material)` | Draws text |
| `paintCaret(gtx, material)` | Draws blinking caret |
| `Len()` | Rune count |
| `Text()` | Returns text as `string` |
| `SetText(s)` | Sets text, resets history |
| `CaretPos()` | Caret line and column |
| `CaretCoords()` | Caret pixel coordinates |
| `Delete(graphemeClusters)` | Deletes grapheme clusters |
| `Insert(s)` | Inserts string at caret |
| `undo()` | Undoes last action |
| `redo()` | Redoes undone action |
| `replace(start, end, s, addHistory)` | Replaces rune range, optionally records history |
| `normalizeInsertedText(s)` | Normalizes line endings, applies `Filter`, expands tabs |
| `MoveCaret(startDelta, endDelta)` | Moves caret by `N` graphemes |
| `deleteWord(distance)` | Deletes word forward or backward |
| `SelectionLen()` | Selection length in runes |
| `Selection()` | Selection start and end |
| `SetCaret(start, end)` | Sets caret/selection position |
| `SelectedText()` | Returns selected text |
| `ClearSelection()` | Clears selection |
| `WriteTo(w)` | Implements `io.WriterTo` |
| `Seek(offset, whence)` | Implements `io.Seeker` |
| `Read(p)` | Implements `io.Reader` |
| `Regions(start, end, regions)` | Returns pixel rects for rune range |

---

### [coloreditor/text.go](coloreditor/text.go)
`textView` — text shaping engine: layout, coloring, cursor navigation.

| Type | Holds |
|---|---|
| `textSource` | Text source interface: `Size`, `ReadAt`, `Changed` |
| `textView` | Shaping cache, scroll position, caret/selection (start/end as rune indices), offset cache, grapheme reader |

| Method | Does |
|---|---|
| `Changed()` | Checks if source changed |
| `Dimensions()` / `FullDimensions()` | Visible / full text size |
| `SetSource(source)` | Binds text source |
| `makeValid()` | Recalculates shaping if needed |
| `Layout(gtx, lt, font, size)` | Runs shaping via Gio text shaper |
| `PaintText(gtx, material)` | Draws all text in one color |
| `PaintTextColored(gtx, lineMaterial)` | Draws text with per-line color (used for `LineColor`) |
| `PaintSelection(gtx, material)` | Draws selection background |
| `PaintCaret(gtx, material)` | Draws caret |
| `Replace(start, end, s)` | Replaces rune range in source |
| `MoveCaret(startDelta, endDelta)` | Moves caret by `N` positions |
| `MoveLines(distance, selAct)` | Moves cursor `distance` lines up/down |
| `MoveLineStart/End(selAct)` | Moves to line start/end |
| `MoveTextStart/End(selAct)` | Moves to text start/end |
| `MoveWord(distance, selAct)` | Moves `distance` words |
| `MovePages(pages, selAct)` | Moves `pages` pages |
| `ScrollToCaret()` | Scrolls view to caret |
| `ScrollRel(dx, dy)` | Relative scroll |
| `Selection()` / `SetCaret()` | Get/set selection |
| `SelectedText(buf)` | Returns selected text |
| `CaretPos()` / `CaretCoords()` | Caret position in lines/pixels |
| `Len()` / `Text(buf)` | Buffer size and content |

---

### [coloreditor/index.go](coloreditor/index.go)
Glyph index for cursor navigation and selection region building.

| Type | Holds |
|---|---|
| `lineInfo` | One line metrics: x/y offsets, width, ascent/descent, glyph count |
| `glyphIndex` | Full index: glyphs, caret positions, lines, current indexing state |
| `screenPos` | Position as line and column |
| `combinedPos` | Full position: rune offset, screen pos, x/y pixels, ascent/descent, run index |
| `Region` | Text rectangular region with `Bounds` and `Baseline` |
| `graphemeReader` | Segments text paragraphs into grapheme clusters |

| Method | Does |
|---|---|
| `(g) reset()` | Clears index for reuse |
| `(g) Glyph(gl)` | Indexes one glyph, updates positions and line metrics |
| `(g) incrementPosition(pos)` | Returns next position after `pos` |
| `(g) insertPosition(pos)` | Adds position to index (with dedup) |
| `(g) closestToRune(runeIdx)` | Binary search for position closest to rune index |
| `(g) closestToLineCol(lineCol)` | Closest position to line/column |
| `(g) closestToXY(x, y)` | Closest position to pixel coordinates |
| `(g) locate(viewport, startRune, endRune, rects)` | Returns `[]Region` for rune range (for selection) |
| `(p) SetSource(source)` | Binds source for graphemeReader |
| `(p) Graphemes()` | Returns grapheme cluster boundaries of next paragraph |

---

### [coloreditor/style.go](coloreditor/style.go)
Editor styling: `Style` links `material.Theme` and `Editor`.

| Type | Holds |
|---|---|
| `Style` | Font, text size, text/hint/selection colors, pointer to `Editor`, shaper |

| Function | Does |
|---|---|
| `NewStyle(th, editor, hint)` | Creates `Style` from Gio theme |
| `(e Style) Layout(gtx)` | Renders hint if empty, then editor |
| `blendDisabledColor(disabled, c)` | Lightens color if widget is disabled |
| `mulAlpha(c, alpha)` | Multiplies color alpha channel |

---

### [coloreditor/textiter.go](coloreditor/textiter.go)
Glyph iterator for per-line text coloring.

| Type | Holds |
|---|---|
| `textIterator` | Viewport, current y position, painted lines, line color function |

| Method | Does |
|---|---|
| `processGlyph(g, ok)` | Checks glyph visibility in viewport, accumulates line glyphs |
| `fixedToFloat(i)` | Converts `fixed.Int26_6` to `float32` |
| `flushLine(gtx, shaper, line)` | Paints accumulated line glyphs with correct color |
| `paintGlyph(gtx, shaper, glyph, line)` | Paints one glyph, calls `flushLine` on line change |
