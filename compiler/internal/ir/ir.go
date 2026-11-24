// Package ir definiert die Zwischendarstellung: je Prozedur ein
// Kontrollflussgraph aus Basisblöcken mit Anweisungen über Ausdrucksbäumen.
//
// Entwurfsregeln (PLAN.md, Scheduler):
//   - Aufrufe stehen nur auf Anweisungsebene (Call, HostCall), nie in
//     Ausdrücken. So kann eine Prozedur später nach jedem Aufruf anhalten.
//   - Variablen werden abstrakt angesprochen (Get, Set, Addr). Wo sie liegen
//     (WASM-Local, Frame auf dem Schatten-Stack, Datensegment), entscheidet
//     erst die Codegenerierung; jede Variable lässt sich in den Frame legen.
package ir

import (
	"fmt"
	"strings"

	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// ValType ist die Darstellung eines Werts auf dem WASM-Stack.
type ValType int

// Werttypen. Gleitkommawerte sind in Ausdrücken immer F64; Single wird nur
// beim Speichern zu f32.
const (
	I32 ValType = iota
	F64
)

// MemType ist die Darstellung eines skalaren Werts im Speicher.
type MemType int

// Speichertypen.
const (
	MemI8 MemType = iota // Byte mit Vorzeichen (ShortInt, Teilbereiche mit negativer Untergrenze)
	MemU8                // Byte ohne Vorzeichen
	MemI32
	MemF32
	MemF64
)

// Size liefert die Größe in Byte.
func (m MemType) Size() int64 {
	switch m {
	case MemI8, MemU8:
		return 1
	case MemF64:
		return 8
	}
	return 4
}

// Storage ist der Ablageort einer Variable.
type Storage int

// Ablageorte.
const (
	Undecided Storage = iota // die Codegenerierung entscheidet: Local oder Frame
	Local                    // WASM-Local
	Frame                    // Slot im Frame auf dem Schatten-Stack
	Global                   // feste Adresse im Datensegment
	Indirect                 // Adresse steht in einem WASM-Parameter (var-Parameter u. ä.)
)

// Var ist eine Variable, ein Parameter oder ein Hilfswert.
type Var struct {
	Name string
	Type types.Type
	// Scalar: Wert passt in VT/Mem; sonst Aggregat, nur über Addr zugänglich.
	Scalar bool
	VT     ValType
	Mem    MemType

	Storage Storage
	Offset  int64 // Frame-Offset (Frame) oder absolute Adresse (Global)
	// Addressed: Die Adresse wird gebraucht (var-Argument); erzwingt den Frame.
	Addressed bool
	// Temp: Hilfswert der Übersetzung; liegt immer in einem WASM-Local.
	Temp bool
	// Param ist der Index des WASM-Parameters, über den der Wert (bei Indirect:
	// die Adresse) hereinkommt, sonst -1.
	Param int
	// Owner ist die Funktion, in deren Frame bzw. Locals die Variable liegt
	// (nil bei globalen). Greift eine innere Prozedur zu, geht es über den
	// Static Link.
	Owner *Func
	// Init sind die Anfangsbytes globaler typisierter Konstanten.
	Init []byte
}

func (v *Var) String() string { return v.Name }

// Size ist die Größe im Speicher.
func (v *Var) Size() int64 {
	if v.Scalar {
		return v.Mem.Size()
	}
	return v.Type.Size()
}

// ---------------------------------------------------------------------------
// Ausdrücke

// Expr ist ein seiteneffektfreier Ausdruck.
type Expr interface {
	VT() ValType
	String() string
}

// IntConst ist eine i32-Konstante.
type IntConst struct{ V int32 }

// FloatConst ist eine f64-Konstante.
type FloatConst struct{ V float64 }

// Get liest eine skalare Variable.
type Get struct{ Var *Var }

// Addr ist die Adresse einer Variable im Speicher.
type Addr struct{ Var *Var }

// Load liest einen Skalar von Adresse+Off.
type Load struct {
	Addr Expr
	Off  int64
	Mem  MemType
}

// Op ist ein Operator.
type Op int

// Operatoren. Suffix I: i32, F: f64.
const (
	AddI Op = iota
	SubI
	MulI
	DivI // mit Vorzeichen, schneidet ab
	RemI
	AndI
	OrI
	XorI
	ShlI
	ShrI // logisch (TP: shr)
	EqI
	NeI
	LtI
	LeI
	GtI
	GeI
	AddF
	SubF
	MulF
	DivF
	EqF
	NeF
	LtF
	LeF
	GtF
	GeF
	// einstellig
	EqzI     // not bei Boolean
	NegF     // -x bei Real
	AbsF     // Abs bei Real
	SqrtF    // Sqrt
	TruncF   // f64.trunc (Richtung null)
	IntToF   // i32 → f64
	FToInt   // f64 → i32 (abschneiden)
	Ext8     // Vorzeichenerweiterung des unteren Bytes
	RoundF32 // auf Single-Genauigkeit runden (f64 → f32 → f64)
)

var opNames = [...]string{
	"add", "sub", "mul", "div", "rem", "and", "or", "xor", "shl", "shr",
	"eq", "ne", "lt", "le", "gt", "ge",
	"fadd", "fsub", "fmul", "fdiv", "feq", "fne", "flt", "fle", "fgt", "fge",
	"eqz", "fneg", "fabs", "fsqrt", "ftrunc", "itof", "ftoi", "ext8", "f32",
}

func (o Op) String() string { return opNames[o] }

// FrameBase ist die Frame-Adresse der (umschließenden) Funktion F, aus der
// aktuellen Funktion über die Kette der Static Links erreicht.
type FrameBase struct{ F *Func }

// HeapBase ist die erste Adresse hinter dem Schatten-Stack (Beginn des Heaps).
type HeapBase struct{}

// MemSize ist die aktuelle Speichergröße in Seiten zu 64 KB.
type MemSize struct{}

// SetCmpOp vergleicht zwei Mengen.
type SetCmpOp int

// Mengenvergleiche.
const (
	SetEq  SetCmpOp = iota // A = B
	SetSub                 // A ⊆ B
)

// SetCmp vergleicht die 32-Byte-Mengen an den Adressen A und B.
type SetCmp struct {
	Op   SetCmpOp
	A, B Expr
}

// Bin ist ein zweistelliger Operator.
type Bin struct {
	Op   Op
	X, Y Expr
}

// Un ist ein einstelliger Operator.
type Un struct {
	Op Op
	X  Expr
}

func (FrameBase) VT() ValType      { return I32 }
func (HeapBase) VT() ValType       { return I32 }
func (MemSize) VT() ValType        { return I32 }
func (SetCmp) VT() ValType         { return I32 }
func (f FrameBase) String() string { return "frame(" + f.F.Name + ")" }
func (HeapBase) String() string    { return "heapbase" }
func (MemSize) String() string     { return "memsize" }
func (s SetCmp) String() string    { return fmt.Sprintf("setcmp%d(%s, %s)", s.Op, s.A, s.B) }

func (IntConst) VT() ValType   { return I32 }
func (FloatConst) VT() ValType { return F64 }
func (g Get) VT() ValType      { return g.Var.VT }
func (Addr) VT() ValType       { return I32 }
func (l Load) VT() ValType {
	if l.Mem == MemF32 || l.Mem == MemF64 {
		return F64
	}
	return I32
}
func (b Bin) VT() ValType {
	if b.Op >= AddF && b.Op <= DivF {
		return F64
	}
	return I32
}
func (u Un) VT() ValType {
	switch u.Op {
	case NegF, AbsF, SqrtF, TruncF, IntToF, RoundF32:
		return F64
	}
	return I32
}

func (c IntConst) String() string   { return fmt.Sprint(c.V) }
func (c FloatConst) String() string { return fmt.Sprint(c.V) }
func (g Get) String() string        { return g.Var.Name }
func (a Addr) String() string       { return "&" + a.Var.Name }
func (l Load) String() string       { return fmt.Sprintf("load%d(%s+%d)", l.Mem, l.Addr, l.Off) }
func (b Bin) String() string        { return fmt.Sprintf("(%s %s %s)", b.Op, b.X, b.Y) }
func (u Un) String() string         { return fmt.Sprintf("(%s %s)", u.Op, u.X) }

// ---------------------------------------------------------------------------
// Anweisungen

// Instr ist eine Anweisung in einem Basisblock.
type Instr interface{ String() string }

// Set weist einer skalaren Variable zu.
type Set struct {
	Var *Var
	Val Expr
}

// Store schreibt einen Skalar nach Adresse+Off.
type Store struct {
	Addr Expr
	Off  int64
	Mem  MemType
	Val  Expr
}

// Copy kopiert N Bytes (memory.copy, Überlappung erlaubt).
type Copy struct {
	Dst, Src Expr
	N        Expr
}

// Fill füllt N Bytes mit Val (memory.fill).
type Fill struct {
	Dst, Val, N Expr
}

// SetOpKind ist eine Mengenoperation.
type SetOpKind int

// Mengenoperationen.
const (
	SetUnion SetOpKind = iota
	SetInter
	SetDiff
)

// SetOp verknüpft die 32-Byte-Mengen A und B nach Dst (v128).
type SetOp struct {
	Op        SetOpKind
	Dst, A, B Expr
}

// Grow vergrößert den Speicher um Pages Seiten; Dst erhält die alte Größe
// oder -1 (memory.grow).
type Grow struct {
	Pages Expr
	Dst   *Var
}

// Pos ist eine Quelltextstelle: Index in Module.Files und Zeile.
type Pos struct{ File, Line int }

// Line merkt die aktuelle Quelltextstelle für Laufzeitfehler vor.
type Line struct{ Pos Pos }

// Call ruft eine Funktion auf; Dst nimmt ein skalares Ergebnis auf. Pos ist
// die Stelle des Aufrufs; sie wird nach der Rückkehr wieder vorgemerkt.
type Call struct {
	Fn   *Func
	Args []Expr
	Dst  *Var
	Pos  Pos
}

// Host ist eine importierte Funktion.
type Host struct {
	Module, Name string
	Params       []ValType
	Result       []ValType
}

// HostCall ruft eine importierte Funktion auf.
type HostCall struct {
	Fn   *Host
	Args []Expr
	Dst  *Var
}

func (s Set) String() string { return fmt.Sprintf("%s := %s", s.Var.Name, s.Val) }
func (s Store) String() string {
	return fmt.Sprintf("store%d(%s+%d) := %s", s.Mem, s.Addr, s.Off, s.Val)
}
func (c Copy) String() string  { return fmt.Sprintf("copy(%s, %s, %s)", c.Dst, c.Src, c.N) }
func (f Fill) String() string  { return fmt.Sprintf("fill(%s, %s, %s)", f.Dst, f.Val, f.N) }
func (s SetOp) String() string { return fmt.Sprintf("setop%d(%s, %s, %s)", s.Op, s.Dst, s.A, s.B) }
func (g Grow) String() string  { return fmt.Sprintf("%s := grow(%s)", g.Dst.Name, g.Pages) }
func (l Line) String() string  { return fmt.Sprintf("line %d:%d", l.Pos.File, l.Pos.Line) }
func (c Call) String() string  { return callString(c.Dst, c.Fn.Name, c.Args) }
func (c HostCall) String() string {
	return callString(c.Dst, c.Fn.Module+"."+c.Fn.Name, c.Args)
}

func callString(dst *Var, name string, args []Expr) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.String()
	}
	s := name + "(" + strings.Join(parts, ", ") + ")"
	if dst != nil {
		s = dst.Name + " := " + s
	}
	return s
}

