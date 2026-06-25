package buildservice

import (
	"strings"
	"testing"
)

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
