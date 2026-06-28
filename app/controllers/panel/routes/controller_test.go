package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazyroutes"
)

func TestIndexProxiesApplicationControlPlaneForJSON(t *testing.T) {
	var gotMethod string
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `[{"method":"GET","path":"/posts","name":"posts","controller":"posts","action":"Index"}]`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &RoutesController{Base: panel.Base{Store: store}}

	request := httptest.NewRequest(http.MethodGet, "/_golazy/routes", nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	if err := controller.Index(response, request); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if gotMethod != http.MethodGet || gotPath != appRoutesPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appRoutesPath)
	}
	if !strings.Contains(response.Body.String(), `"path":"/posts"`) {
		t.Fatalf("body = %s, want routes JSON", response.Body.String())
	}
}

func TestRouteRowsSortsParams(t *testing.T) {
	rows := routeRows(lazyroutes.RouteTable{
		{
			Method: "GET",
			Path:   "/teams/{team_id}/posts/{post_id}",
			NamedParams: map[string]bool{
				"post_id": true,
				"team_id": true,
			},
		},
	})
	if got, want := rows[0].Params, "post_id, team_id"; got != want {
		t.Fatalf("Params = %q, want %q", got, want)
	}
}
