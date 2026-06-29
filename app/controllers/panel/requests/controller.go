package requests

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazysse"
	"golazy.dev/lazyturbo"
)

const appRequestTracesPath = "/requests/traces"

type RequestsController struct {
	panel.Base
}

func New(ctx context.Context) (*RequestsController, error) {
	base, err := panel.NewBase(ctx)
	return &RequestsController{Base: base}, err
}

func (c *RequestsController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setRequestsState(r)
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.TurboFrame: func() error {
			return c.renderRequestDetailFrame(w, r)
		},
		lazycontroller.SSE: func() error {
			return c.streamRequests(w, r)
		},
	})
}

func (c *RequestsController) setRequestsState(r *http.Request) {
	c.Set("state", c.Snapshot())
	c.Set("monitoring", c.RequestMonitoringSnapshot(r.Context()))
	c.Set("cache", c.CacheSnapshot(r.Context()))
	c.Set("requests", c.requestView(r))
}

func (c *RequestsController) streamRequestsInitial(r *http.Request) (string, error) {
	view := c.requestView(r)
	return c.renderRequestsSnapshot(r, view, true)
}

func (c *RequestsController) streamRequests(_ http.ResponseWriter, r *http.Request) error {
	stream, err := c.SSEStream()
	if err != nil {
		return err
	}
	defer stream.Close()
	stream.Heartbeat(15 * time.Second)

	view := c.requestView(r)
	previous := requestSnapshotKey(view)
	initial, err := c.renderRequestsSnapshot(r, view, true)
	if err != nil {
		return err
	}
	if err := sendRequestTurboStream(stream, initial); err != nil {
		return err
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Done():
			return nil
		case <-ticker.C:
			view := c.requestView(r)
			next := requestSnapshotKey(view)
			if next == previous {
				continue
			}
			previous = next
			body, err := c.renderRequestsSnapshot(r, view, false)
			if err != nil || body == "" {
				continue
			}
			if err := sendRequestTurboStream(stream, body); err != nil {
				return err
			}
		}
	}
}

func (c *RequestsController) renderRequestsSnapshot(r *http.Request, view requestView, clear bool) (string, error) {
	body, err := c.RenderPanelPartial(r, "requests", "request_rows", map[string]any{
		"state":      c.Snapshot(),
		"monitoring": c.RequestMonitoringSnapshot(r.Context()),
		"cache":      c.CacheSnapshot(r.Context()),
		"requests":   view,
	})
	if err != nil {
		return "", err
	}
	prefix := ""
	if clear {
		prefix = panel.TurboStreamTargets("update", "[data-request-list]", "") +
			panel.TurboStreamTargets("update", "[data-request-count]", "0 requests")
	}
	return prefix +
		panel.TurboStreamTargets("update", "[data-request-list]", body) +
		panel.TurboStreamTargets("update", "[data-request-count]", view.RequestCountText()), nil
}

func sendRequestTurboStream(stream *lazysse.Stream, body string) error {
	if body == "" {
		return nil
	}
	return stream.Send(lazysse.Event{Data: []string{body}})
}

func (c *RequestsController) renderRequestDetailFrame(w http.ResponseWriter, r *http.Request) error {
	if frameID := lazyturbo.FrameID(r); frameID != "request_detail" {
		return lazycontroller.Error(http.StatusBadRequest, fmt.Errorf("request frame %q is not available", frameID))
	}
	view := c.requestView(r)
	body, err := c.RenderPanelFrame(r, "request_detail", "requests", "request_detail", map[string]any{
		"state":      c.Snapshot(),
		"monitoring": c.RequestMonitoringSnapshot(r.Context()),
		"cache":      c.CacheSnapshot(r.Context()),
		"requests":   view,
	})
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = w.Write([]byte(body))
	return err
}

