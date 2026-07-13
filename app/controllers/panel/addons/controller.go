package addons

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontrolplane"
)

const (
	panelRequestTimeout   = 2 * time.Second
	maxDiscoveryBodyBytes = 256 << 10
	maxPanelBodyBytes     = 1 << 20
	maxActionBodyBytes    = 8 << 10
	panelContentPath      = "/_golazy/addons/content"
	panelActionPath       = "/_golazy/addons/action"
)

const panelContentSecurityPolicy = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; font-src data:; form-action 'none'; frame-ancestors 'self'; img-src data:; media-src 'none'; navigate-to 'none'; object-src 'none'; script-src 'none'; style-src 'unsafe-inline'"

var actionCSRFToken = mustActionCSRFToken()

type AddonsController struct {
	panel.Base
	timeout time.Duration
	client  *http.Client
}

type panelRow struct {
	ID          string
	Owner       string
	Title       string
	Description string
	Path        string
	URL         string
	ContentURL  string
	Actions     []panelActionRow
	Selected    bool
}

type panelActionRow struct {
	ID          string
	Title       string
	Description string
}

type panelContent struct {
	contentType string
	body        []byte
}

func New(ctx context.Context) (*AddonsController, error) {
	base, err := panel.NewBase(ctx)
	return &AddonsController{Base: base}, err
}

func (c *AddonsController) Index(_ http.ResponseWriter, r *http.Request) error {
	c.Set("state", c.Snapshot())
	c.Set("addon_action_path", panelActionPath)
	c.Set("addon_action_csrf_token", actionCSRFToken)
	panels, err := c.discover(r.Context())
	if err != nil {
		c.Set("addons_error", err.Error())
		c.Set("addon_panels", []panelRow{})
		return nil
	}

	rows := makePanelRows(panels, r.URL.Query().Get("panel"))
	c.Set("addon_panels", rows)
	for _, row := range rows {
		if row.Selected {
			c.Set("selected_addon_panel", row)
			c.Set("has_selected_addon_panel", true)
			break
		}
	}
	return nil
}

func (c *AddonsController) Content(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil
	}

	panelID := strings.TrimSpace(r.URL.Query().Get("panel"))
	if panelID == "" {
		http.Error(w, "panel is required", http.StatusBadRequest)
		return nil
	}
	panels, err := c.discover(r.Context())
	if err != nil {
		http.Error(w, "add-on panels are unavailable", http.StatusBadGateway)
		return nil
	}
	descriptor, ok := panelByID(panels, panelID)
	if !ok {
		http.Error(w, "add-on panel not found", http.StatusNotFound)
		return nil
	}

	content, err := c.fetchContent(r.Context(), r.Method, descriptor)
	if err != nil {
		http.Error(w, "add-on panel content is unavailable", http.StatusBadGateway)
		return nil
	}
	setSafePanelHeaders(w.Header(), content.contentType)
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return nil
	}
	_, err = w.Write(content.body)
	return err
}

// Action resolves a trusted panel/action descriptor pair and invokes the
// descriptor's exact POST endpoint. Only the IDs and process CSRF token are
// accepted from the browser; no request URL, query, headers, or body are
// forwarded to the application control plane.
func (c *AddonsController) Action(w http.ResponseWriter, r *http.Request) error {
	setSafePanelHeaders(w.Header(), "text/plain; charset=utf-8")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return nil
	}
	panelID, actionID, status, err := parseActionRequest(w, r)
	if err != nil {
		http.Error(w, err.Error(), status)
		return nil
	}
	panels, err := c.discover(r.Context())
	if err != nil {
		http.Error(w, "add-on panels are unavailable", http.StatusBadGateway)
		return nil
	}
	panelDescriptor, ok := panelByID(panels, panelID)
	if !ok {
		http.Error(w, "add-on panel not found", http.StatusNotFound)
		return nil
	}
	actionDescriptor, ok := actionByID(panelDescriptor.Actions, actionID)
	if !ok {
		http.Error(w, "add-on panel action not found", http.StatusNotFound)
		return nil
	}

	content, err := c.fetchEndpoint(r.Context(), http.MethodPost, actionDescriptor.Path)
	if err != nil {
		http.Error(w, "add-on panel action is unavailable", http.StatusBadGateway)
		return nil
	}
	setSafePanelHeaders(w.Header(), content.contentType)
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(content.body)
	return err
}

