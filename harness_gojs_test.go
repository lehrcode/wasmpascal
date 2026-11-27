package wasmpascal_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
)

// TestProgramsGoJS führt die Testprogramme unter testdata/ mit dem Ziel gojs
// in Node.js aus (web/node-run.js mit wasm_exec.js und web/loader.js) und
// vergleicht mit denselben erwarteten Ausgaben wie TestPrograms. Jeder
// Testfall bekommt wie unter WASI ein eigenes Verzeichnis als Wurzel für
// Dateizugriffe (node-run.js --dir). Ohne node wird der Test übersprungen.
func TestProgramsGoJS(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node nicht gefunden")
	}
	if *update {
		t.Skip("-update schreibt die Ausgaben nur mit dem Ziel wasi")
	}
	h := harness{fsys: os.DirFS("testdata"), dir: "testdata", compile: compiler.Compile, node: node}
	cases, err := h.cases()
	if err != nil {
		t.Fatal(err)
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

// runtimeErrorLine ist die Meldung des Loaders bei einem Laufzeitfehler.
var runtimeErrorLine = regexp.MustCompile(`(?m)^Runtime error (\d+)[^\n]*\n\z`)

// runNode führt ein gojs-Modul mit web/node-run.js aus und liefert die
// Ausgabe im Format von result. Ist root nicht leer, ist es die Wurzel für
// Dateizugriffe; das Modul selbst liegt außerhalb davon.
func (h harness) runNode(ctx context.Context, wasm []byte, args []string, stdin io.Reader, root string, env []string) (string, string, error) {
	bin, err := os.MkdirTemp("", "wasmpascal-gojs-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(bin)
	file := filepath.Join(bin, args[0]+".wasm")
	if err := os.WriteFile(file, wasm, 0o644); err != nil {
		return "", "", err
	}
	script, err := filepath.Abs(filepath.Join("web", "node-run.js"))
	if err != nil {
		return "", "", err
	}
	nodeArgs := []string{script}
	if root != "" {
		nodeArgs = append(nodeArgs, "--dir", root)
	}
	nodeArgs = append(nodeArgs, "--emptyenv") // wie unter WASI: nur die Variablen aus name.env
	for _, kv := range env {
		nodeArgs = append(nodeArgs, "--setenv", kv)
	}
	nodeArgs = append(nodeArgs, file)
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, h.node, append(nodeArgs, args[1:]...)...)
	cmd.Dir = bin
	cmd.Stdin = stdin
	cmd.Stdout = &out
	cmd.Stderr = &out
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ctx.Err() != nil {
			return "", "", fmt.Errorf("node: %w\n%s", err, out.String())
		}
		code = ee.ExitCode()
	}
	// Bei einem Laufzeitfehler ist der Exit-Code die Fehlernummer; die
	// Meldung steht schon in der Ausgabe.
	if m := runtimeErrorLine.FindSubmatch(out.Bytes()); m != nil {
		if n, _ := strconv.Atoi(string(m[1])); n == code {
			code = 0
		}
	}
	if code != 0 {
		endLine(&out)
		fmt.Fprintf(&out, "[Exit-Code %d]\n", code)
	}
	return out.String(), ".out", nil
}

// TestGoJSWithoutFiles prüft, dass Dateizugriffe unter gojs mit I/O-Fehler 5
// (Zugriff verweigert) scheitern und Eingaben bis zum Dateiende reichen.
func TestGoJSWithoutFiles(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node nicht gefunden")
	}
	const prog = `program ohne;
var
  T: Text;
  S: string;
  N: Integer;
begin
  Assign(T, 'x.txt');
  {$I-} Rewrite(T); {$I+}
  N := IOResult;
  WriteLn('Rewrite: ', N);
  {$I-} MkDir('d'); {$I+}
  N := IOResult;
  WriteLn('MkDir: ', N);
  N := 0;
  while not Eof do
  begin
    ReadLn(S);
    N := N + 1;
    WriteLn(N, ': ', S)
  end;
  WriteLn('Ende')
end.
`
	fsys := fstest.MapFS{"ohne.pas": {Data: []byte(prog)}}
	h := harness{fsys: fsys, compile: compiler.Compile, node: node}
	res, err := compiler.Compile(t.Context(), "ohne.pas", compiler.Options{Target: compiler.TargetGoJS, FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := h.runNode(t.Context(), res.Wasm, []string{"ohne"}, strings.NewReader("eins\nzwei äöü\ndrei"), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Rewrite: 5\nMkDir: 5\n1: eins\n2: zwei äöü\n3: drei\nEnde\n"
	if got != want {
		t.Errorf("Ausgabe:\n%s\nerwartet:\n%s", got, want)
	}
}
