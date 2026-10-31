package hitfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvLayersEnvLocal(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "env.json")
	os.WriteFile(envPath, []byte(`{"BASE":"https://x.io","TOKEN":"placeholder"}`), 0o644)
	os.WriteFile(filepath.Join(dir, LocalEnvFile), []byte(`{"TOKEN":"real"}`), 0o644)

	env := LoadEnv(envPath)
	if env["BASE"] != "https://x.io" || env["TOKEN"] != "real" {
		t.Fatalf("LoadEnv = %v, want the local TOKEN over env.json's", env)
	}
	// Editor content for env.json still gets the local layer.
	if got := LayerLocalEnv(map[string]string{"TOKEN": "typed"}, envPath); got["TOKEN"] != "real" {
		t.Fatalf("LayerLocalEnv = %v", got)
	}
}

func TestLoadEnvToleratesMissingAndBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "env.json")
	if env := LoadEnv(envPath); len(env) != 0 {
		t.Fatalf("missing env.json: %v", env)
	}
	os.WriteFile(envPath, []byte(`{"A":"1"}`), 0o644)
	os.WriteFile(filepath.Join(dir, LocalEnvFile), []byte(`{oops`), 0o644)
	if env := LoadEnv(envPath); env["A"] != "1" || len(env) != 1 {
		t.Fatalf("broken env.local.json should be ignored: %v", env)
	}
}

func TestNeedsConfirmation(t *testing.T) {
	prod := map[string]string{ProductionVar: "true"}
	for _, m := range []string{"GET", "head", "OPTIONS"} {
		if NeedsConfirmation(m, prod) {
			t.Errorf("%s should not be confirmed", m)
		}
	}
	for _, m := range []string{"POST", "PUT", "patch", "DELETE"} {
		if !NeedsConfirmation(m, prod) {
			t.Errorf("%s should be confirmed on production", m)
		}
	}
	if NeedsConfirmation("DELETE", map[string]string{ProductionVar: "false"}) || NeedsConfirmation("DELETE", nil) {
		t.Error("only a production environment is guarded")
	}
}
