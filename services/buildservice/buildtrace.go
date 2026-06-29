package buildservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	BuildTracePhaseFetch = "Fetch"
	BuildTracePhaseLoad  = "Load"
	BuildTracePhaseCache = "Cache"
	BuildTracePhaseBuild = "Build"
	BuildTracePhaseLink  = "Link"
	BuildTracePhaseOther = "Other"

	maxBuildTraceRows = 25
)

var buildTracePhaseOrder = []string{
	BuildTracePhaseFetch,
	BuildTracePhaseLoad,
	BuildTracePhaseCache,
	BuildTracePhaseBuild,
	BuildTracePhaseLink,
	BuildTracePhaseOther,
}

type BuildTraceSummary struct {
	Available   bool                `json:"available"`
	BuildNumber int                 `json:"build_number,omitempty"`
	TracePath   string              `json:"trace_path,omitempty"`
	Total       time.Duration       `json:"total,omitempty"`
	Phases      []BuildTracePhase   `json:"phases,omitempty"`
	Packages    []BuildTracePackage `json:"packages,omitempty"`
	Actions     []BuildTraceAction  `json:"actions,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type BuildTracePhase struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
	Count    int           `json:"count"`
}

type BuildTracePackage struct {
	Package  string        `json:"package"`
	Phase    string        `json:"phase"`
	Duration time.Duration `json:"duration"`
	Count    int           `json:"count"`
}

type BuildTraceAction struct {
	Name     string        `json:"name"`
	Phase    string        `json:"phase"`
	Package  string        `json:"package,omitempty"`
	Duration time.Duration `json:"duration"`
}

func (s BuildTraceSummary) Empty() bool {
	return !s.Available && s.BuildNumber == 0 && s.TracePath == "" && s.Error == ""
}

func cloneBuildTraceSummary(summary BuildTraceSummary) BuildTraceSummary {
	summary.Phases = append([]BuildTracePhase(nil), summary.Phases...)
	summary.Packages = append([]BuildTracePackage(nil), summary.Packages...)
	summary.Actions = append([]BuildTraceAction(nil), summary.Actions...)
	return summary
}

func readBuildTraceSummary(tracePath string, buildNumber int) BuildTraceSummary {
	summary := BuildTraceSummary{
		BuildNumber: buildNumber,
		TracePath:   tracePath,
	}
	parsed, err := parseBuildTraceFile(tracePath, buildNumber)
	if err == nil {
		return parsed
	}
	if errors.Is(err, os.ErrNotExist) {
		return summary
	}
	summary.Error = err.Error()
	return summary
}

func parseBuildTraceFile(tracePath string, buildNumber int) (BuildTraceSummary, error) {
	data, err := os.ReadFile(tracePath)
	if err != nil {
		return BuildTraceSummary{}, err
	}
	var events []buildTraceEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return BuildTraceSummary{}, fmt.Errorf("parse Go build trace: %w", err)
	}
	if len(events) == 0 {
		return BuildTraceSummary{}, fmt.Errorf("parse Go build trace: no events")
	}
	return summarizeBuildTrace(events, tracePath, buildNumber), nil
}

type buildTraceEvent struct {
	Name      string  `json:"name"`
	Phase     string  `json:"ph"`
	Timestamp float64 `json:"ts"`
	ThreadID  int64   `json:"tid"`
}

type buildTraceSpan struct {
	Name     string
	Phase    string
	Package  string
	Start    float64
	End      float64
	Duration time.Duration
}

func summarizeBuildTrace(events []buildTraceEvent, tracePath string, buildNumber int) BuildTraceSummary {
	stacks := map[string][]buildTraceEvent{}
	var spans []buildTraceSpan
	minTimestamp := math.Inf(1)
	maxTimestamp := math.Inf(-1)
	var commandDuration time.Duration

	for _, event := range events {
		if event.Timestamp > 0 {
			minTimestamp = min(minTimestamp, event.Timestamp)
			maxTimestamp = max(maxTimestamp, event.Timestamp)
		}
		key := event.stackKey()
		switch event.Phase {
		case "B":
			stacks[key] = append(stacks[key], event)
		case "E":
			stack := stacks[key]
			if len(stack) == 0 {
				continue
			}
			start := stack[len(stack)-1]
			stacks[key] = stack[:len(stack)-1]
			duration := buildTraceDuration(start.Timestamp, event.Timestamp)
			if duration <= 0 {
				continue
			}
			if start.Name == "Running build command" {
				commandDuration = duration
				continue
			}
			phase, pkg, ok := classifyBuildTraceName(start.Name)
			if !ok {
				continue
			}
			spans = append(spans, buildTraceSpan{
				Name:     start.Name,
				Phase:    phase,
				Package:  pkg,
				Start:    start.Timestamp,
				End:      event.Timestamp,
				Duration: duration,
			})
		}
	}

	total := commandDuration
	if total == 0 && !math.IsInf(minTimestamp, 1) && !math.IsInf(maxTimestamp, -1) {
		total = buildTraceDuration(minTimestamp, maxTimestamp)
	}

	return buildTraceSummaryFromSpans(tracePath, buildNumber, total, spans)
}

func (e buildTraceEvent) stackKey() string {
	return strconv.FormatInt(e.ThreadID, 10) + "\x00" + e.Name
}

func buildTraceDuration(start, end float64) time.Duration {
	if end <= start {
		return 0
	}
	return time.Duration(math.Round(end-start)) * time.Microsecond
}

func buildTraceSummaryFromSpans(tracePath string, buildNumber int, total time.Duration, spans []buildTraceSpan) BuildTraceSummary {
	phaseTotals := map[string]BuildTracePhase{}
	packageTotals := map[string]*buildTracePackageAggregate{}
	actions := make([]BuildTraceAction, 0, len(spans))

	for _, span := range spans {
		phase := phaseTotals[span.Phase]
		phase.Name = span.Phase
		phase.Duration += span.Duration
		phase.Count++
		phaseTotals[span.Phase] = phase

		if span.Package != "" {
			aggregate := packageTotals[span.Package]
			if aggregate == nil {
				aggregate = &buildTracePackageAggregate{
					Package:       span.Package,
					PhaseDuration: map[string]time.Duration{},
				}
				packageTotals[span.Package] = aggregate
			}
			aggregate.Duration += span.Duration
			aggregate.Count++
			aggregate.PhaseDuration[span.Phase] += span.Duration
		}

		actions = append(actions, BuildTraceAction{
			Name:     span.Name,
			Phase:    span.Phase,
			Package:  span.Package,
			Duration: span.Duration,
		})
	}

	return BuildTraceSummary{
		Available:   true,
		BuildNumber: buildNumber,
		TracePath:   tracePath,
		Total:       total,
		Phases:      orderedBuildTracePhases(phaseTotals),
		Packages:    topBuildTracePackages(packageTotals),
		Actions:     topBuildTraceActions(actions),
	}
}

type buildTracePackageAggregate struct {
	Package       string
	Duration      time.Duration
	Count         int
	PhaseDuration map[string]time.Duration
}

func orderedBuildTracePhases(totals map[string]BuildTracePhase) []BuildTracePhase {
	phases := make([]BuildTracePhase, 0, len(totals))
	for _, name := range buildTracePhaseOrder {
		phase, ok := totals[name]
		if !ok || phase.Duration <= 0 {
			continue
		}
		phases = append(phases, phase)
		delete(totals, name)
	}
	var remaining []string
	for name := range totals {
		remaining = append(remaining, name)
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		phase := totals[name]
		if phase.Duration > 0 {
			phases = append(phases, phase)
		}
	}
	return phases
}

func topBuildTracePackages(totals map[string]*buildTracePackageAggregate) []BuildTracePackage {
	packages := make([]BuildTracePackage, 0, len(totals))
	for _, aggregate := range totals {
		packages = append(packages, BuildTracePackage{
			Package:  aggregate.Package,
			Phase:    dominantBuildTracePhase(aggregate.PhaseDuration),
			Duration: aggregate.Duration,
			Count:    aggregate.Count,
		})
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Duration == packages[j].Duration {
			return packages[i].Package < packages[j].Package
		}
		return packages[i].Duration > packages[j].Duration
	})
	return limitBuildTracePackages(packages, maxBuildTraceRows)
}

func dominantBuildTracePhase(totals map[string]time.Duration) string {
	type phaseDuration struct {
		Name     string
		Duration time.Duration
	}
	var phases []phaseDuration
	for name, duration := range totals {
		phases = append(phases, phaseDuration{Name: name, Duration: duration})
	}
	sort.Slice(phases, func(i, j int) bool {
		if phases[i].Duration == phases[j].Duration {
			return phaseOrderIndex(phases[i].Name) < phaseOrderIndex(phases[j].Name)
		}
		return phases[i].Duration > phases[j].Duration
	})
	if len(phases) == 0 {
		return BuildTracePhaseOther
	}
	return phases[0].Name
}

func phaseOrderIndex(name string) int {
	for index, phase := range buildTracePhaseOrder {
		if phase == name {
			return index
		}
	}
	return len(buildTracePhaseOrder)
}

func limitBuildTracePackages(packages []BuildTracePackage, limit int) []BuildTracePackage {
	if len(packages) <= limit {
		return packages
	}
	return append([]BuildTracePackage(nil), packages[:limit]...)
}

func topBuildTraceActions(actions []BuildTraceAction) []BuildTraceAction {
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Duration == actions[j].Duration {
			return actions[i].Name < actions[j].Name
		}
		return actions[i].Duration > actions[j].Duration
	})
	if len(actions) <= maxBuildTraceRows {
		return actions
	}
	return append([]BuildTraceAction(nil), actions[:maxBuildTraceRows]...)
}

func classifyBuildTraceName(name string) (string, string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, " -> ") {
		return "", "", false
	}
	lowerName := strings.ToLower(name)
	if action, ok := parenthesizedBuildTraceAction(name, "Executing action ("); ok {
		return classifyBuildTraceAction(action)
	}
	if action, ok := parenthesizedBuildTraceAction(name, "exec.Builder.Do ("); ok {
		return classifyBuildTraceAction(action)
	}
	if strings.Contains(lowerName, "download") || strings.Contains(lowerName, "fetch") {
		return BuildTracePhaseFetch, cleanTracePackage(traceNameSuffix(name)), true
	}
	if strings.HasPrefix(name, "load.") || strings.HasPrefix(name, "modload.") {
		return BuildTracePhaseLoad, cleanTracePackage(traceNameSuffix(name)), true
	}
	return BuildTracePhaseOther, "", false
}

func parenthesizedBuildTraceAction(name string, prefix string) (string, bool) {
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ")") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(name, prefix), ")")), true
}

func classifyBuildTraceAction(action string) (string, string, bool) {
	action = strings.TrimSpace(action)
	if action == "" {
		return "", "", false
	}
	lowerAction := strings.ToLower(action)
	if strings.Contains(lowerAction, "download") || strings.Contains(lowerAction, "fetch") {
		return BuildTracePhaseFetch, cleanTracePackage(actionTail(action)), true
	}
	switch {
	case strings.HasPrefix(action, "build check cache "):
		return BuildTracePhaseCache, cleanTracePackage(strings.TrimPrefix(action, "build check cache ")), true
	case strings.HasPrefix(action, "build "):
		return BuildTracePhaseBuild, cleanTracePackage(strings.TrimPrefix(action, "build ")), true
	case strings.HasPrefix(action, "link "):
		return BuildTracePhaseLink, cleanTracePackage(strings.TrimPrefix(action, "link ")), true
	case strings.HasPrefix(action, "build-install "):
		return BuildTracePhaseBuild, cleanTracePackage(strings.TrimPrefix(action, "build-install ")), true
	case strings.HasPrefix(action, "install "):
		return BuildTracePhaseBuild, cleanTracePackage(strings.TrimPrefix(action, "install ")), true
	}
	if strings.Contains(lowerAction, "link") {
		return BuildTracePhaseLink, cleanTracePackage(actionTail(action)), true
	}
	return BuildTracePhaseOther, cleanTracePackage(actionTail(action)), true
}

func traceNameSuffix(name string) string {
	index := strings.LastIndexByte(name, ' ')
	if index < 0 || index == len(name)-1 {
		return ""
	}
	return name[index+1:]
}

func actionTail(action string) string {
	fields := strings.Fields(action)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func cleanTracePackage(value string) string {
	value = strings.Trim(value, `" `)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return ""
	}
	if strings.ContainsAny(value, "\t\r\n") {
		return strings.Fields(value)[0]
	}
	return value
}
