package devserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
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

	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazy/services/customcertservice"
)

const ReloadPath = "/__lazy/reload"
const PanelPrefix = "/_golazy"
const PanelClientPath = "/_golazy/assets/panel.js"
const HTTPSProbePath = "/_golazy/https-ready"
const CertificateDownloadPath = "/_golazy/local-development-ca.pem"
const requestIDHeader = "X-Request-ID"

var clientScript = []byte(`<script type="module" src="` + PanelClientPath + `"></script>`)

type Server struct {
	listener   net.Listener
	server     *http.Server
	target     atomic.Value
	panel      http.Handler
	store      *buildservice.Store
	broker     *reloadBroker
	localHTTPS bool

	certPaths     customcertservice.Paths
	certAuthority *customcertservice.Authority
	certOnce      sync.Once
	certErr       error
}

type Option func(*options)

type options struct {
	localHTTPS    bool
	certPaths     customcertservice.Paths
	certAuthority *customcertservice.Authority
}

func WithCertificatePaths(paths customcertservice.Paths) Option {
	return func(options *options) {
		options.certPaths = paths
	}
}

func WithCertificateAuthority(authority *customcertservice.Authority) Option {
	return func(options *options) {
		options.certAuthority = authority
	}
}

func WithLocalHTTPS(enabled bool) Option {
	return func(options *options) {
		options.localHTTPS = enabled
	}
}

