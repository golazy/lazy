package jobs

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
)

func TestIndexProxiesApplicationControlPlaneForJSON(t *testing.T) {
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
	controller := &Controller{Base: panel.Base{Store: store}}

	request := httptest.NewRequest(http.MethodGet, "/_golazy/jobs", nil)
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	if err := controller.Index(response, request); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if gotMethod != http.MethodGet || gotPath != appJobsPath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appJobsPath)
	}
	if !strings.Contains(response.Body.String(), `"imports.whatsapp"`) {
		t.Fatalf("body = %s, want jobs JSON", response.Body.String())
	}
}
