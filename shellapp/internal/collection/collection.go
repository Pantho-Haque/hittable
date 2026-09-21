// Package collection imports Postman v2.1 and Insomnia (v4 export) files into
// a hittable/ directory tree (folders → directories, requests → .hit files,
// variables → env.json) and exports a hittable/ tree back to those formats.
// Mapping follows the web app's importers (see IMPORT_EXPORT_FORMATS.md).
package collection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hittable/shellapp/internal/hitfile"
)

// Report summarises an import.
type Report struct {
	Format   string
	Name     string
	Dir      string // directory the collection was written to
	Requests int
	Folders  int
	Vars     int
	Dropped  []string // features that have no hittable equivalent
}

func (r Report) String() string {
	s := fmt.Sprintf("Imported %q from %s: %d requests, %d folders, %d variables → %s", r.Name, r.Format, r.Requests, r.Folders, r.Vars, r.Dir)
	if len(r.Dropped) > 0 {
		s += "\nDropped (no hittable equivalent): " + strings.Join(uniq(r.Dropped), ", ")
	}
	return s
}

// Detect identifies the file format: "postman", "insomnia" or "".
func Detect(data []byte) string {
	var probe struct {
		Info struct {
			Schema string `json:"schema"`
		} `json:"info"`
		Type      string          `json:"_type"`
		Format    int             `json:"__export_format"`
		Resources json.RawMessage `json:"resources"`
	}
	if err := json.Unmarshal(data, &probe); err == nil {
		switch {
		case strings.Contains(probe.Info.Schema, "getpostman.com"):
			return "postman"
		case probe.Type == "export" || probe.Format > 0 || len(probe.Resources) > 0:
			return "insomnia"
		}
	}
	var arr []struct {
		Type string `json:"_type"`
	}
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 && arr[0].Type != "" {
		return "insomnia"
	}
	return ""
}

// Import reads a Postman or Insomnia file and writes it under hittableDir.
func Import(path, hittableDir string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch Detect(data) {
	case "postman":
		return importPostman(data, hittableDir)
	case "insomnia":
		return importInsomnia(data, hittableDir)
	}
	return nil, fmt.Errorf("%s: not a Postman v2.1 collection or an Insomnia export", filepath.Base(path))
}

// ---------- variable syntax ----------

var (
	postmanVar  = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)
	insomniaVar = regexp.MustCompile(`\{\{\s*_\.([A-Za-z0-9_.-]+)\s*\}\}`)
	hitVar      = regexp.MustCompile(`<<(\w+)>>`)
	unsafe      = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)
)

func fromPostman(s string) string { return postmanVar.ReplaceAllString(s, "<<$1>>") }

// fromInsomnia handles "{{ _.KEY }}" first so the generic "{{KEY}}" pass
// never sees the "_." prefix.
func fromInsomnia(s string) string {
	return postmanVar.ReplaceAllString(insomniaVar.ReplaceAllString(s, "<<$1>>"), "<<$1>>")
}
func toPostman(s string) string  { return hitVar.ReplaceAllString(s, "{{$1}}") }
func toInsomnia(s string) string { return hitVar.ReplaceAllString(s, "{{ _.$1 }}") }

// safeName makes a file-system friendly name.
func safeName(s string) string {
	s = strings.TrimSpace(unsafe.ReplaceAllString(s, "_"))
	s = strings.Trim(s, "._ ")
	if s == "" {
		s = "untitled"
	}
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// uniquePath appends " (2)", " (3)"… when a path already exists.
func uniquePath(base, ext string) string {
	p := base + ext
	for i := 2; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
		p = fmt.Sprintf("%s (%d)%s", base, i, ext)
	}
}

func writeHit(dir, name string, h *hitfile.HitFile) error {
	if h.Headers == nil {
		h.Headers = map[string]string{}
	}
	if h.Params == nil {
		h.Params = map[string]string{}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(uniquePath(filepath.Join(dir, safeName(name)), ".hit"), hitfile.Marshal(h), 0o644)
}

// mergeEnv adds keys to env.json without overwriting existing values.
func mergeEnv(hittableDir string, vars map[string]string) (int, error) {
	if len(vars) == 0 {
		return 0, nil
	}
	p := filepath.Join(hittableDir, "env.json")
	env := map[string]string{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &env)
	}
	added := 0
	for k, v := range vars {
		if _, ok := env[k]; !ok {
			env[k] = v
			added++
		}
	}
	if err := os.MkdirAll(hittableDir, 0o755); err != nil {
		return 0, err
	}
	b, _ := json.MarshalIndent(env, "", "  ")
	return added, os.WriteFile(p, append(b, '\n'), 0o644)
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// splitQuery separates "?a=b" from a URL into params.
func splitQuery(u string, params map[string]string) string {
	i := strings.IndexByte(u, '?')
	if i < 0 {
		return u
	}
	for _, kv := range strings.Split(u[i+1:], "&") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, "=")
		if _, exists := params[k]; !exists {
			params[k] = v
		}
	}
	return u[:i]
}

