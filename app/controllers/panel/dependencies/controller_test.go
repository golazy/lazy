package dependencies

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
)

func TestDependencyGraphSnapshotReadsApplicationControlPlane(t *testing.T) {
	var gotMethod string
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"nodes":["app","db","posts"],"edges":[{"from":"app","to":"db"},{"from":"app","to":"posts"},{"from":"posts","to":"db"}]}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	base := panel.Base{Store: store}

	snapshot := base.DependencyGraphSnapshot(context.Background())
	if gotMethod != http.MethodGet || gotPath != "/dependencies" {
		t.Fatalf("proxied request = %s %s, want GET /dependencies", gotMethod, gotPath)
	}
	if snapshot.Error != "" {
		t.Fatalf("snapshot error = %q", snapshot.Error)
	}
	if snapshot.ServiceCount() != 2 || snapshot.EdgeCount() != 3 {
		t.Fatalf("counts = services %d edges %d, want 2 services and 3 edges", snapshot.ServiceCount(), snapshot.EdgeCount())
	}
	rows := snapshot.NodeRows()
	if len(rows) != 3 {
		t.Fatalf("rows = %#v, want 3", rows)
	}
	if rows[2].Name != "posts" || rows[2].DependsOn != "db" || rows[2].UsedBy != "app" {
		t.Fatalf("posts row = %#v, want depends on db and used by app", rows[2])
	}
}

func TestDependencyShutdownSnapshotReadsApplicationControlPlane(t *testing.T) {
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{
			"ready":false,
			"ready_status":503,
			"ready_text":"GET /readyz => 503 not ready",
			"active_requests":2,
			"active_connections":3,
			"phase":"draining",
			"message":"waiting",
			"nodes":[{"name":"app","state":"draining"},{"name":"db","state":"running"}]
		}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	base := panel.Base{Store: store}

	snapshot := base.DependencyShutdownSnapshot(context.Background())
	if gotPath != "/dependencies/shutdown" {
		t.Fatalf("proxied request path = %q, want /dependencies/shutdown", gotPath)
	}
	if snapshot.Error != "" {
		t.Fatalf("snapshot error = %q", snapshot.Error)
	}
	if snapshot.Ready || snapshot.ReadyStatus != http.StatusServiceUnavailable {
		t.Fatalf("ready = %v status = %d, want not ready 503", snapshot.Ready, snapshot.ReadyStatus)
	}
	if snapshot.ActiveRequests != 2 || snapshot.ActiveConnections != 3 {
		t.Fatalf("active counts = %d/%d, want 2/3", snapshot.ActiveRequests, snapshot.ActiveConnections)
	}
}

func TestStartShutdownPostsApplicationControlPlane(t *testing.T) {
	posted := make(chan string, 1)
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posted <- r.Method + " " + r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"running":true}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		AppAddr:          "127.0.0.1:9",
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &DependenciesController{Base: panel.Base{Store: store}}

	request := httptest.NewRequest(http.MethodPost, "/_golazy/dependencies/shutdown", strings.NewReader("seconds=0"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	if err := controller.StartShutdown(response, request); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	select {
	case got := <-posted:
		if got != "POST /dependencies/shutdown?delay_seconds=0" {
			t.Fatalf("posted = %q, want shutdown endpoint", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shutdown POST")
	}
}

func TestShutdownEventsProxiesApplicationControlPlaneStream(t *testing.T) {
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/dependencies/shutdown/events" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: shutdown\ndata: {\"phase\":\"draining\"}\n\n")
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &DependenciesController{Base: panel.Base{Store: store}}

	response := httptest.NewRecorder()
	if err := controller.ShutdownEvents(response, httptest.NewRequest(http.MethodGet, "/_golazy/dependencies/shutdown/events", nil)); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/dependencies/shutdown/events" {
		t.Fatalf("path = %q, want /dependencies/shutdown/events", gotPath)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
	if !strings.Contains(response.Body.String(), `"phase":"draining"`) {
		t.Fatalf("stream body = %q, want draining event", response.Body.String())
	}
}

func TestRunHomeLoadSendsRequests(t *testing.T) {
	var requests atomic.Int64
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, "ok")
	}))
	defer app.Close()

	runHomeLoad(strings.TrimPrefix(app.URL, "http://"), 350*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if got := requests.Load(); got == 0 {
		t.Fatal("home load sent no requests")
	}
}