// ---------------------------------------------------------------------------
// Blöcke und Funktionen

// Term ist der Abschluss eines Basisblocks.
type Term interface{ String() string }

// Jump springt nach To.
type Jump struct{ To *Block }

// Branch verzweigt nach Then, wenn Cond ungleich 0 ist, sonst nach Else.
type Branch struct {
	Cond       Expr
	Then, Else *Block
}

// Return verlässt die Funktion (mit dem Wert der Ergebnisvariable).
type Return struct{}

// Unreachable beendet einen Block, der nie fortgesetzt wird (nach Halt).
type Unreachable struct{}

// Trap bricht mit dem Laufzeitfehler Code an der Stelle Pos ab.
type Trap struct {
	Code int
	Pos  Pos
}

func (j Jump) String() string { return fmt.Sprintf("jump b%d", j.To.ID) }
func (b Branch) String() string {
	return fmt.Sprintf("branch %s b%d b%d", b.Cond, b.Then.ID, b.Else.ID)
}
func (Return) String() string      { return "return" }
func (Unreachable) String() string { return "unreachable" }
func (t Trap) String() string      { return fmt.Sprintf("trap %d", t.Code) }

// Block ist ein Basisblock.
type Block struct {
	ID     int
	Instrs []Instr
	Term   Term
}

