package runner_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lehrcode/wasmpascal/compiler"
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

func TestRunEnv(t *testing.T) {
	var stdout bytes.Buffer
	code, err := runner.Run(context.Background(), wasmtest.Environ(), runner.Options{
		Stdout: &stdout,
		// "=C:" und "=ExitCode" sind die versteckten cmd.exe-Variablen
		// (Raymond Chen, devblogs.microsoft.com/oldnewthing/20100506-00/):
		// WASI kann Namen mit "=" nicht darstellen, sie werden fallengelassen.
		Env: []string{"A=1", "=C:=C:\\versteckt", "B=2=3", "=ExitCode=0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("Exit-Code = %d, want 0", code)
	}
	if want := "A=1\x00B=2=3\x00"; stdout.String() != want {
		t.Errorf("Umgebung = %q, want %q", stdout.String(), want)
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

// external-Routinen: Imports stellt sie bereit; ein fehlender Import oder ein
// reserviertes Modul ergibt einen Fehler vor dem Start (Etappe 17).
func TestRunImports(t *testing.T) {
	const src = `program P;
function Twice(X: LongInt): LongInt; external 'host' name 'twice';
begin
  WriteLn(Twice(21))
end.
`
	res, err := compiler.Compile(t.Context(), "p.pas", compiler.Options{FS: fstest.MapFS{"p.pas": {Data: []byte(src)}}})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := runner.Run(t.Context(), res.Wasm, runner.Options{Stdout: &out,
		Imports: map[string]map[string]any{"host": {"twice": func(x int32) int32 { return 2 * x }}}}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "42\n" {
		t.Errorf("Ausgabe %q", out.String())
	}
	for _, c := range []struct {
		name    string
		imports map[string]map[string]any
		want    string
	}{
		{"fehlt", nil, "host.twice (i32) -> (i32) fehlt"},
		{"signatur", map[string]map[string]any{"host": {"twice": func(x float64) float64 { return x }}}, "Programm erwartet"},
		{"reserviert", map[string]map[string]any{"gojs": {"x": func() {}}}, "reserviert"},
		{"kein func", map[string]map[string]any{"host": {"twice": 42}}, "keine Funktion"},
	} {
		_, err := runner.Run(t.Context(), res.Wasm, runner.Options{Imports: c.imports})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, erwartet %q", c.name, err, c.want)
		}
	}
}
