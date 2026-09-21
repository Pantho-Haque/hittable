package scaffold

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

type HitFile struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers"`
	Params   map[string]string `json:"params"`
	Body     string            `json:"body"`
	Response interface{}       `json:"response"`
}

type EnvFile = map[string]string

func Ensure(root string) error {
	hittableDir := filepath.Join(root, "hittable")

	if _, err := os.Stat(hittableDir); err == nil {
		return nil
	}

	tcDir := filepath.Join(hittableDir, "testcollection")
	notesDir := filepath.Join(hittableDir, "notes")

	for _, d := range []string{hittableDir, tcDir, notesDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	hit := HitFile{
		Method: "GET",
		URL:    "https://jsonplaceholder.typicode.com/posts/1",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Params:   map[string]string{},
		Body:     "",
		Response: nil,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(hit); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tcDir, "test.hit"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	env := EnvFile{
		"BASE_URL":   "https://jsonplaceholder.typicode.com",
		"AUTH_TOKEN": "",
	}
	envData, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	envData = append(envData, '\n')
	if err := os.WriteFile(filepath.Join(hittableDir, "env.json"), envData, 0o644); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(notesDir, "sample.md"), []byte("# Notes\n\nWrite your notes here.\n"), 0o644); err != nil {
		return err
	}

	return nil
}
