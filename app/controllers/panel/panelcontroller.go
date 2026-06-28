package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golazy.dev/lazy/services/buildservice"
)

type Controller struct {
	Base
}

const appCachePath = "/cache"
const appCacheOnPath = "/cache/on"
const appCacheOffPath = "/cache/off"

func New(ctx context.Context) (*Controller, error) {
	base, err := NewBase(ctx)
	return &Controller{Base: base}, err
}

func (c *Controller) Index(w http.ResponseWriter, r *http.Request) error {
	http.Redirect(w, r, "/_golazy/logs", http.StatusSeeOther)
	return nil
}

func (c *Controller) State(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(c.Snapshot())
}

func (c *Controller) Cache(w http.ResponseWriter, r *http.Request) error {
	return c.ProxyAppControl(w, r, http.MethodGet, appCachePath)
}

func (c *Controller) CacheOn(w http.ResponseWriter, r *http.Request) error {
	return c.ProxyAppControl(w, r, http.MethodPost, appCacheOnPath)
}

func (c *Controller) CacheOff(w http.ResponseWriter, r *http.Request) error {
	return c.ProxyAppControl(w, r, http.MethodPost, appCacheOffPath)
}

func (c *Controller) Events(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return nil
	}
	events, unsubscribe := c.Store.Subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprint(w, "event: ready\ndata: ok\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case event := <-events:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if stream, err := c.turboStreamForEvent(r, event); err == nil && stream != "" {
				writeSSE(w, "turbo-stream", stream)
			}
			flusher.Flush()
		}
	}
}

func (c *Controller) Rebuild(w http.ResponseWriter, r *http.Request) error {
	return c.enqueue(w, r, buildservice.ActionRebuild)
}

func (c *Controller) Restart(w http.ResponseWriter, r *http.Request) error {
	return c.enqueue(w, r, buildservice.ActionRestart)
}

func (c *Controller) enqueue(w http.ResponseWriter, r *http.Request, action buildservice.Action) error {
	if err := c.Actions.Enqueue(r.Context(), action); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return nil
	}
	http.Redirect(w, r, "/_golazy/", http.StatusSeeOther)
	return nil
}

func (c *Controller) turboStreamForEvent(r *http.Request, event buildservice.Event) (string, error) {
	var stream string
	snapshot := c.Snapshot()
	variables := map[string]any{"state": snapshot}

	status, err := c.RenderPermanentPanelFrame(r, "status_bar", "status", "status_bar_frame", variables)
	if err != nil {
		return "", err
	}
	stream += TurboStream("replace", "status_bar", status)

	item, err := c.RenderPanelPartialData(r, "logs", "event_item", event)
	if err != nil {
		return "", err
	}
	stream += TurboStream("append", "panel_events", item)

	if event.Type != buildservice.EventOutput {
		logs, err := c.RenderPanelPartial(r, "logs", "logs_frame", variables)
		if err != nil {
			return "", err
		}
		stream += TurboStream("replace", "logs", logs)
	}
	return stream, nil
}

func writeSSE(w http.ResponseWriter, event string, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
