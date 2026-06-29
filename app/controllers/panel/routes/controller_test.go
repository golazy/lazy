package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/app"
	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
	"golazy.dev/lazyroutes"

	_ "golazy.dev/lazyview/gotmpl"
)

func TestRouteRowsSortsParams(t *testing.T) {
	rows := routeRows(lazyroutes.RouteTable{
		{
			Method: "GET",
			Path:   "/posts",
		},
		{
			Method: "GET",
			Path:   "/teams/{team_id}/posts/{post_id}",
			NamedParams: map[string]bool{
				"post_id": true,
				"team_id": true,
			},
		},
	})
	if got, want := rows[1].Params, "post_id, team_id"; got != want {
		t.Fatalf("Params = %q, want %q", got, want)
	}
	if !rows[0].Linkable() || rows[0].Link != "/posts" {
		t.Fatalf("static GET row link = %#v, want /posts", rows[0])
	}
	if rows[1].Linkable() {
		t.Fatalf("parameterized GET row is linkable: %#v", rows[1])
	}
}

func TestRouteRowsOnlyLinkSafeGetPaths(t *testing.T) {
	rows := routeRows(lazyroutes.RouteTable{
		{Method: "POST", Path: "/posts"},
		{Method: "GET", Path: "/posts/{post_id}"},
		{Method: "GET", Path: "/about"},
	})
	if rows[0].Linkable() || rows[1].Linkable() {
		t.Fatalf("unsafe rows are linkable: %#v", rows)
	}
	if !rows[2].Linkable() || rows[2].Link != "/about" {
		t.Fatalf("GET static row = %#v, want /about link", rows[2])
	}
}

func TestFilterRoutesMatchesRenderedFields(t *testing.T) {
	rows := []routeRow{
		{Method: "GET", Path: "/posts", Name: "posts", Target: "posts#Index"},
		{Method: "POST", Path: "/admin/posts", Name: "admin_posts", Target: "admin/posts#Create", Params: "post_id"},
	}
	filtered := filterRoutes(rows, "post_id")
	if len(filtered) != 1 || filtered[0].Name != "admin_posts" {
		t.Fatalf("filtered = %#v, want admin_posts", filtered)
	}
}

func TestRoutesViewUsesDebouncedTableFrame(t *testing.T) {
	var gotPath string
	var gotMethod string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `[
			{"method":"GET","path":"/posts","name":"posts","controller":"posts","action":"Index"},
			{"method":"POST","path":"/admin/posts","name":"admin_posts","controller":"admin/posts","action":"Create","params":{"post_id":true}}
		]`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &RoutesController{Base: panel.Base{Store: store}}
	request := httptest.NewRequest(http.MethodGet, "/_golazy/routes?q=admin", nil)

	data := controller.routesViewData(request)
	if gotMethod != http.MethodGet || gotPath != appRoutesPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appRoutesPath)
	}
	if data["routes_count_text"] != "1 / 2 routes" {
		t.Fatalf("routes_count_text = %#v, want 1 / 2 routes", data["routes_count_text"])
	}

	renderer := newRoutesTestRenderer(t)
	controller.Renderer = renderer
	body, err := controller.RenderPanelPartial(request, "routes", "routes_frame", data)
	if err != nil {
		t.Fatalf("render routes frame: %v", err)
	}
	for _, want := range []string{
		`data-controller="debounced-form"`,
		`data-debounced-form-count-source-value="[data-routes-frame-count]"`,
		`data-debounced-form-count-target-value="[data-routes-count]"`,
		`data-debounced-form-delay-value="250"`,
		`data-turbo-frame="routes_table"`,
		`<turbo-frame id="routes_table" class="routes-table-frame">`,
		`1 / 2 routes`,
		`/admin/posts`,
		`admin_posts`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered routes frame missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `>posts<`) {
		t.Fatalf("rendered routes frame contains unfiltered route:\n%s", body)
	}

	frame, err := controller.RenderPanelFrame(request, "routes_table", "routes", "routes_table", data)
	if err != nil {
		t.Fatalf("render routes table frame: %v", err)
	}
	for _, want := range []string{
		`<turbo-frame id="routes_table">`,
		`<span hidden data-routes-frame-count>1 / 2 routes</span>`,
		`/admin/posts`,
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("rendered routes table frame missing %q:\n%s", want, frame)
		}
	}
}

func newRoutesTestRenderer(t *testing.T) *lazycontroller.Renderer {
	t.Helper()
	views, err := app.Views()
	if err != nil {
		t.Fatalf("open app views: %v", err)
	}
	renderer, err := lazycontroller.NewRenderer(views)
	if err != nil {
		t.Fatalf("new renderer: %v", err)
	}
	renderer.Helper("path_for", func(name string, values ...any) (string, error) {
		return "/_golazy/" + name, nil
	})
	return renderer
}