// ---------- Postman ----------

type pmKV struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
	Type     string `json:"type"`
	Src      any    `json:"src"`
}

type pmAuth struct {
	Type   string `json:"type"`
	Bearer []pmKV `json:"bearer"`
	Basic  []pmKV `json:"basic"`
	APIKey []pmKV `json:"apikey"`
}

type pmItem struct {
	Name    string   `json:"name"`
	Item    []pmItem `json:"item"`
	Request *struct {
		Method string          `json:"method"`
		URL    json.RawMessage `json:"url"`
		Header []pmKV          `json:"header"`
		Body   *struct {
			Mode       string `json:"mode"`
			Raw        string `json:"raw"`
			URLEncoded []pmKV `json:"urlencoded"`
			FormData   []pmKV `json:"formdata"`
		} `json:"body"`
		Auth *pmAuth `json:"auth"`
	} `json:"request"`
	Event    []struct{ Listen string } `json:"event"`
	Variable []pmKV                    `json:"variable"`
	Auth     *pmAuth                   `json:"auth"`
}

type pmCollection struct {
	Info struct {
		Name string `json:"name"`
	} `json:"info"`
	Item     []pmItem `json:"item"`
	Variable []pmKV   `json:"variable"`
	Auth     *pmAuth  `json:"auth"`
}

func authHeaders(a *pmAuth, dropped *[]string) map[string]string {
	if a == nil {
		return nil
	}
	get := func(kvs []pmKV, key string) string {
		for _, kv := range kvs {
			if kv.Key == key {
				return kv.Value
			}
		}
		return ""
	}
	switch a.Type {
	case "bearer":
		return map[string]string{"Authorization": "Bearer " + fromPostman(get(a.Bearer, "token"))}
	case "basic":
		return map[string]string{"Authorization": "Basic <<BASIC_" + safeName(get(a.Basic, "username")) + ">>"}
	case "apikey":
		if get(a.APIKey, "in") != "query" {
			return map[string]string{fromPostman(get(a.APIKey, "key")): fromPostman(get(a.APIKey, "value"))}
		}
	case "", "noauth", "inherit":
		return nil
	}
	*dropped = append(*dropped, "auth:"+a.Type)
	return nil
}

