package runner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Memory ist der lineare Speicher des Programms, wie ihn eine Host-Funktion
// aus Options.Imports sieht (optionaler Parameter nach context.Context).
// Adressen sind die Werte von Pascal-Zeigern und var-Parametern; ok ist
// false, wenn der Bereich außerhalb des Speichers liegt.
type Memory interface {
	// Read liefert eine Kopie von n Bytes ab addr.
	Read(addr, n uint32) (b []byte, ok bool)
	// Write schreibt b ab addr.
	Write(addr uint32, b []byte) (ok bool)
	// ReadUint32 und WriteUint32 lesen bzw. schreiben 4 Byte little-endian
	// (LongInt, Zeiger).
	ReadUint32(addr uint32) (v uint32, ok bool)
	WriteUint32(addr uint32, v uint32) (ok bool)
	// ReadUint64 und WriteUint64 lesen bzw. schreiben 8 Byte little-endian
	// (Int64, Comp).
	ReadUint64(addr uint32) (v uint64, ok bool)
	WriteUint64(addr uint32, v uint64) (ok bool)
	// ReadShortString liest einen Pascal-String (Längenbyte, dann die
	// Bytes), etwa einen const-Parameter vom Typ string.
	ReadShortString(addr uint32) (s string, ok bool)
}

// reservedModules lassen sich in Options.Imports nicht belegen.
var reservedModules = map[string]bool{"wasi_snapshot_preview1": true, "gojs": true}

type memory struct{ m api.Memory }

func (m memory) Read(addr, n uint32) ([]byte, bool) {
	b, ok := m.m.Read(addr, n)
	if !ok {
		return nil, false
	}
	return append([]byte(nil), b...), true
}
func (m memory) Write(addr uint32, b []byte) bool       { return m.m.Write(addr, b) }
func (m memory) ReadUint32(addr uint32) (uint32, bool)  { return m.m.ReadUint32Le(addr) }
func (m memory) WriteUint32(addr uint32, v uint32) bool { return m.m.WriteUint32Le(addr, v) }
func (m memory) ReadUint64(addr uint32) (uint64, bool)  { return m.m.ReadUint64Le(addr) }
func (m memory) WriteUint64(addr uint32, v uint64) bool { return m.m.WriteUint64Le(addr, v) }
func (m memory) ReadShortString(addr uint32) (string, bool) {
	n, ok := m.m.ReadByte(addr)
	if !ok {
		return "", false
	}
	b, ok := m.m.Read(addr+1, uint32(n))
	return string(b), ok
}

var (
	ctxType    = reflect.TypeOf((*context.Context)(nil)).Elem()
	memoryType = reflect.TypeOf((*Memory)(nil)).Elem()
)

// valueType bildet einen Go-Typ auf einen WASM-Werttyp ab.
func valueType(t reflect.Type) (api.ValueType, bool) {
	switch t.Kind() {
	case reflect.Int32, reflect.Uint32:
		return api.ValueTypeI32, true
	case reflect.Int64, reflect.Uint64:
		return api.ValueTypeI64, true
	case reflect.Float32:
		return api.ValueTypeF32, true
	case reflect.Float64:
		return api.ValueTypeF64, true
	}
	return 0, false
}

// hostFunc ist eine Go-Funktion aus Options.Imports, zerlegt für wazero.
type hostFunc struct {
	fn               reflect.Value
	withCtx, withMem bool
	params, results  []api.ValueType
	paramTypes       []reflect.Type
}

func newHostFunc(f any) (*hostFunc, error) {
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		return nil, errors.New("keine Funktion")
	}
	t := v.Type()
	h := &hostFunc{fn: v}
	i := 0
	if i < t.NumIn() && t.In(i) == ctxType {
		h.withCtx = true
		i++
	}
	if i < t.NumIn() && t.In(i) == memoryType {
		h.withMem = true
		i++
	}
	for ; i < t.NumIn(); i++ {
		vt, ok := valueType(t.In(i))
		if !ok {
			return nil, fmt.Errorf("Parametertyp %s nicht erlaubt", t.In(i))
		}
		h.params = append(h.params, vt)
		h.paramTypes = append(h.paramTypes, t.In(i))
	}
	if t.NumOut() > 1 {
		return nil, errors.New("höchstens ein Ergebnis")
	}
	for i := 0; i < t.NumOut(); i++ {
		vt, ok := valueType(t.Out(i))
		if !ok {
			return nil, fmt.Errorf("Ergebnistyp %s nicht erlaubt", t.Out(i))
		}
		h.results = append(h.results, vt)
	}
	return h, nil
}

