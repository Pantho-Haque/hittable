package commitmsg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hittable/shellapp/internal/gitx"
)

// TestSecretsNeverPacked drives the real thing end to end -- a temp repo, a
// real staged diff, Collect then Pack -- and asserts a canary value planted in
// every sensitive-looking file is absent from the prompt while the filenames
// survive. The traffic is localhost-only today, but this is the code path that
// would carry a diff to a remote provider the day anyone adds one.
func TestSecretsNeverPacked(t *testing.T) {
	dir := t.TempDir()
	run := func(a ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, a...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=T", "GIT_COMMITTER_EMAIL=t@x")
		if o, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", a, e, o)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@x")
	run("config", "user.name", "T")
	os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("x\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "feat: seed")

	const canary = "SUPER_SECRET_CANARY_VALUE_9137"
	files := map[string]string{
		".env":            "API_KEY=" + canary + "\n",
		".env.production": "TOKEN=" + canary + "\n",
		"server.pem":      "-----BEGIN KEY-----\n" + canary + "\n",
		"tls.key":         canary + "\n",
		"id_rsa":          canary + "\n",
		"my_credentials":  canary + "\n",
		"ok.go":           "package a\n\nfunc Fine() {}\n",
	}
	for n, b := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-f", ".")

	d, err := Collect(gitx.Open(dir))
	if err != nil {
		t.Fatal(err)
	}
	packed := d.Pack(12000)
	if strings.Contains(packed, canary) {
		t.Fatalf("SECRET LEAKED into the packed prompt:\n%s", packed)
	}
	for _, name := range []string{".env", "server.pem", "tls.key", "id_rsa"} {
		if !strings.Contains(packed, name) {
			t.Errorf("%s should still be listed by name", name)
		}
	}
	t.Logf("no canary in %d bytes; sensitive files listed by name only", len(packed))
}
