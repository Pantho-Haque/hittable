package envfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type EnvFile struct {
	Path string
	Data map[string]string
}

var keyPattern = regexp.MustCompile(`<<(\w+)>>`)

func Load(hittableDir string) (*EnvFile, error) {
	p := filepath.Join(hittableDir, "env.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return &EnvFile{Path: p, Data: map[string]string{}}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]string{}
	}
	return &EnvFile{Path: p, Data: m}, nil
}

func (e *EnvFile) Interpolate(s string) string {
	return keyPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-2]
		if val, ok := e.Data[key]; ok {
			return val
		}
		return match
	})
}

func InterpolateAll(text string, headers, params map[string]string, e *EnvFile) (string, map[string]string, map[string]string, string) {
	url := e.Interpolate(text)
	newHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		newHeaders[k] = e.Interpolate(v)
	}
	newParams := make(map[string]string, len(params))
	for k, v := range params {
		newParams[k] = e.Interpolate(v)
	}
	return url, newHeaders, newParams, ""
}

func LoadContent(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

func SaveContent(path string, m map[string]string) error {
	if m == nil {
		m = map[string]string{}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func InterpolateBody(text string, e *EnvFile) string {
	return e.Interpolate(text)
}

func ResolveEnvPath(hittableDir string) string {
	return filepath.Join(hittableDir, "env.json")
}

func ParseEnvContent(data []byte) (map[string]string, error) {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

func MarshalEnv(m map[string]string) ([]byte, error) {
	if m == nil {
		m = map[string]string{}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func InterpolateString(s string, env map[string]string) string {
	return keyPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := match[2 : len(match)-2]
		if val, ok := env[key]; ok {
			return val
		}
		return match
	})
}

func ExtractBaseURL(env map[string]string) string {
	if v, ok := env["BASE_URL"]; ok {
		return strings.TrimRight(v, "/")
	}
	return ""
}
