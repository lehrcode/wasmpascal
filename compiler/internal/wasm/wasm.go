// Package wasm ist ein Binär-Encoder für WebAssembly-Module: Sections, LEB128
// und Instruktionen. Er prüft nichts; die Validierung übernehmen die Tests.
package wasm

import (
	"encoding/binary"
	"math"
)

// ValType ist ein Werttyp.
type ValType byte

// Werttypen.
const (
	I32  ValType = 0x7F
	I64  ValType = 0x7E
	F32  ValType = 0x7D
	F64  ValType = 0x7C
	V128 ValType = 0x7B
)

// BlockEmpty ist der Blocktyp ohne Ergebnis.
const BlockEmpty byte = 0x40

// FuncType ist eine Funktionssignatur.
type FuncType struct {
	Params, Results []ValType
}

func (a FuncType) equal(b FuncType) bool {
	if len(a.Params) != len(b.Params) || len(a.Results) != len(b.Results) {
		return false
	}
	for i := range a.Params {
		if a.Params[i] != b.Params[i] {
			return false
		}
	}
	for i := range a.Results {
		if a.Results[i] != b.Results[i] {
			return false
		}
	}
	return true
}

// Import ist eine importierte Funktion.
type Import struct {
	Module, Name string
	Type         uint32
}

// Func ist eine definierte Funktion. Code enthält die Instruktionen samt
// abschließendem end.
type Func struct {
	Type   uint32
	Locals []ValType
	Code   []byte
}

// Global ist eine globale Variable mit konstantem Initialwert.
type Global struct {
	Type    ValType
	Mutable bool
	Init    []byte // Konstantenausdruck ohne end
}

// Export-Arten.
const (
	ExportFunc   byte = 0
	ExportMemory byte = 2
	ExportGlobal byte = 3
)

// Export ist ein exportiertes Objekt.
type Export struct {
	Name  string
	Kind  byte
	Index uint32
}

// Data ist ein aktives Datensegment für Speicher 0.
type Data struct {
	Offset uint32
	Bytes  []byte
}

// Custom ist eine benutzerdefinierte Section.
type Custom struct {
	Name  string
	Bytes []byte
}

// Module ist ein WebAssembly-Modul. Importierte Funktionen haben die
// Indizes 0..len(Imports)-1, definierte folgen danach.
type Module struct {
	Types   []FuncType
	Imports []Import
	Funcs   []*Func
	// Table: Tabelle 0 (funcref) für call_indirect; Table[i] ist der
	// Funktionsindex an Position i+1, Position 0 bleibt leer (nil).
	Table []uint32
	// NeedTable legt die Tabelle auch ohne Einträge an (call_indirect).
	NeedTable bool
	// Speicher 0 mit MemMin Seiten (64 KB); ohne Obergrenze.
	MemMin  uint32
	Globals []Global
	Exports []Export
	Data    []Data
	Customs []Custom
}

// TypeIndex liefert den Index eines Funktionstyps und legt ihn bei Bedarf an.
func (m *Module) TypeIndex(t FuncType) uint32 {
	for i, u := range m.Types {
		if u.equal(t) {
			return uint32(i)
		}
	}
	m.Types = append(m.Types, t)
	return uint32(len(m.Types) - 1)
}

