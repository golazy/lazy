package panel

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golazy.dev/lazy/services/buildservice"
)

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
		if response.Code != http.StatusSeeOther {
			t.Fatalf("%s status = %d, want %d", call.name, response.Code, http.StatusSeeOther)
		}
		if got, want := response.Header().Get("Location"), "/_golazy/cache"; got != want {
			t.Fatalf("%s Location = %q, want %q", call.name, got, want)
		}
	}

	want := []string{appCacheOnPath, appCacheOffPath}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("proxied paths = %#v, want %#v", paths, want)
	}
}

func TestRequestMonitoringRedirectsAndProxiesApplicationControlPlaneCommands(t *testing.T) {
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

	response := httptest.NewRecorder()
	if err := controller.RequestMonitoring(response, httptest.NewRequest(http.MethodGet, "/_golazy/request-monitoring", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusSeeOther {
		t.Fatalf("state status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	if got, want := response.Header().Get("Location"), "/_golazy/requests"; got != want {
		t.Fatalf("state Location = %q, want %q", got, want)
	}

	for _, call := range []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request) error
	}{
		{name: "on", fn: controller.RequestMonitoringOn},
		{name: "off", fn: controller.RequestMonitoringOff},
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/_golazy/request-monitoring/"+call.name, nil)
		if err := call.fn(response, request); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusSeeOther {
			t.Fatalf("%s status = %d, want %d: %s", call.name, response.Code, http.StatusSeeOther, response.Body.String())
		}
		if got, want := response.Header().Get("Location"), "/_golazy/requests"; got != want {
			t.Fatalf("%s Location = %q, want %q", call.name, got, want)
		}
	}

	want := []string{
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

func TestRequestToolbarCommandsPreserveSafePanelRedirect(t *testing.T) {
	var paths []string
	appControl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer appControl.Close()

	store := buildservice.NewStore(10)
	store.Update(buildservice.Snapshot{
		State:            buildservice.StateRunning,
		ControlPlaneAddr: strings.TrimPrefix(appControl.URL, "http://"),
	})
	controller := &Controller{Base: Base{Store: store}}

	form := url.Values{"redirect": {"/_golazy/requests?q=pools&type=assets"}}
	request := httptest.NewRequest(http.MethodPost, "/_golazy/request-traces/clear", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	if err := controller.RequestTracesClear(response, request); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Header().Get("Location"), "/_golazy/requests?q=pools&type=assets"; got != want {
		t.Fatalf("safe redirect Location = %q, want %q", got, want)
	}

	request = httptest.NewRequest(http.MethodPost, "/_golazy/cache/off", strings.NewReader(url.Values{"redirect": {"https://example.test"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	if err := controller.CacheOff(response, request); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Header().Get("Location"), "/_golazy/cache"; got != want {
		t.Fatalf("unsafe redirect Location = %q, want fallback %q", got, want)
	}

	want := []string{appRequestTracesClearPath, appCacheOffPath}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("proxied paths = %#v, want %#v", paths, want)
	}
}

func TestCacheCommandReportsUnavailableControlPlane(t *testing.T) {
	controller := &Controller{Base: Base{Store: buildservice.NewStore(10)}}
	response := httptest.NewRecorder()
	if err := controller.CacheOn(response, httptest.NewRequest(http.MethodPost, "/_golazy/cache/on", nil)); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
}
