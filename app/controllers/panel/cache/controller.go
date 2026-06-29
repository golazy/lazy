package cache

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazycontroller"
)

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
			c.setCacheState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboInitial(w, r, nil)
		},
	})
}

func (c *CacheController) setCacheState(r *http.Request) {
	snapshot := c.CacheSnapshot(r.Context())
	view := cacheView{
		Snapshot:    snapshot,
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		SelectedKey: strings.TrimSpace(r.URL.Query().Get("key")),
	}
	if view.SelectedKey != "" {
		view.Selected = c.CacheEntry(r.Context(), view.SelectedKey)
	}
	c.Set("state", c.Snapshot())
	c.Set("cache", view)
}

type cacheView struct {
	Snapshot    panel.CacheSnapshot
	Query       string
	SelectedKey string
	Selected    panel.CacheEntryDetail
}

func (v cacheView) Enabled() bool {
	return v.Snapshot.Enabled
}

func (v cacheView) Error() string {
	return v.Snapshot.Error
}

func (v cacheView) Entries() []cacheEntryRow {
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
	if v.Snapshot.Error != "" {
		return v.Snapshot.Error
	}
	if v.Query != "" {
		return "No matching cache keys."
	}
	return "No cache keys."
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
