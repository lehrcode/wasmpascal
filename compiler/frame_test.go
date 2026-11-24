package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/runner"
)

// TestForceFrame übersetzt die Testprogramme der Etappen 4 und 5 so, dass
// alle Variablen im Frame auf dem Schatten-Stack statt in WASM-Locals liegen,
// und erwartet dieselbe Ausgabe. Das sichert die Planregel ab, dass sich jede
// Variable auslagern lässt (Voraussetzung für Etappe 8).
func TestForceFrame(t *testing.T) {
	var files []string
	for _, dir := range []string{"etappe4", "etappe5"} {
		f, _ := filepath.Glob(filepath.Join("..", "testdata", dir, "*.pas"))
		files = append(files, f...)
	}
	if len(files) == 0 {
		t.Fatal("keine Testprogramme")
	}
	for _, file := range files {
		rel, _ := filepath.Rel(filepath.Join("..", "testdata"), file)
		rel = filepath.ToSlash(rel)
		t.Run(strings.TrimSuffix(rel, ".pas"), func(t *testing.T) {
			want, err := os.ReadFile(strings.TrimSuffix(file, ".pas") + ".out")
			if err != nil {
				t.Skip("keine .out")
			}
			res, err := Compile(context.Background(), rel, Options{FS: os.DirFS("../testdata"), forceFrame: true})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			code, err := runner.Run(context.Background(), res.Wasm, runner.Options{Stdout: &out})
			var rte *runner.RuntimeError
			switch {
			case errors.As(err, &rte):
				out.WriteString(rte.Error() + "\n")
			case err != nil:
				t.Fatal(err)
			case code != 0:
				fmt.Fprintf(&out, "[Exit-Code %d]\n", code)
			}
			if out.String() != string(want) {
				t.Errorf("Ausgabe mit Frame weicht ab:\n%s", out.String())
			}
		})
	}
}

// TestNoLines: Ohne Zeilentabelle nennen Laufzeitfehler nur die Nummer.
func TestNoLines(t *testing.T) {
	src := fstest.MapFS{"p.pas": {Data: []byte("program P;\nvar a: Integer;\nbegin\n  a := 0;\n  WriteLn(1 div a)\nend.\n")}}
	for _, noLines := range []bool{false, true} {
		res, err := Compile(context.Background(), "p.pas", Options{FS: src, NoLines: noLines})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.Run(context.Background(), res.Wasm, runner.Options{})
		want := "Runtime error 200 at p.pas(5) in P"
		if noLines {
			want = "Runtime error 200"
		}
		if err == nil || err.Error() != want {
			t.Errorf("NoLines=%v: %v, want %q", noLines, err, want)
		}
	}
}
