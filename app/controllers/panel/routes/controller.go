package routes

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazyroutes"
)

const appRoutesPath = "/routes"

type RoutesController struct {
	panel.Base
}

func New(ctx context.Context) (*RoutesController, error) {
	base, err := panel.NewBase(ctx)
	return &RoutesController{Base: base}, err
}

func (c *RoutesController) Index(w http.ResponseWriter, r *http.Request) error {
	if acceptsJSON(r) {
		return c.ProxyAppControl(w, r, http.MethodGet, appRoutesPath)
	}

	c.SetState()
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	var routes lazyroutes.RouteTable
	if err := c.FetchAppControlJSON(r.Context(), appRoutesPath, &routes); err != nil {
		c.Set("routes_error", err.Error())
		c.Set("routes_query", query)
		c.Set("routes_total", 0)
		c.Set("routes_visible", 0)
		c.Set("routes", []routeRow{})
		return nil
	}

	rows := routeRows(routes)
	filtered := filterRoutes(rows, query)
	c.Set("routes_query", query)
	c.Set("routes_total", len(rows))
	c.Set("routes_visible", len(filtered))
	c.Set("routes", filtered)
	return nil
}

type routeRow struct {
	Method    string
	Path      string
	Name      string
	Target    string
	Namespace string
	Params    string
}

func routeRows(routes lazyroutes.RouteTable) []routeRow {
	rows := make([]routeRow, 0, len(routes))
	for _, route := range routes {
		rows = append(rows, routeRow{
			Method:    route.Method,
			Path:      route.Path,
			Name:      route.Name,
			Target:    routeTarget(route),
			Namespace: route.Namespace,
			Params:    routeParams(route.NamedParams),
		})
	}
	return rows
}

func filterRoutes(routes []routeRow, query string) []routeRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return routes
	}
	filtered := make([]routeRow, 0, len(routes))
	for _, route := range routes {
		if strings.Contains(strings.ToLower(strings.Join([]string{
			route.Method,
			route.Path,
			route.Name,
			route.Target,
			route.Namespace,
			route.Params,
		}, " ")), query) {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func routeTarget(route lazyroutes.Route) string {
	switch {
	case route.Controller != "" && route.Action != "":
		return route.Controller + "#" + route.Action
	case route.Controller != "":
		return route.Controller
	case route.Action != "":
		return "#" + route.Action
	default:
		return ""
	}
}

func routeParams(params map[string]bool) string {
	if len(params) == 0 {
		return ""
	}
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func acceptsJSON(r *http.Request) bool {
	if r == nil || strings.TrimSpace(r.Header.Get("Turbo-Frame")) != "" {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Accept"), ",") {
		if strings.Contains(strings.ToLower(part), "application/json") {
			return true
		}
	}
	return false
}