func (c *AddonsController) discover(ctx context.Context) ([]lazycontrolplane.PanelInfo, error) {
	target, err := c.controlPlaneURL(lazycontrolplane.PanelsPath)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := c.do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch panel discovery: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("panel discovery returned %s", response.Status)
	}
	if !allowedJSONContentType(response.Header.Get("Content-Type")) {
		return nil, fmt.Errorf("panel discovery returned unsupported content type")
	}
	body, err := readBounded(response.Body, maxDiscoveryBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read panel discovery: %w", err)
	}
	var discovered lazycontrolplane.PanelsResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&discovered); err != nil {
		return nil, fmt.Errorf("decode panel discovery: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, fmt.Errorf("decode panel discovery: trailing JSON data")
	}
	if discovered.Schema != 1 {
		return nil, fmt.Errorf("panel discovery schema %d is not supported", discovered.Schema)
	}
	if err := validatePanels(discovered.Panels); err != nil {
		return nil, err
	}
	panels := append([]lazycontrolplane.PanelInfo(nil), discovered.Panels...)
	sort.Slice(panels, func(i, j int) bool {
		if panels[i].Order != panels[j].Order {
			return panels[i].Order < panels[j].Order
		}
		if panels[i].Title != panels[j].Title {
			return panels[i].Title < panels[j].Title
		}
		return panels[i].ID < panels[j].ID
	})
	return panels, nil
}

func (c *AddonsController) fetchContent(ctx context.Context, method string, descriptor lazycontrolplane.PanelInfo) (panelContent, error) {
	return c.fetchEndpoint(ctx, method, descriptor.Path)
}

func (c *AddonsController) fetchEndpoint(ctx context.Context, method string, endpointPath string) (panelContent, error) {
	target, err := c.controlPlaneURL(endpointPath)
	if err != nil {
		return panelContent{}, err
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return panelContent{}, err
	}
	request.Header.Set("Accept", "text/html, application/json, text/plain")

	response, err := c.do(request)
	if err != nil {
		return panelContent{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return panelContent{}, fmt.Errorf("panel endpoint returned %s", response.Status)
	}
	contentType, err := safeContentType(response.Header.Get("Content-Type"))
	if err != nil {
		return panelContent{}, err
	}
	if method == http.MethodHead {
		return panelContent{contentType: contentType}, nil
	}
	body, err := readBounded(response.Body, maxPanelBodyBytes)
	if err != nil {
		return panelContent{}, err
	}
	if !utf8.Valid(body) {
		return panelContent{}, errors.New("panel endpoint returned invalid UTF-8")
	}
	return panelContent{contentType: contentType, body: body}, nil
}

func (c *AddonsController) do(request *http.Request) (*http.Response, error) {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = panelRequestTimeout
	}
	client := http.Client{}
	if c.client != nil {
		client = *c.client
	}
	if client.Timeout <= 0 || client.Timeout > timeout {
		client.Timeout = timeout
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client.Do(request)
}

func (c *AddonsController) controlPlaneURL(endpointPath string) (*url.URL, error) {
	addr := strings.TrimSpace(c.Snapshot().ControlPlaneAddr)
	if addr == "" {
		return nil, errors.New("application control plane is not available")
	}
	base, err := url.Parse("http://" + addr)
	if err != nil || base.Scheme != "http" || base.Host == "" || base.User != nil || base.Path != "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("application control plane address is invalid")
	}
	host := base.Hostname()
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("application control plane is not loopback")
	}
	base.Path = endpointPath
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	return base, nil
}

func validatePanels(panels []lazycontrolplane.PanelInfo) error {
	ids := make(map[string]struct{}, len(panels))
	for _, descriptor := range panels {
		if err := validatePanel(descriptor); err != nil {
			return err
		}
		if _, exists := ids[descriptor.ID]; exists {
			return fmt.Errorf("panel discovery contains duplicate ID %q", descriptor.ID)
		}
		ids[descriptor.ID] = struct{}{}
	}
	return nil
}

