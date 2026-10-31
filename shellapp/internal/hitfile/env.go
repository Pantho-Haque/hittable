package hitfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// LocalEnvFile sits next to env.json and holds values that must not reach
// git (it is listed in hittable/.gitignore). Its keys override env.json's.
// The web app layers the two the same way.
const LocalEnvFile = "env.local.json"

// LoadEnv reads env.json at envPath with env.local.json beside it layered on
// top. A missing or broken file contributes nothing.
func LoadEnv(envPath string) map[string]string {
	return LayerLocalEnv(readFlatMap(envPath), envPath)
}

// LayerLocalEnv returns env with the env.local.json next to envPath on top,
// for when env.json's content comes from an editor rather than disk.
func LayerLocalEnv(env map[string]string, envPath string) map[string]string {
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = v
	}
	for k, v := range readFlatMap(filepath.Join(filepath.Dir(envPath), LocalEnvFile)) {
		out[k] = v
	}
	return out
}

func readFlatMap(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	var m map[string]string
	if json.Unmarshal(data, &m) != nil || m == nil {
		return map[string]string{}
	}
	return m
}

// ProductionVar marks an environment as production (shared with the web app).
const ProductionVar = "HITTABLE_PRODUCTION"

// NeedsConfirmation reports whether sending method to this environment should
// be confirmed first: the environment is marked production and the method
// changes state.
func NeedsConfirmation(method string, env map[string]string) bool {
	switch strings.ToLower(strings.TrimSpace(env[ProductionVar])) {
	case "true", "1", "yes":
	default:
		return false
	}
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return false
	}
	return true
}
