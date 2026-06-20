package reloadproxy

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const reloadPath = "/__lazy/reload"

var reloadScript = []byte(`<script>
(() => {
  if (window.__lazyReloadSource) return;
  const source = new EventSource("` + reloadPath + `");
  window.__lazyReloadSource = source;
  source.addEventListener("reload", () => window.location.reload());
})();
</script>`)

type State string

const (
	StateQueued      State = "queued"
	StateBuilding    State = "building"
	StateBuildFailed State = "build_failed"
	StateRunning     State = "running"
	StateRunFailed   State = "run_failed"
	StateStopped     State = "stopped"
)

type Status struct {
	State       State
	Message     string
	CommandPath string
	WatchedRoot string
	BuildCount  int
	Duration    time.Duration
	StartedAt   time.Time
	Output      string
	Changed     []string
}

type Proxy struct {
	listener net.Listener
	server   *http.Server
	target   atomic.Value
	broker   *reloadBroker

	statusMu sync.RWMutex
	status   Status
}

func New(listenAddr string) (*Proxy, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	proxy := &Proxy{
		listener: listener,
		broker:   newReloadBroker(),
	}
	proxy.server = &http.Server{Handler: proxy}
	return proxy, nil
}

func (p *Proxy) Start() error {
	go func() {
		if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
			// The next proxied request will surface the failure through the dev page.
		}
	}()
	return nil
}

func (p *Proxy) Shutdown(ctx context.Context) error {
	return p.server.Shutdown(ctx)
}

func (p *Proxy) Addr() string {
	return p.listener.Addr().String()
}

func (p *Proxy) SetTarget(target string) {
	p.target.Store(target)
}

func (p *Proxy) ClearTarget() {
	p.target.Store("")
}

func (p *Proxy) UpdateStatus(status Status) {
	p.statusMu.Lock()
	p.status = status
	p.statusMu.Unlock()
}

func (p *Proxy) statusSnapshot() Status {
	p.statusMu.RLock()
	defer p.statusMu.RUnlock()
	return p.status
}

func (p *Proxy) BroadcastReload() {
	p.broker.broadcast()
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == reloadPath {
		p.broker.serveHTTP(w, r)
		return
	}

	target, _ := p.target.Load().(string)
	if target == "" {
		p.serveStatusPage(w, r)
		return
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		p.serveStatusPage(w, r)
		return
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.ModifyResponse = injectReloadClient
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.ClearTarget()
		p.serveStatusPage(w, r)
	}
	reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) serveStatusPage(w http.ResponseWriter, r *http.Request) {
	status := p.statusSnapshot()
	body := statusPage(status)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusServiceUnavailable)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func injectReloadClient(response *http.Response) error {
	if !shouldInjectReloadClient(response) {
		return nil
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	_ = response.Body.Close()

	body = injectScript(body)
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	response.Header.Del("ETag")
	return nil
}

func shouldInjectReloadClient(response *http.Response) bool {
	if !isHTMLResponse(response) {
		return false
	}
	if response.Request != nil && strings.TrimSpace(response.Request.Header.Get("Turbo-Frame")) != "" {
		return false
	}
	return true
}

func isHTMLResponse(response *http.Response) bool {
	contentType := response.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.EqualFold(mediaType, "text/html")
	}
	return strings.Contains(strings.ToLower(contentType), "text/html")
}

func injectScript(body []byte) []byte {
	if bytes.Contains(body, []byte("__lazyReloadSource")) {
		return body
	}
	lower := bytes.ToLower(body)
	index := bytes.LastIndex(lower, []byte("</body>"))
	if index < 0 {
		return body
	}
	out := make([]byte, 0, len(body)+len(reloadScript))
	out = append(out, body[:index]...)
	out = append(out, reloadScript...)
	out = append(out, body[index:]...)
	return out
}

func statusPage(status Status) []byte {
	var builder strings.Builder
	builder.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	builder.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	builder.WriteString("<title>lazy</title>")
	builder.WriteString("<style>body{font-family:system-ui,sans-serif;margin:2rem;line-height:1.5;max-width:72rem}pre{white-space:pre-wrap;background:#111;color:#f4f4f4;padding:1rem;overflow:auto}code{background:#eee;padding:.1rem .25rem}</style>")
	builder.WriteString("</head><body>")
	builder.WriteString("<h1>lazy</h1>")
	builder.WriteString("<p><strong>Status:</strong> ")
	builder.WriteString(html.EscapeString(string(status.State)))
	builder.WriteString("</p>")
	if status.Message != "" {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(status.Message))
		builder.WriteString("</p>")
	}
	builder.WriteString("<dl>")
	writeStatusItem(&builder, "Command", status.CommandPath)
	writeStatusItem(&builder, "Watched root", status.WatchedRoot)
	if status.BuildCount > 0 {
		writeStatusItem(&builder, "Build", strconv.Itoa(status.BuildCount))
	}
	if status.Duration > 0 {
		writeStatusItem(&builder, "Duration", status.Duration.Round(time.Millisecond).String())
	}
	if !status.StartedAt.IsZero() {
		writeStatusItem(&builder, "Started", status.StartedAt.Format(time.RFC3339))
	}
	builder.WriteString("</dl>")
	if len(status.Changed) > 0 {
		builder.WriteString("<h2>Changed files</h2><ul>")
		for _, path := range status.Changed {
			builder.WriteString("<li>")
			builder.WriteString(html.EscapeString(path))
			builder.WriteString("</li>")
		}
		builder.WriteString("</ul>")
	}
	if status.Output != "" {
		builder.WriteString("<h2>Build output</h2><pre>")
		builder.WriteString(html.EscapeString(status.Output))
		builder.WriteString("</pre>")
	}
	builder.Write(reloadScript)
	builder.WriteString("</body></html>")
	return []byte(builder.String())
}

func writeStatusItem(builder *strings.Builder, name string, value string) {
	if value == "" {
		return
	}
	builder.WriteString("<dt>")
	builder.WriteString(html.EscapeString(name))
	builder.WriteString("</dt><dd><code>")
	builder.WriteString(html.EscapeString(value))
	builder.WriteString("</code></dd>")
}

type reloadBroker struct {
	mu      sync.Mutex
	clients map[chan struct{}]struct{}
}

func newReloadBroker() *reloadBroker {
	return &reloadBroker{clients: map[chan struct{}]struct{}{}}
}

func (b *reloadBroker) serveHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	events := make(chan struct{}, 1)
	b.mu.Lock()
	b.clients[events] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.clients, events)
		b.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprint(w, "event: ready\ndata: ok\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-events:
			fmt.Fprint(w, "event: reload\ndata: now\n\n")
			flusher.Flush()
		}
	}
}

func (b *reloadBroker) broadcast() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for events := range b.clients {
		select {
		case events <- struct{}{}:
		default:
		}
	}
}
