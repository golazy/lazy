package app

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

const maxAppLogRows = 100

type AppController struct {
	panel.Base
}

func New(ctx context.Context) (*AppController, error) {
	base, err := panel.NewBase(ctx)
	return &AppController{Base: base}, err
}

func (c *AppController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setAppState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurbo(w, r, c.streamApp)
		},
	})
}

func (c *AppController) setAppState(r *http.Request) {
	for key, value := range appViewData(c.Snapshot()) {
		c.Set(key, value)
	}
}

func (c *AppController) streamApp(r *http.Request, event buildservice.Event) (string, error) {
	switch event.Type {
	case buildservice.EventState, buildservice.EventFileChange, buildservice.EventReload, buildservice.EventManual, buildservice.EventOutput:
	default:
		return "", nil
	}
	body, err := c.RenderPanelPartial(r, "app", "app_frame", appViewData(c.Snapshot()))
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("replace", "[data-app-panel]", body), nil
}

func appViewData(state buildservice.Snapshot) map[string]any {
	return map[string]any{
		"state":         state,
		"service_rows":  appServiceRows(state),
		"app_log_rows":  appLogRows(state.Events),
		"change_groups": appChangeGroups(state),
	}
}

type appServiceRow struct {
	Name    string
	State   string
	Message string
}

type appLogRow struct {
	Time    string
	Source  string
	Event   string
	Message string
	Details string
}

type appChangeGroup struct {
	Time     string
	Build    string
	Duration string
	Message  string
	Files    []string
}

func appServiceRows(state buildservice.Snapshot) []appServiceRow {
	rows := make([]appServiceRow, 0, len(state.Services)+1)
	rows = append(rows, appServiceRow{Name: "app", State: string(state.State), Message: state.Message})
	for _, service := range state.Services {
		rows = append(rows, appServiceRow{
			Name:    service.Name,
			State:   string(service.State),
			Message: service.Message,
		})
	}
	return rows
}

func appLogRows(events []buildservice.Event) []appLogRow {
	start := 0
	if len(events) > maxAppLogRows {
		start = len(events) - maxAppLogRows
	}
	rows := make([]appLogRow, 0, len(events)-start)
	for i := len(events) - 1; i >= start; i-- {
		event := events[i]
		rows = append(rows, appLogRow{
			Time:    event.Time.Local().Format("15:04:05"),
			Source:  appLogSource(event),
			Event:   appLogEvent(event),
			Message: appLogMessage(event),
			Details: appLogDetails(event),
		})
	}
	return rows
}

func appChangeGroups(state buildservice.Snapshot) []appChangeGroup {
	seen := map[string]struct{}{}
	groups := make([]appChangeGroup, 0)
	for i := len(state.Events) - 1; i >= 0 && len(groups) < 20; i-- {
		event := state.Events[i]
		if len(event.Changed) == 0 {
			continue
		}
		key := strings.Join(event.Changed, "\x00") + "|" + strconv.Itoa(event.Build) + "|" + event.Duration + "|" + event.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		groups = append(groups, appChangeGroup{
			Time:     event.Time.Local().Format("15:04:05"),
			Build:    buildLabel(event.Build),
			Duration: event.Duration,
			Message:  appLogMessage(event),
			Files:    append([]string(nil), event.Changed...),
		})
	}
	if len(groups) == 0 && len(state.Changed) > 0 {
		groups = append(groups, appChangeGroup{
			Build:    buildLabel(state.BuildCount),
			Duration: state.Duration,
			Message:  state.Message,
			Files:    append([]string(nil), state.Changed...),
		})
	}
	return groups
}

func appLogSource(event buildservice.Event) string {
	if event.Service != "" {
		return event.Service
	}
	return "app"
}

func appLogEvent(event buildservice.Event) string {
	if event.Task != "" {
		return string(event.Type) + ":" + event.Task
	}
	if event.State != "" {
		return string(event.State)
	}
	return string(event.Type)
}

func appLogMessage(event buildservice.Event) string {
	if strings.TrimSpace(event.Message) != "" {
		return event.Message
	}
	if strings.TrimSpace(event.Output) != "" {
		return strings.TrimSpace(event.Output)
	}
	if len(event.Changed) > 0 {
		return "Files changed."
	}
	return string(event.Type)
}

func appLogDetails(event buildservice.Event) string {
	parts := make([]string, 0, 4)
	if event.Build > 0 {
		parts = append(parts, "build="+strconv.Itoa(event.Build))
	}
	if event.Duration != "" {
		parts = append(parts, "duration="+event.Duration)
	}
	if len(event.Changed) > 0 {
		changed := append([]string(nil), event.Changed...)
		sort.Strings(changed)
		parts = append(parts, "changed="+strings.Join(changed, ", "))
	}
	if event.Run > 0 {
		parts = append(parts, "run="+strconv.Itoa(event.Run))
	}
	return strings.Join(parts, " ")
}

func buildLabel(build int) string {
	if build <= 0 {
		return "-"
	}
	return "#" + strconv.Itoa(build)
}