func (c *RequestsController) requestView(r *http.Request) requestView {
	var snapshot requestTraceSnapshot
	view := requestView{
		Directory: ".tmp/traces",
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Tab:       normalizeRequestTab(r.URL.Query().Get("tab")),
		Type:      normalizeRequestType(r.URL.Query().Get("type")),
		Framework: r.URL.Query().Get("framework") == "1",
	}
	if err := c.FetchAppControlJSON(r.Context(), view.ControlPlanePath(), &snapshot); err != nil {
		view.Error = err.Error()
		return view
	}
	if snapshot.Directory != "" {
		view.Directory = snapshot.Directory
	}
	view.Errors = snapshot.Errors
	view.Requests = snapshot.Traces
	selected := selectRequestTrace(view.Requests, r.URL.Query().Get("request"))
	if selected == nil {
		return view
	}
	view.Selected = *selected
	view.HasSelected = true
	visible := visibleRequestSpans(selected.Spans, view.Framework)
	selectedSpan := selectRequestSpan(visible, r.URL.Query().Get("span"))
	if selectedSpan != nil {
		view.SelectedSpan = *selectedSpan
		view.HasSelectedSpan = true
	}
	view.Rows = requestRows(view.Requests, selected.RequestID, view.Query, view.Tab, view.Type, view.Framework)
	view.TimelineRows = requestSpanRows(*selected, visible, selectedRequestSpanID(selectedSpan), view.Query, view.Tab, view.Type, view.Framework)
	view.FlameRows = requestFlameRows(*selected, visible, selectedRequestSpanID(selectedSpan), view.Query, view.Tab, view.Type, view.Framework)
	view.Logs = selected.Logs
	return view
}

func requestSnapshotKey(view requestView) string {
	var builder strings.Builder
	builder.WriteString(view.Error)
	builder.WriteString("\n")
	builder.WriteString(strings.Join(view.Errors, "\n"))
	builder.WriteString("\n")
	for _, trace := range view.Requests {
		builder.WriteString(trace.RequestID)
		builder.WriteString("|")
		builder.WriteString(trace.Method)
		builder.WriteString("|")
		builder.WriteString(trace.Path)
		builder.WriteString("|")
		builder.WriteString(strconv.Itoa(trace.Status))
		builder.WriteString("|")
		builder.WriteString(trace.HandledBy)
		builder.WriteString("|")
		builder.WriteString(trace.Category)
		builder.WriteString("|")
		builder.WriteString(strconv.FormatFloat(trace.DurationMS, 'f', -1, 64))
		builder.WriteString("|")
		builder.WriteString(strconv.Itoa(len(trace.Spans)))
		builder.WriteString("|")
		builder.WriteString(strconv.Itoa(len(trace.Logs)))
		builder.WriteString("\n")
	}
	return builder.String()
}

type requestTraceSnapshot struct {
	Directory string         `json:"directory"`
	Traces    []requestTrace `json:"traces"`
	Errors    []string       `json:"errors"`
}

type requestView struct {
	Directory       string
	Error           string
	Errors          []string
	Query           string
	Tab             string
	Type            string
	Framework       bool
	Requests        []requestTrace
	Rows            []requestRow
	Selected        requestTrace
	HasSelected     bool
	SelectedSpan    requestSpan
	HasSelectedSpan bool
	TimelineRows    []requestSpanRow
	FlameRows       []requestSpanRow
	Logs            []requestLog
}

func (v requestView) StreamURL() string {
	return requestURL(v.SelectedRequestID(), v.SelectedSpanID(), v.Query, v.Tab, v.Type, v.Framework)
}

func (v requestView) ControlPlanePath() string {
	values := url.Values{}
	if v.Query != "" {
		values.Set("q", v.Query)
	}
	if v.Type != "" && v.Type != "all" {
		values.Set("type", v.Type)
	}
	encoded := values.Encode()
	if encoded == "" {
		return appRequestTracesPath
	}
	return appRequestTracesPath + "?" + encoded
}

func (v requestView) CurrentURL() string {
	return requestURL(v.SelectedRequestID(), v.SelectedSpanID(), v.Query, v.Tab, v.Type, v.Framework)
}

func (v requestView) DetailURL() string {
	if !v.HasSelected {
		return ""
	}
	return requestURL(v.SelectedRequestID(), v.SelectedSpanID(), v.Query, v.Tab, v.Type, v.Framework)
}

