package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golazy.dev/lazy/app/controllers"
	"golazy.dev/lazy/services/buildservice"
)

type Controller struct {
	controllers.BaseController
	store   *buildservice.Store
	actions buildservice.Actions
}

const appCachePath = "/cache"
const appCacheOnPath = "/cache/on"
const appCacheOffPath = "/cache/off"
const appJobsPath = "/jobs"

var appControlClient = &http.Client{Timeout: 2 * time.Second}

func New(ctx context.Context) (*Controller, error) {
	base, err := controllers.NewBaseController(ctx)
	if err != nil {
		return nil, err
	}
	store, ok := buildservice.StoreFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("dev panel store is missing")
	}
	actions, ok := buildservice.ActionsFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("dev panel actions are missing")
	}
	return &Controller{BaseController: base, store: store, actions: actions}, nil
}

func (c *Controller) Index(_ http.ResponseWriter, _ *http.Request) error {
	c.Set("state", c.store.Snapshot())
	return nil
}

func (c *Controller) State(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(c.store.Snapshot())
}

func (c *Controller) Cache(w http.ResponseWriter, r *http.Request) error {
	return c.proxyAppControl(w, r, http.MethodGet, appCachePath)
}

func (c *Controller) CacheOn(w http.ResponseWriter, r *http.Request) error {
	return c.proxyAppControl(w, r, http.MethodPost, appCacheOnPath)
}

func (c *Controller) CacheOff(w http.ResponseWriter, r *http.Request) error {
	return c.proxyAppControl(w, r, http.MethodPost, appCacheOffPath)
}

func (c *Controller) Jobs(w http.ResponseWriter, r *http.Request) error {
	return c.proxyAppControl(w, r, http.MethodGet, appJobsPath)
}

func (c *Controller) Events(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return nil
	}
	events, unsubscribe := c.store.Subscribe()
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
	if err := c.actions.Enqueue(r.Context(), action); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return nil
	}
	http.Redirect(w, r, "/_golazy/", http.StatusSeeOther)
	return nil
}

func (c *Controller) proxyAppControl(w http.ResponseWriter, r *http.Request, method string, path string) error {
	addr := c.store.Snapshot().ControlPlaneAddr
	if addr == "" {
		http.Error(w, "application control plane is not available", http.StatusServiceUnavailable)
		return nil
	}
	request, err := http.NewRequestWithContext(r.Context(), method, "http://"+addr+path, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	request.Header.Set("Accept", "application/json")

	response, err := appControlClient.Do(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	defer response.Body.Close()

	w.Header().Set("Cache-Control", "no-store")
	if contentType := response.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
	return nil
}
