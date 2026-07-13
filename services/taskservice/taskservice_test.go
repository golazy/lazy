package taskservice

import (
	"context"
	"testing"

	"golazy.dev/lazy/services/execservice"
)

func TestRunTaskContextPassesContextToRunner(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(context.Background(), contextKey("task"), "value")
	err := RunTaskContext(ctx, func(command string, args []string, options execservice.Options) error {
		if command != "mise" || len(args) != 2 || args[0] != "run" || args[1] != "assets:build" {
			t.Fatalf("command = %s %#v", command, args)
		}
		if options.Context != ctx {
			t.Fatalf("context = %v, want caller context", options.Context)
		}
		return nil
	}, "/app", "assets:build", nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTaskNameFromFile(t *testing.T) {
	for filename, want := range map[string]string{
		"assets/build":    "assets:build",
		"assets/build.sh": "assets:build",
		"release.go":      "release",
	} {
		if got := TaskNameFromFile(filename); got != want {
			t.Errorf("TaskNameFromFile(%q) = %q, want %q", filename, got, want)
		}
	}
}

func TestRunTaskCompatibilityWrapperProvidesContext(t *testing.T) {
	err := RunTask(func(_ string, _ []string, options execservice.Options) error {
		if options.Context == nil {
			t.Fatal("compatibility wrapper passed nil context")
		}
		return nil
	}, "/app", "assets:build", nil, nil, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
}