func validatePanel(descriptor lazycontrolplane.PanelInfo) error {
	if !safeDescriptorText(descriptor.ID, 128) {
		return errors.New("panel discovery contains an invalid panel ID")
	}
	if !safeDescriptorText(descriptor.Owner, 256) {
		return fmt.Errorf("panel %q has an invalid owner", descriptor.ID)
	}
	if !safeDescriptorText(descriptor.Title, 256) {
		return fmt.Errorf("panel %q has an invalid title", descriptor.ID)
	}
	if !safeDescriptorText(descriptor.Description, 2048) {
		return fmt.Errorf("panel %q has an invalid description", descriptor.ID)
	}
	if !safeDescriptorText(descriptor.EndpointID, 256) {
		return fmt.Errorf("panel %q has an invalid endpoint ID", descriptor.ID)
	}
	method := strings.ToUpper(strings.TrimSpace(descriptor.Method))
	if method != http.MethodGet && method != "ANY" {
		return fmt.Errorf("panel %q does not support GET", descriptor.ID)
	}
	if !safeExactPath(descriptor.Path) {
		return fmt.Errorf("panel %q has an unsafe endpoint path", descriptor.ID)
	}
	wantPattern := descriptor.Path
	if method == http.MethodGet {
		wantPattern = http.MethodGet + " " + descriptor.Path
	}
	if descriptor.Pattern != wantPattern {
		return fmt.Errorf("panel %q endpoint pattern does not match its path", descriptor.ID)
	}
	actionIDs := make(map[string]struct{}, len(descriptor.Actions))
	actionPatterns := make(map[string]struct{}, len(descriptor.Actions))
	for _, action := range descriptor.Actions {
		if err := validatePanelAction(descriptor.ID, action); err != nil {
			return err
		}
		if _, exists := actionIDs[action.ID]; exists {
			return fmt.Errorf("panel %q contains duplicate action ID %q", descriptor.ID, action.ID)
		}
		if _, exists := actionPatterns[action.Pattern]; exists {
			return fmt.Errorf("panel %q contains duplicate action method and path %q", descriptor.ID, action.Pattern)
		}
		actionIDs[action.ID] = struct{}{}
		actionPatterns[action.Pattern] = struct{}{}
	}
	return nil
}

func validatePanelAction(panelID string, action lazycontrolplane.PanelActionInfo) error {
	if !safeDescriptorText(action.ID, 128) {
		return fmt.Errorf("panel %q contains an invalid action ID", panelID)
	}
	if !safeDescriptorText(action.Title, 256) {
		return fmt.Errorf("panel %q action %q has an invalid title", panelID, action.ID)
	}
	if !safeDescriptorText(action.Description, 2048) {
		return fmt.Errorf("panel %q action %q has an invalid description", panelID, action.ID)
	}
	if !safeDescriptorText(action.EndpointID, 256) {
		return fmt.Errorf("panel %q action %q has an invalid endpoint ID", panelID, action.ID)
	}
	if action.Method != http.MethodPost {
		return fmt.Errorf("panel %q action %q is not POST-only", panelID, action.ID)
	}
	if !safeExactPath(action.Path) {
		return fmt.Errorf("panel %q action %q has an unsafe endpoint path", panelID, action.ID)
	}
	if action.Pattern != http.MethodPost+" "+action.Path {
		return fmt.Errorf("panel %q action %q endpoint pattern does not match its path", panelID, action.ID)
	}
	return nil
}

func safeDescriptorText(value string, max int) bool {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || value == "" || len(value) > max || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < ' ' || character == 0x7f {
			return false
		}
	}
	return true
}

func safeExactPath(value string) bool {
	if len(value) < 2 || len(value) > 2048 || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return false
	}
	if strings.ContainsAny(value, "\\?#%{}*") || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < ' ' || character == 0x7f {
			return false
		}
	}
	return path.Clean(value) == value
}

func makePanelRows(panels []lazycontrolplane.PanelInfo, selectedID string) []panelRow {
	selectedID = strings.TrimSpace(selectedID)
	if selectedID == "" && len(panels) > 0 {
		selectedID = panels[0].ID
	}
	rows := make([]panelRow, 0, len(panels))
	for _, descriptor := range panels {
		selected := descriptor.ID == selectedID
		rows = append(rows, panelRow{
			ID:          descriptor.ID,
			Owner:       descriptor.Owner,
			Title:       descriptor.Title,
			Description: descriptor.Description,
			Path:        descriptor.Path,
			URL:         addonPanelURL(descriptor.ID),
			ContentURL:  addonPanelContentURL(descriptor.ID),
			Actions:     makePanelActionRows(descriptor.Actions),
			Selected:    selected,
		})
	}
	return rows
}

func makePanelActionRows(actions []lazycontrolplane.PanelActionInfo) []panelActionRow {
	rows := make([]panelActionRow, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, panelActionRow{
			ID:          action.ID,
			Title:       action.Title,
			Description: action.Description,
		})
	}
	return rows
}

func panelByID(panels []lazycontrolplane.PanelInfo, id string) (lazycontrolplane.PanelInfo, bool) {
	for _, descriptor := range panels {
		if descriptor.ID == id {
			return descriptor, true
		}
	}
	return lazycontrolplane.PanelInfo{}, false
}

