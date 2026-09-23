package commitmsg

import (
	"strings"
	"testing"
)

func specDigest() *Digest {
	return &Digest{
		Prefix:     "shellapp/",
		Convention: true,
		Files: []Change{
			{Path: "shellapp/internal/llmhost/install.go", Status: 'A', Added: 400,
				Doc:     "Package llmhost downloads, verifies, installs and supervises llama-server.",
				Symbols: []string{"Install", "Progress", "Supervisor", "Close", "Endpoint", "Installed", "Extra"}},
			{Path: "shellapp/internal/llmhost/artifacts.go", Status: 'A', Added: 60,
				Symbols: []string{"Asset"}},
			{Path: "shellapp/internal/llm/client.go", Status: 'A', Added: 300,
				Doc:     "Package llm is a stdlib HTTP client for a local OpenAI-compatible server.",
				Symbols: []string{"HTTPClient", "Complete", "Stream"}},
			{Path: "shellapp/ui/screens/model.go", Status: 'M', Added: 40, Removed: 5,
				Symbols: []string{"CloseAI"}},
			{Path: "shellapp/cmd/hittable/main.go", Status: 'M', Added: 2, Removed: 1},
			{Path: "shellapp/Makefile", Status: 'M', Added: 1, Removed: 1},
		},
	}
}

func TestBuildSpec(t *testing.T) {
	s := buildSpec(specDigest())
	out := s.String()

	// Largest first.
	if s.Blocks[0].Dir != "shellapp/internal/llmhost" {
		t.Errorf("blocks = %q, want the largest directory first", s.Blocks[0].Dir)
	}
	// The author's own words are the highest-value field.
	if s.Blocks[0].Purpose != "Package llmhost downloads, verifies, installs and supervises llama-server." {
		t.Errorf("purpose = %q", s.Blocks[0].Purpose)
	}
	// Symbols merge across files in a directory, capped.
	if len(s.Blocks[0].API) != specAPIMax {
		t.Errorf("API = %v, want %d names", s.Blocks[0].API, specAPIMax)
	}
	// Numbers make it transcribe instead of describe.
	for _, digit := range []string{"400", "300", "+", "−", "files", "lines"} {
		if strings.Contains(out, digit) {
			t.Errorf("the spec leaks counts (%q):\n%s", digit, out)
		}
	}
	// A one-line edit with nothing new does not earn a bullet of its own.
	if !strings.Contains(out, "also touched:") {
		t.Errorf("trivial directories were not merged:\n%s", out)
	}
	for _, minor := range []string{"shellapp/cmd/hittable", "shellapp"} {
		if !containsAny(s.Minor, minor) {
			continue
		}
	}
	if len(s.Minor) == 0 {
		t.Errorf("Minor = %v, want the trivial directories", s.Minor)
	}
	if buildSpec(specDigest()).String() != out {
		t.Error("buildSpec is not deterministic")
	}
}

func containsAny(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestBuildSpecSkipsSecrets(t *testing.T) {
	d := specDigest()
	d.Files = append(d.Files, Change{Path: "shellapp/.env", Status: 'A', Added: 3, Symbols: []string{"API_TOKEN"}})
	if strings.Contains(buildSpec(d).String(), ".env") {
		t.Error("a secret reached the specification")
	}
}

func TestGround(t *testing.T) {
	s := buildSpec(specDigest())
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{
			name: "grounded",
			body: "- Added shellapp/internal/llmhost, which downloads and supervises llama-server, with Install to set it up and Supervisor to run it.\n" +
				"- Added shellapp/internal/llm, a client for a local OpenAI-compatible server, with Complete and Stream.",
			ok: true,
		},
		{
			name: "prose with no names is fine",
			body: "- Reworked the way the editor loads its buffer.",
			ok:   true,
		},
		{
			// The real failure: CloseAI exists, but not here.
			name: "symbol attributed to the wrong directory",
			body: "- Modified shellapp/cmd/hittable to include new APIs for CloseAI.",
		},
		{
			name: "invented identifier",
			body: "- Added shellapp/internal/llm, which exposes a RetryPolicy for failed calls.",
		},
		{
			name: "invented path",
			body: "- Added shellapp/internal/telemetry to report usage.",
		},
		{
			name: "a parent directory is a legitimate way to refer to a block",
			body: "- Reworked shellapp/internal, adding Install and Complete.",
			ok:   true,
		},
		{
			name: "acronyms are prose, not identifiers",
			body: "- Added shellapp/internal/llm, an HTTP client that speaks JSON over the OpenAI API.",
			ok:   true,
		},
		{
			// "LLMs" rejected a true bullet in a real run.
			name: "pluralised acronyms are prose too",
			body: "- Added shellapp/internal/llm, an HTTP client for interacting with LLMs, with Complete and Stream.",
			ok:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ground(tt.body, s)
			if tt.ok && err != nil {
				t.Errorf("ground() = %v, want nil", err)
			}
			if !tt.ok && err == nil {
				t.Errorf("ground() accepted an ungrounded bullet:\n%s", tt.body)
			}
		})
	}
}