// Succs liefert die Nachfolger.
func (b *Block) Succs() []*Block {
	switch t := b.Term.(type) {
	case Jump:
		return []*Block{t.To}
	case Branch:
		return []*Block{t.Then, t.Else}
	}
	return nil
}

// Func ist eine übersetzte Prozedur, Funktion oder das Hauptprogramm.
type Func struct {
	Name string
	// Display ist der Name in Laufzeitfehlermeldungen; Pos die Deklaration.
	Display string
	Pos     Pos
	Proc    *types.Proc // nil beim Hauptprogramm
	// Parent ist die umschließende Funktion bei verschachtelten Prozeduren.
	// Dann ist StaticLink der versteckte erste Parameter: die Frame-Adresse
	// von Parent. Er liegt immer im Frame, damit tiefere Ebenen ihn finden.
	Parent     *Func
	StaticLink *Var
	// HasNested: Es gibt innere Prozeduren; die Funktion braucht einen Frame.
	HasNested bool
	Params    []*Var // WASM-Parameter in Reihenfolge
	// Result ist die Ergebnisvariable: skalar → Rückgabewert; Aggregat →
	// Indirect über den versteckten ersten Parameter.
	Result *Var
	Vars   []*Var // alle Variablen der Funktion (auch Parameter und Hilfswerte)
	Blocks []*Block
	Entry  *Block
}