func actionByID(actions []lazycontrolplane.PanelActionInfo, id string) (lazycontrolplane.PanelActionInfo, bool) {
	for _, descriptor := range actions {
		if descriptor.ID == id {
			return descriptor, true
		}
	}
	return lazycontrolplane.PanelActionInfo{}, false
}

func parseActionRequest(w http.ResponseWriter, r *http.Request) (string, string, int, error) {
	if r.URL.RawQuery != "" {
		return "", "", http.StatusBadRequest, errors.New("action query parameters are not allowed")
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return "", "", http.StatusUnsupportedMediaType, errors.New("action form content type is required")
	}
	if err := validateSameOriginActionRequest(r); err != nil {
		return "", "", http.StatusForbidden, err
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxActionBodyBytes)
	if err := r.ParseForm(); err != nil {
		return "", "", http.StatusBadRequest, errors.New("invalid action form")
	}
	allowed := map[string]struct{}{"panel": {}, "action": {}, "csrf_token": {}}
	for key := range r.PostForm {
		if _, ok := allowed[key]; !ok {
			return "", "", http.StatusBadRequest, errors.New("unexpected action form field")
		}
	}
	panelID, ok := singleFormValue(r.PostForm, "panel")
	if !ok || !safeDescriptorText(panelID, 128) {
		return "", "", http.StatusBadRequest, errors.New("invalid panel ID")
	}
	actionID, ok := singleFormValue(r.PostForm, "action")
	if !ok || !safeDescriptorText(actionID, 128) {
		return "", "", http.StatusBadRequest, errors.New("invalid action ID")
	}
	token, ok := singleFormValue(r.PostForm, "csrf_token")
	if !ok || !validActionCSRFToken(token) {
		return "", "", http.StatusForbidden, errors.New("invalid action CSRF token")
	}
	return panelID, actionID, http.StatusOK, nil
}

func singleFormValue(values url.Values, key string) (string, bool) {
	items, ok := values[key]
	return firstSingle(items, ok)
}

func firstSingle(values []string, exists bool) (string, bool) {
	if !exists || len(values) != 1 {
		return "", false
	}
	return values[0], true
}

func validateSameOriginActionRequest(r *http.Request) error {
	if fetchSite := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); fetchSite != "" && fetchSite != "same-origin" {
		return errors.New("cross-origin action request is not allowed")
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !requestOriginMatches(r, origin, false) {
		return errors.New("action origin does not match the panel")
	}
	if referer := strings.TrimSpace(r.Header.Get("Referer")); referer != "" && !requestOriginMatches(r, referer, true) {
		return errors.New("action referrer does not match the panel")
	}
	return nil
}

func requestOriginMatches(r *http.Request, value string, allowPath bool) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if !allowPath && (parsed.Path != "" || parsed.RawQuery != "") {
		return false
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return parsed.Scheme == scheme && strings.EqualFold(parsed.Host, r.Host)
}

func mustActionCSRFToken() string {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		panic(fmt.Errorf("generate add-on action CSRF token: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func validActionCSRFToken(value string) bool {
	want := sha256.Sum256([]byte(actionCSRFToken))
	got := sha256.Sum256([]byte(value))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func addonPanelURL(id string) string {
	values := url.Values{"panel": {id}}
	return "/_golazy/addons?" + values.Encode()
}

func addonPanelContentURL(id string) string {
	values := url.Values{"panel": {id}}
	return panelContentPath + "?" + values.Encode()
}

func allowedJSONContentType(value string) bool {
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil || mediaType != "application/json" {
		return false
	}
	charset := strings.ToLower(strings.TrimSpace(parameters["charset"]))
	return charset == "" || charset == "utf-8"
}

func safeContentType(value string) (string, error) {
	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil {
		return "", errors.New("panel endpoint returned an invalid content type")
	}
	charset := strings.ToLower(strings.TrimSpace(parameters["charset"]))
	if charset != "" && charset != "utf-8" {
		return "", errors.New("panel endpoint returned a non-UTF-8 content type")
	}
	switch mediaType {
	case "text/html", "text/plain", "application/json":
		return mediaType + "; charset=utf-8", nil
	default:
		return "", errors.New("panel endpoint returned an unsupported content type")
	}
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maximum {
		return nil, errors.New("response body exceeds the limit")
	}
	return body, nil
}

func setSafePanelHeaders(header http.Header, contentType string) {
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", panelContentSecurityPolicy)
	header.Set("Content-Type", contentType)
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	header.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=(), usb=()")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "SAMEORIGIN")
}
