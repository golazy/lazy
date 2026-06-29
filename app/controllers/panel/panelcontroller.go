package panel

import (
	"context"
	"net/http"

	"golazy.dev/lazy/services/buildservice"
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

func (c *Controller) Cache(w http.ResponseWriter, r *http.Request) error {
	http.Redirect(w, r, "/_golazy/actions", http.StatusSeeOther)
	return nil
}

func (c *Controller) CacheOn(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appCacheOnPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	http.Redirect(w, r, "/_golazy/actions", http.StatusSeeOther)
	return nil
}

func (c *Controller) CacheOff(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appCacheOffPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	http.Redirect(w, r, "/_golazy/actions", http.StatusSeeOther)
	return nil
}

func (c *Controller) RequestMonitoring(w http.ResponseWriter, r *http.Request) error {
	http.Redirect(w, r, "/_golazy/traces", http.StatusSeeOther)
	return nil
}

func (c *Controller) RequestMonitoringOn(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appRequestMonitoringOnPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	http.Redirect(w, r, "/_golazy/traces", http.StatusSeeOther)
	return nil
}

func (c *Controller) RequestMonitoringOff(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appRequestMonitoringOffPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	http.Redirect(w, r, "/_golazy/traces", http.StatusSeeOther)
	return nil
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