func TestIdentifierShaped(t *testing.T) {
	tests := []struct {
		tok  string
		want bool
	}{
		{"GenChunkMsg", true},
		{"CloseAI", true},
		{"llmHost", true},
		{"Added", false},
		{"Modified", false},
		{"the", false},
		{"HTTP", false},
		{"JSON", false},
		{"API", false},
		{"LLMs", false}, // a pluralised acronym is still a word
		{"APIs", false},
		{"URLs", false},
		{"Install", false}, // a single capital is indistinguishable from a sentence start
	}
	for _, tt := range tests {
		if got := identifierShaped(tt.tok); got != tt.want {
			t.Errorf("identifierShaped(%q) = %v, want %v", tt.tok, got, tt.want)
		}
	}
}

// A one-directory change must not be asked for four bullets: padding is where
// invention comes from, and small focused commits are the common case.
func TestBulletRangeScalesWithTheChange(t *testing.T) {
	tests := []struct {
		blocks, lo, hi int
	}{
		{0, 1, 1},
		{1, 1, 2},
		{2, 1, 3},
		{4, 3, 5},
		{12, 3, 6},
	}
	for _, tt := range tests {
		s := spec{Blocks: make([]specBlock, tt.blocks)}
		lo, hi := bulletRange(s)
		if lo != tt.lo || hi != tt.hi {
			t.Errorf("bulletRange(%d blocks) = %d..%d, want %d..%d", tt.blocks, lo, hi, tt.lo, tt.hi)
		}
	}
}

// The scraped-from-a-code-fence bug: a design document is not a package.
func TestSpecNeverNamesADocumentAsAnAPI(t *testing.T) {
	d := &Digest{Files: []Change{
		{Path: "shellapp/docs/plan.md", Status: 'A', Added: 200,
			Hunks: []string{"@@ -0,0 +1,3 @@\n+```go\n+func Runner() {}\n+type Change struct{}\n+```"}},
	}}
	// Collect is what fills Symbols, so exercise the scan the way it does.
	for i := range d.Files {
		d.Files[i].Symbols = scanSymbols(strings.Split(strings.Join(d.Files[i].Hunks, "\n"), "\n"), d.Files[i].Path)
	}
	if out := buildSpec(d).String(); strings.Contains(out, "new API") {
		t.Errorf("symbols were scraped out of a markdown code fence:\n%s", out)
	}
}

// Case is a rendering choice, not a claim. "the GitPanel component" names the
// gitpanel package; "CloseAI" names nothing at all.
func TestGroundFoldsCaseForNamesButNotForAttribution(t *testing.T) {
	s := buildSpec(specDigest())
	if err := ground("- Added the LLMHost supervisor under shellapp/internal/llmhost.", s); err != nil {
		t.Errorf("a case-folded rendering of a real name was rejected: %v", err)
	}
	if err := ground("- Added shellapp/internal/llmhost, with a RetryPolicy for failed downloads.", s); err == nil {
		t.Error("an invented name was accepted")
	}
	// Attribution stays exact: CloseAI is real, but not here.
	if err := ground("- Modified shellapp/internal/llm to add CloseAI.", s); err == nil {
		t.Error("a misattributed symbol was accepted")
	}
}

// Purpose lines are the author's own prose and routinely contain paths that
// are not part of the change. The model quoting one back is faithfulness, not
// invention — rejecting it discarded three good bodies in four on a real repo.
func TestGroundAcceptsTokensQuotedFromTheSpec(t *testing.T) {
	s := spec{Blocks: []specBlock{{
		Dir:     "internal/appconfig",
		New:     true,
		Purpose: "Package appconfig loads the app's configuration from ~/.hittable/config.json, an optional per-project <root>/hittable/config.json and the environment.",
		API:     []string{"AI", "Config", "Load", "Save"},
	}}}
	body := "- Added internal/appconfig, which loads configuration from ~/.hittable/config.json " +
		"and an optional per-project <root>/hittable/config.json, with Config, Load and Save."
	if err := ground(body, s); err != nil {
		t.Errorf("rejected a bullet quoting our own purpose line: %v", err)
	}
}

// A short identifier used with an article is English, not a reference to the
// type of the same name. "which closes the AI" must not be read as naming
// appconfig.AI and attributed to the wrong directory.
func TestGroundReadsArticledWordsAsProse(t *testing.T) {
	s := spec{Blocks: []specBlock{
		{Dir: "internal/appconfig", New: true, API: []string{"AI", "Config"}},
		{Dir: "ui/screens", API: []string{"CloseAI"}},
	}}
	ok := "- Modified ui/screens, which closes the AI, with CloseAI to stop the server."
	if err := ground(ok, s); err != nil {
		t.Errorf("rejected a correct bullet: %v", err)
	}
	// Real mis-attribution must still be caught: CloseAI does not live there.
	bad := "- Modified internal/appconfig to add CloseAI."
	if err := ground(bad, s); err == nil {
		t.Error("accepted a symbol attributed to the wrong directory")
	}
}
