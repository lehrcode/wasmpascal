// Package types enthält das Typsystem von WasmPascal: Typen mit Größen und
// Ausrichtung nach dem Speichermodell aus PLAN.md, Konstantenwerte, Symbole
// und Scopes.
//
// Typidentität ist Objektidentität: Jeder Typausdruck im Quelltext erzeugt ein
// neues Typobjekt, ein Typbezeichner liefert das Objekt seiner Deklaration.
// Das entspricht der Namensäquivalenz von Turbo Pascal.
package types

import (
	"fmt"
	"strings"
)

// Type ist ein Pascal-Typ.
type Type interface {
	// String liefert den Namen oder, bei anonymen Typen, die Struktur.
	String() string
	// Size ist die Größe im linearen Speicher in Byte.
	Size() int64
	// Align ist die Ausrichtung im Speicher.
	Align() int64
}

// named ist in alle zusammengesetzten Typen eingebettet und trägt den Namen
// der ersten Typdeklaration, die den Typ benennt.
type named struct{ name string }

// SetName benennt einen anonymen Typ; ein vorhandener Name bleibt.
func (n *named) SetName(s string) {
	if n.name == "" {
		n.name = s
	}
}

// Name liefert den Namen oder "".
func (n *named) Name() string { return n.name }

// Nameable ist ein Typ, der durch eine Typdeklaration benannt werden kann.
type Nameable interface {
	Type
	SetName(string)
	Name() string
}

// ---------------------------------------------------------------------------
// Einfache Typen

// BasicKind unterscheidet die vordefinierten einfachen Typen.
type BasicKind int

// Vordefinierte einfache Typen.
const (
	KBoolean BasicKind = iota
	KChar
	KWideChar
	KShortInt
	KByte
	KSmallInt
	KInteger
	KWord
	KLongInt
	KReal
	KSingle
	KDouble
	KExtended
	KComp
)

// Basic ist ein vordefinierter einfacher Typ.
type Basic struct {
	Kind   BasicKind
	name   string
	size   int64
	Lo, Hi int64 // Wertebereich bei Ordinaltypen
}

func (b *Basic) String() string { return b.name }
func (b *Basic) Size() int64    { return b.size }
func (b *Basic) Align() int64   { return b.size }

// IsInteger meldet Ganzzahltypen.
func (b *Basic) IsInteger() bool { return b.Kind >= KShortInt && b.Kind <= KLongInt }

// IsReal meldet Gleitkommatypen.
func (b *Basic) IsReal() bool { return b.Kind >= KReal }

// Vordefinierte Typen. Integer hat den Bereich von LongInt (PLAN.md,
// Festlegungen zu Wertebereichen); SmallInt und Word belegen wie in TP 2 Byte.
var (
	Boolean  = &Basic{Kind: KBoolean, name: "Boolean", size: 1, Lo: 0, Hi: 1}
	Char     = &Basic{Kind: KChar, name: "Char", size: 1, Lo: 0, Hi: 255}
	WideChar = &Basic{Kind: KWideChar, name: "WideChar", size: 2, Lo: 0, Hi: 0xFFFF}
	ShortInt = &Basic{Kind: KShortInt, name: "ShortInt", size: 1, Lo: -128, Hi: 127}
	Byte     = &Basic{Kind: KByte, name: "Byte", size: 1, Lo: 0, Hi: 255}
	SmallInt = &Basic{Kind: KSmallInt, name: "SmallInt", size: 2, Lo: -32768, Hi: 32767}
	Integer  = &Basic{Kind: KInteger, name: "Integer", size: 4, Lo: MinLongInt, Hi: MaxLongInt}
	Word     = &Basic{Kind: KWord, name: "Word", size: 2, Lo: 0, Hi: 65535}
	LongInt  = &Basic{Kind: KLongInt, name: "LongInt", size: 4, Lo: MinLongInt, Hi: MaxLongInt}
	Real     = &Basic{Kind: KReal, name: "Real", size: 8}
	Single   = &Basic{Kind: KSingle, name: "Single", size: 4}
	Double   = &Basic{Kind: KDouble, name: "Double", size: 8}
	Extended = &Basic{Kind: KExtended, name: "Extended", size: 8}
	Comp     = &Basic{Kind: KComp, name: "Comp", size: 8}
)

