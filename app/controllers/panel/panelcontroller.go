package panel

import (
	"context"
	"encoding/json"
	"net/http"

	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazysse"
)

type Controller struct {
	Base
}

const appCachePath = "/cache"
const appCacheOnPath = "/cache/on"
const appCacheOffPath = "/cache/off"
const appRequestMonitoringPath = "/requests/monitoring"
const appRequestMonitoringOnPath = "/requests/monitoring/on"
const appRequestMonitoringOffPath = "/requests/monitoring/off"

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

func (c *Controller) RequestMonitoring(w http.ResponseWriter, r *http.Request) error {
	return c.ProxyAppControl(w, r, http.MethodGet, appRequestMonitoringPath)
}

func (c *Controller) RequestMonitoringOn(w http.ResponseWriter, r *http.Request) error {
	return c.ProxyAppControl(w, r, http.MethodPost, appRequestMonitoringOnPath)
}

func (c *Controller) RequestMonitoringOff(w http.ResponseWriter, r *http.Request) error {
	return c.ProxyAppControl(w, r, http.MethodPost, appRequestMonitoringOffPath)
}

func (c *Controller) Events(w http.ResponseWriter, r *http.Request) error {
	stream, err := c.SSEStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	events, unsubscribe := c.Store.Subscribe()
	defer unsubscribe()

	if err := stream.Send(lazysse.Event{Event: "ready", Data: []string{"ok"}}); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Done():
			return nil
		case event := <-events:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if err := stream.Send(lazysse.Event{Event: string(event.Type), Data: []string{string(data)}}); err != nil {
				return err
			}
			if body, err := c.turboStreamForEvent(r, event); err == nil && body != "" {
				if err := stream.Send(lazysse.Event{Event: "turbo-stream", Data: []string{body}}); err != nil {
					return err
				}
			}
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
