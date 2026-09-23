package commitmsg

import (
	"strings"
	"testing"
)

func digest(branch string, files ...Change) *Digest {
	return &Digest{Branch: branch, Files: files, Convention: true}
}

func mod(path string, added, removed int) Change {
	return Change{Path: path, Status: 'M', Added: added, Removed: removed}
}

func add(path string, added int) Change {
	return Change{Path: path, Status: 'A', Added: added}
}

func del(path string, removed int) Change {
	return Change{Path: path, Status: 'D', Removed: removed}
}

// Every type rule from the generation table, in the order it fires.
func TestInferType(t *testing.T) {
	tests := []struct {
		name string
		d    *Digest
		want string
	}{
		{"tests only", digest("", mod("internal/x/x_test.go", 20, 0), add("internal/x/y_test.go", 30)), "test"},
		{"js specs", digest("", add("src/app.spec.ts", 10)), "test"},
		{"docs only", digest("", mod("README.md", 5, 1), mod("docs/plan.md", 9, 0)), "docs"},
		{"license counts as docs", digest("", mod("LICENSE", 1, 1)), "docs"},
		{"styles only", digest("", mod("web/app.css", 4, 2)), "style"},
		{"build files", digest("", mod("go.mod", 1, 0), mod("go.sum", 3, 0)), "build"},
		{"lockfile is build", digest("", mod("web/package-lock.json", 40, 2)), "build"},
		{"workflows", digest("", mod(".github/workflows/ci.yml", 8, 1)), "ci"},
		{"fix branch wins over a plain edit", digest("fix/panel-scroll", mod("ui/view.go", 3, 3)), "fix"},
		{"hotfix branch", digest("hotfix/crash", mod("ui/view.go", 3, 3)), "fix"},
		{"added source file", digest("main", add("internal/commitmsg/collect.go", 200), mod("ui/view.go", 2, 0)), "feat"},
		{"deletions only", digest("main", del("ui/old.go", 40), del("ui/older.go", 12)), "refactor"},
		{"plain edit", digest("main", mod("ui/screens/view.go", 6, 2)), "feat"},
		{"nothing staged", digest("main"), "chore"},
		{"a fix branch never overrides an all-docs change", digest("fix/typo", mod("docs/plan.md", 1, 1)), "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferType(tt.d); got != tt.want {
				t.Errorf("InferType() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Every subject rule, in the order it fires.
func TestGenerateSubject(t *testing.T) {
	folded := Change{
		Path: "ui/components/texteditor/fold.go", Status: 'M', Added: 412, Removed: 8,
		Hunks: []string{"@@ -1,2 +1,6 @@\n+func (e *TextEditor) FoldAll() {\n+func (e *TextEditor) UnfoldAll() {"},
	}

	tests := []struct {
		name string
		d    *Digest
		want string
	}{
		{
			"rename",
			digest("main", Change{Path: "internal/gitx/gitx.go", OldPath: "internal/git/git.go", Status: 'R', Added: 1, Removed: 1}),
			"rename git.go to gitx.go",
		},
		{
			"several renames are counted",
			digest("main",
				Change{Path: "a/one.go", OldPath: "a/1.go", Status: 'R'},
				Change{Path: "a/two.go", OldPath: "a/2.go", Status: 'R'}),
			"rename 2 files",
		},
		{"single added file", digest("main", add("internal/commitmsg/collect.go", 210)), "add collect.go"},
		{"deletions only", digest("main", del("ui/old.go", 40), del("ui/older.go", 12)), "remove old.go and older.go"},
		{
			"many deletions are summarised",
			digest("main", del("a/one.go", 1), del("a/two.go", 1), del("a/three.go", 1), del("a/four.go", 1)),
			"remove one.go, two.go, three.go and 1 more",
		},
		{"new exported symbols", digest("main", folded, mod("ui/screens/view.go", 96, 31)), "add fold all and unfold all"},
		{"tests only", digest("main", mod("internal/envfile/envfile_test.go", 30, 0), add("internal/envfile/more_test.go", 10)), "add tests for envfile"},
		{"fallback", digest("main", mod("ui/screens/view.go", 6, 2), mod("ui/screens/update.go", 3, 1)), "update screens"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Generate(tt.d, Options{})
			if got.Subject != tt.want {
				t.Errorf("subject = %q, want %q", got.Subject, tt.want)
			}
			if got.Source != SourceHeuristic {
				t.Errorf("Source = %v, want SourceHeuristic", got.Source)
			}
			if err := Validate(got.String(), DefaultRules()); err != nil {
				t.Errorf("generated message does not validate: %v\n%s", err, got)
			}
		})
	}
}

// A change is never named after its tests. TestComposeNothingStaged is
// exported because the toolchain requires it, not because it is the point of
// the commit.
func TestGenerateNeverNamesAChangeAfterATestFunction(t *testing.T) {
	d := digest("main",
		Change{
			Path: "ui/components/gitpanel/compose.go", Status: 'A', Added: 420,
			Symbols: []string{"GenChunkMsg", "GenDoneMsg"},
		},
		Change{
			Path: "ui/components/gitpanel/compose_test.go", Status: 'A', Added: 203,
			Symbols: []string{"TestComposeNothingStaged", "TestComposeStreamsModelOutput"},
		},
	)
	m := Generate(d, Options{})
	if strings.Contains(m.Subject, "test") || strings.Contains(m.Subject, "compose nothing staged") {
		t.Errorf("subject was named after a test function: %q", m.Subject)
	}
	if m.Subject != "add gen chunk msg and gen done msg" {
		t.Errorf("subject = %q, want the first symbols of the highest-ranked non-test file", m.Subject)
	}
}

// The scan reads whole files, not the trimmed hunks: a newly added file is one
// hunk, and head-and-tail trimming would leave only the import block and the
// last declaration to choose from.
func TestExportedSymbolsSkipsTestFunctions(t *testing.T) {
	lines := []string{
		"+func TestCompose(t *testing.T) {",
		"+func BenchmarkCompose(b *testing.B) {",
		"+func FuzzCompose(f *testing.F) {",
		"+func ExampleCompose() {",
		"+func Helper() string {",
		"+type Harness struct {",
	}
	got := scanSymbols(lines, "ui/compose_test.go")
	want := []string{"Helper", "Harness"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("scanSymbols(test file) = %v, want %v", got, want)
	}
	// In a non-test file a type genuinely called Harness or TestServer stays.
	if got := scanSymbols([]string{"+type TestServer struct {"}, "internal/x/x.go"); len(got) != 1 || got[0] != "TestServer" {
		t.Errorf("scanSymbols(source file) = %v, want [TestServer]", got)
	}
}

// Symbols are visited best-ranked file first, declaration order within.
func TestNewSymbolsOrder(t *testing.T) {
	d := &Digest{
		Prefix: "shellapp/",
		Files: []Change{
			{Path: "web/small.ts", Status: 'M', Added: 2, Symbols: []string{"LowRank"}},
			{Path: "shellapp/big.go", Status: 'A', Added: 300, Symbols: []string{"First", "Second"}},
		},
	}
	got := d.newSymbols()
	want := []string{"first", "second", "low rank"}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("newSymbols() = %v, want %v", got, want)
		}
	}
}

// A tests-only change falls through to the "add tests for x" rule rather than
// naming itself after a helper.
func TestGenerateTestsOnlyChangeIgnoresSymbols(t *testing.T) {
	d := digest("main", Change{
		Path: "internal/envfile/envfile_test.go", Status: 'M', Added: 40,
		Symbols: []string{"Helper"},
	})
	if got := Generate(d, Options{}).Subject; got != "add tests for envfile" {
		t.Errorf("subject = %q, want %q", got, "add tests for envfile")
	}
}

func TestGenerateScope(t *testing.T) {
	files := []Change{
		mod("shellapp/ui/components/texteditor/fold.go", 40, 2),
		mod("shellapp/ui/components/texteditor/view.go", 8, 1),
	}

	tests := []struct {
		name string
		d    *Digest
		want string
	}{
		{
			name: "deepest meaningful segment when history is silent",
			d:    &Digest{Files: files, Convention: true},
			want: "texteditor",
		},
		{
			name: "a scope the history actually uses wins",
			d:    &Digest{Files: files, Convention: true, Scopes: []string{"shellapp", "workspace"}},
			want: "shellapp",
		},
		{
			name: "the most frequent matching scope wins",
			d:    &Digest{Files: files, Convention: true, Scopes: []string{"texteditor", "shellapp"}},
			want: "texteditor",
		},
		{
			name: "generic layer names lose to anything specific",
			d:    &Digest{Files: []Change{mod("internal/gitx/gitx.go", 5, 1)}, Convention: true},
			want: "gitx",
		},
		{
			name: "a path that only names a layer yields no scope",
			d:    &Digest{Files: []Change{mod("src/app.go", 2, 1), mod("src/util.go", 1, 1)}, Convention: true},
			want: "",
		},
		{
			name: "no common directory means no scope",
			d:    &Digest{Files: []Change{mod("README.md", 1, 1), mod("ui/view.go", 1, 1)}, Convention: true},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Generate(tt.d, Options{}).Scope; got != tt.want {
				t.Errorf("scope = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateHeaderIsCappedAtSeventyTwo(t *testing.T) {
	long := "internal/commitmsg/" + strings.Repeat("a-very-long-file-name-", 4) + ".go"
	m := Generate(digest("main", add(long, 10)), Options{})
	if len(m.Header()) > 72 {
		t.Errorf("header is %d chars: %q", len(m.Header()), m.Header())
	}
	if strings.HasSuffix(m.Header(), " ") {
		t.Errorf("header was not trimmed: %q", m.Header())
	}
	if err := Validate(m.String(), DefaultRules()); err != nil {
		t.Errorf("capped message does not validate: %v", err)
	}
}

// A directory name long enough to blow the header on its own must not take
// the whole message down with it.
func TestGenerateSurvivesAnAbsurdPath(t *testing.T) {
	dir := strings.Repeat("a-very-long-directory-name/", 4)
	m := Generate(digest("main", add(dir+strings.Repeat("name", 30)+".go", 10)), Options{})
	if got := len([]rune(m.Header())); got > 72 {
		t.Errorf("header is %d chars: %q", got, m.Header())
	}
	if m.Scope != "" {
		t.Errorf("scope = %q, want it dropped", m.Scope)
	}
	if err := Validate(m.String(), DefaultRules()); err != nil {
		t.Errorf("%q: %v", m.Header(), err)
	}
}

func TestGenerateBody(t *testing.T) {
	d := digest("main",
		mod("ui/components/texteditor/fold.go", 412, 8),
		mod("ui/components/texteditor/complete.go", 12, 3),
		mod("ui/screens/view.go", 96, 31),
		mod("main.go", 2, 0),
	)
	body := Generate(d, Options{}).Body
	want := []string{
		"- ui/components/texteditor: fold.go, complete.go (+424 −11)",
		"- ui/screens: view.go (+96 −31)",
		"- main.go (+2 −0)",
	}
	for _, line := range want {
		if !strings.Contains(body, line) {
			t.Errorf("body is missing %q:\n%s", line, body)
		}
	}
}

func TestGenerateBodyCapsFileNames(t *testing.T) {
	var files []Change
	for _, n := range []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go"} {
		files = append(files, mod("ui/"+n, 1, 1))
	}
	body := Generate(digest("main", files...), Options{}).Body
	if !strings.Contains(body, "a.go, b.go, c.go, d.go, …") {
		t.Errorf("basenames were not capped at four:\n%s", body)
	}
}

func TestGenerateHonoursTheForcedType(t *testing.T) {
	d := digest("main", add("internal/commitmsg/collect.go", 210))
	if got := Generate(d, Options{Type: "refactor"}).Type; got != "refactor" {
		t.Errorf("type = %q, want the picker's %q", got, "refactor")
	}
}

// A repository that does not follow conventional commits gets a plain
// imperative subject: the tool must not impose a convention the repo lacks.
func TestGenerateWithoutAConvention(t *testing.T) {
	d := &Digest{Branch: "main", Files: []Change{add("src/fold.go", 40)}}
	m := Generate(d, Options{})
	if m.Type != "" || m.Scope != "" {
		t.Errorf("non-conventional repo got a type prefix: %q", m.Header())
	}
	if m.Header() != "add fold.go" {
		t.Errorf("header = %q, want %q", m.Header(), "add fold.go")
	}
	// The picker still wins if the user asked for a type explicitly.
	if got := Generate(d, Options{Type: "feat"}).Header(); got != "feat: add fold.go" {
		t.Errorf("forced type header = %q", got)
	}
}

// Rules.RequireType = false must suppress the type everywhere, so a
// repository that does not use conventional commits never has one imposed on
// it — including when its own history happens to look conventional.
func TestPlainRulesNeverInventAType(t *testing.T) {
	rules := PlainRules()
	d := digest("main", add("src/greeting.go", 10))
	d.Convention = true

	if got := Generate(d, Options{Rules: rules}).Header(); got != "add greeting.go" {
		t.Errorf("Generate header = %q, want no type", got)
	}
	if got, ok := Repair("add greeting", rules, "feat"); !ok || got != "add greeting" {
		t.Errorf("Repair = %q, %v; want %q, true", got, ok, "add greeting")
	}
	if got, ok := Repair("Add greeting.", rules, ""); !ok || got != "add greeting" {
		t.Errorf("Repair = %q, %v; want %q, true", got, ok, "add greeting")
	}
	// The picker still wins: an explicit choice is not an invention.
	if got := Generate(d, Options{Type: "feat", Rules: rules}).Header(); got != "feat: add greeting.go" {
		t.Errorf("forced type header = %q", got)
	}
}

func TestGenerateWithNothingStaged(t *testing.T) {
	m := Generate(&Digest{Convention: true}, Options{})
	if m.Subject == "" {
		t.Error("Generate returned an empty subject")
	}
	if err := Validate(m.String(), DefaultRules()); err != nil {
		t.Errorf("empty digest produced an invalid message %q: %v", m, err)
	}
}

func TestMessageString(t *testing.T) {
	tests := []struct {
		m    Message
		want string
	}{
		{Message{Type: "feat", Scope: "git", Subject: "add the picker"}, "feat(git): add the picker"},
		{Message{Type: "feat", Subject: "add the picker"}, "feat: add the picker"},
		{Message{Subject: "add the picker"}, "add the picker"},
		{Message{Type: "fix", Subject: "stop the scroll", Body: "- ui: view.go (+1 −1)"},
			"fix: stop the scroll\n\n- ui: view.go (+1 −1)"},
	}
	for _, tt := range tests {
		if got := tt.m.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestSplitCamel(t *testing.T) {
	tests := []struct{ in, want string }{
		{"FoldAll", "fold all"},
		{"UnfoldAll", "unfold all"},
		{"Generate", "generate"},
		{"HTTPClient", "http client"},
		{"ParseV2", "parse v2"},
	}
	for _, tt := range tests {
		if got := splitCamel(tt.in); got != tt.want {
			t.Errorf("splitCamel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