func importPostman(data []byte, hittableDir string) (*Report, error) {
	var col pmCollection
	if err := json.Unmarshal(data, &col); err != nil {
		return nil, fmt.Errorf("postman: %w", err)
	}
	name := col.Info.Name
	if name == "" {
		name = "Postman Collection"
	}
	rep := &Report{Format: "Postman v2.1", Name: name}
	rep.Dir = uniquePath(filepath.Join(hittableDir, safeName(name)), "")
	vars := map[string]string{}
	for _, v := range col.Variable {
		vars[v.Key] = v.Value
	}
	inherited := authHeaders(col.Auth, &rep.Dropped)

	var walk func(items []pmItem, dir string, auth map[string]string) error
	walk = func(items []pmItem, dir string, auth map[string]string) error {
		for _, it := range items {
			for _, ev := range it.Event {
				rep.Dropped = append(rep.Dropped, ev.Listen+" script")
			}
			for _, v := range it.Variable {
				vars[v.Key] = v.Value
			}
			if it.Request == nil {
				rep.Folders++
				folderAuth := auth
				if h := authHeaders(it.Auth, &rep.Dropped); h != nil {
					folderAuth = h
				}
				if err := walk(it.Item, filepath.Join(dir, safeName(it.Name)), folderAuth); err != nil {
					return err
				}
				continue
			}
			r := it.Request
			h := &hitfile.HitFile{Method: strings.ToUpper(r.Method), Headers: map[string]string{}, Params: map[string]string{}}
			if h.Method == "" {
				h.Method = "GET"
			}
			// URL: string or {raw, query}
			var raw string
			if err := json.Unmarshal(r.URL, &raw); err != nil {
				var obj struct {
					Raw   string `json:"raw"`
					Query []pmKV `json:"query"`
				}
				_ = json.Unmarshal(r.URL, &obj)
				raw = obj.Raw
				for _, q := range obj.Query {
					if !q.Disabled {
						h.Params[q.Key] = fromPostman(q.Value)
					}
				}
			}
			h.URL = splitQuery(fromPostman(raw), h.Params)
			// Auth: the request's own setting wins; "noauth" opts out of the
			// inherited collection / folder auth.
			switch {
			case r.Auth == nil || r.Auth.Type == "" || r.Auth.Type == "inherit":
				for k, v := range auth {
					h.Headers[k] = v
				}
			case r.Auth.Type == "noauth":
			default:
				for k, v := range authHeaders(r.Auth, &rep.Dropped) {
					h.Headers[k] = v
				}
			}
			for _, hd := range r.Header {
				if !hd.Disabled && hd.Key != "" {
					h.Headers[hd.Key] = fromPostman(hd.Value)
				}
			}
			if b := r.Body; b != nil {
				switch b.Mode {
				case "raw":
					h.Body = fromPostman(b.Raw)
				case "urlencoded", "formdata":
					kvs := b.URLEncoded
					if b.Mode == "formdata" {
						kvs = b.FormData
					}
					m := map[string]string{}
					for _, kv := range kvs {
						if kv.Disabled {
							continue
						}
						if kv.Type == "file" {
							m[kv.Key] = "@" + fmt.Sprint(kv.Src)
						} else {
							m[kv.Key] = fromPostman(kv.Value)
						}
					}
					bb, _ := json.MarshalIndent(m, "", "  ")
					h.Body = string(bb)
					if b.Mode == "urlencoded" {
						if _, ok := h.Headers["Content-Type"]; !ok {
							h.Headers["Content-Type"] = "application/x-www-form-urlencoded"
						}
					}
				case "graphql", "file":
					rep.Dropped = append(rep.Dropped, "body:"+b.Mode)
				}
			}
			if err := writeHit(dir, it.Name, h); err != nil {
				return err
			}
			rep.Requests++
		}
		return nil
	}
	if err := walk(col.Item, rep.Dir, inherited); err != nil {
		return nil, err
	}
	n, err := mergeEnv(hittableDir, vars)
	if err != nil {
		return nil, err
	}
	rep.Vars = n
	return rep, nil
}

// ---------- Insomnia ----------

type inResource struct {
	Type     string          `json:"_type"`
	ID       string          `json:"_id"`
	ParentID string          `json:"parentId"`
	Name     string          `json:"name"`
	Method   string          `json:"method"`
	URL      string          `json:"url"`
	Headers  json.RawMessage `json:"headers"`
	Params   []struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Disabled bool   `json:"disabled"`
	} `json:"parameters"`
	Body *struct {
		MimeType string `json:"mimeType"`
		Text     string `json:"text"`
		Params   []struct {
			Name     string `json:"name"`
			Value    string `json:"value"`
			FileName string `json:"fileName"`
			Disabled bool   `json:"disabled"`
		} `json:"params"`
	} `json:"body"`
	Auth *struct {
		Type     string `json:"type"`
		Token    string `json:"token"`
		Prefix   string `json:"prefix"`
		Username string `json:"username"`
		Key      string `json:"key"`
		Value    string `json:"value"`
		Disabled bool   `json:"disabled"`
	} `json:"authentication"`
	Data map[string]any `json:"data"`
}

