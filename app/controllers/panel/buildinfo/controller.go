package buildinfo

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golazy.dev/lazy/app/controllers/panel"
	"golazy.dev/lazy/services/buildservice"
	"golazy.dev/lazycontroller"
)

type BuildInfoController struct {
	panel.Base
}

func New(ctx context.Context) (*BuildInfoController, error) {
	base, err := panel.NewBase(ctx)
	return &BuildInfoController{Base: base}, err
}

func (c *BuildInfoController) Index(w http.ResponseWriter, r *http.Request) error {
	return c.Wants(lazycontroller.Formats{
		lazycontroller.HTML: func() error {
			c.setBuildInfoState(r)
			return nil
		},
		lazycontroller.SSE: func() error {
			return c.StreamTurboWithInitial(w, r, c.streamBuildInfoInitial, c.streamBuildInfo)
		},
	})
}

func (c *BuildInfoController) setBuildInfoState(r *http.Request) {
	for key, value := range c.buildInfoViewData(r) {
		c.Set(key, value)
	}
}

func (c *BuildInfoController) streamBuildInfoInitial(r *http.Request) (string, error) {
	return c.renderBuildInfo(r)
}

func (c *BuildInfoController) streamBuildInfo(r *http.Request, event buildservice.Event) (string, error) {
	if event.Type != buildservice.EventState {
		return "", nil
	}
	switch event.State {
	case buildservice.StateBuildFailed, buildservice.StateRunFailed, buildservice.StateRunning:
	default:
		return "", nil
	}
	return c.renderBuildInfo(r)
}

func (c *BuildInfoController) renderBuildInfo(r *http.Request) (string, error) {
	body, err := c.RenderPanelPartial(r, "buildinfo", "buildinfo_frame", c.buildInfoViewData(r))
	if err != nil {
		return "", err
	}
	return panel.TurboStreamTargets("replace", "[data-buildinfo-panel]", body), nil
}

func (c *BuildInfoController) buildInfoViewData(r *http.Request) map[string]any {
	snapshot := c.Snapshot()
	return map[string]any{
		"state":      snapshot,
		"buildinfo":  c.BuildInfoSnapshot(r.Context()),
		"buildtabs":  newBuildInfoTabs(r.URL.Query().Get("tab")),
		"buildtrace": newBuildTraceView(snapshot.BuildTrace),
	}
}

type buildInfoTabs struct {
	Tab             string
	RuntimeTab      bool
	SettingsTab     bool
	DependenciesTab bool
}

func newBuildInfoTabs(tab string) buildInfoTabs {
	normalized := normalizeBuildInfoTab(tab)
	return buildInfoTabs{
		Tab:             normalized,
		RuntimeTab:      normalized == "runtime",
		SettingsTab:     normalized == "settings",
		DependenciesTab: normalized == "dependencies",
	}
}

func normalizeBuildInfoTab(tab string) string {
	normalized := strings.ToLower(strings.TrimSpace(tab))
	switch normalized {
	case "settings", "dependencies":
		return normalized
	default:
		return "runtime"
	}
}

func (t buildInfoTabs) TabURL(tab string) string {
	tab = normalizeBuildInfoTab(tab)
	values := url.Values{}
	if tab != "runtime" {
		values.Set("tab", tab)
	}
	if encoded := values.Encode(); encoded != "" {
		return "/_golazy/buildinfo?" + encoded
	}
	return "/_golazy/buildinfo"
}

type buildTraceView struct {
	Available          bool
	Error              string
	BuildNumber        int
	Total              string
	TopPhase           string
	TopPhaseDuration   string
	TopPackage         string
	TopPackageDuration string
	Phases             []buildTracePhaseRow
	Packages           []buildTracePackageRow
}

type buildTracePhaseRow struct {
	Name     string
	Duration string
	Count    string
	Width    string
}

type buildTracePackageRow struct {
	Package  string
	Phase    string
	Duration string
	Count    string
	Width    string
}

func newBuildTraceView(summary buildservice.BuildTraceSummary) buildTraceView {
	view := buildTraceView{
		Available:   summary.Available,
		Error:       summary.Error,
		BuildNumber: summary.BuildNumber,
		Total:       formatBuildTraceDuration(summary.Total),
	}
	if !summary.Available {
		return view
	}

	phaseMax := maxBuildTracePhaseDuration(summary.Phases)
	for _, phase := range summary.Phases {
		if phase.Duration == phaseMax {
			view.TopPhase = phase.Name
			view.TopPhaseDuration = formatBuildTraceDuration(phase.Duration)
		}
		view.Phases = append(view.Phases, buildTracePhaseRow{
			Name:     phase.Name,
			Duration: formatBuildTraceDuration(phase.Duration),
			Count:    traceCountText(phase.Count),
			Width:    buildTraceWidth(phase.Duration, phaseMax),
		})
	}

	packageMax := maxBuildTracePackageDuration(summary.Packages)
	if len(summary.Packages) > 0 {
		view.TopPackage = summary.Packages[0].Package
		view.TopPackageDuration = formatBuildTraceDuration(summary.Packages[0].Duration)
	}
	for _, pkg := range summary.Packages {
		if len(view.Packages) >= 5 {
			break
		}
		view.Packages = append(view.Packages, buildTracePackageRow{
			Package:  pkg.Package,
			Phase:    pkg.Phase,
			Duration: formatBuildTraceDuration(pkg.Duration),
			Count:    traceCountText(pkg.Count),
			Width:    buildTraceWidth(pkg.Duration, packageMax),
		})
	}
	return view
}

func maxBuildTracePhaseDuration(phases []buildservice.BuildTracePhase) time.Duration {
	var maxDuration time.Duration
	for _, phase := range phases {
		if phase.Duration > maxDuration {
			maxDuration = phase.Duration
		}
	}
	return maxDuration
}

func maxBuildTracePackageDuration(packages []buildservice.BuildTracePackage) time.Duration {
	var maxDuration time.Duration
	for _, pkg := range packages {
		if pkg.Duration > maxDuration {
			maxDuration = pkg.Duration
		}
	}
	return maxDuration
}

func buildTraceWidth(duration time.Duration, maxDuration time.Duration) string {
	if duration <= 0 || maxDuration <= 0 {
		return "0%"
	}
	percent := int(math.Round(float64(duration) / float64(maxDuration) * 100))
	if percent < 2 {
		percent = 2
	}
	if percent > 100 {
		percent = 100
	}
	return fmt.Sprintf("%d%%", percent)
}

func traceCountText(count int) string {
	if count == 1 {
		return "1 span"
	}
	return fmt.Sprintf("%d spans", count)
}

func formatBuildTraceDuration(duration time.Duration) string {
	if duration <= 0 {
		return "0ms"
	}
	switch {
	case duration < time.Millisecond:
		return fmt.Sprintf("%dus", duration.Microseconds())
	case duration < time.Second:
		return fmt.Sprintf("%.1fms", float64(duration)/float64(time.Millisecond))
	default:
		return duration.Round(10 * time.Millisecond).String()
	}
}