// Grenzen von LongInt (und Integer).
const (
	MinLongInt = -1 << 31
	MaxLongInt = 1<<31 - 1
)

// MaxStructSize ist die größte erlaubte Struktur (2 GB, PLAN.md Größengrenzen).
const MaxStructSize = 1<<31 - 1

// ---------------------------------------------------------------------------
// Aufzählungen und Teilbereiche

// Enum ist ein Aufzählungstyp.
type Enum struct {
	named
	Names []string
}

func (e *Enum) String() string {
	if e.name != "" {
		return e.name
	}
	return "(" + strings.Join(e.Names, ", ") + ")"
}

// Size: 1 Byte, wenn alle Werte in 0..255 passen, 2 Byte bis 65535, sonst 4
// (wie TP).
func (e *Enum) Size() int64 {
	switch {
	case len(e.Names) <= 256:
		return 1
	case len(e.Names) <= 65536:
		return 2
	}
	return 4
}
func (e *Enum) Align() int64 { return e.Size() }

// Subrange ist ein Teilbereich lo..hi eines Ordinaltyps.
type Subrange struct {
	named
	Base   Type // Basic (ordinal) oder Enum, nie selbst ein Subrange
	Lo, Hi int64
}

func (s *Subrange) String() string {
	if s.name != "" {
		return s.name
	}
	return fmt.Sprintf("%s..%s", FormatOrd(s.Base, s.Lo), FormatOrd(s.Base, s.Hi))
}

// Size richtet sich wie in TP nach dem Wertebereich (PLAN.md): 1 Byte, wenn
// alle Werte in 0..255 oder -128..127 passen, 2 Byte in 0..65535 oder
// -32768..32767, sonst 4.
func (s *Subrange) Size() int64 {
	switch {
	case (s.Lo >= 0 && s.Hi <= 255) || (s.Lo >= -128 && s.Hi <= 127):
		return 1
	case (s.Lo >= 0 && s.Hi <= 65535) || (s.Lo >= -32768 && s.Hi <= 32767):
		return 2
	}
	return 4
}
func (s *Subrange) Align() int64 { return s.Size() }

// FormatOrd schreibt einen Ordinalwert des Typs t lesbar.
func FormatOrd(t Type, v int64) string {
	switch t := Underlying(t).(type) {
	case *Enum:
		if v >= 0 && v < int64(len(t.Names)) {
			return t.Names[v]
		}
	case *Basic:
		switch t.Kind {
		case KChar:
			if v >= 32 && v < 127 && v != '\'' {
				return fmt.Sprintf("'%c'", rune(v))
			}
			return fmt.Sprintf("#%d", v)
		case KWideChar:
			if v >= 32 && v < 127 && v != '\'' {
				return fmt.Sprintf("'%c'", rune(v))
			}
			return fmt.Sprintf("#$%04X", v)
		case KBoolean:
			if v == 0 {
				return "False"
			}
			return "True"
		}
	}
	return fmt.Sprint(v)
}

// ---------------------------------------------------------------------------
// Strings, Mengen, Arrays, Records

// String ist string[Max]; im Speicher immer 256 Byte (PLAN.md).
type String struct {
	named
	Max int64
}

func (s *String) String() string {
	if s.name != "" {
		return s.name
	}
	if s.Max == 255 {
		return "string"
	}
	return fmt.Sprintf("string[%d]", s.Max)
}
func (s *String) Size() int64  { return 256 }
func (s *String) Align() int64 { return 1 }

