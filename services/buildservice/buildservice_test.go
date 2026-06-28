package buildservice

import (
	"reflect"
	"strings"
	"testing"
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
		Stream:  "stderr",
		Output:  "ready\n",
	})

	events := store.Snapshot().Events
	event := events[len(events)-1]
	if event.Service != "postgres" || event.Stream != "stderr" || event.Output != "ready\n" {
		t.Fatalf("event = %#v, want service output", event)
	}
	services := store.Snapshot().Services
	if len(services) != 1 || services[0].State != ServiceReady {
		t.Fatalf("services = %#v, want service state preserved", services)
	}
}

func TestDevelopmentAppEnvEnablesTelemetryAndDeduplicatesValues(t *testing.T) {
	env := developmentAppEnv([]string{
		"PATH=/bin",
		"ADDR=old",
		"OTEL_SDK_DISABLED=true",
		"OTEL_TRACES_EXPORTER=none",
		"OTEL_TRACES_EXPORTER=console",
	}, "apps/sample_repo", "127.0.0.1:3001", "127.0.0.1:3002")

	for key, want := range map[string]string{
		"ADDR":                        "127.0.0.1:3001",
		"CONTROL_PLANE_ADDR":          "127.0.0.1:3002",
		"OTEL_SDK_DISABLED":           "false",
		"OTEL_SERVICE_NAME":           "sample_repo",
		"OTEL_TRACES_EXPORTER":        "otlp",
		"OTEL_LOGS_EXPORTER":          "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
	} {
		if got := envValue(env, key); got != want {
			t.Fatalf("%s = %q, want %q in %#v", key, got, want, env)
		}
		if got := envCount(env, key); got != 1 {
			t.Fatalf("%s appears %d times in %#v", key, got, env)
		}
	}
	if got := envValue(env, "PATH"); got != "/bin" {
		t.Fatalf("PATH = %q, want /bin", got)
	}
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
