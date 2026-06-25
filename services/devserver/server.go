package devserver

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

	"golazy.dev/lazy/services/buildservice"
)

const ReloadPath = "/__lazy/reload"
const PanelPrefix = "/_golazy"
const PanelClientPath = "/_golazy/assets/panel.js"

var clientScript = []byte(`<script type="module" src="` + PanelClientPath + `"></script>`)

type Server struct {
	listener net.Listener
	server   *http.Server
	target   atomic.Value
	panel    http.Handler
	store    *buildservice.Store
	broker   *reloadBroker
}

func New(listenAddr string, panel http.Handler, store *buildservice.Store) (*Server, error) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	if panel == nil {
		panel = http.NotFoundHandler()
	}
	if store == nil {
		store = buildservice.NewStore(200)
	}
	server := &Server{
		listener: listener,
		panel:    panel,
		store:    store,
		broker:   newReloadBroker(),
	}
	server.server = &http.Server{Handler: server}
	return server, nil
}

func (s *Server) Start() error {
	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.store.AddEvent(buildservice.Event{Type: buildservice.EventState, Message: err.Error()})
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

func (s *Server) SetTarget(target string) {
	s.target.Store(target)
}

func (s *Server) ClearTarget() {
	s.target.Store("")
}

func (s *Server) BroadcastReload() {
	s.broker.broadcast()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == ReloadPath {
		s.broker.serveHTTP(w, r)
		return
	}
	if isPanelPath(r.URL.Path) {
		s.panel.ServeHTTP(w, r)
		return
	}

	target, _ := s.target.Load().(string)
	if target == "" {
		s.serveUnavailable(w, r)
		return
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		s.serveUnavailable(w, r)
		return
	}
	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.ModifyResponse = injectClient
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.ClearTarget()
		s.store.AddEvent(buildservice.Event{Type: buildservice.EventState, State: buildservice.StateRunFailed, Message: err.Error()})
		s.serveUnavailable(w, r)
	}
	reverseProxy.ServeHTTP(w, r)
}

func isPanelPath(path string) bool {
	return path == PanelPrefix || strings.HasPrefix(path, PanelPrefix+"/")
}

func (s *Server) serveUnavailable(w http.ResponseWriter, r *http.Request) {
	snapshot := s.store.Snapshot()
	body := unavailablePage(snapshot)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusServiceUnavailable)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func injectClient(response *http.Response) error {
	if !shouldInjectClient(response) {
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

func shouldInjectClient(response *http.Response) bool {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false
	}
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
	if bytes.Contains(body, []byte(PanelClientPath)) {
		return body
	}
	lower := bytes.ToLower(body)
	index := bytes.LastIndex(lower, []byte("</body>"))
	if index < 0 {
		return body
	}
	out := make([]byte, 0, len(body)+len(clientScript))
	out = append(out, body[:index]...)
	out = append(out, clientScript...)
	out = append(out, body[index:]...)
	return out
}

func unavailablePage(snapshot buildservice.Snapshot) []byte {
	var builder strings.Builder
	builder.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	builder.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	builder.WriteString("<title>GoLazy development panel</title>")
	builder.WriteString("<style>body{font-family:system-ui,sans-serif;margin:2rem;line-height:1.5;max-width:72rem}pre{white-space:pre-wrap;background:#111;color:#f4f4f4;padding:1rem;overflow:auto}a{font-weight:700}</style>")
	builder.WriteString("</head><body>")
	builder.WriteString("<h1>GoLazy development panel</h1>")
	builder.WriteString("<p>The application is not currently available. Open <a href=\"/_golazy/\">/_golazy/</a> for details.</p>")
	builder.WriteString("<p><strong>Status:</strong> ")
	builder.WriteString(html.EscapeString(string(snapshot.State)))
	builder.WriteString("</p>")
	if snapshot.Message != "" {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(snapshot.Message))
		builder.WriteString("</p>")
	}
	if snapshot.Output != "" {
		builder.WriteString("<pre>")
		builder.WriteString(html.EscapeString(snapshot.Output))
		builder.WriteString("</pre>")
	}
	builder.WriteString("</body></html>")
	return []byte(builder.String())
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