// StringType ist der Typ string (= string[255]).
var StringType = &String{Max: 255}

// Set ist set of Elem; im Speicher immer 32 Byte (256 Bit).
type Set struct {
	named
	Packed bool
	Elem   Type // Ordinaltyp mit Werten in 0..255; nil für die leere Menge []
}

func (s *Set) String() string {
	if s.name != "" {
		return s.name
	}
	if s.Elem == nil {
		return "set of ()"
	}
	return "set of " + s.Elem.String()
}
func (s *Set) Size() int64  { return 32 }
func (s *Set) Align() int64 { return 4 }

// Array ist array[Index] of Elem; mehrdimensionale Arrays sind verschachtelt.
type Array struct {
	named
	Packed bool
	Index  Type // Ordinaltyp
	Lo, Hi int64
	Elem   Type
}

func (a *Array) String() string {
	if a.name != "" {
		return a.name
	}
	p := ""
	if a.Packed {
		p = "packed "
	}
	return fmt.Sprintf("%sarray[%s] of %s", p, a.Index, a.Elem)
}

// Len ist die Zahl der Elemente.
func (a *Array) Len() int64      { return a.Hi - a.Lo + 1 }
func (a *Array) Size() int64     { return a.Len() * a.Elem.Size() }
func (a *Array) Align() int64    { return a.Elem.Align() }
func (a *Array) ElemSize() int64 { return a.Elem.Size() }

// Field ist ein Recordfeld mit Offset.
type Field struct {
	Name    string // normalisiert
	Text    string // Schreibweise der Deklaration
	Type    Type
	Offset  int64
	Variant int // 0: fester Teil, sonst Nummer des Variantenzweigs (für die Ausgabe)
}

// Record ist ein Record; variante Teile überlagern sich wie eine Union.
type Record struct {
	named
	Packed bool
	Fields []*Field
	size   int64
	align  int64
}

func (r *Record) String() string {
	if r.name != "" {
		return r.name
	}
	var parts []string
	for _, f := range r.Fields {
		parts = append(parts, f.Text+": "+f.Type.String())
	}
	return "record " + strings.Join(parts, "; ") + " end"
}
func (r *Record) Size() int64  { return r.size }
func (r *Record) Align() int64 { return r.align }

