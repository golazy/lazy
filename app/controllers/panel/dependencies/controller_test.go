package dependencies

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
