package commitmsg

import (
	"fmt"
	"strings"
	"testing"
)

// hunk builds a hunk with context lines, so the first rung of the drop ladder
// has something to give up.
func hunk(n int, tag string) string {
	return fmt.Sprintf("@@ -%d,3 +%d,4 @@\n context line %s one padded out\n+added line %s\n context line %s two padded out",
		n, n+2, tag, tag, tag)
}

func ladderDigest() *Digest {
	return &Digest{
		Branch: "feat/commit-messages",
		Prefix: "shellapp/",
		Files: []Change{
			{Path: "shellapp/ui/a.go", Status: 'M', Added: 10, Removed: 2,
				Hunks: []string{hunk(1, "a1"), hunk(20, "a2"), hunk(40, "a3")}},
			{Path: "web/b.ts", Status: 'M', Added: 3, Removed: 1,
				Hunks: []string{hunk(1, "b1")}},
		},
	}
}

// The ladder in order: context lines, then trailing hunks, then whole
// low-rank files, then everything but the file list.
func TestPackDropLadder(t *testing.T) {
	full := ladderDigest().Pack(100000)
	if !strings.Contains(full, "context line a1") {
		t.Fatalf("full pack is missing context lines:\n%s", full)
	}
	if fresh := ladderDigest(); fresh.Pack(100000) != full || fresh.Truncated {
		t.Error("a pack that fits must not set Truncated")
	}

	noContext := ladderDigest()
	lvl1 := noContext.Pack(len(full) - 1)
	if strings.Contains(lvl1, "context line") {
		t.Errorf("rung 1 should drop context lines first:\n%s", lvl1)
	}
	if !strings.Contains(lvl1, "added line a3") || !strings.Contains(lvl1, "--- web/b.ts") {
		t.Errorf("rung 1 dropped more than context lines:\n%s", lvl1)
	}
	if !noContext.Truncated {
		t.Error("Truncated was not set after degrading")
	}

	lvl2 := ladderDigest().Pack(len(lvl1) - 1)
	if strings.Contains(lvl2, "added line a2") || strings.Contains(lvl2, "added line a3") {
		t.Errorf("rung 2 should drop trailing hunks:\n%s", lvl2)
	}
	if !strings.Contains(lvl2, "added line a1") || !strings.Contains(lvl2, "--- web/b.ts") {
		t.Errorf("rung 2 dropped whole files too early:\n%s", lvl2)
	}

	lvl3 := ladderDigest().Pack(len(lvl2) - 1)
	if strings.Contains(lvl3, "--- web/b.ts") {
		t.Errorf("rung 3 should drop the lowest-ranked file:\n%s", lvl3)
	}
	if !strings.Contains(lvl3, "--- shellapp/ui/a.go") {
		t.Errorf("rung 3 dropped the highest-ranked file:\n%s", lvl3)
	}

	lvl4 := ladderDigest().Pack(len(lvl3) - 1)
	if strings.Contains(lvl4, "\n--- ") {
		t.Errorf("rung 4 should keep the file list only:\n%s", lvl4)
	}
	for _, want := range []string{"branch: feat/commit-messages", "M shellapp/ui/a.go (+10 -2)", "M web/b.ts (+3 -1)"} {
		if !strings.Contains(lvl4, want) {
			t.Errorf("the file list must always survive, missing %q:\n%s", want, lvl4)
		}
	}
}

func TestPackStaysUnderBudget(t *testing.T) {
	var files []Change
	for i := 0; i < 500; i++ {
		files = append(files, Change{
			Path:   fmt.Sprintf("shellapp/internal/pkg%03d/file%03d.go", i, i),
			Status: 'M', Added: 10 + i, Removed: 2,
			Hunks: []string{hunk(1, fmt.Sprintf("f%03d", i)), hunk(30, fmt.Sprintf("g%03d", i))},
		})
	}
	for _, budget := range []int{200, 1000, 4000, DefaultBudget} {
		d := &Digest{Branch: "main", Prefix: "shellapp/", Files: files}
		got := d.Pack(budget)
		if len(got) > budget {
			t.Errorf("Pack(%d) produced %d bytes", budget, len(got))
		}
		if !d.Truncated {
			t.Errorf("Pack(%d) did not set Truncated", budget)
		}
		if !strings.Contains(got, "staged files (500)") {
			t.Errorf("Pack(%d) lost the file count:\n%s", budget, got)
		}
	}
}