// Field sucht ein Feld nach normalisiertem Namen.
func (r *Record) Field(name string) *Field {
	for _, f := range r.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// SetLayout setzt Größe und Ausrichtung (berechnet vom Checker).
func (r *Record) SetLayout(size, align int64) { r.size, r.align = size, align }

// AlignTo rundet off auf ein Vielfaches von align auf.
func AlignTo(off, align int64) int64 {
	if align <= 1 {
		return off
	}
	return (off + align - 1) / align * align
}

// ---------------------------------------------------------------------------
// Zeiger und Dateien

// Pointer ist ^Elem; Elem nil ist der untypisierte Typ Pointer. Elem kann bei
// Vorwärtsverweisen im type-Teil zunächst nil sein und wird nachgetragen.
type Pointer struct {
	named
	Elem Type
}

func (p *Pointer) String() string {
	if p.name != "" {
		return p.name
	}
	if p.Elem == nil {
		return "Pointer"
	}
	return "^" + p.Elem.String()
}
func (p *Pointer) Size() int64  { return 4 }
func (p *Pointer) Align() int64 { return 4 }

// PointerType ist der vordefinierte untypisierte Zeigertyp Pointer.
var PointerType = &Pointer{named: named{name: "Pointer"}}

// NilType ist der Typ von nil.
type NilType struct{}

func (NilType) String() string { return "nil" }
func (NilType) Size() int64    { return 4 }
func (NilType) Align() int64   { return 4 }

// Nil ist der Typ der Konstante nil.
var Nil Type = NilType{}

// Größen der Laufzeit-Records für Dateien; sie müssen zu TFileRec und
// TTextRec in rtl/system.pas passen (geprüft in den Tests des Compilers).
const (
	FileRecSize = 272 // Handle, Mode, RecSize, Reserve, Name
	TextRecSize = 528 // Handle, Mode, BufPos, BufEnd, Name, Puffer mit 256 Byte
)

// File ist file, file of Elem oder Text.
type File struct {
	named
	Packed bool
	Text   bool
	Elem   Type // nil bei untypisierten Dateien und Text
}

func (f *File) String() string {
	switch {
	case f.name != "":
		return f.name
	case f.Text:
		return "Text"
	case f.Elem == nil:
		return "file"
	}
	return "file of " + f.Elem.String()
}
func (f *File) Size() int64 {
	if f.Text {
		return TextRecSize
	}
	return FileRecSize
}
func (f *File) Align() int64 { return 4 }

// Text ist der vordefinierte Typ Text.
var Text = &File{Text: true, named: named{name: "Text"}}

// ---------------------------------------------------------------------------
// Prozeduren

// ParamMode ist die Übergabeart.
type ParamMode int

// Übergabearten.
const (
	ByValue ParamMode = iota
	ByVar
	ByConst
)

// Param ist ein formaler Parameter. Type ist nil bei untypisierten var-/const-
// Parametern.
type Param struct {
	Name string
	Mode ParamMode
	Type Type
	Var  *Var
}

// Signature ist die Parameterliste mit Ergebnistyp (nil bei Prozeduren).
type Signature struct {
	Params []*Param
	Result Type
}

// String formatiert die Signatur wie im Quelltext.
func (s *Signature) String() string {
	var b strings.Builder
	if len(s.Params) > 0 {
		b.WriteString("(")
		for i, p := range s.Params {
			if i > 0 {
				b.WriteString("; ")
			}
			switch p.Mode {
			case ByVar:
				b.WriteString("var ")
			case ByConst:
				b.WriteString("const ")
			}
			b.WriteString(p.Name)
			if p.Type != nil {
				b.WriteString(": " + p.Type.String())
			}
		}
		b.WriteString(")")
	}
	if s.Result != nil {
		b.WriteString(": " + s.Result.String())
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Hilfsfunktionen

// Underlying liefert bei Teilbereichen den Basistyp, sonst t selbst.
func Underlying(t Type) Type {
	if s, ok := t.(*Subrange); ok {
		return s.Base
	}
	return t
}

// IsOrdinal meldet Ordinaltypen: Ganzzahlen, Char, Boolean, Aufzählungen und
// Teilbereiche.
func IsOrdinal(t Type) bool {
	switch t := Underlying(t).(type) {
	case *Basic:
		return !t.IsReal()
	case *Enum:
		return true
	}
	return false
}

// IsInteger meldet Ganzzahltypen samt deren Teilbereichen.
func IsInteger(t Type) bool {
	b, ok := Underlying(t).(*Basic)
	return ok && b.IsInteger()
}

// IsReal meldet Gleitkommatypen.
func IsReal(t Type) bool {
	b, ok := t.(*Basic)
	return ok && b.IsReal()
}

// IsNumeric meldet Ganzzahl- und Gleitkommatypen.
func IsNumeric(t Type) bool { return IsInteger(t) || IsReal(t) }

// Is meldet, ob t (oder sein Basistyp) der vordefinierte Typ b ist.
func Is(t Type, b *Basic) bool { return Underlying(t) == b }

// IsString meldet String-Typen.
func IsString(t Type) bool { _, ok := t.(*String); return ok }

// Bounds liefert den Wertebereich eines Ordinaltyps.
func Bounds(t Type) (lo, hi int64) {
	switch t := t.(type) {
	case *Subrange:
		return t.Lo, t.Hi
	case *Enum:
		return 0, int64(len(t.Names)) - 1
	case *Basic:
		return t.Lo, t.Hi
	}
	return 0, -1
}
