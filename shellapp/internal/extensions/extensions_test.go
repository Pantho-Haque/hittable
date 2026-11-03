package extensions

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/chroma"
)

func TestEngineAllows(t *testing.T) {
	cases := []struct {
		rng, v string
		ok     bool
	}{
		{"^1.80.0", "1.105.0", true},
		{"^1.106.0", "1.105.0", false},
		{"^2.0.0", "1.105.0", false},
		{">=1.60.0", "1.105.0", true},
		{"~1.105.0", "1.105.9", true},
		{"~1.104.0", "1.105.0", false},
		{"1.105.x", "1.105.3", true},
		{"*", "1.0.0", true},
		{"", "1.0.0", true},
		{"^1.82.0-insider", "1.105.0", true},
		{">=1.50.0 <1.60.0", "1.105.0", false},
		{"^1.50.0 || ^1.105.0", "1.105.0", true},
	}
	for _, c := range cases {
		if got := EngineAllows(c.rng, c.v); got != c.ok {
			t.Errorf("EngineAllows(%q, %q) = %v", c.rng, c.v, got)
		}
	}
}

func TestStripJSONC(t *testing.T) {
	in := "\ufeff{\n // a comment\n \"a\": \"http://x // not a comment\", /* block */\n \"b\": [1, 2,],\n}"
	var v map[string]any
	if err := json.Unmarshal(StripJSONC([]byte(in)), &v); err != nil {
		t.Fatalf("%v: %s", err, StripJSONC([]byte(in)))
	}
	if v["a"] != "http://x // not a comment" || len(v["b"].([]any)) != 2 {
		t.Errorf("got %v", v)
	}
}

func TestExpandSnippet(t *testing.T) {
	cases := []struct {
		in, out string
		cur     int
	}{
		{"for (${1:i} = 0; $1 < ${2:n}; $1++) {\n\t$0\n}", "for (i = 0;  < n; ++) {\n\t\n}", 5},
		{"console.log($0)", "console.log()", 12},
		{"${1|one,two|} end", "one end", 0},
		{"${TM_FILENAME:default}.x", "default.x", 9},
		{"cost \\$5", "cost $5", 7},
		{"plain", "plain", 5},
	}
	for _, c := range cases {
		out, cur := ExpandSnippet(c.in)
		if out != c.out || cur != c.cur {
			t.Errorf("ExpandSnippet(%q) = %q, %d; want %q, %d", c.in, out, cur, c.out, c.cur)
		}
	}
}

