package buildservice

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStorePreservesServicesAcrossSnapshotUpdates(t *testing.T) {
	store := NewStore(10)
	store.SetServices([]string{"postgres", "minio"})
	store.UpdateService("postgres", ServiceReady, "Service is ready.")

	store.Update(Snapshot{
		State:      StateRunning,
		Message:    "Application is running.",
		BuildCount: 1,
	})

	snapshot := store.Snapshot()
	want := []ServiceSnapshot{
		{Name: "postgres", State: ServiceReady, Message: "Service is ready."},
		{Name: "minio", State: ServiceNotReady},
	}
	if !reflect.DeepEqual(snapshot.Services, want) {
		t.Fatalf("services = %#v, want %#v", snapshot.Services, want)
	}
}

func TestStoreRecordsServiceTaggedOutputEvents(t *testing.T) {
	store := NewStore(10)
	store.SetServices([]string{"postgres"})
	store.UpdateService("postgres", ServiceReady, "Service is ready.")
	store.AddEvent(Event{
		Type:    EventOutput,
		Service: "postgres",
		Task:    "check",
		Run:     2,
		Stream:  "stderr",
		Output:  "ready\n",
	})

	events := store.Snapshot().Events
	event := events[len(events)-1]
	if event.Service != "postgres" || event.Task != "check" || event.Run != 2 || event.Stream != "stderr" || event.Output != "ready\n" {
		t.Fatalf("event = %#v, want service output", event)
	}
	services := store.Snapshot().Services
	if len(services) != 1 || services[0].State != ServiceReady {
		t.Fatalf("services = %#v, want service state preserved", services)
	}
}

func TestStorePreservesAndClonesBuildTraceAcrossSnapshotUpdates(t *testing.T) {
	store := NewStore(10)
	store.Update(Snapshot{
		State:      StateRunning,
		BuildCount: 1,
		BuildTrace: BuildTraceSummary{
			Available:   true,
			BuildNumber: 1,
			Total:       100 * time.Millisecond,
			Packages: []BuildTracePackage{{
				Package:  "example.test/app",
				Phase:    BuildTracePhaseBuild,
				Duration: 80 * time.Millisecond,
				Count:    1,
			}},
		},
	})

	snapshot := store.Snapshot()
	snapshot.BuildTrace.Packages[0].Package = "mutated"

	store.Update(Snapshot{
		State:      StateRunning,
		BuildCount: 1,
	})

	trace := store.Snapshot().BuildTrace
	if !trace.Available || trace.Packages[0].Package != "example.test/app" {
		t.Fatalf("trace = %#v, want preserved cloned trace", trace)
	}
}

func TestAddDebugTraceArgKeepsGoBuildFlagsBeforePackage(t *testing.T) {
	args := addDebugTraceArg([]string{"build", "-tags", "lazydev", "-o", "app", "./cmd/app"}, "trace.json")
	want := []string{"build", "-debug-trace=trace.json", "-tags", "lazydev", "-o", "app", "./cmd/app"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

func TestParseBuildTraceSummarizesPhasesAndPackages(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	trace := `[
{"name":"Running build command","ph":"B","ts":0,"pid":0,"tid":1},
{"name":"modload.download example.com/mod","ph":"B","ts":1000,"pid":0,"tid":1},
{"name":"modload.download example.com/mod","ph":"E","ts":11000,"pid":0,"tid":1},
{"name":"load.loadPackageData example.com/app/pkg","ph":"B","ts":11000,"pid":0,"tid":2},
{"name":"load.loadPackageData example.com/app/pkg","ph":"E","ts":16000,"pid":0,"tid":2},
{"name":"Executing action (build check cache example.com/app/pkg)","ph":"B","ts":16000,"pid":0,"tid":3},
{"name":"Executing action (build check cache example.com/app/pkg)","ph":"E","ts":18000,"pid":0,"tid":3},
{"name":"Executing action (build example.com/app/pkg)","ph":"B","ts":18000,"pid":0,"tid":3},
{"name":"Executing action (build example.com/app/pkg)","ph":"E","ts":68000,"pid":0,"tid":3},
{"name":"Executing action (link example.com/app/cmd/app)","ph":"B","ts":68000,"pid":0,"tid":4},
{"name":"Executing action (link example.com/app/cmd/app)","ph":"E","ts":98000,"pid":0,"tid":4},
{"name":"Running build command","ph":"E","ts":100000,"pid":0,"tid":1}
]`
	if err := os.WriteFile(tracePath, []byte(trace), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := parseBuildTraceFile(tracePath, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Available || summary.BuildNumber != 7 || summary.Total != 100*time.Millisecond {
		t.Fatalf("summary = %#v, want available build 7 with 100ms total", summary)
	}
	for phase, want := range map[string]time.Duration{
		BuildTracePhaseFetch: 10 * time.Millisecond,
		BuildTracePhaseLoad:  5 * time.Millisecond,
		BuildTracePhaseCache: 2 * time.Millisecond,
		BuildTracePhaseBuild: 50 * time.Millisecond,
		BuildTracePhaseLink:  30 * time.Millisecond,
	} {
		if got := phaseDuration(summary.Phases, phase); got != want {
			t.Fatalf("%s duration = %s, want %s in %#v", phase, got, want, summary.Phases)
		}
	}
	if len(summary.Packages) < 3 {
		t.Fatalf("packages = %#v, want slow package rows", summary.Packages)
	}
	if got := summary.Packages[0]; got.Package != "example.com/app/pkg" || got.Duration != 57*time.Millisecond || got.Phase != BuildTracePhaseBuild {
		t.Fatalf("top package = %#v, want app pkg dominated by build", got)
	}
	if got := summary.Actions[0]; got.Phase != BuildTracePhaseBuild || got.Duration != 50*time.Millisecond {
		t.Fatalf("top action = %#v, want build action", got)
	}
}

func TestDevelopmentAppEnvPreservesTelemetryAndDeduplicatesValues(t *testing.T) {
	env := developmentAppEnv([]string{
		"PATH=/bin",
		"ADDR=old",
		"OTEL_SDK_DISABLED=true",
		"OTEL_TRACES_EXPORTER=none",
		"OTEL_TRACES_EXPORTER=console",
	}, "127.0.0.1:3001", "127.0.0.1:3002")

	for key, want := range map[string]string{
		"ADDR":               "127.0.0.1:3001",
		"CONTROL_PLANE_ADDR": "127.0.0.1:3002",
	} {
		if got := envValue(env, key); got != want {
			t.Fatalf("%s = %q, want %q in %#v", key, got, want, env)
		}
		if got := envCount(env, key); got != 1 {
			t.Fatalf("%s appears %d times in %#v", key, got, env)
		}
	}
	for key, want := range map[string]string{
		"OTEL_SDK_DISABLED":    "true",
		"OTEL_TRACES_EXPORTER": "none",
	} {
		if got := envValue(env, key); got != want {
			t.Fatalf("%s = %q, want preserved %q in %#v", key, got, want, env)
		}
	}
	if got := envValue(env, "PATH"); got != "/bin" {
		t.Fatalf("PATH = %q, want /bin", got)
	}
}

func phaseDuration(phases []BuildTracePhase, name string) time.Duration {
	for _, phase := range phases {
		if phase.Name == name {
			return phase.Duration
		}
	}
	return 0
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func envCount(env []string, key string) int {
	prefix := key + "="
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			count++
		}
	}
	return count
}
