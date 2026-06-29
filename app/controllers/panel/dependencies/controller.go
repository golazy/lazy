package dependencies

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

const (
	shutdownActionPath = "/_golazy/dependencies/shutdown"
	shutdownEventsPath = "/_golazy/dependencies/shutdown/events"
	homeLoadRate       = 10
)

type DependenciesController struct {
	panel.Base
}

func New(ctx context.Context) (*DependenciesController, error) {
	base, err := panel.NewBase(ctx)
	return &DependenciesController{Base: base}, err
}

func (c *DependenciesController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setDependenciesState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamDependenciesInitial, c.streamDependencies)
		},
	})
}

func (c *DependenciesController) StartShutdown(w http.ResponseWriter, r *http.Request) error {
	delaySeconds := shutdownDelaySeconds(r)
	snapshot := c.Snapshot()
	if strings.TrimSpace(snapshot.AppAddr) == "" {
		http.Error(w, "application address is not available", http.StatusServiceUnavailable)
		return nil
	}
	if strings.TrimSpace(snapshot.ControlPlaneAddr) == "" {
		http.Error(w, "application control plane is not available", http.StatusServiceUnavailable)
		return nil
	}

	go runShutdownSimulation(snapshot.AppAddr, snapshot.ControlPlaneAddr, delaySeconds)
	http.Redirect(w, r, "/_golazy/dependencies", http.StatusSeeOther)
	return nil
}

func (c *DependenciesController) ShutdownEvents(w http.ResponseWriter, r *http.Request) error {
	addr := strings.TrimSpace(c.Snapshot().ControlPlaneAddr)
	if addr == "" {
		http.Error(w, "application control plane is not available", http.StatusServiceUnavailable)
		return nil
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://"+addr+"/dependencies/shutdown/events", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "text/event-stream")

	response, err := shutdownStreamClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("application shutdown stream returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	buffer := make([]byte, 16*1024)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return nil
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if readErr == nil {
			continue
		}
		if readErr == io.EOF {
			return nil
		}
		return readErr
	}
}

func (c *DependenciesController) setDependenciesState(r *http.Request) {
	for key, value := range c.dependenciesViewData(r) {
		c.Set(key, value)
	}
}

func (c *DependenciesController) streamDependenciesInitial(r *http.Request) (string, error) {
	return c.renderDependencies(r)
}

func (c *DependenciesController) streamDependencies(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState || event.State != buildservice.StateRunning {
		return "", nil
	}
	return c.renderDependencies(r)
}

func (c *DependenciesController) renderDependencies(r *http.Request) (string, error) {
	body, err := c.RenderPanelPartial(r, "dependencies", "dependencies_frame", c.dependenciesViewData(r))
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("replace", "[data-dependencies-panel]", body), nil
}

func (c *DependenciesController) dependenciesViewData(r *http.Request) map[string]any {
	graph := c.DependencyGraphSnapshot(r.Context())
	shutdown := c.DependencyShutdownSnapshot(r.Context())
	nodes := graph.NodeRows()
	applyShutdownNodeState(nodes, shutdown)
	return map[string]any{
		"state":                        c.Snapshot(),
		"dependencies":                 graph,
		"dependency_nodes":             nodes,
		"dependency_edges":             graph.Edges,
		"dependency_counts":            dependencyCounts{Services: graph.ServiceCount(), Edges: graph.EdgeCount()},
		"dependency_shutdown":          shutdown,
		"dependency_shutdown_action":   shutdownActionPath,
		"dependency_shutdown_events":   shutdownEventsPath,
		"dependency_shutdown_load_rps": homeLoadRate,
	}
}

type dependencyCounts struct {
	Services int
	Edges    int
}

var (
	shutdownPostClient   = &http.Client{Timeout: 2 * time.Second}
	shutdownLoadClient   = &http.Client{Timeout: 2 * time.Second}
	shutdownStreamClient = &http.Client{}
)

func applyShutdownNodeState(rows []panel.DependencyNodeRow, shutdown panel.DependencyShutdownSnapshot) {
	states := map[string]string{}
	for _, node := range shutdown.Nodes {
		if node.Name != "" && node.State != "" {
			states[node.Name] = node.State
		}
	}
	for index := range rows {
		rows[index].State = states[rows[index].Name]
		if rows[index].State == "" {
			rows[index].State = "running"
		}
	}
}

func shutdownDelaySeconds(r *http.Request) int {
	if r == nil {
		return 0
	}
	if err := r.ParseForm(); err != nil {
		return 0
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(r.FormValue("seconds")))
	if err != nil || seconds < 0 {
		return 0
	}
	if seconds > 120 {
		return 120
	}
	return seconds
}

func runShutdownSimulation(appAddr string, controlAddr string, delaySeconds int) {
	if delaySeconds > 0 {
		go runHomeLoad(appAddr, time.Duration(delaySeconds)*time.Second)
	}
	postAppShutdown(controlAddr, delaySeconds)
}

func postAppShutdown(controlAddr string, delaySeconds int) {
	controlAddr = strings.TrimSpace(controlAddr)
	if controlAddr == "" {
		return
	}
	requestURL := "http://" + controlAddr + "/dependencies/shutdown?delay_seconds=" + strconv.Itoa(delaySeconds)
	request, err := http.NewRequest(http.MethodPost, requestURL, nil)
	if err != nil {
		return
	}
	request.Header.Set("Accept", "application/json")
	response, err := shutdownPostClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
}

func runHomeLoad(appAddr string, duration time.Duration) {
	appAddr = strings.TrimSpace(appAddr)
	if appAddr == "" || duration <= 0 {
		return
	}
	homeURL := appHomeURL(appAddr)
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	ticker := time.NewTicker(time.Second / homeLoadRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go requestHome(homeURL)
		}
	}
}

func requestHome(homeURL string) {
	request, err := http.NewRequest(http.MethodGet, homeURL, nil)
	if err != nil {
		return
	}
	request.Header.Set("Connection", "close")
	response, err := shutdownLoadClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
}

func appHomeURL(appAddr string) string {
	value := strings.TrimSpace(appAddr)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		parsed, err := url.Parse(value)
		if err == nil {
			parsed.Path = "/"
			parsed.RawQuery = ""
			parsed.Fragment = ""
			return parsed.String()
		}
		return value
	}
	return "http://" + value + "/"
}