func (v requestView) SelectedRequestID() string {
	if !v.HasSelected {
		return ""
	}
	return v.Selected.RequestID
}

func (v requestView) SelectedSpanID() string {
	if !v.HasSelectedSpan {
		return ""
	}
	return v.SelectedSpan.SpanID
}

func (v requestView) RequestCountText() string {
	count := len(v.Rows)
	if count == 1 {
		return "1 request"
	}
	return fmt.Sprintf("%d requests", count)
}

func (v requestView) SpanCountText() string {
	count := len(v.TimelineRows)
	if count == 1 {
		return "1 region"
	}
	return fmt.Sprintf("%d regions", count)
}

func (v requestView) FrameworkValue() string {
	if v.Framework {
		return "1"
	}
	return "0"
}

func (v requestView) FrameworkToggleText() string {
	if v.Framework {
		return "Hide framework"
	}
	return "Show framework"
}

func (v requestView) FrameworkToggleURL() string {
	return requestURL(v.SelectedRequestID(), v.SelectedSpanID(), v.Query, v.Tab, v.Type, !v.Framework)
}

func (v requestView) TabSelected(tab string) bool {
	return v.Tab == normalizeRequestTab(tab)
}

func (v requestView) HeadersTab() bool {
	return v.Tab == "headers"
}

func (v requestView) TracingTab() bool {
	return v.Tab == "tracing"
}

func (v requestView) LogsTab() bool {
	return v.Tab == "logs"
}

func (v requestView) TabURL(tab string) string {
	return requestURL(v.SelectedRequestID(), v.SelectedSpanID(), v.Query, normalizeRequestTab(tab), v.Type, v.Framework)
}

func (v requestView) TypeSelected(requestType string) bool {
	return v.Type == normalizeRequestType(requestType)
}

func (v requestView) TypeURL(requestType string) string {
	return requestURL(v.SelectedRequestID(), v.SelectedSpanID(), v.Query, v.Tab, normalizeRequestType(requestType), v.Framework)
}

func (v requestView) TypeValue() string {
	return v.Type
}

type requestTrace struct {
	RequestID  string                `json:"request_id"`
	Method     string                `json:"method"`
	Path       string                `json:"path"`
	Status     int                   `json:"status"`
	Bytes      int                   `json:"bytes"`
	StartedAt  time.Time             `json:"started_at"`
	DurationMS float64               `json:"duration_ms"`
	TraceFile  string                `json:"trace_file"`
	HandledBy  string                `json:"handled_by"`
	Category   string                `json:"category"`
	Runtime    requestRuntimeSummary `json:"runtime"`
	Memory     requestMemorySummary  `json:"memory"`
	Spans      []requestSpan         `json:"spans"`
	Logs       []requestLog          `json:"logs"`
}

func (t requestTrace) PathText() string {
	if strings.TrimSpace(t.Path) != "" {
		return t.Path
	}
	if strings.TrimSpace(t.RequestID) != "" {
		return t.RequestID
	}
	return "request"
}

func (t requestTrace) MethodText() string {
	if strings.TrimSpace(t.Method) != "" {
		return t.Method
	}
	return "-"
}

func (t requestTrace) StatusText() string {
	if t.Status == 0 {
		return "-"
	}
	return strconv.Itoa(t.Status)
}

func (t requestTrace) StatusClass() string {
	if t.Status >= 500 {
		return "error"
	}
	if t.Status >= 400 {
		return "warn"
	}
	if t.Status >= 200 && t.Status < 400 {
		return "ok"
	}
	return "unknown"
}

func (t requestTrace) DomainText() string {
	if t.HandledBy != "" {
		return t.HandledBy
	}
	return "-"
}

func (t requestTrace) TypeText() string {
	switch normalizeRequestType(t.Category) {
	case "framework":
		return "Framework"
	case "assets":
		return "Assets"
	case "other":
		return "Other"
	default:
		return "All"
	}
}

func (t requestTrace) InitiatorText() string {
	return "lazydev"
}

func (t requestTrace) SizeText() string {
	if t.Bytes <= 0 {
		return "-"
	}
	return formatRequestBytes(uint64(t.Bytes))
}

