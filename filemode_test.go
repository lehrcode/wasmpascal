package wasmpascal_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
	"github.com/lehrcode/wasmpascal/runner"
)

// TestReadOnlyFile prüft FileMode mit einer schreibgeschützten Datei auf dem
// Host, unter WASI (wazero) und gojs (Node): Mit dem Standardwert 2 scheitert
// Reset wie in TP mit E/A-Fehler 5, mit FileMode := 0 lässt sich die Datei
// lesen.
func TestReadOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Schreibschutz über Dateirechte nur unter Unix")
	}
	const prog = `program Fest;
var
  F: file of Integer;
  I, N: Integer;
begin
  Assign(F, 'fest.dat');
  {$I-} Reset(F); {$I+}
  N := IOResult;
  WriteLn('FileMode 2: ', N);
  FileMode := 0;
  Reset(F);
  while not Eof(F) do
  begin
    Read(F, I);
    Write(I, ' ')
  end;
  WriteLn;
  Close(F)
end.
`
	const want = "FileMode 2: 5\n1 2 3 \n"

	// legt die schreibgeschützte Datei in einem neuen Verzeichnis an
	newRoot := func(t *testing.T) string {
		dir := t.TempDir()
		data := binary.LittleEndian.AppendUint32(nil, 1)
		data = binary.LittleEndian.AppendUint32(data, 2)
		data = binary.LittleEndian.AppendUint32(data, 3)
		file := filepath.Join(dir, "fest.dat")
		if err := os.WriteFile(file, data, 0o444); err != nil {
			t.Fatal(err)
		}
		if f, err := os.OpenFile(file, os.O_WRONLY, 0); err == nil {
			f.Close()
			t.Skip("Schreibschutz wirkt nicht (Ausführung als root?)")
		}
		return dir
	}
	src := fstest.MapFS{"fest.pas": {Data: []byte(prog)}}

	t.Run("wasi", func(t *testing.T) {
		root := newRoot(t)
		res, err := compiler.Compile(t.Context(), "fest.pas", compiler.Options{FS: src})
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if _, err := runner.Run(t.Context(), res.Wasm, runner.Options{
			Args:   []string{"fest"},
			Stdout: &out,
			Dirs:   map[string]string{"/": root},
		}); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Errorf("Ausgabe %q, erwartet %q", out.String(), want)
		}
	})

	t.Run("gojs", func(t *testing.T) {
		node, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node nicht gefunden")
		}
		root := newRoot(t)
		res, err := compiler.Compile(t.Context(), "fest.pas", compiler.Options{Target: compiler.TargetGoJS, FS: src})
		if err != nil {
			t.Fatal(err)
		}
		h := harness{node: node}
		got, _, err := h.runNode(t.Context(), res.Wasm, []string{"fest"}, nil, root)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Ausgabe %q, erwartet %q", got, want)
		}
	})
}
