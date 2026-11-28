package wasmtest

import (
	"fmt"
	"io"

	"github.com/lehrcode/wasmpascal/runner"
)

// Imports liefert das Modul "test" für Programme mit external-Routinen
// (testdata/etappe17/extern.pas): Rechenfunktionen in allen erlaubten
// Typen und Zugriffe auf den Speicher. print und printstr schreiben nach out.
func Imports(out io.Writer) map[string]map[string]any {
	return map[string]map[string]any{"test": {
		"add":   func(a, b int32) int32 { return a + b },
		"add64": func(a, b int64) int64 { return a + b },
		"half":  func(x float64) float64 { return x / 2 },
		"twice": func(x float32) float32 { return 2 * x },
		"not": func(b int32) int32 {
			if b == 0 {
				return 1
			}
			return 0
		},
		"store": func(m runner.Memory, addr uint32, v int32) { m.WriteUint32(addr, uint32(v)) },
		"print": func(m runner.Memory, addr, n uint32) {
			b, _ := m.Read(addr, n)
			fmt.Fprintf(out, "Host: %s\n", b)
		},
		"printstr": func(m runner.Memory, addr uint32) {
			s, _ := m.ReadShortString(addr)
			fmt.Fprintf(out, "Host: %s\n", s)
		},
	}}
}
