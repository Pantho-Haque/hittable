package exthost

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// WebviewServer shows extension webview panels (HTML pages) in the user's
// browser, since a terminal cannot render them. It listens on 127.0.0.1
// only, and every URL carries a random token, so other local pages and
// users cannot reach a panel or the files it may read. Messages from the
// page come in as POSTs; messages to it go out over server-sent events.
type WebviewServer struct {
	ln    net.Listener
	token string
	host  string // 127.0.0.1:port
	srv   *http.Server

	mu    sync.Mutex
	views map[string]*webview

	// OnMessage receives a message posted by a panel's page.
	OnMessage func(handle string, msg json.RawMessage)
	colors    map[string]string // --vscode-* CSS properties of the active theme
}

// SetColors sets the --vscode-* CSS properties pages get (the active theme).
func (s *WebviewServer) SetColors(c map[string]string) {
	s.mu.Lock()
	s.colors = c
	s.mu.Unlock()
}

type webview struct {
	handle, title, html string
	roots               []string
	clients             map[chan sse]struct{}
	queue               []sse // messages sent before the page connected
	closed              bool
}

type sse struct{ event, data string }

// StartWebviewServer listens on a free loopback port.
func StartWebviewServer() (*WebviewServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	s := &WebviewServer{ln: ln, token: hex.EncodeToString(b), host: ln.Addr().String(), views: map[string]*webview{}}
	s.srv = &http.Server{Handler: s}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// Base is the URL prefix every panel URL starts with.
func (s *WebviewServer) Base() string { return "http://" + s.host + "/" + s.token }

// URL is the page for one panel.
func (s *WebviewServer) URL(handle string) string { return s.Base() + "/webview/" + handle }

// Close stops the server.
func (s *WebviewServer) Close() { _ = s.srv.Close() }

// Create registers a panel. roots are folders its page may load files from.
func (s *WebviewServer) Create(handle, title string, roots []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.views[handle] = &webview{handle: handle, title: title, roots: roots, clients: map[chan sse]struct{}{}}
}

// SetHTML replaces a panel's page; open browser tabs reload.
func (s *WebviewServer) SetHTML(handle, doc string) {
	s.push(handle, func(v *webview) sse { v.html = doc; return sse{"reload", "1"} }, false)
}

// SetTitle renames a panel.
func (s *WebviewServer) SetTitle(handle, title string) {
	t, _ := json.Marshal(title)
	s.push(handle, func(v *webview) sse { v.title = title; return sse{"title", string(t)} }, false)
}

// Post delivers a message to the page (window "message" event).
func (s *WebviewServer) Post(handle string, msg json.RawMessage) {
	s.push(handle, func(*webview) sse { return sse{"post", string(msg)} }, true)
}

// Dispose closes a panel; open tabs show that it was closed.
func (s *WebviewServer) Dispose(handle string) {
	s.mu.Lock()
	v := s.views[handle]
	delete(s.views, handle)
	s.mu.Unlock()
	if v == nil {
		return
	}
	for c := range v.clients {
		select {
		case c <- sse{"closed", "1"}:
		default:
		}
	}
}

// Title is a panel's current title ("" when it does not exist).
func (s *WebviewServer) Title(handle string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v := s.views[handle]; v != nil {
		return v.title
	}
	return ""
}

func (s *WebviewServer) push(handle string, f func(*webview) sse, queue bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.views[handle]
	if v == nil {
		return
	}
	e := f(v)
	if len(v.clients) == 0 {
		if queue {
			v.queue = append(v.queue, e)
		}
		return
	}
	for c := range v.clients {
		select {
		case c <- e:
		default: // a stalled tab drops a message rather than blocking the UI
		}
	}
}

func (s *WebviewServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A page on another site that resolves its own name to 127.0.0.1 (DNS
	// rebinding) would send its own Host header; only ours is served.
	if r.Host != s.host && r.Host != strings.Replace(s.host, "127.0.0.1", "localhost", 1) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/"+s.token+"/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	kind, tailPath, _ := strings.Cut(rest, "/")
	switch kind {
	case "webview":
		s.servePage(w, tailPath)
	case "events":
		s.serveEvents(w, r, tailPath)
	case "post":
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
		if err != nil || !json.Valid(body) {
			http.Error(w, "bad message", http.StatusBadRequest)
			return
		}
		if s.OnMessage != nil {
			s.OnMessage(tailPath, json.RawMessage(body))
		}
		w.WriteHeader(http.StatusNoContent)
	case "res":
		handle, file, _ := strings.Cut(tailPath, "/")
		s.serveResource(w, r, handle, "/"+file)
	default:
		http.NotFound(w, r)
	}
}

func (s *WebviewServer) servePage(w http.ResponseWriter, handle string) {
	s.mu.Lock()
	v := s.views[handle]
	var doc, title string
	if v != nil {
		doc, title = v.html, v.title
	}
	s.mu.Unlock()
	if v == nil {
		http.Error(w, "This panel is closed.", http.StatusNotFound)
		return
	}
	if strings.TrimSpace(doc) == "" {
		doc = "<!DOCTYPE html><html><head><title>" + html.EscapeString(title) + "</title></head><body><p>Loading…</p></body></html>"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, s.inject(doc, handle, title))
}

// attrEscape quotes a double-quoted attribute value without touching the
// single quotes CSP keywords ('self', 'sha256-…') are written with.
var attrEscape = strings.NewReplacer(`&`, "&amp;", `"`, "&quot;", `<`, "&lt;", `>`, "&gt;")

var (
	cspMeta = regexp.MustCompile(`(?is)<meta\s+[^>]*http-equiv\s*=\s*["']?content-security-policy["']?[^>]*>`)
	cspAttr = regexp.MustCompile(`(?is)content\s*=\s*("([^"]*)"|'([^']*)')`)
	headTag = regexp.MustCompile(`(?is)<head[^>]*>`)
)

// inject adds the acquireVsCodeApi() bridge and theme variables ahead of the
// page's own scripts, and widens the page's Content-Security-Policy just
// enough for them: the bridge script's hash, and this server as a
// connect-src for the message channel.
func (s *WebviewServer) inject(doc, handle, title string) string {
	boot := bootScript(s.Base(), handle)
	sum := sha256.Sum256([]byte(boot))
	hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	origin := "http://" + s.host

	doc = cspMeta.ReplaceAllStringFunc(doc, func(tag string) string {
		m := cspAttr.FindStringSubmatchIndex(tag)
		if m == nil {
			return tag
		}
		var policy string
		if m[4] >= 0 {
			policy = tag[m[4]:m[5]]
		} else {
			policy = tag[m[6]:m[7]]
		}
		policy = widenCSP(html.UnescapeString(policy), hash, origin)
		return tag[:m[0]] + `content="` + attrEscape.Replace(policy) + `"` + tag[m[1]:]
	})
	head := "<script>" + boot + "</script><style>" + s.themeCSS() + "</style>"
	if loc := headTag.FindStringIndex(doc); loc != nil {
		return doc[:loc[1]] + head + doc[loc[1]:]
	}
	return "<!DOCTYPE html><html><head><title>" + html.EscapeString(title) + "</title>" + head + "</head><body>" + doc + "</body></html>"
}

func widenCSP(policy, scriptHash, origin string) string {
	dirs := map[string]string{}
	var order []string
	for _, d := range strings.Split(policy, ";") {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		name, val, _ := strings.Cut(d, " ")
		name = strings.ToLower(name)
		if _, ok := dirs[name]; !ok {
			order = append(order, name)
		}
		dirs[name] = strings.TrimSpace(val)
	}
	add := func(name, src string) {
		v, ok := dirs[name]
		if !ok {
			v = dirs["default-src"]
			order = append(order, name)
		}
		if strings.Contains(v, "'none'") {
			v = strings.TrimSpace(strings.ReplaceAll(v, "'none'", ""))
		}
		dirs[name] = strings.TrimSpace(v + " " + src)
	}
	add("script-src", scriptHash)
	add("connect-src", origin)
	add("style-src", "'unsafe-inline'")
	var out []string
	for _, n := range order {
		out = append(out, strings.TrimSpace(n+" "+dirs[n]))
	}
	return strings.Join(out, "; ")
}

func bootScript(base, handle string) string {
	b, _ := json.Marshal(base)
	h, _ := json.Marshal(handle)
	return `(function(){var base=` + string(b) + `,handle=` + string(h) + `;var key="hittable-state-"+handle;var state=null;try{state=JSON.parse(localStorage.getItem(key)||"null")}catch(e){}
var api={postMessage:function(m){fetch(base+"/post/"+handle,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(m===undefined?null:m)});return true},getState:function(){return state},setState:function(s){state=s;try{localStorage.setItem(key,JSON.stringify(s))}catch(e){}return s}};
var got=false;window.acquireVsCodeApi=function(){if(got)throw new Error("An instance of the VS Code API has already been acquired");got=true;return api};
var es=new EventSource(base+"/events/"+handle);es.addEventListener("post",function(e){window.dispatchEvent(new MessageEvent("message",{data:JSON.parse(e.data)}))});
es.addEventListener("reload",function(){location.reload()});es.addEventListener("title",function(e){document.title=JSON.parse(e.data)});
es.addEventListener("closed",function(){es.close();document.body.innerHTML='<p style="font:14px sans-serif;padding:2em">This panel was closed in hittable.</p>'});
document.addEventListener("DOMContentLoaded",function(){document.body.classList.add("vscode-dark");document.body.setAttribute("data-vscode-theme-kind","vscode-dark")});})();`
}

func (s *WebviewServer) themeCSS() string {
	c := map[string]string{
		"--vscode-font-family":              "-apple-system, BlinkMacSystemFont, 'Segoe UI', Ubuntu, sans-serif",
		"--vscode-font-size":                "13px",
		"--vscode-font-weight":              "normal",
		"--vscode-editor-font-family":       "'SF Mono', Menlo, Consolas, monospace",
		"--vscode-editor-font-size":         "13px",
		"--vscode-editor-background":        "#282a36",
		"--vscode-editor-foreground":        "#f8f8f2",
		"--vscode-foreground":               "#f8f8f2",
		"--vscode-descriptionForeground":    "#bfbfbf",
		"--vscode-focusBorder":              "#8be9fd",
		"--vscode-button-background":        "#44475a",
		"--vscode-button-foreground":        "#f8f8f2",
		"--vscode-button-hoverBackground":   "#6272a4",
		"--vscode-input-background":         "#21222c",
		"--vscode-input-foreground":         "#f8f8f2",
		"--vscode-input-border":             "#44475a",
		"--vscode-textLink-foreground":      "#8be9fd",
		"--vscode-sideBar-background":       "#21222c",
		"--vscode-panel-border":             "#44475a",
		"--vscode-list-hoverBackground":     "#44475a",
		"--vscode-badge-background":         "#bd93f9",
		"--vscode-badge-foreground":         "#282a36",
		"--vscode-errorForeground":          "#ff5555",
		"--vscode-textCodeBlock-background": "#21222c",
	}
	s.mu.Lock()
	for k, v := range s.colors {
		c[k] = v
	}
	s.mu.Unlock()
	var b strings.Builder
	b.WriteString(":root{")
	for k, v := range c {
		fmt.Fprintf(&b, "%s:%s;", k, v)
	}
	b.WriteString("}body{background:var(--vscode-editor-background);color:var(--vscode-foreground);font-family:var(--vscode-font-family);font-size:var(--vscode-font-size);margin:0;padding:0 20px}")
	return b.String()
}

func (s *WebviewServer) serveEvents(w http.ResponseWriter, r *http.Request, handle string) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch := make(chan sse, 256)
	s.mu.Lock()
	v := s.views[handle]
	if v == nil {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	v.clients[ch] = struct{}{}
	queued := v.queue
	v.queue = nil
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if v := s.views[handle]; v != nil {
			delete(v.clients, ch)
		}
		s.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	write := func(e sse) {
		fmt.Fprintf(w, "event: %s\n", e.event)
		for _, line := range strings.Split(e.data, "\n") {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		fmt.Fprint(w, "\n")
		fl.Flush()
	}
	fmt.Fprint(w, ": connected\n\n")
	fl.Flush()
	for _, e := range queued {
		write(e)
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			write(e)
			if e.event == "closed" {
				return
			}
		}
	}
}

// serveResource serves a file a page asked for via asWebviewUri, but only
// from inside the panel's localResourceRoots.
func (s *WebviewServer) serveResource(w http.ResponseWriter, r *http.Request, handle, encoded string) {
	s.mu.Lock()
	v := s.views[handle]
	var roots []string
	if v != nil {
		roots = v.roots
	}
	s.mu.Unlock()
	if v == nil {
		http.NotFound(w, r)
		return
	}
	p, err := url.PathUnescape(encoded)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if runtime.GOOS == "windows" {
		p = strings.TrimPrefix(p, "/")
	}
	p = realPath(filepath.Clean(filepath.FromSlash(p)))
	allowed := false
	for _, root := range roots {
		root = realPath(filepath.Clean(root))
		if p == root || strings.HasPrefix(p, root+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "outside the panel's resource roots", http.StatusForbidden)
		return
	}
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, p)
}

// OpenBrowser opens url in the user's browser. It reports false when there
// is no browser to open (an SSH session without a display), so the caller
// can show the URL instead.
func OpenBrowser(u string) bool {
	if os.Getenv("SSH_CONNECTION") != "" && os.Getenv("DISPLAY") == "" && runtime.GOOS != "darwin" {
		return false
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		if b, err := os.ReadFile("/proc/version"); err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft") {
			if _, err := exec.LookPath("wslview"); err == nil {
				cmd = exec.Command("wslview", u)
			} else {
				cmd = exec.Command("cmd.exe", "/c", "start", strings.ReplaceAll(u, "&", "^&"))
			}
		} else if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		} else {
			cmd = exec.Command("xdg-open", u)
		}
	}
	return cmd.Start() == nil
}

// realPath resolves symlinks so a root and a file inside it compare equal
// however each was spelled (/var and /private/var on macOS). A path that
// does not exist is returned as given.
func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
