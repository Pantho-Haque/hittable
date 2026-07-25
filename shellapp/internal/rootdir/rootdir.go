package rootdir

import (
	"fmt"
	"os"
	"path/filepath"
)

func Resolve(args []string) (string, error) {
	if len(args) > 0 {
		p := args[0]
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("resolving path %q: %w", p, err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("path %q: %w", abs, err)
		}
		if !fi.IsDir() {
			return "", fmt.Errorf("path %q is not a directory", abs)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return cwd, nil
}
