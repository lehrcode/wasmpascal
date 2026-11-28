package wasmpascal_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lehrcode/wasmpascal/compiler"
	"github.com/lehrcode/wasmpascal/internal/wasmtest"
	"github.com/lehrcode/wasmpascal/runner"
)

var update = flag.Bool("update", false, "erwartete Ausgaben (*.out, *.err) unter testdata/ neu schreiben")

// caseTimeout begrenzt Übersetzung plus Ausführung eines Testprogramms.
const caseTimeout = 10 * time.Second

// TestPrograms übersetzt jedes Testprogramm unter testdata/, führt es aus und
// vergleicht das Ergebnis mit der erwarteten Ausgabe (siehe testdata/README.md).
func TestPrograms(t *testing.T) {
	h := harness{fsys: os.DirFS("testdata"), dir: "testdata", compile: compiler.Compile, update: *update}
	cases, err := h.cases()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Log("keine Testprogramme unter testdata/")
	}
	for _, c := range cases {
		t.Run(strings.TrimSuffix(c, ".pas"), func(t *testing.T) {
			t.Parallel()
			if err := h.check(t.Context(), c, t.TempDir()); err != nil {
				t.Error(err)
			}
		})
	}
}

// compileFunc hat die Signatur von compiler.Compile; Tests des Harness
// setzen eine Attrappe ein.
type compileFunc func(ctx context.Context, mainFile string, opts compiler.Options) (*compiler.Result, error)

type harness struct {
	fsys    fs.FS  // Wurzel der Testprogramme
	dir     string // dieselbe Wurzel als Hostpfad, für -update
	compile compileFunc
	update  bool
	node    string // Ziel gojs: Pfad zu node; leer = Ziel wasi mit wazero
}

// cases liefert alle *.pas, zu denen es eine *.out oder *.err gibt.
// Units und Include-Dateien haben keine und werden so übersprungen.
func (h harness) cases() ([]string, error) {
	var cases []string
	err := fs.WalkDir(h.fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".pas" {
			return err
		}
		base := strings.TrimSuffix(p, ".pas")
		if exists(h.fsys, base+".out") || exists(h.fsys, base+".err") {
			cases = append(cases, p)
		}
		return nil
	})
	return cases, err
}

// check übersetzt und startet ein Testprogramm und vergleicht das Ergebnis.
// Dateizugriffe des Programms landen im Verzeichnis dir.
func (h harness) check(ctx context.Context, file, dir string) error {
	ctx, cancel := context.WithTimeout(ctx, caseTimeout)
	defer cancel()

	base := strings.TrimSuffix(file, ".pas")
	got, kind, err := h.result(ctx, file, dir)
	if err != nil {
		return err
	}
	other := map[string]string{".out": ".err", ".err": ".out"}[kind]

	if h.update {
		if err := os.WriteFile(filepath.Join(h.dir, filepath.FromSlash(base+kind)), []byte(got), 0o644); err != nil {
			return err
		}
		if exists(h.fsys, base+other) {
			return os.Remove(filepath.Join(h.dir, filepath.FromSlash(base+other)))
		}
		return nil
	}

	want, err := fs.ReadFile(h.fsys, base+kind)
	if err != nil {
		if exists(h.fsys, base+other) {
			if kind == ".err" {
				return fmt.Errorf("%s: unerwarteter Compilerfehler:\n%s", file, got)
			}
			return fmt.Errorf("%s: Compilerfehler erwartet, aber übersetzt; Ausgabe:\n%s", file, got)
		}
		return err
	}
	if got != string(want) {
		return fmt.Errorf("%s: Ausgabe weicht von %s ab:\n%s", file, base+kind, diff(string(want), got))
	}
	return nil
}

// result liefert die Diagnose (kind ".err") oder die Programmausgabe (kind ".out").
//
// Die Programmausgabe umfasst stdout und stderr in Schreibreihenfolge; ein
// Laufzeitfehler wird als eigene Zeile angehängt, ein Exit-Code ungleich 0 als
// "[Exit-Code N]". Eingaben kommen aus *.in, falls vorhanden.
func (h harness) result(ctx context.Context, file, dir string) (got, kind string, err error) {
	target := compiler.TargetWASI
	if h.node != "" {
		target = compiler.TargetGoJS
	}
	res, err := h.compile(ctx, file, compiler.Options{Target: target, FS: h.fsys})
	if err != nil {
		var d *compiler.Diagnostic
		if !errors.As(err, &d) {
			return "", "", fmt.Errorf("%s: Übersetzung: %w", file, err)
		}
		return d.Error() + "\n", ".err", nil
	}

	var stdin io.Reader
	if in, err := fs.ReadFile(h.fsys, strings.TrimSuffix(file, ".pas")+".in"); err == nil {
		stdin = bytes.NewReader(in)
	}
	args := []string{path.Base(strings.TrimSuffix(file, ".pas"))}
	if a, err := fs.ReadFile(h.fsys, strings.TrimSuffix(file, ".pas")+".args"); err == nil {
		args = append(args, strings.Fields(string(a))...)
	}
	env := readEnv(h.fsys, strings.TrimSuffix(file, ".pas")+".env")
	if h.node != "" {
		return h.runNode(ctx, res.Wasm, args, stdin, dir, env)
	}
	var out bytes.Buffer
	code, err := runner.Run(ctx, res.Wasm, runner.Options{
		Args:    args,
		Stdin:   stdin,
		Stdout:  &out,
		Stderr:  &out,
		Dirs:    dirs(dir),
		Env:     env,
		Imports: wasmtest.Imports(&out),
	})
	if err != nil {
		var rte *runner.RuntimeError
		if !errors.As(err, &rte) {
			return "", "", fmt.Errorf("%s: Ausführung: %w", file, err)
		}
		endLine(&out)
		out.WriteString(rte.Error() + "\n")
		code = 0 // die Fehlernummer steht schon in der Meldung
	}
	if code != 0 {
		endLine(&out)
		fmt.Fprintf(&out, "[Exit-Code %d]\n", code)
	}
	return out.String(), ".out", nil
}

// endLine schließt eine angefangene Ausgabezeile ab, damit Zusätze des
// Harness immer auf einer eigenen Zeile stehen.
func endLine(b *bytes.Buffer) {
	if b.Len() > 0 && !bytes.HasSuffix(b.Bytes(), []byte("\n")) {
		b.WriteByte('\n')
	}
}

// readEnv liest name.env: je Zeile NAME=WERT, Leerzeilen zählen nicht.
func readEnv(fsys fs.FS, name string) []string {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil
	}
	var env []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			env = append(env, line)
		}
	}
	return env
}

func dirs(dir string) map[string]string {
	if dir == "" {
		return nil
	}
	return map[string]string{"/": dir}
}

func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}

// diff zeigt die erste abweichende Zeile mit etwas Kontext.
func diff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	i := 0
	for i < len(w) && i < len(g) && w[i] == g[i] {
		i++
	}
	line := func(ls []string) string {
		if i < len(ls) {
			return fmt.Sprintf("%q", ls[i])
		}
		return "(Ende)"
	}
	return fmt.Sprintf("Zeile %d:\n  erwartet: %s\n  erhalten: %s\n--- erwartet ---\n%s--- erhalten ---\n%s",
		i+1, line(w), line(g), want, got)
}
