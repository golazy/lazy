package panel

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

const appJobsControlPath = "/jobs"
const appCacheEntryPath = "/cache/entry"

type CacheSnapshot struct {
	Enabled bool         `json:"enabled"`
	Stats   CacheStats   `json:"stats"`
	Keys    []string     `json:"keys"`
	Entries []CacheEntry `json:"entries"`
	Error   string       `json:"-"`
}

type CacheStats struct {
	Entries    int   `json:"entries"`
	MaxEntries int   `json:"max_entries"`
	SizeBytes  int64 `json:"size_bytes"`
	Hits       int   `json:"hits"`
	Misses     int   `json:"misses"`
	Sets       int   `json:"sets"`
	Evictions  int   `json:"evictions"`
}

type CacheEntry struct {
	Key            string    `json:"key"`
	SizeBytes      int64     `json:"size_bytes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

type CacheEntryDetail struct {
	CacheEntry
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Error       string `json:"-"`
}

func (c CacheSnapshot) StatusText() string {
	if c.Error != "" {
		return "Cache unavailable"
	}
	if c.Enabled {
		return "Cache on"
	}
	return "Cache off"
}

type RequestMonitoringSnapshot struct {
	Enabled   bool   `json:"enabled"`
	Directory string `json:"directory"`
	Error     string `json:"-"`
}

func (s RequestMonitoringSnapshot) StatusText() string {
	if s.Error != "" {
		return "Monitoring unavailable"
	}
	if s.Enabled {
		return "Monitoring on"
	}
	return "Monitoring off"
}

type JobsSnapshot struct {
	Running     bool            `json:"running"`
	Definitions []JobDefinition `json:"definitions"`
	Stats       JobStats        `json:"stats"`
	Recent      []JobRecord     `json:"recent"`
	Error       string          `json:"-"`
}

type JobDefinition struct {
	Kind        string `json:"kind"`
	Type        string `json:"type"`
	Queue       string `json:"queue"`
	MaxAttempts int    `json:"max_attempts"`
}

type JobStats struct {
	Total   int            `json:"total"`
	ByState map[string]int `json:"by_state"`
	ByKind  map[string]int `json:"by_kind"`
	ByQueue map[string]int `json:"by_queue"`
}

type JobRecord struct {
	ID          int64     `json:"id"`
	Kind        string    `json:"kind"`
	Queue       string    `json:"queue"`
	State       string    `json:"state"`
	Attempt     int       `json:"attempt"`
	MaxAttempts int       `json:"max_attempts"`
	RunAt       time.Time `json:"run_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	LastError   string    `json:"last_error"`
}

func (s JobsSnapshot) StateText() string {
	if s.Error != "" {
		return "Jobs unavailable"
	}
	if s.Running {
		return "Runner active"
	}
	return "Runner stopped"
}

func (s JobsSnapshot) RunningText() string {
	if s.Error != "" {
		return "Unavailable"
	}
	if s.Running {
		return "Running"
	}
	return "Stopped"
}

func (s JobsSnapshot) Count(state string) int {
	if s.Stats.ByState == nil {
		return 0
	}
	return s.Stats.ByState[state]
}

func (r JobRecord) AttemptText() string {
	return fmt.Sprintf("%d/%d", r.Attempt, r.MaxAttempts)
}

func (r JobRecord) RunAtText() string {
	return formatTime(r.RunAt)
}

func (b *Base) CacheSnapshot(ctx context.Context) CacheSnapshot {
	var snapshot CacheSnapshot
	if err := b.FetchAppControlJSON(ctx, appCachePath, &snapshot); err != nil {
		snapshot.Error = err.Error()
	}
	if len(snapshot.Entries) == 0 && len(snapshot.Keys) > 0 {
		for _, key := range snapshot.Keys {
			snapshot.Entries = append(snapshot.Entries, CacheEntry{Key: key})
		}
	}
	return snapshot
}

func (b *Base) CacheEntry(ctx context.Context, key string) CacheEntryDetail {
	var detail CacheEntryDetail
	if key == "" {
		return detail
	}
	path := appCacheEntryPath + "?key=" + url.QueryEscape(key)
	if err := b.FetchAppControlJSON(ctx, path, &detail); err != nil {
		detail.CacheEntry.Key = key
		detail.Error = err.Error()
	}
	return detail
}

func (b *Base) RequestMonitoringSnapshot(ctx context.Context) RequestMonitoringSnapshot {
	var snapshot RequestMonitoringSnapshot
	if err := b.FetchAppControlJSON(ctx, appRequestMonitoringPath, &snapshot); err != nil {
		snapshot.Error = err.Error()
	}
	if snapshot.Directory == "" {
		snapshot.Directory = ".tmp/traces"
	}
	return snapshot
}

func (b *Base) JobsSnapshot(ctx context.Context) JobsSnapshot {
	var snapshot JobsSnapshot
	if err := b.FetchAppControlJSON(ctx, appJobsControlPath, &snapshot); err != nil {
		snapshot.Error = err.Error()
	}
	return snapshot
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