func (t requestTrace) DurationText() string {
	return formatRequestDuration(t.DurationMS)
}

func (t requestTrace) MemoryText() string {
	return fmt.Sprintf("mallocs %d, allocated %s", t.Memory.MallocsDelta, formatRequestBytes(t.Memory.TotalAllocBytesDelta))
}

func (t requestTrace) RuntimeText() string {
	if t.Runtime.GoVersion == "" {
		return ""
	}
	goroutines := ""
	if t.Runtime.GoroutinesStart != 0 || t.Runtime.GoroutinesEnd != 0 {
		goroutines = fmt.Sprintf(", goroutines %d->%d", t.Runtime.GoroutinesStart, t.Runtime.GoroutinesEnd)
	}
	return strings.TrimSpace(fmt.Sprintf("%s %s/%s%s", t.Runtime.GoVersion, t.Runtime.GOOS, t.Runtime.GOARCH, goroutines))
}

type requestRuntimeSummary struct {
	GoVersion       string `json:"go_version"`
	GOOS            string `json:"goos"`
	GOARCH          string `json:"goarch"`
	GoroutinesStart int    `json:"goroutines_start"`
	GoroutinesEnd   int    `json:"goroutines_end"`
}

type requestMemorySummary struct {
	TotalAllocBytesDelta uint64 `json:"total_alloc_bytes_delta"`
	MallocsDelta         uint64 `json:"mallocs_delta"`
}

type requestSpanMemory struct {
	TotalAllocBytesDelta     uint64 `json:"total_alloc_bytes_delta"`
	MallocsDelta             uint64 `json:"mallocs_delta"`
	FreesDelta               uint64 `json:"frees_delta"`
	SelfTotalAllocBytesDelta uint64 `json:"self_total_alloc_bytes_delta"`
	SelfMallocsDelta         uint64 `json:"self_mallocs_delta"`
	SelfFreesDelta           uint64 `json:"self_frees_delta"`
}

type requestSpan struct {
	Name           string             `json:"name"`
	TraceID        string             `json:"trace_id"`
	SpanID         string             `json:"span_id"`
	ParentID       string             `json:"parent_id"`
	StartedAt      time.Time          `json:"started_at"`
	EndedAt        time.Time          `json:"ended_at"`
	DurationMS     float64            `json:"duration_ms"`
	SelfDurationMS *float64           `json:"self_duration_ms"`
	Memory         *requestSpanMemory `json:"memory"`
}

func (s requestSpan) DurationText() string {
	return formatRequestDuration(s.DurationMS)
}

func (s requestSpan) SelfDurationText() string {
	if s.SelfDurationMS == nil {
		return "Not captured"
	}
	return formatRequestDuration(*s.SelfDurationMS)
}

func (s requestSpan) DurationSummaryText() string {
	if s.SelfDurationMS == nil {
		return s.DurationText()
	}
	return fmt.Sprintf("%s total, %s self", s.DurationText(), s.SelfDurationText())
}

func (s requestSpan) AllocationSummaryText() string {
	if s.Memory == nil {
		return "Not captured per region"
	}
	return fmt.Sprintf("%s total, %s self",
		formatRequestBytes(s.Memory.TotalAllocBytesDelta),
		formatRequestBytes(s.Memory.SelfTotalAllocBytesDelta),
	)
}

func (s requestSpan) MallocsSummaryText() string {
	if s.Memory == nil {
		return "Not captured per region"
	}
	return fmt.Sprintf("%s total, %s self",
		formatRequestCount(s.Memory.MallocsDelta, "malloc"),
		formatRequestCount(s.Memory.SelfMallocsDelta, "malloc"),
	)
}

func (s requestSpan) FreesSummaryText() string {
	if s.Memory == nil {
		return "Not captured per region"
	}
	return fmt.Sprintf("%s total, %s self",
		formatRequestCount(s.Memory.FreesDelta, "free"),
		formatRequestCount(s.Memory.SelfFreesDelta, "free"),
	)
}

