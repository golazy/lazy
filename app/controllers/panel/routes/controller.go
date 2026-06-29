package routes

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
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
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setRoutesState(r)
			c.Set("defer_panel_lists", true)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamRoutesInitial, c.streamRoutes)
		},
	})
}

func (c *RoutesController) setRoutesState(r *http.Request) {
	for key, value := range c.routesViewData(r) {
		c.Set(key, value)
	}
}

func (c *RoutesController) routesViewData(r *http.Request) map[string]any {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	var routes lazyroutes.RouteTable
	if err := c.FetchAppControlJSON(r.Context(), appRoutesPath, &routes); err != nil {
		return map[string]any{
			"state":          c.Snapshot(),
			"routes_error":   err.Error(),
			"routes_query":   query,
			"routes_total":   0,
			"routes_visible": 0,
			"routes":         []routeRow{},
		}
	}

	rows := routeRows(routes)
	filtered := filterRoutes(rows, query)
	return map[string]any{
		"state":          c.Snapshot(),
		"routes_query":   query,
		"routes_total":   len(rows),
		"routes_visible": len(filtered),
		"routes":         filtered,
	}
}

func (c *RoutesController) streamRoutesInitial(r *http.Request) (string, error) {
	return c.renderRouteList(r)
}

func (c *RoutesController) streamRoutes(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState || event.State != buildservice.StateRunning {
		return "", nil
	}
	return c.renderRouteList(r)
}

func (c *RoutesController) renderRouteList(r *http.Request) (string, error) {
	variables := c.routesViewData(r)
	body, err := c.RenderPanelPartial(r, "routes", "route_rows", variables)
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("update", "[data-routes-list]", body) +
		panel.TurboStreamTargets("update", "[data-routes-count]", routeCountText(variables)), nil
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

func routeCountText(variables map[string]any) string {
	if variables["routes_error"] != nil {
		return "Routes unavailable"
	}
	total, _ := variables["routes_total"].(int)
	visible, _ := variables["routes_visible"].(int)
	query, _ := variables["routes_query"].(string)
	if query != "" {
		return strconv.Itoa(visible) + " / " + strconv.Itoa(total) + " routes"
	}
	return strconv.Itoa(total) + " routes"
}