func importInsomnia(data []byte, hittableDir string) (*Report, error) {
	var export struct {
		Resources []inResource `json:"resources"`
	}
	if err := json.Unmarshal(data, &export); err != nil || len(export.Resources) == 0 {
		if err := json.Unmarshal(data, &export.Resources); err != nil {
			return nil, fmt.Errorf("insomnia: %w", err)
		}
	}
	res := export.Resources
	byID := map[string]inResource{}
	name := "Insomnia Export"
	for _, r := range res {
		byID[r.ID] = r
		if r.Type == "workspace" && r.Name != "" {
			name = r.Name
		}
	}
	rep := &Report{Format: "Insomnia", Name: name}
	rep.Dir = uniquePath(filepath.Join(hittableDir, safeName(name)), "")

	// Folder path for a resource: chain of request_group parents.
	var dirOf func(id string) string
	dirOf = func(id string) string {
		r, ok := byID[id]
		if !ok || r.Type != "request_group" {
			return rep.Dir
		}
		return filepath.Join(dirOf(r.ParentID), safeName(r.Name))
	}

	vars := map[string]string{}
	for _, r := range res {
		switch r.Type {
		case "request_group":
			rep.Folders++
		case "environment":
			for k, v := range r.Data {
				vars[k] = fmt.Sprint(v)
			}
		case "cookie_jar", "api_spec", "unit_test", "unit_test_suite", "proto_file", "websocket_request", "grpc_request":
			rep.Dropped = append(rep.Dropped, r.Type)
		}
	}
	for _, r := range res {
		if r.Type != "request" {
			continue
		}
		h := &hitfile.HitFile{Method: strings.ToUpper(r.Method), Headers: map[string]string{}, Params: map[string]string{}}
		if h.Method == "" {
			h.Method = "GET"
		}
		for _, p := range r.Params {
			if !p.Disabled {
				h.Params[p.Name] = fromInsomnia(p.Value)
			}
		}
		h.URL = splitQuery(fromInsomnia(r.URL), h.Params)
		// Headers: array of {name,value,disabled} (v4) or an object.
		var arr []struct {
			Name     string `json:"name"`
			Value    string `json:"value"`
			Disabled bool   `json:"disabled"`
		}
		if json.Unmarshal(r.Headers, &arr) == nil {
			for _, hd := range arr {
				if !hd.Disabled && hd.Name != "" {
					h.Headers[hd.Name] = fromInsomnia(hd.Value)
				}
			}
		} else {
			var obj map[string]string
			if json.Unmarshal(r.Headers, &obj) == nil {
				for k, v := range obj {
					h.Headers[k] = fromInsomnia(v)
				}
			}
		}
		if a := r.Auth; a != nil && !a.Disabled {
			switch a.Type {
			case "bearer":
				prefix := a.Prefix
				if prefix == "" {
					prefix = "Bearer"
				}
				h.Headers["Authorization"] = prefix + " " + fromInsomnia(a.Token)
			case "basic":
				h.Headers["Authorization"] = "Basic <<BASIC_" + safeName(a.Username) + ">>"
			case "apikey":
				h.Headers[fromInsomnia(a.Key)] = fromInsomnia(a.Value)
			case "", "none":
			default:
				rep.Dropped = append(rep.Dropped, "auth:"+a.Type)
			}
		}
		if b := r.Body; b != nil {
			switch {
			case b.Text != "":
				h.Body = fromInsomnia(b.Text)
				if b.MimeType != "" {
					if _, ok := h.Headers["Content-Type"]; !ok {
						h.Headers["Content-Type"] = b.MimeType
					}
				}
			case len(b.Params) > 0:
				m := map[string]string{}
				for _, p := range b.Params {
					if p.Disabled {
						continue
					}
					if p.FileName != "" {
						m[p.Name] = "@" + p.FileName
					} else {
						m[p.Name] = fromInsomnia(p.Value)
					}
				}
				bb, _ := json.MarshalIndent(m, "", "  ")
				h.Body = string(bb)
			}
		}
		if err := writeHit(dirOf(r.ParentID), r.Name, h); err != nil {
			return nil, err
		}
		rep.Requests++
	}
	n, err := mergeEnv(hittableDir, vars)
	if err != nil {
		return nil, err
	}
	rep.Vars = n
	return rep, nil
}

// ---------- export ----------

type node struct {
	name  string
	dirs  []*node
	files []struct {
		name string
		hit  *hitfile.HitFile
	}
}

// readTree loads every .hit under dir (env.json and notes are skipped).
func readTree(dir string) (*node, int, error) {
	n := &node{name: filepath.Base(dir)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	count := 0
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			child, c, err := readTree(p)
			if err != nil {
				return nil, 0, err
			}
			if c > 0 {
				n.dirs = append(n.dirs, child)
				count += c
			}
			continue
		}
		if filepath.Ext(e.Name()) != ".hit" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		h, err := hitfile.Parse(b)
		if err != nil {
			continue
		}
		n.files = append(n.files, struct {
			name string
			hit  *hitfile.HitFile
		}{strings.TrimSuffix(e.Name(), ".hit"), h})
		count++
	}
	return n, count, nil
}

func readEnv(hittableDir string) map[string]string {
	env := map[string]string{}
	if b, err := os.ReadFile(filepath.Join(hittableDir, "env.json")); err == nil {
		_ = json.Unmarshal(b, &env)
	}
	return env
}