func New(listenAddr string, panel http.Handler, store *buildservice.Store, opts ...Option) (*Server, error) {
	config := options{localHTTPS: true}
	for _, option := range opts {
		option(&config)
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	paths := config.certPaths
	if config.certAuthority != nil {
		paths = config.certAuthority.Paths()
	} else if config.localHTTPS && paths.Certificate == "" && paths.PrivateKey == "" && paths.Dir == "" {
		paths, err = customcertservice.DefaultPaths()
		if err != nil {
			_ = listener.Close()
			return nil, err
		}
	}
	if panel == nil {
		panel = http.NotFoundHandler()
	}
	if store == nil {
		store = buildservice.NewStore(200)
	}
	server := &Server{
		listener:      listener,
		panel:         panel,
		store:         store,
		broker:        newReloadBroker(),
		certPaths:     paths,
		certAuthority: config.certAuthority,
		localHTTPS:    config.localHTTPS,
	}
	server.server = &http.Server{Handler: server}
	return server, nil
}

func (s *Server) Start() error {
	listener := s.listener
	if s.localHTTPS {
		authority, err := s.certificateAuthority()
		if err != nil {
			return err
		}
		tlsConfig, err := authority.TLSConfig(s.Addr())
		if err != nil {
			return err
		}
		s.server.TLSConfig = tlsConfig
		listener = mixedProtocolListener{Listener: s.listener, TLSConfig: tlsConfig}
	}
	go func() {
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
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
	if s.localHTTPS && r.TLS == nil {
		s.serveHTTP(w, r)
		return
	}
	if r.URL.Path == HTTPSProbePath {
		serveHTTPSProbe(w, r)
		return
	}
	if r.URL.Path == ReloadPath {
		s.broker.serveHTTP(w, r)
		return
	}
	if r.URL.Path == PanelPrefix+"/" {
		r = requestWithPath(r, PanelPrefix)
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
	director := reverseProxy.Director
	reverseProxy.Director = func(request *http.Request) {
		director(request)
		ensureRequestTraceHeaders(request)
	}
	reverseProxy.ModifyResponse = injectClient
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		s.ClearTarget()
		s.store.AddEvent(buildservice.Event{Type: buildservice.EventState, State: buildservice.StateRunFailed, Message: err.Error()})
		s.serveUnavailable(w, r)
	}
	reverseProxy.ServeHTTP(w, r)
}

func (s *Server) certificateAuthority() (*customcertservice.Authority, error) {
	s.certOnce.Do(func() {
		if s.certAuthority != nil {
			return
		}
		s.certAuthority, s.certErr = customcertservice.LoadOrCreate(s.certPaths)
	})
	return s.certAuthority, s.certErr
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == CertificateDownloadPath:
		s.serveCertificateDownload(w, r)
	case isHTTPWelcomeAsset(r.URL.Path):
		s.panel.ServeHTTP(w, r)
	default:
		s.serveCertificateWelcome(w, r)
	}
}

func isHTTPWelcomeAsset(path string) bool {
	switch path {
	case "/_golazy/assets/golazy-mark.svg", "/_golazy/assets/golazy-horizontal.svg":
		return true
	default:
		return false
	}
}

func serveHTTPSProbe(w http.ResponseWriter, r *http.Request) {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) serveCertificateDownload(w http.ResponseWriter, r *http.Request) {
	authority, err := s.certificateAuthority()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body := authority.CertificatePEM()
	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", `attachment; filename="golazy-local-development-ca.pem"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func (s *Server) serveCertificateWelcome(w http.ResponseWriter, r *http.Request) {
	variant := instructionVariantForRequest(r)
	body := certificateWelcomePage(certificateWelcomeData{
		Variant:          variant,
		Host:             customcertservice.NormalizeHost(r.Host),
		HTTPSURL:         httpsURLForRequest(r),
		HTTPSProbeURL:    httpsProbeURLForRequest(r),
		CertificateURL:   CertificateDownloadPath,
		CertificatePath:  s.certPaths.Certificate,
		PrivateKeyPath:   s.certPaths.PrivateKey,
		InstructionLinks: instructionLinksForRequest(r),
		RequestedVariant: strings.TrimSpace(r.URL.Query().Get("os")),
		CertificateDir:   s.certPaths.Dir,
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

type mixedProtocolListener struct {
	net.Listener
	TLSConfig *tls.Config
}

func (l mixedProtocolListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		var first [1]byte
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(first[:])
		_ = conn.SetReadDeadline(time.Time{})
		if err != nil || n == 0 {
			_ = conn.Close()
			continue
		}
		peeked := &peekedConn{Conn: conn, prefix: first[:n]}
		if first[0] == 0x16 {
			return tls.Server(peeked, l.TLSConfig), nil
		}
		return peeked, nil
	}
}

type peekedConn struct {
	net.Conn
	prefix []byte
}

func (c *peekedConn) Read(p []byte) (int, error) {
	if len(c.prefix) > 0 {
		n := copy(p, c.prefix)
		c.prefix = c.prefix[n:]
		return n, nil
	}
	return c.Conn.Read(p)
}

func ensureRequestTraceHeaders(r *http.Request) {
	requestID := requestIDFromHeader(r.Header.Get(requestIDHeader))
	if requestID == "" {
		requestID = generateRequestID()
		r.Header.Set(requestIDHeader, requestID)
	}
	if !validTraceparent(r.Header.Get("traceparent")) {
		r.Header.Set("traceparent", generateTraceparent())
		r.Header.Del("tracestate")
	}
}

func requestIDFromHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.Contains(value, ",") {
		return ""
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' ||
			char == '_' || char == '-' || char == '.' || char == ':' || char == '/' {
			continue
		}
		return ""
	}
	return value
}

func generateRequestID() string {
	return base64.RawURLEncoding.EncodeToString(randomBytes(16))
}

func generateTraceparent() string {
	return "00-" + hex.EncodeToString(randomBytes(16)) + "-" + hex.EncodeToString(randomBytes(8)) + "-01"
}

func validTraceparent(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 {
		return false
	}
	version, traceID, spanID, flags := parts[0], strings.ToLower(parts[1]), strings.ToLower(parts[2]), strings.ToLower(parts[3])
	if len(version) != 2 || len(traceID) != 32 || len(spanID) != 16 || len(flags) != 2 {
		return false
	}
	return version != "ff" &&
		isLowerHex(version) &&
		isLowerHex(traceID) &&
		isLowerHex(spanID) &&
		isLowerHex(flags) &&
		!allZero(traceID) &&
		!allZero(spanID)
}

func randomBytes(size int) []byte {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		panic(fmt.Errorf("devserver: generate trace identifiers: %w", err))
	}
	return data
}

func isLowerHex(value string) bool {
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func allZero(value string) bool {
	for _, char := range value {
		if char != '0' {
			return false
		}
	}
	return true
}

func isPanelPath(path string) bool {
	return path == PanelPrefix || strings.HasPrefix(path, PanelPrefix+"/")
}

func requestWithPath(r *http.Request, path string) *http.Request {
	clone := r.Clone(r.Context())
	url := *clone.URL
	url.Path = path
	url.RawPath = ""
	clone.URL = &url
	return clone
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
