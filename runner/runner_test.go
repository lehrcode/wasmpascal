package runner_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lehrcode/wasmpascal/internal/wasmtest"
	"github.com/lehrcode/wasmpascal/runner"
)

func TestRunOutputAndExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := runner.Run(context.Background(), wasmtest.Print(1, "Hallo\n", 3), runner.Options{
		Stdout: &stdout, Stderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Errorf("Exit-Code = %d, want 3", code)
	}
	if stdout.String() != "Hallo\n" || stderr.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	if _, err := runner.Run(context.Background(), wasmtest.Print(2, "Fehler\n", 0), runner.Options{
		Stdout: &stdout, Stderr: &stderr,
	}); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || stderr.String() != "Fehler\n" {
		t.Errorf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestRunReturnWithoutExit(t *testing.T) {
	code, err := runner.Run(context.Background(), wasmtest.Return(), runner.Options{})
	if err != nil || code != 0 {
		t.Errorf("Run = %d, %v", code, err)
	}
}

func TestRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := runner.Run(ctx, wasmtest.Loop(), runner.Options{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Run = %v, want DeadlineExceeded", err)
	}
}

func TestRunInvalidModule(t *testing.T) {
	if _, err := runner.Run(context.Background(), []byte("kein wasm"), runner.Options{}); err == nil {
		t.Error("Run mit ungültigem Modul: kein Fehler")
	}
}

func TestRuntimeErrorMessage(t *testing.T) {
	tests := []struct {
		e    runner.RuntimeError
		want string
	}{
		{runner.RuntimeError{Code: 200, File: "zahlen.pas", Line: 42, Proc: "Auswerten"}, "Runtime error 200 at zahlen.pas(42) in Auswerten"},
		{runner.RuntimeError{Code: 2}, "Runtime error 2"},
	}
	for _, tt := range tests {
		if got := tt.e.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}
