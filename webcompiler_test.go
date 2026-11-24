package wasmpascal_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lehrcode/wasmpascal/compiler"
)

// TestWebCompiler baut den Compiler für den Browser (cmd/pascaljs, GOOS=js),
// übersetzt damit in Node.js alle Testprogramme unter testdata/ (über
// web/node-compile.js und pascalCompile) und verlangt dasselbe Ergebnis wie
// der native Compiler mit dem Ziel gojs: byte-gleiche Module bzw. dieselben
// Compilermeldungen (mit Dateinamen ohne Verzeichnis).
func TestWebCompiler(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node nicht gefunden")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go-Werkzeug nicht im PATH")
	}
	tmp := t.TempDir()
	compilerWasm := filepath.Join(tmp, "compiler.wasm")
	build := exec.Command(goTool, "build", "-o", compilerWasm, "./cmd/pascaljs")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/pascaljs: %v\n%s", err, out)
	}

	h := harness{fsys: os.DirFS("testdata")}
	cases, err := h.cases()
	if err != nil {
		t.Fatal(err)
	}
	script, _ := filepath.Abs(filepath.Join("web", "node-compile.js"))
	outDir := filepath.Join(tmp, "out")
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), node, append([]string{script, compilerWasm, "-o", outDir}, cases...)...)
	cmd.Dir = "testdata"
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 1 {
			t.Fatalf("node-compile: %v\n%s", err, stderr.String())
		}
	}

	var wantErrs []string
	for _, c := range cases {
		res, err := compiler.Compile(t.Context(), c, compiler.Options{Target: compiler.TargetGoJS, FS: h.fsys})
		if err != nil {
			var d *compiler.Diagnostic
			if !errors.As(err, &d) {
				t.Fatalf("%s: %v", c, err)
			}
			d.File = path.Base(d.File)
			wantErrs = append(wantErrs, d.Error())
			continue
		}
		got, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(strings.TrimSuffix(c, ".pas")+".wasm")))
		if err != nil {
			t.Errorf("%s: %v", c, err)
			continue
		}
		if !bytes.Equal(got, res.Wasm) {
			t.Errorf("%s: Modul aus dem Browser-Compiler weicht ab (%d statt %d Byte)", c, len(got), len(res.Wasm))
		}
	}
	gotErrs := strings.Split(strings.TrimSuffix(stderr.String(), "\n"), "\n")
	if stderr.Len() == 0 {
		gotErrs = nil
	}
	slices.Sort(wantErrs)
	slices.Sort(gotErrs)
	if !slices.Equal(gotErrs, wantErrs) {
		t.Errorf("Compilermeldungen:\n%s\nerwartet:\n%s", strings.Join(gotErrs, "\n"), strings.Join(wantErrs, "\n"))
	}
}

// TestWebExamples übersetzt die Beispiele der Demo-Seite (web/beispiele/).
func TestWebExamples(t *testing.T) {
	fsys := os.DirFS(filepath.Join("web", "beispiele"))
	names, err := fs.Glob(fsys, "*.pas")
	if err != nil || len(names) == 0 {
		t.Fatalf("keine Beispiele: %v", err)
	}
	for _, name := range names {
		if _, err := compiler.Compile(t.Context(), name, compiler.Options{Target: compiler.TargetGoJS, FS: fsys}); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
