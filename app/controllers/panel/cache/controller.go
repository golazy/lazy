package cache

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazysse"
)

const appCacheEventsPath = "/cache/events"

var appControlEventClient = &http.Client{}

type CacheController struct {
	panel.Base
}

func New(ctx context.Context) (*CacheController, error) {
	base, err := panel.NewBase(ctx)
	return &CacheController{Base: base}, err
}

func (c *CacheController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setCacheShellState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.streamCache(w, r)
		},
	})
}

func (c *CacheController) setCacheShellState(r *http.Request) {
	view := c.cacheShellView(r)
	c.Set("state", c.Snapshot())
	c.Set("cache", view)
}

func (c *CacheController) cacheShellView(r *http.Request) cacheView {
	return cacheView{
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		SelectedKey: strings.TrimSpace(r.URL.Query().Get("key")),
		Defer:       true,
	}
}

func (c *CacheController) cacheSnapshotView(r *http.Request) cacheView {
	snapshot := c.CacheSnapshot(r.Context())
	view := cacheView{
		Snapshot:    snapshot,
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		SelectedKey: strings.TrimSpace(r.URL.Query().Get("key")),
	}
	if view.SelectedKey != "" {
		view.Selected = c.CacheEntry(r.Context(), view.SelectedKey)
	}
	return view
}

func (c *CacheController) streamCache(_ http.ResponseWriter, r *http.Request) error {
	stream, err := c.SSEStream()
	if err != nil {
		return err
	}
	defer stream.Close()
	stream.Heartbeat(15 * time.Second)

	view := c.cacheSnapshotView(r)
	known := cacheKnownKeys(view)
	initial, err := c.renderCacheFrameStream(r, view)
	if err != nil {
		return err
	}
	if err := sendCacheTurboStream(stream, initial); err != nil {
		return err
	}

	events, closeEvents, err := c.subscribeCacheEvents(stream.Context())
	if err != nil {
		body := cacheStreamText("[data-cache-status]", "Cache events unavailable")
		if sendErr := sendCacheTurboStream(stream, body); sendErr != nil {
			return sendErr
		}
		<-stream.Done()
		return nil
	}
	defer closeEvents()

	for {
		select {
		case <-stream.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}
			body, err := c.renderCacheEventStream(r, event, known)
			if err != nil || body == "" {
				continue
			}
			if err := sendCacheTurboStream(stream, body); err != nil {
				return err
			}
		}
	}
}

func (c *CacheController) renderCacheFrameStream(r *http.Request, view cacheView) (string, error) {
	body, err := c.RenderPanelPartial(r, "cache", "cache_frame", map[string]any{
		"state": c.Snapshot(),
		"cache": view,
	})
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("replace", "[data-cache-panel]", body), nil
}

func (c *CacheController) renderCacheEventStream(r *http.Request, event cacheEvent, known map[string]struct{}) (string, error) {
	view := cacheView{
		Snapshot: panel.CacheSnapshot{
			Enabled: event.Enabled,
			Stats:   event.Stats,
		},
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		SelectedKey: strings.TrimSpace(r.URL.Query().Get("key")),
	}

	body := renderCacheSummaryStreams(view)
	if event.Entry == nil || !cacheEntryMatches(event.Entry.Key, view.Query) {
		return body, nil
	}

	rowBody, err := c.RenderPanelPartial(r, "cache", "cache_rows", map[string]any{
		"cache": cacheView{
			Snapshot:    panel.CacheSnapshot{Entries: []panel.CacheEntry{*event.Entry}},
			Query:       view.Query,
			SelectedKey: view.SelectedKey,
		},
	})
	if err != nil {
		return "", err
	}
	if _, ok := known[event.Entry.Key]; ok {
		return body + panel.TurboStream("replace", cacheRowID(event.Entry.Key), rowBody), nil
	}
	known[event.Entry.Key] = struct{}{}
	return body +
		panel.TurboStreamTargets("remove", "[data-cache-empty]", "") +
		panel.TurboStreamTargets("append", "[data-cache-list]", rowBody), nil
}

