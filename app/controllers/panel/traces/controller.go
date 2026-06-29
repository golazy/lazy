package traces

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
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

const appRequestTracesPath = "/requests/traces"

type TracesController struct {
	panel.Base
}

func New(ctx context.Context) (*TracesController, error) {
	base, err := panel.NewBase(ctx)
	return &TracesController{Base: base}, err
}

func (c *TracesController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setTracesState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamTraces)
		},
	})
}

func (c *TracesController) setTracesState(r *http.Request) {
	c.Set("state", c.Snapshot())
	c.Set("monitoring", c.RequestMonitoringSnapshot(r.Context()))
	c.Set("traces", c.traceView(r))
}

func (c *TracesController) streamTraces(r *http.Request, _ buildservice.Event) (string, error) {
	body, err := c.RenderPanelPartial(r, "traces", "traces_frame", map[string]any{
		"state":      c.Snapshot(),
		"monitoring": c.RequestMonitoringSnapshot(r.Context()),
		"traces":     c.traceView(r),
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStream("replace", "traces", body), nil
}

func (c *TracesController) traceView(r *http.Request) traceView {
	var snapshot traceSnapshot
	view := traceView{
		Directory: ".tmp/traces",
		Query:     strings.TrimSpace(r.URL.Query().Get("q")),
		Framework: r.URL.Query().Get("framework") == "1",
	}
	if err := c.FetchAppControlJSON(r.Context(), appRequestTracesPath, &snapshot); err != nil {
		view.Error = err.Error()
		return view
	}
	if snapshot.Directory != "" {
		view.Directory = snapshot.Directory
	}
	view.Errors = snapshot.Errors
	view.Traces = filterTraces(snapshot.Traces, view.Query)
	selected := selectTrace(view.Traces, r.URL.Query().Get("trace"))
	if selected == nil {
		return view
	}
	view.Selected = *selected
	view.HasSelected = true
	visible := visibleSpans(selected.Spans, view.Framework)
	selectedSpan := selectSpan(visible, r.URL.Query().Get("span"))
	if selectedSpan != nil {
		view.SelectedSpan = *selectedSpan
		view.HasSelectedSpan = true
	}
	view.TraceRows = traceRows(view.Traces, selected.RequestID, view.Query, view.Framework)
	view.TimelineRows = spanRows(*selected, visible, selectedSpanID(selectedSpan), view.Query, view.Framework)
	view.FlameRows = flameRows(*selected, visible, selectedSpanID(selectedSpan), view.Query, view.Framework)
	view.Logs = selected.Logs
	return view
}

type traceSnapshot struct {
	Directory string         `json:"directory"`
	Traces    []requestTrace `json:"traces"`
	Errors    []string       `json:"errors"`
}

type traceView struct {
	Directory       string
	Error           string
	Errors          []string
	Query           string
	Framework       bool
	Traces          []requestTrace
	TraceRows       []traceRow
	Selected        requestTrace
	HasSelected     bool
	SelectedSpan    traceSpan
	HasSelectedSpan bool
	TimelineRows    []spanRow
	FlameRows       []spanRow
	Logs            []traceLog
}

func (v traceView) FrameworkValue() string {
	if v.Framework {
		return "1"
	}
	return "0"
}

func (v traceView) FrameworkToggleText() string {
	if v.Framework {
		return "Hide framework"
	}
	return "Show framework"
}

func (v traceView) StreamURL() string {
	return traceURL(v.SelectedTraceID(), v.SelectedSpanID(), v.Query, v.Framework)
}

func (v traceView) FrameworkToggleURL() string {
	return traceURL(v.SelectedTraceID(), v.SelectedSpanID(), v.Query, !v.Framework)
}

func (v traceView) SelectedTraceID() string {
	if !v.HasSelected {
		return ""
	}
	return v.Selected.RequestID
}

func (v traceView) SelectedSpanID() string {
	if !v.HasSelectedSpan {
		return ""
	}
	return v.SelectedSpan.SpanID
}

func (v traceView) SpanCountText() string {
	count := len(v.TimelineRows)
	if count == 1 {
		return "1 region"
	}
	return fmt.Sprintf("%d regions", count)
}

type requestTrace struct {
	RequestID  string              `json:"request_id"`
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	Status     int                 `json:"status"`
	StartedAt  time.Time           `json:"started_at"`
	DurationMS float64             `json:"duration_ms"`
	TraceFile  string              `json:"trace_file"`
	Runtime    traceRuntimeSummary `json:"runtime"`
	Memory     traceMemorySummary  `json:"memory"`
	Spans      []traceSpan         `json:"spans"`
	Logs       []traceLog          `json:"logs"`
}

func (t requestTrace) Title() string {
	title := strings.TrimSpace(t.Method + " " + t.Path)
	if title != "" {
		return title
	}
	return t.RequestID
}

func (t requestTrace) DurationText() string {
	return formatDuration(t.DurationMS)
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

func (t requestTrace) MemoryText() string {
	return fmt.Sprintf("mallocs %d, allocated %s", t.Memory.MallocsDelta, formatBytes(t.Memory.TotalAllocBytesDelta))
}

type traceRuntimeSummary struct {
	GoVersion       string `json:"go_version"`
	GOOS            string `json:"goos"`
	GOARCH          string `json:"goarch"`
	GoroutinesStart int    `json:"goroutines_start"`
	GoroutinesEnd   int    `json:"goroutines_end"`
}

type traceMemorySummary struct {
	TotalAllocBytesDelta uint64 `json:"total_alloc_bytes_delta"`
	MallocsDelta         uint64 `json:"mallocs_delta"`
}

type traceSpanMemory struct {
	TotalAllocBytesDelta     uint64 `json:"total_alloc_bytes_delta"`
	MallocsDelta             uint64 `json:"mallocs_delta"`
	FreesDelta               uint64 `json:"frees_delta"`
	SelfTotalAllocBytesDelta uint64 `json:"self_total_alloc_bytes_delta"`
	SelfMallocsDelta         uint64 `json:"self_mallocs_delta"`
	SelfFreesDelta           uint64 `json:"self_frees_delta"`
}

type traceSpan struct {
	Name           string           `json:"name"`
	TraceID        string           `json:"trace_id"`
	SpanID         string           `json:"span_id"`
	ParentID       string           `json:"parent_id"`
	StartedAt      time.Time        `json:"started_at"`
	EndedAt        time.Time        `json:"ended_at"`
	DurationMS     float64          `json:"duration_ms"`
	SelfDurationMS *float64         `json:"self_duration_ms"`
	Memory         *traceSpanMemory `json:"memory"`
}

func (s traceSpan) DurationText() string {
	return formatDuration(s.DurationMS)
}

func (s traceSpan) SelfDurationText() string {
	if s.SelfDurationMS == nil {
		return "Not captured"
	}
	return formatDuration(*s.SelfDurationMS)
}

func (s traceSpan) DurationSummaryText() string {
	if s.SelfDurationMS == nil {
		return s.DurationText()
	}
	return fmt.Sprintf("%s total, %s self", s.DurationText(), s.SelfDurationText())
}

func (s traceSpan) AllocationSummaryText() string {
	if s.Memory == nil {
		return "Not captured per region"
	}
	return fmt.Sprintf("%s total, %s self",
		formatBytes(s.Memory.TotalAllocBytesDelta),
		formatBytes(s.Memory.SelfTotalAllocBytesDelta),
	)
}

func (s traceSpan) MallocsSummaryText() string {
	if s.Memory == nil {
		return "Not captured per region"
	}
	return fmt.Sprintf("%s total, %s self",
		formatCount(s.Memory.MallocsDelta, "malloc"),
		formatCount(s.Memory.SelfMallocsDelta, "malloc"),
	)
}

func (s traceSpan) FreesSummaryText() string {
	if s.Memory == nil {
		return "Not captured per region"
	}
	return fmt.Sprintf("%s total, %s self",
		formatCount(s.Memory.FreesDelta, "free"),
		formatCount(s.Memory.SelfFreesDelta, "free"),
	)
}

func (s traceSpan) FlameLabel() string {
	parts := []string{s.Name, s.DurationText()}
	if s.SelfDurationMS != nil {
		parts = append(parts, "self "+s.SelfDurationText())
	}
	if s.Memory != nil {
		parts = append(parts, "self alloc "+formatBytes(s.Memory.SelfTotalAllocBytesDelta))
	}
	return strings.Join(parts, " | ")
}

func (s traceSpan) Framework() bool {
	return frameworkSpan(s)
}

type traceLog struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	SpanID  string `json:"span_id"`
}

func (l traceLog) TimeText() string {
	if l.Time == "" {
		return ""
	}
	value, err := time.Parse(time.RFC3339Nano, l.Time)
	if err != nil {
		return l.Time
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

type traceRow struct {
	Trace    requestTrace
	URL      string
	Selected bool
}

type spanRow struct {
	Span         traceSpan
	URL          string
	Selected     bool
	Depth        int
	LeftPercent  string
	WidthPercent string
}

func (r spanRow) LabelPadding() string {
	return strconv.Itoa(min(r.Depth, 8)*12+6) + "px"
}

func (r spanRow) FlameMargin() string {
	return strconv.Itoa(min(r.Depth, 8)*14) + "px"
}

func filterTraces(traces []requestTrace, query string) []requestTrace {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return traces
	}
	filtered := make([]requestTrace, 0, len(traces))
	for _, trace := range traces {
		values := []string{trace.RequestID, trace.Method, trace.Path, strconv.Itoa(trace.Status)}
		if strings.Contains(strings.ToLower(strings.Join(values, " ")), query) {
			filtered = append(filtered, trace)
		}
	}
	return filtered
}

func selectTrace(traces []requestTrace, id string) *requestTrace {
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

func traceRows(traces []requestTrace, selected string, query string, framework bool) []traceRow {
	rows := make([]traceRow, 0, len(traces))
	for _, trace := range traces {
		rows = append(rows, traceRow{
			Trace:    trace,
			URL:      traceURL(trace.RequestID, "", query, framework),
			Selected: trace.RequestID == selected,
		})
	}
	return rows
}

func visibleSpans(spans []traceSpan, framework bool) []traceSpan {
	if framework {
		return spans
	}
	visible := make([]traceSpan, 0, len(spans))
	for _, span := range spans {
		if span.Name == "http.server.request" || !frameworkSpan(span) {
			visible = append(visible, span)
		}
	}
	return visible
}

func selectSpan(spans []traceSpan, id string) *traceSpan {
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

func spanRows(trace requestTrace, spans []traceSpan, selected string, query string, framework bool) []spanRow {
	base := traceBase(trace)
	duration := math.Max(0.001, trace.DurationMS)
	spanMap := map[string]traceSpan{}
	visibleSet := map[string]bool{}
	for _, span := range trace.Spans {
		spanMap[span.SpanID] = span
	}
	for _, span := range spans {
		visibleSet[span.SpanID] = true
	}
	rows := make([]spanRow, 0, len(spans))
	for _, span := range spans {
		rows = append(rows, spanRowFor(trace, span, selected, query, framework, base, duration, spanMap, visibleSet))
	}
	return rows
}

func flameRows(trace requestTrace, spans []traceSpan, selected string, query string, framework bool) []spanRow {
	if selected == "" {
		return spanRows(trace, spans, selected, query, framework)
	}
	selectedSet := descendantSpanIDs(trace.Spans, selected)
	flameSpans := make([]traceSpan, 0, len(spans))
	for _, span := range spans {
		if selectedSet[span.SpanID] {
			flameSpans = append(flameSpans, span)
		}
	}
	return spanRows(trace, flameSpans, selected, query, framework)
}

func spanRowFor(trace requestTrace, span traceSpan, selected string, query string, framework bool, base time.Time, duration float64, spanMap map[string]traceSpan, visibleSet map[string]bool) spanRow {
	offset := math.Max(0, span.StartedAt.Sub(base).Seconds()*1000)
	left := math.Min(100, offset/duration*100)
	width := math.Min(100, math.Max(0.25, span.DurationMS/duration*100))
	return spanRow{
		Span:         span,
		URL:          traceURL(trace.RequestID, span.SpanID, query, framework),
		Selected:     span.SpanID == selected,
		Depth:        visibleDepth(span, spanMap, visibleSet),
		LeftPercent:  fmt.Sprintf("%.3f%%", left),
		WidthPercent: fmt.Sprintf("%.3f%%", width),
	}
}

func traceBase(trace requestTrace) time.Time {
	if len(trace.Spans) > 0 && !trace.Spans[0].StartedAt.IsZero() {
		return trace.Spans[0].StartedAt
	}
	if !trace.StartedAt.IsZero() {
		return trace.StartedAt
	}
	return time.Now()
}

func visibleDepth(span traceSpan, spanMap map[string]traceSpan, visibleSet map[string]bool) int {
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

func descendantSpanIDs(spans []traceSpan, rootID string) map[string]bool {
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

func frameworkSpan(span traceSpan) bool {
	name := span.Name
	return name == "router" || strings.HasPrefix(name, "middleware ") || strings.HasPrefix(name, "dispatch ")
}

func traceURL(traceID string, spanID string, query string, framework bool) string {
	values := url.Values{}
	if query != "" {
		values.Set("q", query)
	}
	if framework {
		values.Set("framework", "1")
	}
	if traceID != "" {
		values.Set("trace", traceID)
	}
	if spanID != "" {
		values.Set("span", spanID)
	}
	encoded := values.Encode()
	if encoded == "" {
		return "/_golazy/traces"
	}
	return "/_golazy/traces?" + encoded
}

func selectedSpanID(span *traceSpan) string {
	if span == nil {
		return ""
	}
	return span.SpanID
}

func formatDuration(value float64) string {
	if !isFinite(value) {
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

func formatBytes(value uint64) string {
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

func formatCount(value uint64, noun string) string {
	if value == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", value, noun)
}

func isFinite(value float64) bool {
	return !math.IsInf(value, 0) && !math.IsNaN(value)
}