// Generated and vendored files are listed, but they are the last to have
// their contents packed.
func TestPackDemotesGeneratedFiles(t *testing.T) {
	for _, path := range []string{"web/package-lock.json", "web/node_modules/dep/index.js", "shellapp/go.sum", "web/dist/bundle.min.js"} {
		d := &Digest{
			Prefix: "shellapp/",
			Files: []Change{
				{Path: path, Status: 'M', Added: 4000, Removed: 900, Hunks: []string{hunk(1, "lock")}},
				{Path: "shellapp/ui/view.go", Status: 'M', Added: 5, Removed: 1, Hunks: []string{hunk(1, "src")}},
			},
		}
		full := d.Pack(100000)
		if strings.Index(full, "--- shellapp/ui/view.go") > strings.Index(full, "--- "+path) {
			t.Errorf("%s outranked the source file:\n%s", path, full)
		}
		if !strings.Contains(full, path) {
			t.Errorf("%s should still be listed by name:\n%s", path, full)
		}
	}
}

// Files under the working root rank first: this monorepo's staged diff is
// repo-wide, so pressing c in shellapp must not pack the web app's churn.
func TestPackRanksPrefixFilesFirst(t *testing.T) {
	d := &Digest{
		Prefix: "shellapp/",
		Files: []Change{
			{Path: "web/app.ts", Status: 'M', Added: 100, Removed: 40, Hunks: []string{hunk(1, "web")}},
			{Path: "shellapp/ui/view.go", Status: 'M', Added: 5, Removed: 1, Hunks: []string{hunk(1, "go")}},
		},
	}
	got := d.Pack(100000)
	if strings.Index(got, "--- shellapp/ui/view.go") > strings.Index(got, "--- web/app.ts") {
		t.Errorf("a file under Prefix should be packed first:\n%s", got)
	}
}

// Localhost-only today, but this is the exact path that would ship a diff to a
// remote provider the day one is added.
func TestPackNeverIncludesSecretContents(t *testing.T) {
	d := &Digest{Files: []Change{
		{Path: "shellapp/.env", Status: 'M', Added: 1, Hunks: []string{"@@ -0,0 +1 @@\n+API_TOKEN=hunter2"}},
		{Path: ".env.local", Status: 'A', Added: 1, Hunks: []string{"@@ -0,0 +1 @@\n+DB_PASSWORD=hunter2"}},
		{Path: "certs/server.pem", Status: 'A', Added: 1, Hunks: []string{"@@ -0,0 +1 @@\n+BEGIN-PRIVATE-hunter2"}},
		{Path: "certs/tls.key", Status: 'A', Added: 1, Hunks: []string{"@@ -0,0 +1 @@\n+key-hunter2"}},
		{Path: "deploy/id_rsa", Status: 'A', Added: 1, Hunks: []string{"@@ -0,0 +1 @@\n+rsa-hunter2"}},
		{Path: "config/aws-credentials.json", Status: 'M', Added: 1, Hunks: []string{"@@ -0,0 +1 @@\n+creds-hunter2"}},
	}}
	got := d.Pack(100000)
	if strings.Contains(got, "hunter2") {
		t.Fatalf("a secret's contents reached the packed prompt:\n%s", got)
	}
	for _, name := range []string{"shellapp/.env", "certs/server.pem", "deploy/id_rsa"} {
		if !strings.Contains(got, name) {
			t.Errorf("%s should still be listed by name:\n%s", name, got)
		}
	}
	if !strings.Contains(got, "contents withheld") {
		t.Errorf("the withholding should be stated, not silent:\n%s", got)
	}
}

func TestPackRendersBinariesWithoutContent(t *testing.T) {
	d := &Digest{Files: []Change{
		{Path: "assets/logo.png", Status: 'A', Binary: true, Hunks: []string{"@@\n+binarygarbage"}},
	}}
	got := d.Pack(100000)
	if strings.Contains(got, "binarygarbage") {
		t.Fatalf("binary content was packed:\n%s", got)
	}
	if !strings.Contains(got, "A assets/logo.png (binary)") {
		t.Errorf("binary not rendered as a counts-only entry:\n%s", got)
	}
}

func TestPackRendersRenamesAndCounts(t *testing.T) {
	d := &Digest{Files: []Change{
		{Path: "internal/gitx/gitx.go", OldPath: "internal/git/git.go", Status: 'R', Added: 3, Removed: 1},
		{Path: "ui/old.go", Status: 'D', Removed: 40},
	}}
	got := d.Pack(100000)
	for _, want := range []string{
		"R internal/git/git.go -> internal/gitx/gitx.go (+3 -1)",
		"D ui/old.go (+0 -40)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

func TestIsSecret(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".env", true},
		{"shellapp/.env.local", true},
		{"certs/server.pem", true},
		{"certs/tls.key", true},
		{"deploy/id_rsa", true},
		{"deploy/id_rsa.pub", true},
		{"config/aws-credentials.json", true},
		{"internal/envfile/envfile.go", false},
		{"ui/theme/theme.go", false},
		{"docs/environment.md", false},
	}
	for _, tt := range tests {
		if got := IsSecret(tt.path); got != tt.want {
			t.Errorf("IsSecret(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
