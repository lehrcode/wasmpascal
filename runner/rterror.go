package runner

import (
	"encoding/binary"
	"strings"

	"github.com/tetratelabs/wazero/api"
)

// runtimeError übersetzt einen Trap in einen Pascal-Laufzeitfehler. Der
// Compiler hinterlegt Fehlernummer, Datei, Zeile und Funktion in exportierten
// Globalen; bei einem echten WASM-Trap (Nummer 0) bestimmt die Art des Traps
// die Nummer. Namen kommen aus der Custom Section wasmpascal.lines. Für Module
// ohne diese Globalen liefert runtimeError nil.
func runtimeError(mod api.Module, wasm []byte, trap error) *RuntimeError {
	get := func(name string) (uint32, bool) {
		g := mod.ExportedGlobal(name)
		if g == nil {
			return 0, false
		}
		return uint32(g.Get()), true
	}
	code, ok := get("__rt_code")
	if !ok {
		return nil
	}
	file, _ := get("__rt_file")
	line, _ := get("__rt_line")
	fn, _ := get("__rt_func")
	// RunError setzt Bit 16, damit auch RunError(0) von einem echten Trap
	// (Nummer 0) zu unterscheiden ist.
	e := &RuntimeError{Code: int(code & 0xFFFF)}
	if code == 0 {
		e.Code = trapCode(trap.Error())
	}
	if files, funcs, ok := readLines(wasm); ok && line > 0 {
		if int(file) < len(files) {
			e.File = files[file]
		}
		e.Line = int(line)
		e.Proc = funcs[fn]
	}
	return e
}

// trapCode ordnet einem WASM-Trap eine TP-Fehlernummer zu.
func trapCode(msg string) int {
	switch {
	case strings.Contains(msg, "integer divide by zero"):
		return 200 // Division by zero
	case strings.Contains(msg, "stack overflow"), strings.Contains(msg, "call stack exhausted"):
		return 202 // Stack overflow
	case strings.Contains(msg, "invalid conversion to integer"), strings.Contains(msg, "integer overflow"):
		return 207 // Invalid floating point operation
	}
	// Speicherzugriff außerhalb des gültigen Bereichs und alles Übrige:
	// wie "General protection fault" in Borland Pascal 7.
	return 216
}

// readLines liest Dateinamen und Prozedurnamen aus wasmpascal.lines.
func readLines(wasm []byte) (files []string, funcs map[uint32]string, ok bool) {
	payload, found := customSection(wasm, "wasmpascal.lines")
	if !found {
		return nil, nil, false
	}
	r := reader{b: payload, ok: true}
	if r.u32() != 1 {
		return nil, nil, false
	}
	n := r.u32()
	for i := uint32(0); i < n && r.ok; i++ {
		files = append(files, r.name())
	}
	funcs = map[uint32]string{}
	n = r.u32()
	for i := uint32(0); i < n && r.ok; i++ {
		idx := r.u32()
		funcs[idx] = r.name()
		r.u32() // Datei der Deklaration
		r.u32() // Zeile der Deklaration
	}
	return files, funcs, r.ok
}

// customSection sucht eine Custom Section im Binärmodul.
func customSection(wasm []byte, want string) ([]byte, bool) {
	if len(wasm) < 8 {
		return nil, false
	}
	r := reader{b: wasm[8:], ok: true}
	for r.ok && len(r.b) > 0 {
		id := r.b[0]
		r.b = r.b[1:]
		size := r.u32()
		if !r.ok || int(size) > len(r.b) {
			return nil, false
		}
		body := r.b[:size]
		r.b = r.b[size:]
		if id == 0 {
			sr := reader{b: body, ok: true}
			if name := sr.name(); sr.ok && name == want {
				return sr.b, true
			}
		}
	}
	return nil, false
}

type reader struct {
	b  []byte
	ok bool
}

func (r *reader) u32() uint32 {
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.ok = false
		return 0
	}
	r.b = r.b[n:]
	return uint32(v)
}

func (r *reader) name() string {
	n := r.u32()
	if !r.ok || int(n) > len(r.b) {
		r.ok = false
		return ""
	}
	s := string(r.b[:n])
	r.b = r.b[n:]
	return s
}
