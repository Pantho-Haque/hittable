package explorer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIconResolvedAtBuildTime(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hello"), 0644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644)

	root, err := BuildTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, child := range root.Children {
		if child.Kind == KindDir {
			continue
		}
		if child.Icon == "" {
			t.Errorf("node %s has empty icon - should be resolved at build time", child.Name)
		}
	}
}

func TestIconNotResolvedPerFrame(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main"), 0644)

	root, err := BuildTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	icon1 := Icon(root.Children[0])
	icon2 := Icon(root.Children[0])
	if icon1 != icon2 {
		t.Fatalf("icons should be consistent: %q vs %q", icon1, icon2)
	}
}

func TestIconForCustomTypes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "api.hit"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "env.json"), []byte("{}"), 0644)

	root, err := BuildTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, child := range root.Children {
		if child.Name == "api.hit" && child.Icon != "⚡" {
			t.Errorf("expected ⚡ for .hit files, got %q", child.Icon)
		}
		if child.Name == "env.json" && child.Icon != "🔧" {
			t.Errorf("expected 🔧 for env.json, got %q", child.Icon)
		}
	}
}