func (c *CacheController) subscribeCacheEvents(ctx context.Context) (<-chan cacheEvent, func(), error) {
	addr := strings.TrimSpace(c.Snapshot().ControlPlaneAddr)
	if addr == "" {
		return nil, nil, fmt.Errorf("application control plane is not available")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+appCacheEventsPath, nil)
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", "text/event-stream")

	response, err := appControlEventClient.Do(request)
	if err != nil {
		return nil, nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, nil, fmt.Errorf("application control plane returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	events := make(chan cacheEvent, 32)
	go readCacheEventStream(ctx, response.Body, events)
	return events, func() { _ = response.Body.Close() }, nil
}

type cacheView struct {
	Snapshot    panel.CacheSnapshot
	Query       string
	SelectedKey string
	Selected    panel.CacheEntryDetail
	Defer       bool
}

func (v cacheView) Enabled() bool {
	return v.Snapshot.Enabled
}

func (v cacheView) Error() string {
	return v.Snapshot.Error
}

func (v cacheView) Entries() []cacheEntryRow {
	if v.Defer {
		return nil
	}
	if v.Query == "" {
		return cacheEntryRows(v.Snapshot.Entries)
	}
	query := strings.ToLower(v.Query)
	entries := make([]cacheEntryRow, 0, len(v.Snapshot.Entries))
	for _, entry := range v.Snapshot.Entries {
		if strings.Contains(strings.ToLower(entry.Key), query) {
			entries = append(entries, cacheEntryRow{CacheEntry: entry})
		}
	}
	return entries
}

func (v cacheView) HasSelected() bool {
	return v.SelectedKey != ""
}

func (v cacheView) SelectedError() string {
	return v.Selected.Error
}

func (v cacheView) SelectedSizeText() string {
	return formatBytes(v.Selected.SizeBytes)
}

func (v cacheView) StatusText() string {
	if v.Defer {
		return "Waiting for cache snapshot"
	}
	return v.Snapshot.StatusText()
}

func (v cacheView) SizeText() string {
	return formatBytes(v.Snapshot.Stats.SizeBytes)
}

func (v cacheView) UsageText() string {
	maxEntries := v.Snapshot.Stats.MaxEntries
	if maxEntries <= 0 {
		return "n/a"
	}
	usage := float64(v.Snapshot.Stats.Entries) / float64(maxEntries) * 100
	return formatPercent(usage)
}

func (v cacheView) HitRateText() string {
	total := v.Snapshot.Stats.Hits + v.Snapshot.Stats.Misses
	if total == 0 {
		return "0%"
	}
	return formatPercent(float64(v.Snapshot.Stats.Hits) / float64(total) * 100)
}

func (v cacheView) KeyCount() int {
	if len(v.Snapshot.Entries) > 0 && v.Snapshot.Stats.Entries == 0 {
		return len(v.Snapshot.Entries)
	}
	return v.Snapshot.Stats.Entries
}

func (v cacheView) PanelSizeValue() string {
	if v.HasSelected() {
		return "68%"
	}
	return "100%"
}

func (v cacheView) CurrentURL() string {
	values := url.Values{}
	if v.Query != "" {
		values.Set("q", v.Query)
	}
	if v.SelectedKey != "" {
		values.Set("key", v.SelectedKey)
	}
	return cacheURL(values)
}

func (v cacheView) SearchURL() string {
	return "/_golazy/cache"
}

func (v cacheView) KeyURL(key string) string {
	values := url.Values{}
	if v.Query != "" {
		values.Set("q", v.Query)
	}
	if key != "" {
		values.Set("key", key)
	}
	return cacheURL(values)
}

func (v cacheView) EmptyText() string {
	if v.Defer {
		return ""
	}
	if v.Snapshot.Error != "" {
		return v.Snapshot.Error
	}
	if v.Query != "" {
		return "No matching cache keys."
	}
	return "No cache keys."
}

func cacheKnownKeys(view cacheView) map[string]struct{} {
	known := map[string]struct{}{}
	for _, row := range view.Entries() {
		known[row.Key] = struct{}{}
	}
	return known
}

func cacheEntryMatches(key string, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(key), query)
}

func renderCacheSummaryStreams(view cacheView) string {
	return cacheStreamText("[data-cache-size]", view.SizeText()) +
		cacheStreamText("[data-cache-usage]", view.UsageText()) +
		cacheStreamText("[data-cache-key-count]", fmt.Sprint(view.KeyCount())) +
		cacheStreamText("[data-cache-hits]", fmt.Sprint(view.Snapshot.Stats.Hits)) +
		cacheStreamText("[data-cache-misses]", fmt.Sprint(view.Snapshot.Stats.Misses)) +
		cacheStreamText("[data-cache-sets]", fmt.Sprint(view.Snapshot.Stats.Sets)) +
		cacheStreamText("[data-cache-evictions]", fmt.Sprint(view.Snapshot.Stats.Evictions)) +
		cacheStreamText("[data-cache-hit-rate]", view.HitRateText()) +
		cacheStreamText("[data-cache-status]", view.StatusText())
}

func cacheStreamText(target string, value string) string {
	return panel.TurboStreamTargets("update", target, html.EscapeString(value))
}

func cacheURL(values url.Values) string {
	if len(values) == 0 {
		return "/_golazy/cache"
	}
	return "/_golazy/cache?" + values.Encode()
}

type cacheEntryRow struct {
	panel.CacheEntry
}

func cacheEntryRows(entries []panel.CacheEntry) []cacheEntryRow {
	rows := make([]cacheEntryRow, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, cacheEntryRow{CacheEntry: entry})
	}
	return rows
}

