package panel

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golazy.dev/lazy/services/buildservice"
)

func TestCacheProxiesApplicationControlPlane(t *testing.T) {
	var gotMethod string
	var gotPath string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"enabled":true,"stats":{"entries":1},"keys":["users-1"]}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &Controller{Base: Base{Store: store}}

	response := httptest.NewRecorder()
	if err := controller.Cache(response, httptest.NewRequest(http.MethodGet, "/_golazy/cache", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if gotMethod != http.MethodGet || gotPath != appCachePath {
		t.Fatalf("proxied request = %s %s, want GET %s", gotMethod, gotPath, appCachePath)
	}
	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(response.Body.String(), `"keys":["users-1"]`) {
		t.Fatalf("body = %s, want cache JSON", response.Body.String())
	}
}

func TestCacheOnAndOffProxyApplicationControlPlane(t *testing.T) {
	var paths []string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"enabled":true,"stats":{},"keys":[]}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &Controller{Base: Base{Store: store}}

	for _, call := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request) error
	}{
		{name: "on", fn: controller.CacheOn},
		{name: "off", fn: controller.CacheOff},
	} {
		response := httptest.NewRecorder()
		if err := call.fn(response, httptest.NewRequest(http.MethodPost, "/_golazy/cache/"+call.name, nil)); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", call.name, response.Code, http.StatusOK)
		}
	}

	want := []string{appCacheOnPath, appCacheOffPath}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("proxied paths = %#v, want %#v", paths, want)
	}
}

func TestRequestMonitoringProxiesApplicationControlPlane(t *testing.T) {
	var paths []string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{"enabled":true,"directory":".tmp/traces"}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &Controller{Base: Base{Store: store}}

	for _, call := range []struct {
		name   string
		method string
		fn     func(http.ResponseWriter, *http.Request) error
	}{
		{name: "state", method: http.MethodGet, fn: controller.RequestMonitoring},
		{name: "on", method: http.MethodPost, fn: controller.RequestMonitoringOn},
		{name: "off", method: http.MethodPost, fn: controller.RequestMonitoringOff},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(call.method, "/_golazy/request-monitoring/"+call.name, nil)
		if err := call.fn(response, request); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d: %s", call.name, response.Code, http.StatusOK, response.Body.String())
		}
	}

	want := []string{
		http.MethodGet + " " + appRequestMonitoringPath,
		http.MethodPost + " " + appRequestMonitoringOnPath,
		http.MethodPost + " " + appRequestMonitoringOffPath,
	}
	if len(paths) != len(want) {
		t.Fatalf("proxied paths = %#v, want %#v", paths, want)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("proxied paths = %#v, want %#v", paths, want)
		}
	}
}

func TestCacheReportsUnavailableControlPlane(t *testing.T) {
	controller := &Controller{Base: Base{Store: buildservice.NewStore(10)}}
	response := httptest.NewRecorder()
	if err := controller.Cache(response, httptest.NewRequest(http.MethodGet, "/_golazy/cache", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
