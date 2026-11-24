package wasmpascal_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lehrcode/wasmpascal/compiler"
	"github.com/lehrcode/wasmpascal/internal/wasmtest"
)

// TestHarness prüft den Harness selbst mit einer Compiler-Attrappe.
func TestHarness(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	modules := map[string][]byte{
		"ok.pas":       wasmtest.Print(1, "Hallo\n", 0),
		"sub/exit.pas": wasmtest.Print(2, "x", 3),
		"wrong.pas":    wasmtest.Print(1, "richtig\n", 0),
		"neu.pas":      wasmtest.Print(1, "neu\n", 0),
		"wechsel.pas":  wasmtest.Print(1, "läuft\n", 0),
	}
	fake := func(ctx context.Context, file string, opts compiler.Options) (*compiler.Result, error) {
		if m, ok := modules[file]; ok {
			return &compiler.Result{Wasm: m}, nil
		}
		return nil, &compiler.Diagnostic{File: file, Line: 1, Col: 9, Code: 85, Msg: `";" expected`}
	}

	for _, name := range []string{"ok", "sub/exit", "bad", "wrong", "neu", "wechsel", "unit"} {
		write(name+".pas", "")
	}
	write("ok.out", "Hallo\n")
	write("sub/exit.out", "x\n[Exit-Code 3]\n")
	write("bad.err", "bad.pas(1,9): Error 85: \";\" expected\n")
	write("wrong.out", "falsch\n")
	write("neu.out", "")
	write("wechsel.err", "alt\n")

	h := harness{fsys: os.DirFS(dir), dir: dir, compile: fake}
	cases, err := h.cases()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"bad.pas", "neu.pas", "ok.pas", "sub/exit.pas", "wechsel.pas", "wrong.pas"}
	if !slices.Equal(cases, want) {
		t.Fatalf("cases = %q, want %q (unit.pas ohne Erwartung überspringen)", cases, want)
	}

	ctx := context.Background()
	for _, c := range []string{"ok.pas", "sub/exit.pas", "bad.pas"} {
		if err := h.check(ctx, c); err != nil {
			t.Errorf("check(%s): %v", c, err)
		}
	}
	for c, msg := range map[string]string{
		"wrong.pas":   "Zeile 1",
		"neu.pas":     "weicht",
		"wechsel.pas": "Compilerfehler erwartet",
	} {
		if err := h.check(ctx, c); err == nil || !strings.Contains(err.Error(), msg) {
			t.Errorf("check(%s) = %v, want Fehler mit %q", c, err, msg)
		}
	}

	h.update = true
	for _, c := range []string{"neu.pas", "wechsel.pas"} {
		if err := h.check(ctx, c); err != nil {
			t.Errorf("update %s: %v", c, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "neu.out")); string(b) != "neu\n" {
		t.Errorf("neu.out = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "wechsel.err")); !os.IsNotExist(err) {
		t.Errorf("wechsel.err nicht entfernt: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wechsel.out")); string(b) != "läuft\n" {
		t.Errorf("wechsel.out = %q", b)
	}
}

// TestCompilerBuildsForWasm prüft, dass der Compiler für GOOS=js und GOOS=wasip1
// baut und nur die Standardbibliothek nutzt.
func TestCompilerBuildsForWasm(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go-Werkzeug nicht im PATH")
	}
	for _, goos := range []string{"js", "wasip1"} {
		t.Run(goos, func(t *testing.T) {
			t.Parallel()
			cmd := exec.Command(goTool, "build", "./compiler/...")
			cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=wasm")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("GOOS=%s GOARCH=wasm go build ./compiler/...: %v\n%s", goos, err, out)
			}
		})
	}
	t.Run("stdlib", func(t *testing.T) {
		t.Parallel()
		out, err := exec.Command(goTool, "list", "-deps",
			"-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", "./compiler/...").Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range strings.Fields(string(out)) {
			if !strings.HasPrefix(imp, "github.com/lehrcode/wasmpascal/compiler") {
				t.Errorf("compiler hängt von %s ab; erlaubt ist nur die Standardbibliothek", imp)
			}
		}
	})
}