// Encode erzeugt das Binärformat.
func (m *Module) Encode() []byte {
	out := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	if len(m.Types) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Types)))
		for _, t := range m.Types {
			b = append(b, 0x60)
			b = appendValTypes(b, t.Params)
			b = appendValTypes(b, t.Results)
		}
		out = section(out, 1, b)
	}
	if len(m.Imports) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Imports)))
		for _, im := range m.Imports {
			b = appendName(b, im.Module)
			b = appendName(b, im.Name)
			b = append(b, 0x00)
			b = AppendU32(b, im.Type)
		}
		out = section(out, 2, b)
	}
	if len(m.Funcs) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Funcs)))
		for _, f := range m.Funcs {
			b = AppendU32(b, f.Type)
		}
		out = section(out, 3, b)
	}
	if len(m.Table) > 0 || m.NeedTable {
		var b []byte
		b = AppendU32(b, 1)
		b = append(b, 0x70, 0x00) // funcref, nur Minimum
		b = AppendU32(b, uint32(len(m.Table)+1))
		out = section(out, 4, b)
	}
	{
		var b []byte
		b = AppendU32(b, 1)
		b = append(b, 0x00) // nur Minimum
		b = AppendU32(b, m.MemMin)
		out = section(out, 5, b)
	}
	if len(m.Globals) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Globals)))
		for _, g := range m.Globals {
			b = append(b, byte(g.Type))
			if g.Mutable {
				b = append(b, 1)
			} else {
				b = append(b, 0)
			}
			b = append(b, g.Init...)
			b = append(b, OpEnd)
		}
		out = section(out, 6, b)
	}
	if len(m.Exports) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Exports)))
		for _, e := range m.Exports {
			b = appendName(b, e.Name)
			b = append(b, e.Kind)
			b = AppendU32(b, e.Index)
		}
		out = section(out, 7, b)
	}
	if len(m.Table) > 0 {
		var b []byte
		b = AppendU32(b, 1)
		b = append(b, 0x00, OpI32Const) // aktiv, Tabelle 0, ab Position 1
		b = AppendS32(b, 1)
		b = append(b, OpEnd)
		b = AppendU32(b, uint32(len(m.Table)))
		for _, f := range m.Table {
			b = AppendU32(b, f)
		}
		out = section(out, 9, b)
	}
	if len(m.Funcs) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Funcs)))
		for _, f := range m.Funcs {
			body := appendLocals(nil, f.Locals)
			body = append(body, f.Code...)
			b = AppendU32(b, uint32(len(body)))
			b = append(b, body...)
		}
		out = section(out, 10, b)
	}
	if len(m.Data) > 0 {
		var b []byte
		b = AppendU32(b, uint32(len(m.Data)))
		for _, d := range m.Data {
			b = append(b, 0x00) // aktiv, Speicher 0
			b = append(b, OpI32Const)
			b = AppendS32(b, int32(d.Offset))
			b = append(b, OpEnd)
			b = AppendU32(b, uint32(len(d.Bytes)))
			b = append(b, d.Bytes...)
		}
		out = section(out, 11, b)
	}
	for _, c := range m.Customs {
		b := appendName(nil, c.Name)
		b = append(b, c.Bytes...)
		out = section(out, 0, b)
	}
	return out
}

func section(out []byte, id byte, content []byte) []byte {
	out = append(out, id)
	out = AppendU32(out, uint32(len(content)))
	return append(out, content...)
}

func appendValTypes(b []byte, ts []ValType) []byte {
	b = AppendU32(b, uint32(len(ts)))
	for _, t := range ts {
		b = append(b, byte(t))
	}
	return b
}

// appendLocals fasst aufeinanderfolgende gleiche Typen zusammen.
func appendLocals(b []byte, ls []ValType) []byte {
	type run struct {
		n uint32
		t ValType
	}
	var runs []run
	for _, t := range ls {
		if len(runs) > 0 && runs[len(runs)-1].t == t {
			runs[len(runs)-1].n++
		} else {
			runs = append(runs, run{1, t})
		}
	}
	b = AppendU32(b, uint32(len(runs)))
	for _, r := range runs {
		b = AppendU32(b, r.n)
		b = append(b, byte(r.t))
	}
	return b
}

func appendName(b []byte, s string) []byte {
	b = AppendU32(b, uint32(len(s)))
	return append(b, s...)
}

// AppendU32 hängt v als vorzeichenloses LEB128 an.
func AppendU32(b []byte, v uint32) []byte {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if v == 0 {
			return append(b, c)
		}
		b = append(b, c|0x80)
	}
}

// AppendS32 hängt v als vorzeichenbehaftetes LEB128 an.
func AppendS32(b []byte, v int32) []byte { return AppendS64(b, int64(v)) }

// AppendS64 hängt v als vorzeichenbehaftetes LEB128 an.
func AppendS64(b []byte, v int64) []byte {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if (v == 0 && c&0x40 == 0) || (v == -1 && c&0x40 != 0) {
			return append(b, c)
		}
		b = append(b, c|0x80)
	}
}

// AppendF64 hängt v als IEEE-754 little-endian an.
func AppendF64(b []byte, v float64) []byte {
	return binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
}