func TestThemeToChroma(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	dir := t.TempDir()
	write(t, filepath.Join(dir, "base.json"), `{"tokenColors":[{"scope":"comment","settings":{"foreground":"#111111"}}]}`)
	write(t, filepath.Join(dir, "theme.json"), `{
		// a theme with an include
		"include": "./base.json",
		"colors": {"editor.background": "#202020", "editor.foreground": "#eeeeee"},
		"tokenColors": [
			{"scope": ["string", "string.quoted"], "settings": {"foreground": "#22aa22"}},
			{"scope": "keyword.control, storage", "settings": {"foreground": "#ff00ffcc", "fontStyle": "bold italic"}},
		]
	}`)
	st, colors, err := ThemeChoice{Label: "T", path: filepath.Join(dir, "theme.json")}.ChromaStyle()
	if err != nil {
		t.Fatal(err)
	}
	if colors["editor.background"] != "#202020" {
		t.Errorf("colors: %v", colors)
	}
	check := func(tok chroma.TokenType, want string) {
		t.Helper()
		if got := st.Get(tok).Colour.String(); got != want {
			t.Errorf("%v = %s, want %s", tok, got, want)
		}
	}
	check(chroma.Comment, "#111111")       // from the include
	check(chroma.LiteralString, "#22aa22") // most specific rule
	check(chroma.Keyword, "#ff00ff")       // alpha dropped
	if e := st.Get(chroma.Keyword); e.Bold != chroma.Yes || e.Italic != chroma.Yes {
		t.Errorf("keyword font style lost: %+v", e)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// vsixBytes builds a .vsix holding the given files under extension/.
func vsixBytes(t *testing.T, files map[string]string, modes map[string]os.FileMode) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: "extension/" + name, Method: zip.Deflate}
		if m, ok := modes[name]; ok {
			h.SetMode(m)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func manifest(pub, name, version, engine string, extra string) string {
	return fmt.Sprintf(`{"name":%q,"publisher":%q,"version":%q,"engines":{"vscode":%q}%s}`, name, pub, version, engine, extra)
}

type fakeVersion struct {
	id, version, engine, platform string
	pre                           bool
	vsix                          []byte
	badSig                        bool
}

// fakeRegistry serves the slice of the Open VSX API the installer uses.
func fakeRegistry(t *testing.T, versions []fakeVersion) *httptest.Server {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(pub)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	var srv *httptest.Server
	file := func(v fakeVersion, kind string) string {
		return fmt.Sprintf("%s/files/%s/%s/%s/%s", srv.URL, v.id, v.version, v.platform, kind)
	}
	find := func(id, ver, plat string) *fakeVersion {
		for i := range versions {
			if versions[i].id == id && versions[i].version == ver && versions[i].platform == plat {
				return &versions[i]
			}
		}
		return nil
	}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/-/query":
			id, plat := r.URL.Query().Get("extensionId"), r.URL.Query().Get("targetPlatform")
			var out []map[string]any
			for _, v := range versions {
				if v.id != id || v.platform != plat {
					continue
				}
				ns, name, _ := strings.Cut(v.id, ".")
				out = append(out, map[string]any{
					"namespace": ns, "name": name, "version": v.version, "targetPlatform": v.platform,
					"preRelease": v.pre, "engines": map[string]string{"vscode": v.engine},
					"files": map[string]string{"download": file(v, "vsix"), "sha256": file(v, "sha256"),
						"signature": file(v, "sigzip"), "publicKey": srv.URL + "/key"},
				})
			}
			json.NewEncoder(w).Encode(map[string]any{"extensions": out, "totalSize": len(out)})
		case r.URL.Path == "/key":
			w.Write(keyPEM)
		case strings.HasPrefix(r.URL.Path, "/files/"):
			p := strings.Split(strings.TrimPrefix(r.URL.Path, "/files/"), "/")
			v := find(p[0], p[1], p[2])
			if v == nil {
				http.NotFound(w, r)
				return
			}
			switch p[3] {
			case "vsix":
				w.Write(v.vsix)
			case "sha256":
				sum := sha256.Sum256(v.vsix)
				fmt.Fprintf(w, "%s  file.vsix\n", hex.EncodeToString(sum[:]))
			case "sigzip":
				signed := v.vsix
				if v.badSig {
					signed = append([]byte("tampered"), signed...)
				}
				var buf bytes.Buffer
				zw := zip.NewWriter(&buf)
				f, _ := zw.Create(".signature.sig")
				f.Write(ed25519.Sign(priv, signed))
				zw.Close()
				w.Write(buf.Bytes())
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(EnvRegistry, srv.URL)
	return srv
}

func TestInstallFromRegistry(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	here := TargetPlatform()
	lib := vsixBytes(t, map[string]string{"package.json": manifest("acme", "lib", "1.0.0", "^1.80.0", "")}, nil)
	app := func(ver, engine string) []byte {
		return vsixBytes(t, map[string]string{
			"package.json":  manifest("acme", "app", ver, engine, `,"main":"./main.js","extensionDependencies":["acme.lib"]`),
			"main.js":       "exports.activate = () => {}",
			"server/bin/ls": "#!/bin/sh\n",
		}, map[string]os.FileMode{"server/bin/ls": 0o755})
	}
	fakeRegistry(t, []fakeVersion{
		{id: "acme.lib", version: "1.0.0", engine: "^1.80.0", platform: "universal", vsix: lib},
		{id: "acme.app", version: "3.0.0", engine: "^1.200.0", platform: "universal", vsix: app("3.0.0", "^1.200.0")}, // too new
		{id: "acme.app", version: "2.1.0", engine: "^1.80.0", platform: "universal", pre: true, vsix: app("2.1.0", "^1.80.0")},
		{id: "acme.app", version: "2.0.0", engine: "^1.80.0", platform: here, vsix: app("2.0.0", "^1.80.0")},
		{id: "acme.app", version: "1.9.0", engine: "^1.80.0", platform: "universal", vsix: app("1.9.0", "^1.80.0")},
	})

	res, err := Install(context.Background(), "acme.app", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Newest stable version that accepts our API, built for this machine.
	if res.Installed.Version != "2.0.0" || res.Installed.TargetPlatform != here || !res.Installed.Signed {
		t.Errorf("installed %+v", res.Installed)
	}
	if len(res.Deps) != 1 || res.Deps[0] != "acme.lib" {
		t.Errorf("deps = %v", res.Deps)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(res.Installed.Path(), "server", "bin", "ls"))
		if err != nil || fi.Mode().Perm()&0o100 == 0 {
			t.Errorf("bundled binary lost its exec bit: %v %v", fi.Mode(), err)
		}
	}
	all, _ := List()
	if len(all) != 2 {
		t.Fatalf("index has %d entries", len(all))
	}
	if err := Uninstall("acme.lib"); err == nil || !strings.Contains(err.Error(), "acme.app needs acme.lib") {
		t.Errorf("uninstalling a dependency: %v", err)
	}
	if err := SetEnabled("acme.app", false); err != nil {
		t.Fatal(err)
	}
	if len(Enabled()) != 1 {
		t.Errorf("disable did not stick")
	}
	if err := Uninstall("acme.app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.Installed.Path()); !os.IsNotExist(err) {
		t.Errorf("uninstall left %s", res.Installed.Path())
	}
}

func TestInstallRefuses(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	good := vsixBytes(t, map[string]string{"package.json": manifest("acme", "good", "1.0.0", "^1.80.0", "")}, nil)
	proposals := vsixBytes(t, map[string]string{"package.json": manifest("acme", "prop", "1.0.0", "^1.80.0", `,"enabledApiProposals":["chatProvider"]`)}, nil)
	wrongID := vsixBytes(t, map[string]string{"package.json": manifest("evil", "other", "1.0.0", "^1.80.0", "")}, nil)
	fakeRegistry(t, []fakeVersion{
		{id: "acme.good", version: "1.0.0", engine: "^1.80.0", platform: "universal", vsix: good, badSig: true},
		{id: "acme.prop", version: "1.0.0", engine: "^1.80.0", platform: "universal", vsix: proposals},
		{id: "acme.swap", version: "1.0.0", engine: "^1.80.0", platform: "universal", vsix: wrongID},
		{id: "acme.new", version: "1.0.0", engine: "^9.0.0", platform: "universal", vsix: good},
	})
	for spec, want := range map[string]string{
		"acme.good":                "failed signature verification",
		"acme.prop":                "proposed VS Code APIs",
		"acme.swap":                "refusing it",
		"acme.new":                 "needs a newer VS Code",
		"acme.missing":             "is not on",
		"ms-python.vscode-pylance": "licensed by Microsoft",
		"not an id":                "is not an extension id",
	} {
		if _, err := Install(context.Background(), spec, nil); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Install(%q) = %v, want an error containing %q", spec, err, want)
		}
	}
	if all, _ := List(); len(all) != 0 {
		t.Errorf("a refused install left %d entries", len(all))
	}
	entries, _ := os.ReadDir(filepath.Join(os.Getenv("HITTABLE_HOME"), "extensions"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("staging folder left behind: %s", e.Name())
		}
	}
}

func TestInstallLocalVSIXAndLanguages(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	p := filepath.Join(t.TempDir(), "lang.vsix")
	os.WriteFile(p, vsixBytes(t, map[string]string{
		"package.json": manifest("acme", "lang", "0.1.0", "^1.80.0",
			`,"displayName":"%name%","contributes":{"languages":[{"id":"hcl","extensions":[".hcl",".nomad"]}],"snippets":[{"language":"hcl","path":"./s.json"}]}`),
		"package.nls.json": `{"name":"HCL Language"}`,
		"s.json":           `{"Resource":{"prefix":["res","resource"],"body":["resource \"${1:type}\" {","  $0","}"]}}`,
	}, nil), 0o644)
	res, err := Install(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Installed.Source != "vsix" || res.Installed.Manifest.Label() != "HCL Language" {
		t.Errorf("installed %+v (label %q)", res.Installed, res.Installed.Manifest.Label())
	}
	ResetLanguageCache()
	if id := LanguageID("/x/job.nomad"); id != "hcl" {
		t.Errorf("contributed language: %q", id)
	}
	if id := LanguageID("/x/a.yml"); id != "yaml" {
		t.Errorf("built-in language: %q", id)
	}
	sn := Snippets("hcl")
	if len(sn) != 2 || sn[0].Prefix != "res" || !strings.Contains(sn[0].Body, "\n") {
		t.Errorf("snippets: %+v", sn)
	}
}

func TestTrust(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	root := t.TempDir()
	if Trusted(root) {
		t.Fatal("trusted before asking")
	}
	if err := Trust(root); err != nil {
		t.Fatal(err)
	}
	if !Trusted(filepath.Join(root, "sub")) || !Trusted(root) {
		t.Error("trust does not cover the folder and its children")
	}
	if Trusted(root + "-other") {
		t.Error("trust leaked to a sibling with the same prefix")
	}
}

func TestSetJSONCValueKeepsComments(t *testing.T) {
	src := []byte("// my settings\n{\n  // the theme I like\n  \"workbench.colorTheme\": \"Dracula\",\n  \"editor.tabSize\": 4, // tabs\n  \"files.exclude\": {\"**/.git\": true}\n}\n")
	out, ok := SetJSONCValue(src, "workbench.colorTheme", "Nord")
	if !ok || !strings.Contains(string(out), `"workbench.colorTheme": "Nord",`) || !strings.Contains(string(out), "// the theme I like") {
		t.Fatalf("replace: %v\n%s", ok, out)
	}
	out, ok = SetJSONCValue(out, "editor.tabSize", nil)
	if !ok || strings.Contains(string(out), "tabSize") || !strings.Contains(string(out), "// my settings") {
		t.Fatalf("delete: %v\n%s", ok, out)
	}
	out, ok = SetJSONCValue(out, "yaml.format.enable", false)
	if !ok || !strings.Contains(string(out), `"yaml.format.enable": false,`) {
		t.Fatalf("insert: %v\n%s", ok, out)
	}
	var v map[string]any
	if err := json.Unmarshal(StripJSONC(out), &v); err != nil || v["workbench.colorTheme"] != "Nord" || v["yaml.format.enable"] != false {
		t.Fatalf("result does not parse: %v %v\n%s", err, v, out)
	}
	if _, ok := SetJSONCValue(src, "files.exclude", "x"); ok {
		t.Error("an object value must not be edited in place")
	}
	if _, ok := SetJSONCValue(src, "k", map[string]any{"a": 1}); ok {
		t.Error("an object value must not be written in place")
	}
	if out, ok := SetJSONCValue([]byte("{}"), "a", "b"); !ok || !strings.Contains(string(out), `"a": "b"`) {
		t.Errorf("empty object: %s", out)
	}
}

func TestStaleLockIsTakenOver(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	os.MkdirAll(filepath.Join(os.Getenv("HITTABLE_HOME"), "extensions"), 0o755)
	// A pid that is certainly not running.
	os.WriteFile(filepath.Join(os.Getenv("HITTABLE_HOME"), "extensions", ".lock"), []byte("999999"), 0o644)
	done := make(chan error, 1)
	go func() { done <- SetEnabled("nothing.here", true) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not installed") {
			t.Errorf("expected the real error after taking the lock, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a lock left by a dead process blocked the install")
	}
}

func TestInstallAnywayTakesANewerEngine(t *testing.T) {
	t.Setenv("HITTABLE_HOME", t.TempDir())
	vs := vsixBytes(t, map[string]string{"package.json": manifest("acme", "future", "2.0.0", "^9.0.0", "")}, nil)
	fakeRegistry(t, []fakeVersion{{id: "acme.future", version: "2.0.0", engine: "^9.0.0", platform: "universal", vsix: vs}})
	if _, err := Install(context.Background(), "acme.future", nil); !errors.Is(err, ErrEngine) {
		t.Fatalf("Install: %v, want ErrEngine", err)
	}
	res, err := InstallAnyway(context.Background(), "acme.future", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Installed.Version != "2.0.0" || len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "asks for VS Code ^9.0.0") {
		t.Errorf("installed %+v, warnings %v", res.Installed, res.Warnings)
	}
}
