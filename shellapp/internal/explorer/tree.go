package explorer

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/epilande/go-devicons"
)

type FileKind int

const (
	KindDir FileKind = iota
	KindHit
	KindEnv
	KindMarkdown
	KindJson
	KindGeneric
)

type FileNode struct {
	Name     string
	Path     string
	Kind     FileKind
	Expanded bool
	Children []*FileNode
	Parent   *FileNode
	Icon     string
}

func BuildTree(root string) (*FileNode, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	rootNode := &FileNode{
		Name:     info.Name(),
		Path:     root,
		Kind:     KindDir,
		Expanded: true,
	}
	if err := buildChildren(rootNode, root); err != nil {
		return nil, err
	}
	return rootNode, nil
}

func buildChildren(parent *FileNode, dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		di := entries[i].IsDir()
		dj := entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})
	parent.Children = make([]*FileNode, 0, len(entries))
	for _, entry := range entries {
		childPath := filepath.Join(dirPath, entry.Name())
		node := &FileNode{
			Name:   entry.Name(),
			Path:   childPath,
			Parent: parent,
		}
		if entry.IsDir() {
			node.Kind = KindDir
			node.Expanded = false
			if err := buildChildren(node, childPath); err != nil {
				return err
			}
		} else {
			node.Kind = classifyFile(entry.Name())
			node.Icon = resolveIcon(node, entry)
		}
		parent.Children = append(parent.Children, node)
	}
	return nil
}

func classifyFile(name string) FileKind {
	ext := filepath.Ext(name)
	switch ext {
	case ".hit":
		return KindHit
	case ".md":
		return KindMarkdown
	case ".json":
		return KindJson
	}
	if name == "env.json" {
		return KindEnv
	}
	return KindGeneric
}

func resolveIcon(node *FileNode, entry os.DirEntry) string {
	customIcons := map[string]string{
		".hit": "⚡",
	}
	ext := filepath.Ext(node.Name)
	if icon, ok := customIcons[ext]; ok {
		return icon
	}
	if node.Name == "env.json" {
		return "🔧"
	}
	info, err := entry.Info()
	if err == nil && info != nil {
		style := devicons.IconForInfo(info)
		if style.Icon != "" {
			return style.Icon
		}
	}
	switch node.Kind {
	case KindMarkdown:
		return "📝"
	case KindJson:
		return "📋"
	default:
		return "📄"
	}
}

func VisibleNodes(root *FileNode) []*FileNode {
	var nodes []*FileNode
	var walk func(node *FileNode)
	walk = func(node *FileNode) {
		if node != root {
			nodes = append(nodes, node)
		}
		if node.Kind == KindDir && node.Expanded {
			for _, child := range node.Children {
				walk(child)
			}
		}
	}
	walk(root)
	return nodes
}

func Icon(node *FileNode) string {
	if node.Kind == KindDir {
		if node.Expanded {
			return "📂"
		}
		return "📁"
	}
	if node.Icon != "" {
		return node.Icon
	}
	return "📄"
}

func Indent(node *FileNode) int {
	depth := 0
	p := node.Parent
	for p != nil && p.Parent != nil {
		depth++
		p = p.Parent
	}
	return depth
}

func FindNode(root *FileNode, path string) *FileNode {
	if root.Path == path {
		return root
	}
	for _, child := range root.Children {
		if found := FindNode(child, path); found != nil {
			return found
		}
	}
	return nil
}

func RemoveNode(node *FileNode) {
	if node.Parent == nil {
		return
	}
	parent := node.Parent
	for i, child := range parent.Children {
		if child == node {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}

func AddChild(parent *FileNode, child *FileNode) {
	child.Parent = parent
	parent.Children = append(parent.Children, child)
}
