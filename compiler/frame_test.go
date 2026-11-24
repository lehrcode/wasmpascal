package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
	"github.com/lehrcode/wasmpascal/runner"
)

// TestForceFrame übersetzt die Testprogramme der Etappen 4 bis 7 so, dass
// alle Variablen im Frame auf dem Schatten-Stack statt in WASM-Locals liegen,
// und erwartet dieselbe Ausgabe. Das sichert die Planregel ab, dass sich jede
// Variable auslagern lässt (Voraussetzung für Etappe 8).
func TestForceFrame(t *testing.T) {
	var files []string
	for _, dir := range []string{"etappe4", "etappe5", "etappe6", "etappe7", "etappe8"} {
		_ = filepath.WalkDir(filepath.Join("..", "testdata", dir), func(p string, d fs.DirEntry, err error) error {
			if err == nil && strings.HasSuffix(p, ".pas") {
				if _, err := os.Stat(strings.TrimSuffix(p, ".pas") + ".out"); err == nil {
					files = append(files, p)
				}
			}
			return err
		})
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
			opts := runner.Options{Stdout: &out, Args: []string{strings.TrimSuffix(filepath.Base(rel), ".pas")},
				Dirs: map[string]string{"/": t.TempDir()}}
			if a, err := os.ReadFile(strings.TrimSuffix(file, ".pas") + ".args"); err == nil {
				opts.Args = append(opts.Args, strings.Fields(string(a))...)
			}
			if in, err := os.ReadFile(strings.TrimSuffix(file, ".pas") + ".in"); err == nil {
				opts.Stdin = bytes.NewReader(in)
			}
			code, err := runner.Run(context.Background(), res.Wasm, opts)
			var rte *runner.RuntimeError
			if (err != nil || code != 0) && out.Len() > 0 && !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
				out.WriteByte('\n')
			}
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

// TestNoLines: Ohne Zeilentabelle nennen Laufzeitfehler nur die Nummer. Mit
// Zeilentabelle steht nur der Dateiname ohne Verzeichnis im Modul.
func TestNoLines(t *testing.T) {
	src := fstest.MapFS{"home/user/prog/p.pas": {Data: []byte("program P;\nvar a: Integer;\nbegin\n  a := 0;\n  WriteLn(1 div a)\nend.\n")}}
	for _, noLines := range []bool{false, true} {
		res, err := Compile(context.Background(), "home/user/prog/p.pas", Options{FS: src, NoLines: noLines})
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
		if bytes.Contains(res.Wasm, []byte("home/user")) {
			t.Errorf("NoLines=%v: Modul enthält das Verzeichnis", noLines)
		}
	}
}

// TestFileRecSizes prüft, dass die Größen der Datei-Records im Compiler zu
// TFileRec und TTextRec in rtl/system.pas passen.
func TestFileRecSizes(t *testing.T) {
	system, err := parseSystem([]string{"WASI"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := check.Check(&ast.Program{Block: &ast.Block{Body: &ast.CompoundStmt{}}}, check.Config{System: system})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int64{"_filerecsize": types.FileRecSize, "_textrecsize": types.TextRecSize} {
		c, ok := info.SystemImpl.Syms[name].(*types.Const)
		if !ok {
			t.Fatalf("%s fehlt in rtl/system.pas", name)
		}
		if c.Val.Int != want {
			t.Errorf("%s = %d in rtl/system.pas, im Compiler %d", name, c.Val.Int, want)
		}
	}
}
