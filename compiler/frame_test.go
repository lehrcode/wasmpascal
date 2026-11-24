package compiler

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lehrcode/wasmpascal/runner"
)

// TestForceFrame übersetzt die Testprogramme von Etappe 4 so, dass alle
// Variablen im Frame auf dem Schatten-Stack statt in WASM-Locals liegen, und
// erwartet dieselbe Ausgabe. Das sichert die Planregel ab, dass sich jede
// Variable auslagern lässt (Voraussetzung für Etappe 8).
func TestForceFrame(t *testing.T) {
	files, _ := filepath.Glob("../testdata/etappe4/*.pas")
	if len(files) == 0 {
		t.Fatal("keine Testprogramme")
	}
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(strings.TrimSuffix(name, ".pas"), func(t *testing.T) {
			want, err := os.ReadFile(strings.TrimSuffix(file, ".pas") + ".out")
			if err != nil {
				t.Skip("keine .out")
			}
			res, err := Compile(context.Background(), name, Options{FS: os.DirFS("../testdata/etappe4"), forceFrame: true})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			code, err := runner.Run(context.Background(), res.Wasm, runner.Options{Stdout: &out})
			if err != nil {
				t.Fatal(err)
			}
			if code != 0 {
				out.WriteString("[Exit-Code " + string(rune('0'+code)) + "]\n")
			}
			if out.String() != string(want) {
				t.Errorf("Ausgabe mit Frame weicht ab:\n%s", out.String())
			}
		})
	}
}
