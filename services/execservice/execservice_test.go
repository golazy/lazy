package execservice

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestExecFunctionsHonorCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, run := range map[string]func() error{
		"exec": func() error {
			return Exec(os.Args[0], []string{"-test.run=^$"}, Options{Context: ctx, Capture: true})
		},
		"output": func() error {
			_, err := ExecOutput(os.Args[0], []string{"-test.run=^$"}, Options{Context: ctx})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context canceled", err)
			}
		})
	}
}