// call ruft die Go-Funktion mit den Werten aus stack auf und legt das
// Ergebnis dort ab (Konvention von api.GoModuleFunc).
func (h *hostFunc) call(ctx context.Context, mod api.Module, stack []uint64) {
	var in []reflect.Value
	if h.withCtx {
		in = append(in, reflect.ValueOf(ctx))
	}
	if h.withMem {
		in = append(in, reflect.ValueOf(Memory(memory{mod.Memory()})))
	}
	for i, t := range h.paramTypes {
		w := stack[i]
		var x reflect.Value
		switch t.Kind() {
		case reflect.Int32:
			x = reflect.ValueOf(api.DecodeI32(w))
		case reflect.Uint32:
			x = reflect.ValueOf(api.DecodeU32(w))
		case reflect.Int64:
			x = reflect.ValueOf(int64(w))
		case reflect.Uint64:
			x = reflect.ValueOf(w)
		case reflect.Float32:
			x = reflect.ValueOf(api.DecodeF32(w))
		case reflect.Float64:
			x = reflect.ValueOf(api.DecodeF64(w))
		}
		in = append(in, x.Convert(t))
	}
	out := h.fn.Call(in)
	if len(out) == 1 {
		r := out[0]
		switch r.Kind() {
		case reflect.Int32:
			stack[0] = api.EncodeI32(int32(r.Int()))
		case reflect.Uint32:
			stack[0] = api.EncodeU32(uint32(r.Uint()))
		case reflect.Int64:
			stack[0] = uint64(r.Int())
		case reflect.Uint64:
			stack[0] = r.Uint()
		case reflect.Float32:
			stack[0] = api.EncodeF32(float32(r.Float()))
		case reflect.Float64:
			stack[0] = api.EncodeF64(r.Float())
		}
	}
}

// typeNames formatiert WASM-Werttypen für Fehlermeldungen.
func typeNames(ts []api.ValueType) string {
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = api.ValueTypeName(t)
	}
	return "(" + strings.Join(names, ", ") + ")"
}

func sameTypes(a, b []api.ValueType) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// instantiateImports prüft die Importe des Moduls gegen opts.Imports und legt
// die Host-Module an. Fehlt ein Import oder passt seine Signatur nicht, gibt
// es einen Fehler vor dem Start.
func instantiateImports(ctx context.Context, rt wazero.Runtime, compiled wazero.CompiledModule, imports map[string]map[string]any) error {
	modules := make([]string, 0, len(imports))
	for m := range imports {
		if reservedModules[m] {
			return fmt.Errorf("runner: Modul %q in Options.Imports ist reserviert", m)
		}
		modules = append(modules, m)
	}
	sort.Strings(modules)
	funcs := map[string]map[string]*hostFunc{}
	for _, m := range modules {
		funcs[m] = map[string]*hostFunc{}
		for name, f := range imports[m] {
			h, err := newHostFunc(f)
			if err != nil {
				return fmt.Errorf("runner: Import %s.%s: %v", m, name, err)
			}
			funcs[m][name] = h
		}
	}
	for _, def := range compiled.ImportedFunctions() {
		m, name, _ := def.Import()
		if reservedModules[m] {
			continue
		}
		sig := typeNames(def.ParamTypes()) + " -> " + typeNames(def.ResultTypes())
		h := funcs[m][name]
		if h == nil {
			return fmt.Errorf("runner: Import %s.%s %s fehlt in Options.Imports", m, name, sig)
		}
		if !sameTypes(h.params, def.ParamTypes()) || !sameTypes(h.results, def.ResultTypes()) {
			return fmt.Errorf("runner: Import %s.%s: Programm erwartet %s, Go-Funktion hat %s -> %s",
				m, name, sig, typeNames(h.params), typeNames(h.results))
		}
	}
	for _, m := range modules {
		b := rt.NewHostModuleBuilder(m)
		names := make([]string, 0, len(funcs[m]))
		for name := range funcs[m] {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			h := funcs[m][name]
			b.NewFunctionBuilder().
				WithGoModuleFunction(api.GoModuleFunc(h.call), h.params, h.results).
				Export(name)
		}
		if _, err := b.Instantiate(ctx); err != nil {
			return fmt.Errorf("runner: Modul %s: %w", m, err)
		}
	}
	return nil
}
