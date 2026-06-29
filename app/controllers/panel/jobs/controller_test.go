package jobs

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

func TestJobsSnapshotReadsApplicationControlPlane(t *testing.T) {
	var gotMethod string
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"running":true,"definitions":[{"kind":"imports.whatsapp"}],"recent":[]}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	base := panel.Base{Store: store}

	snapshot := base.JobsSnapshot(context.Background())
	if gotMethod != http.MethodGet || gotPath != appJobsPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appJobsPath)
	}
	if snapshot.Error != "" {
		t.Fatalf("snapshot error = %q", snapshot.Error)
	}
	if !snapshot.Running {
		t.Fatalf("snapshot.Running = false, want true")
	}
	if len(snapshot.Definitions) != 1 || snapshot.Definitions[0].Kind != "imports.whatsapp" {
		t.Fatalf("definitions = %#v, want imports.whatsapp", snapshot.Definitions)
	}
}