func (s requestSpan) FlameLabel() string {
	parts := []string{s.Name, s.DurationText()}
	if s.SelfDurationMS != nil {
		parts = append(parts, "self "+s.SelfDurationText())
	}
	if s.Memory != nil {
		parts = append(parts, "self alloc "+formatRequestBytes(s.Memory.SelfTotalAllocBytesDelta))
	}
	return strings.Join(parts, " | ")
}

func (s requestSpan) Framework() bool {
	return frameworkRequestSpan(s)
}

type requestLog struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	SpanID  string `json:"span_id"`
}

func (l requestLog) TimeText() string {
	if l.Time == "" {
		return ""
	}
	value, err := time.Parse(time.RFC3339Nano, l.Time)
	if err != nil {
		return l.Time
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

type requestRow struct {
	Trace    requestTrace
	URL      string
	Selected bool
}

type requestSpanRow struct {
	Span         requestSpan
	URL          string
	Selected     bool
	Depth        int
	LeftPercent  string
	WidthPercent string
}

func (r requestSpanRow) LabelPadding() string {
	return strconv.Itoa(min(r.Depth, 8)*12+6) + "px"
}

func (r requestSpanRow) FlameMargin() string {
	return strconv.Itoa(min(r.Depth, 8)*14) + "px"
}

func normalizeRequestTab(tab string) string {
	switch strings.ToLower(strings.TrimSpace(tab)) {
	case "tracing", "logs":
		return strings.ToLower(strings.TrimSpace(tab))
	default:
		return "headers"
	}
}

func normalizeRequestType(requestType string) string {
	switch strings.ToLower(strings.TrimSpace(requestType)) {
	case "framework", "assets", "other":
		return strings.ToLower(strings.TrimSpace(requestType))
	default:
		return "all"
	}
}

func selectRequestTrace(traces []requestTrace, id string) *requestTrace {
	for index := range traces {
		if traces[index].RequestID == id {
			return &traces[index]
		}
	}
	if len(traces) == 0 {
		return nil
	}
	return &traces[0]
}

func requestRows(traces []requestTrace, selected string, query string, tab string, requestType string, framework bool) []requestRow {
	rows := make([]requestRow, 0, len(traces))
	for _, trace := range traces {
		rows = append(rows, requestRow{
			Trace:    trace,
			URL:      requestURL(trace.RequestID, "", query, tab, requestType, framework),
			Selected: trace.RequestID == selected,
		})
	}
	return rows
}

func visibleRequestSpans(spans []requestSpan, framework bool) []requestSpan {
	if framework {
		return spans
	}
	visible := make([]requestSpan, 0, len(spans))
	for _, span := range spans {
		if span.Name == "http.server.request" || !frameworkRequestSpan(span) {
			visible = append(visible, span)
		}
	}
	return visible
}

func selectRequestSpan(spans []requestSpan, id string) *requestSpan {
	for index := range spans {
		if spans[index].SpanID == id {
			return &spans[index]
		}
	}
	for index := range spans {
		if spans[index].Name != "http.server.request" {
			return &spans[index]
		}
	}
	if len(spans) == 0 {
		return nil
	}
	return &spans[0]
}

func requestSpanRows(trace requestTrace, spans []requestSpan, selected string, query string, tab string, requestType string, framework bool) []requestSpanRow {
	base := requestTraceBase(trace)
	duration := math.Max(0.001, trace.DurationMS)
	spanMap := map[string]requestSpan{}
	visibleSet := map[string]bool{}
	for _, span := range trace.Spans {
		spanMap[span.SpanID] = span
	}
	for _, span := range spans {
		visibleSet[span.SpanID] = true
	}
	rows := make([]requestSpanRow, 0, len(spans))
	for _, span := range spans {
		rows = append(rows, requestSpanRowFor(trace, span, selected, query, tab, requestType, framework, base, duration, spanMap, visibleSet))
	}
	return rows
}

func requestFlameRows(trace requestTrace, spans []requestSpan, selected string, query string, tab string, requestType string, framework bool) []requestSpanRow {
	if selected == "" {
		return requestSpanRows(trace, spans, selected, query, tab, requestType, framework)
	}
	selectedSet := descendantRequestSpanIDs(trace.Spans, selected)
	flameSpans := make([]requestSpan, 0, len(spans))
	for _, span := range spans {
		if selectedSet[span.SpanID] {
			flameSpans = append(flameSpans, span)
		}
	}
	return requestSpanRows(trace, flameSpans, selected, query, tab, requestType, framework)
}

func requestSpanRowFor(trace requestTrace, span requestSpan, selected string, query string, tab string, requestType string, framework bool, base time.Time, duration float64, spanMap map[string]requestSpan, visibleSet map[string]bool) requestSpanRow {
	offset := math.Max(0, span.StartedAt.Sub(base).Seconds()*1000)
	left := math.Min(100, offset/duration*100)
	width := math.Min(100, math.Max(0.25, span.DurationMS/duration*100))
	return requestSpanRow{
		Span:         span,
		URL:          requestURL(trace.RequestID, span.SpanID, query, tab, requestType, framework),
		Selected:     span.SpanID == selected,
		Depth:        visibleRequestDepth(span, spanMap, visibleSet),
		LeftPercent:  fmt.Sprintf("%.3f%%", left),
		WidthPercent: fmt.Sprintf("%.3f%%", width),
	}
}

func requestTraceBase(trace requestTrace) time.Time {
	if len(trace.Spans) > 0 && !trace.Spans[0].StartedAt.IsZero() {
		return trace.Spans[0].StartedAt
	}
	if !trace.StartedAt.IsZero() {
		return trace.StartedAt
	}
	return time.Now()
}

func visibleRequestDepth(span requestSpan, spanMap map[string]requestSpan, visibleSet map[string]bool) int {
	depth := 0
	seen := map[string]bool{span.SpanID: true}
	current, ok := spanMap[span.ParentID]
	for ok {
		if seen[current.SpanID] {
			break
		}
		seen[current.SpanID] = true
		if visibleSet[current.SpanID] {
			depth++
		}
		current, ok = spanMap[current.ParentID]
	}
	return depth
}

func descendantRequestSpanIDs(spans []requestSpan, rootID string) map[string]bool {
	children := map[string][]string{}
	for _, span := range spans {
		if span.ParentID == "" {
			continue
		}
		children[span.ParentID] = append(children[span.ParentID], span.SpanID)
	}
	result := map[string]bool{}
	stack := []string{rootID}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == "" || result[id] {
			continue
		}
		result[id] = true
		stack = append(stack, children[id]...)
	}
	return result
}

func frameworkRequestSpan(span requestSpan) bool {
	name := span.Name
	return name == "router" || strings.HasPrefix(name, "middleware ") || strings.HasPrefix(name, "dispatch ")
}

func requestURL(requestID string, spanID string, query string, tab string, requestType string, framework bool) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	requestType = normalizeRequestType(requestType)
	if requestType != "all" {
		values.Set("type", requestType)
	}
	tab = normalizeRequestTab(tab)
	if tab != "headers" {
		values.Set("tab", tab)
	}
	if framework {
		values.Set("framework", "1")
	}
	if requestID != "" {
		values.Set("request", requestID)
	}
	if spanID != "" {
		values.Set("span", spanID)
	}
	encoded := values.Encode()
	if encoded == "" {
		return "/_golazy/requests"
	}
	return "/_golazy/requests?" + encoded
}

func selectedRequestSpanID(span *requestSpan) string {
	if span == nil {
		return ""
	}
	return span.SpanID
}

func formatRequestDuration(value float64) string {
	if !isFiniteRequestNumber(value) {
		return ""
	}
	if value < 1 {
		return fmt.Sprintf("%.0fus", value*1000)
	}
	if value < 1000 {
		precision := 1
		if value < 10 {
			precision = 2
		}
		return strconv.FormatFloat(value, 'f', precision, 64) + "ms"
	}
	return strconv.FormatFloat(value/1000, 'f', 2, 64) + "s"
}

func formatRequestBytes(value uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", size, units[unit])
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}

func formatRequestCount(value uint64, noun string) string {
	if value == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", value, noun)
}

func isFiniteRequestNumber(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}