// NewBlock legt einen Block an.
func (f *Func) NewBlock() *Block {
	b := &Block{ID: len(f.Blocks)}
	f.Blocks = append(f.Blocks, b)
	return b
}

// NewVar legt eine Variable der Funktion an.
func (f *Func) NewVar(v *Var) *Var {
	v.Owner = f
	f.Vars = append(f.Vars, v)
	return v
}

// ResultVT liefert den WASM-Ergebnistyp oder false bei Prozeduren und
// Aggregat-Ergebnissen.
func (f *Func) ResultVT() (ValType, bool) {
	if f.Result == nil || !f.Result.Scalar {
		return 0, false
	}
	return f.Result.VT, true
}

// String gibt die Funktion lesbar aus (für Tests und Fehlersuche).
func (f *Func) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "func %s(", f.Name)
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
	}
	b.WriteString(")\n")
	for _, blk := range f.Blocks {
		fmt.Fprintf(&b, "b%d:\n", blk.ID)
		for _, in := range blk.Instrs {
			fmt.Fprintf(&b, "  %s\n", in)
		}
		fmt.Fprintf(&b, "  %s\n", blk.Term)
	}
	return b.String()
}

// DataBase ist die Adresse des Datensegments; darunter liegt die freie
// Seite für nil (PLAN.md, Speicheraufteilung).
const DataBase = 65536

// Module ist das übersetzte Programm.
type Module struct {
	Files     []string // Dateinamen für Pos.File
	Funcs     []*Func  // alle Funktionen; Main ist enthalten
	Main      *Func
	Globals   []*Var // im Datensegment, Offset = absolute Adresse
	Data      []byte // Inhalt des Datensegments ab DataBase
	StackSize int64
	Hosts     []*Host // benutzte Importe in Reihenfolge der ersten Verwendung
}
