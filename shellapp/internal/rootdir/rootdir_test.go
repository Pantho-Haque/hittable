package rootdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "f.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	spaced := filepath.Join(tmp, "my project ü")
	os.Mkdir(spaced, 0o755)

	t.Chdir(tmp)
	cwd, _ := os.Getwd()

	if got, err := Resolve(nil); err != nil || got != cwd {
		t.Errorf("no args = %q, %v; want cwd %q", got, err, cwd)
	}
	if got, err := Resolve([]string{"."}); err != nil || got != cwd {
		t.Errorf(". = %q, %v", got, err)
	}
	if got, err := Resolve([]string{"my project ü"}); err != nil || got != filepath.Join(cwd, "my project ü") {
		t.Errorf("relative dir = %q, %v", got, err)
	}
	if got, err := Resolve([]string{spaced}); err != nil || got != spaced {
		t.Errorf("absolute dir = %q, %v", got, err)
	}
	if got, err := Resolve([]string{spaced + "/../my project ü/"}); err != nil || got != spaced {
		t.Errorf("unclean path = %q, %v", got, err)
	}
	if _, err := Resolve([]string{"missing"}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("missing dir: %v", err)
	}
	if _, err := Resolve([]string{file}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("file: %v", err)
	}
}