func (e cacheEntryRow) AgeText() string {
	value := e.UpdatedAt
	if value.IsZero() {
		value = e.CreatedAt
	}
	if value.IsZero() {
		return ""
	}
	age := time.Since(value)
	if age < time.Second {
		return "now"
	}
	if age < time.Minute {
		return fmt.Sprintf("%ds", int(age.Seconds()))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm", int(age.Minutes()))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%dh", int(age.Hours()))
	}
	return fmt.Sprintf("%dd", int(age.Hours()/24))
}

func (e cacheEntryRow) SizeText() string {
	return formatBytes(e.SizeBytes)
}

func (e cacheEntryRow) ID() string {
	return cacheRowID(e.Key)
}

func cacheRowID(key string) string {
	sum := sha1.Sum([]byte(key))
	return "cache_key_" + hex.EncodeToString(sum[:8])
}

type cacheEvent struct {
	Kind    string            `json:"kind"`
	Key     string            `json:"key"`
	Enabled bool              `json:"enabled"`
	Stats   panel.CacheStats  `json:"stats"`
	Entry   *panel.CacheEntry `json:"entry"`
	At      time.Time         `json:"at"`
}

func readCacheEventStream(ctx context.Context, body io.ReadCloser, events chan<- cacheEvent) {
	defer close(events)
	defer body.Close()

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 1<<20)
	var data []string
	flush := func() bool {
		if len(data) == 0 {
			return true
		}
		payload := strings.Join(data, "\n")
		data = nil
		var event cacheEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case events <- event:
			return true
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		if line == "" {
			if !flush() {
				return
			}
			continue
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(value, " "))
		}
	}
	flush()
}

func sendCacheTurboStream(stream *lazysse.Stream, body string) error {
	if body == "" {
		return nil
	}
	return stream.Send(lazysse.Event{Data: []string{body}})
}

func formatBytes(size int64) string {
	if size < 0 {
		size = 0
	}
	units := []string{"B", "KB", "MB", "GB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%dB", size)
	}
	if value >= 10 {
		return fmt.Sprintf("%.0f%s", value, units[unit])
	}
	return fmt.Sprintf("%.1f%s", value, units[unit])
}

func formatPercent(value float64) string {
	if value < 0 {
		value = 0
	}
	if value >= 10 {
		return fmt.Sprintf("%.0f%%", value)
	}
	return fmt.Sprintf("%.1f%%", value)
}
