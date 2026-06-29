package panel

import (
	"context"
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
const appRequestMonitoringPath = "/requests/monitoring"
const appRequestMonitoringOnPath = "/requests/monitoring/on"
const appRequestMonitoringOffPath = "/requests/monitoring/off"
const appRequestTracesClearPath = "/requests/traces/clear"

func New(ctx context.Context) (*Controller, error) {
	base, err := NewBase(ctx)
	return &Controller{Base: base}, err
}

func (c *Controller) Index(w http.ResponseWriter, r *http.Request) error {
	http.Redirect(w, r, "/_golazy/app", http.StatusSeeOther)
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
	c.redirectPanel(w, r, "/_golazy/actions")
	return nil
}

func (c *Controller) CacheOff(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appCacheOffPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	c.redirectPanel(w, r, "/_golazy/actions")
	return nil
}

func (c *Controller) RequestMonitoring(w http.ResponseWriter, r *http.Request) error {
	http.Redirect(w, r, "/_golazy/requests", http.StatusSeeOther)
	return nil
}

func (c *Controller) RequestMonitoringOn(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appRequestMonitoringOnPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	c.redirectPanel(w, r, "/_golazy/requests")
	return nil
}

func (c *Controller) RequestMonitoringOff(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appRequestMonitoringOffPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	c.redirectPanel(w, r, "/_golazy/requests")
	return nil
}

func (c *Controller) RequestTracesClear(w http.ResponseWriter, r *http.Request) error {
	if err := c.PostAppControl(r.Context(), appRequestTracesClearPath); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil
	}
	c.redirectPanel(w, r, "/_golazy/requests")
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

func (c *Controller) redirectPanel(w http.ResponseWriter, r *http.Request, fallback string) {
	if err := r.ParseForm(); err == nil {
		if target := r.FormValue("redirect"); safePanelRedirect(target) {
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, fallback, http.StatusSeeOther)
}

func safePanelRedirect(target string) bool {
	target = strings.TrimSpace(target)
	return strings.HasPrefix(target, "/_golazy") &&
		!strings.HasPrefix(target, "//") &&
		!strings.ContainsAny(target, "\r\n")
}