// ExportPostman writes a Postman v2.1 collection for hittableDir.
func ExportPostman(hittableDir, name string) ([]byte, int, error) {
	root, count, err := readTree(hittableDir)
	if err != nil {
		return nil, 0, err
	}
	var items func(n *node) []any
	items = func(n *node) []any {
		var out []any
		for _, d := range n.dirs {
			out = append(out, map[string]any{"name": d.name, "item": items(d)})
		}
		for _, f := range n.files {
			h := f.hit
			var query []map[string]string
			for _, k := range sortedKeys(h.Params) {
				query = append(query, map[string]string{"key": k, "value": toPostman(h.Params[k])})
			}
			raw := toPostman(h.URL)
			if len(query) > 0 {
				var qs []string
				for _, q := range query {
					qs = append(qs, q["key"]+"="+q["value"])
				}
				raw += "?" + strings.Join(qs, "&")
			}
			var headers []map[string]any
			for _, k := range sortedKeys(h.Headers) {
				headers = append(headers, map[string]any{"key": k, "value": toPostman(h.Headers[k]), "type": "text"})
			}
			req := map[string]any{
				"method": h.Method,
				"header": headers,
				"url":    map[string]any{"raw": raw, "query": query},
			}
			if h.Body != "" {
				req["body"] = map[string]any{"mode": "raw", "raw": toPostman(h.Body), "options": map[string]any{"raw": map[string]any{"language": "json"}}}
			}
			out = append(out, map[string]any{"name": f.name, "request": req})
		}
		return out
	}
	env := readEnv(hittableDir)
	var vars []map[string]string
	for _, k := range sortedKeys(env) {
		vars = append(vars, map[string]string{"key": k, "value": env[k], "type": "string"})
	}
	col := map[string]any{
		"info": map[string]any{
			"name":        name,
			"_postman_id": fmt.Sprintf("hittable-%d", time.Now().Unix()),
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		"item":     items(root),
		"variable": vars,
	}
	b, err := json.MarshalIndent(col, "", "  ")
	return append(b, '\n'), count, err
}

// ExportInsomnia writes an Insomnia v4 export for hittableDir.
func ExportInsomnia(hittableDir, name string) ([]byte, int, error) {
	root, count, err := readTree(hittableDir)
	if err != nil {
		return nil, 0, err
	}
	ts := time.Now().UnixMilli()
	seq := 0
	id := func(prefix string) string { seq++; return fmt.Sprintf("%s_hittable%d_%d", prefix, ts, seq) }
	wsID := id("wrk")
	resources := []any{map[string]any{"_type": "workspace", "_id": wsID, "parentId": nil, "name": name, "scope": "collection"}}
	var walk func(n *node, parent string)
	walk = func(n *node, parent string) {
		for _, d := range n.dirs {
			gid := id("fld")
			resources = append(resources, map[string]any{"_type": "request_group", "_id": gid, "parentId": parent, "name": d.name})
			walk(d, gid)
		}
		for _, f := range n.files {
			h := f.hit
			var headers []map[string]string
			for _, k := range sortedKeys(h.Headers) {
				headers = append(headers, map[string]string{"name": k, "value": toInsomnia(h.Headers[k])})
			}
			var params []map[string]string
			for _, k := range sortedKeys(h.Params) {
				params = append(params, map[string]string{"name": k, "value": toInsomnia(h.Params[k])})
			}
			body := map[string]any{}
			if h.Body != "" {
				mime := h.Headers["Content-Type"]
				if mime == "" {
					mime = "application/json"
				}
				body = map[string]any{"mimeType": mime, "text": toInsomnia(h.Body)}
			}
			resources = append(resources, map[string]any{
				"_type": "request", "_id": id("req"), "parentId": parent, "name": f.name,
				"method": h.Method, "url": toInsomnia(h.URL), "headers": headers, "parameters": params, "body": body,
			})
		}
	}
	walk(root, wsID)
	env := readEnv(hittableDir)
	data := map[string]any{}
	for k, v := range env {
		data[k] = v
	}
	resources = append(resources, map[string]any{"_type": "environment", "_id": id("env"), "parentId": wsID, "name": "Base Environment", "data": data})
	out := map[string]any{"_type": "export", "__export_format": 4, "__export_source": "hittable.sh", "resources": resources}
	b, err := json.MarshalIndent(out, "", "  ")
	return append(b, '\n'), count, err
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
