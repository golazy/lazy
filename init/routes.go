package appinit

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	panelcontroller "golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/app/controllers/panel/actions"
	panelapp "golazy.dev/lazy/app/controllers/panel/app"
	"golazy.dev/lazy/app/controllers/panel/assets"
	"golazy.dev/lazy/app/controllers/panel/jobs"
	"golazy.dev/lazy/app/controllers/panel/requests"
	"golazy.dev/lazy/app/controllers/panel/routes"
	"golazy.dev/lazy/app/controllers/panel/services"
	"golazy.dev/lazy/app/controllers/panel/status"
	"golazy.dev/lazy/app/controllers/panel/traces"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazyroutes"
	"golazy.dev/lazysupport/inflection"
)

const devToolsWorkspacePath = "/.well-known/appspecific/com.chrome.devtools.json"

var devToolsWorkspaceNamespace = [16]byte{0x67, 0x6f, 0x6c, 0x61, 0x7a, 0x79, 0x2d, 0x64, 0x65, 0x76, 0x74, 0x6f, 0x6f, 0x6c, 0x73, 0x31}

func init() {
	inflection.Irregular("status", "status")
}

func Draw(router *lazyroutes.Scope) {
	router.HandleFunc(http.MethodGet, devToolsWorkspacePath, func(w http.ResponseWriter, r *http.Request) error {
		return serveDevToolsWorkspace(router.Context, w, r)
	})
	router.Path("_golazy", func(panel *lazyroutes.Scope) {
		panel.Resources(panelcontroller.New, func(resource *lazyroutes.Resource) {
			resource.Singular("panel")
			resource.Plural("panel")
			resource.Path("")
			resource.Get("cache", (*panelcontroller.Controller).Cache)
			resource.Post("cache/on", (*panelcontroller.Controller).CacheOn)
			resource.Post("cache/off", (*panelcontroller.Controller).CacheOff)
			resource.Get("request-monitoring", (*panelcontroller.Controller).RequestMonitoring)
			resource.Post("request-monitoring/on", (*panelcontroller.Controller).RequestMonitoringOn)
			resource.Post("request-monitoring/off", (*panelcontroller.Controller).RequestMonitoringOff)
			resource.Post("request-traces/clear", (*panelcontroller.Controller).RequestTracesClear)
			resource.Post("rebuild", (*panelcontroller.Controller).Rebuild)
			resource.Post("restart", (*panelcontroller.Controller).Restart)
		})
		panel.Resources(panelapp.New, func(resource *lazyroutes.Resource) {
			resource.Singular("app")
			resource.Plural("app")
			resource.Path("app")
		})
		panel.Resources(requests.New)
		panel.Resources(services.New, func(resource *lazyroutes.Resource) {
			resource.MemberPost("start", (*services.ServicesController).Start)
			resource.MemberPost("stop", (*services.ServicesController).Stop)
			resource.MemberPost("restart", (*services.ServicesController).Restart)
		})
		panel.Resources(traces.New)
		panel.Resources(routes.New)
		panel.Resources(jobs.New)
		panel.Resources(assets.New)
		panel.Resources(actions.New)
		panel.Resources(status.New)
	})
}

type devToolsWorkspaceResponse struct {
	Workspace devToolsWorkspace `json:"workspace"`
}

type devToolsWorkspace struct {
	Root string `json:"root"`
	UUID string `json:"uuid"`
}

func serveDevToolsWorkspace(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	store, ok := buildservice.StoreFromContext(ctx)
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	root := strings.TrimSpace(store.Snapshot().WatchedRoot)
	if root == "" {
		http.NotFound(w, r)
		return nil
	}
	jsRoot, err := filepath.Abs(filepath.Join(root, "app", "js"))
	if err != nil {
		return fmt.Errorf("resolve DevTools workspace root: %w", err)
	}
	response := devToolsWorkspaceResponse{Workspace: devToolsWorkspace{
		Root: jsRoot,
		UUID: devToolsWorkspaceUUID(jsRoot),
	}}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		return nil
	}
	return json.NewEncoder(w).Encode(response)
}

func devToolsWorkspaceUUID(root string) string {
	hash := sha1.New()
	_, _ = hash.Write(devToolsWorkspaceNamespace[:])
	_, _ = hash.Write([]byte(root))
	sum := hash.Sum(nil)
	var uuid [16]byte
	copy(uuid[:], sum)
	uuid[6] = (uuid[6] & 0x0f) | 0x50
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
