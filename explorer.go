package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"mycharm/coloreditor"
)

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

func newExplorerState() explorerState {
	var s explorerState
	s.list.Axis = layout.Vertical
	return s
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

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

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
	metrics := state.appearance.explorerMetrics()
	return layoutWithSideBorders(gtx,
		color.NRGBA{R: 0xFB, G: 0xF8, B: 0xF2, A: 0xFF},
		color.NRGBA{R: 0xD0, G: 0xD7, B: 0xDE, A: 0xFF},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := labelWithMetrics(th, "Explorer", metrics)
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
	metrics := state.appearance.explorerMetrics()
	return node.Row.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		height := textBlockHeightPx(gtx, metrics, 10, 30)
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
				label := labelWithMetrics(th, explorerLabel(node), metrics)
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