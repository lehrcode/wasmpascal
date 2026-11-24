package types

import "github.com/lehrcode/wasmpascal/compiler/internal/source"

// Symbol ist ein deklarierter Name.
type Symbol interface {
	// Name ist der normalisierte Name.
	Name() string
	// Pos ist die Stelle der Deklaration (leer bei vordefinierten Symbolen).
	Pos() source.Pos
}

type symBase struct {
	name string
	text string
	pos  source.Pos
}

func (s *symBase) Name() string    { return s.name }
func (s *symBase) Pos() source.Pos { return s.pos }

// Text ist die Schreibweise der Deklaration.
func (s *symBase) Text() string { return s.text }

func sb(name, text string, pos source.Pos) symBase {
	return symBase{name: name, text: text, pos: pos}
}

// NewConst erzeugt eine Konstante.
func NewConst(name, text string, pos source.Pos, t Type, v *Value) *Const {
	return &Const{symBase: sb(name, text, pos), Type: t, Val: v}
}

// NewTypeName erzeugt einen Typbezeichner.
func NewTypeName(name, text string, pos source.Pos, t Type) *TypeName {
	return &TypeName{symBase: sb(name, text, pos), Type: t}
}

// NewVar erzeugt eine Variable.
func NewVar(name, text string, pos source.Pos, kind VarKind, t Type) *Var {
	return &Var{symBase: sb(name, text, pos), Kind: kind, Type: t}
}

// NewProc erzeugt eine Prozedur oder Funktion.
func NewProc(name, text string, pos source.Pos, isFunc bool) *Proc {
	return &Proc{symBase: sb(name, text, pos), IsFunc: isFunc}
}

// NewBuiltin erzeugt eine Standardprozedur oder -funktion.
func NewBuiltin(text string, id BuiltinID, isFunc bool) *Builtin {
	return &Builtin{symBase: sb(lower(text), text, source.Pos{}), ID: id, IsFunc: isFunc}
}

// NewUnit erzeugt einen Unit-Namen.
func NewUnit(text string, scope *Scope) *Unit {
	return &Unit{symBase: sb(lower(text), text, source.Pos{}), Scope: scope}
}

// NewLabel erzeugt eine Sprungmarke.
func NewLabel(name, text string, pos source.Pos) *Label {
	return &Label{symBase: sb(name, text, pos)}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// Const ist eine untypisierte Konstante (auch Aufzählungswerte, True, False).
type Const struct {
	symBase
	Type       Type
	Val        *Value
	EnumMember bool // Wert eines Aufzählungstyps
}

// TypeName ist ein Typbezeichner.
type TypeName struct {
	symBase
	Type Type
}

// VarKind unterscheidet Variablen.
type VarKind int

// Arten von Variablen.
const (
	GlobalVar  VarKind = iota // globale Variable (Datensegment)
	LocalVar                  // lokale Variable einer Prozedur
	ParamVar                  // formaler Parameter
	ResultVar                 // implizite Ergebnisvariable einer Funktion (Result)
	TypedConst                // typisierte Konstante (Datensegment, schreibgeschützt)
	BuiltinVar                // vordefinierte Variable der System-Unit (Input, Output, ...)
)

// Var ist eine Variable, ein Parameter oder eine typisierte Konstante.
type Var struct {
	symBase
	Kind     VarKind
	Type     Type // nil bei untypisierten var-/const-Parametern
	Mode     ParamMode
	ReadOnly bool
	Owner    *Proc  // umschließende Prozedur, nil auf oberster Ebene
	Val      *Value // Wert typisierter Konstanten
}

// Proc ist eine Prozedur oder Funktion.
type Proc struct {
	symBase
	Sig     *Signature
	IsFunc  bool
	Level   int   // 1 = oberste Ebene, je Verschachtelung +1
	Parent  *Proc // umschließende Prozedur
	Scope   *Scope
	Result  *Var // implizites Result bei Funktionen
	Forward bool // deklariert, Rumpf steht noch aus (forward oder Interface)
	Defined bool // Rumpf vorhanden
}

// BuiltinID kennzeichnet eine Standardprozedur oder -funktion.
type BuiltinID int

// Builtin ist eine Standardprozedur oder -funktion der System-Unit.
type Builtin struct {
	symBase
	ID          BuiltinID
	IsFunc      bool
	Unsupported bool // in WasmPascal bewusst nicht vorhanden (Error 902)
}

// Unit ist ein Unit-Name für qualifizierte Bezeichner.
type Unit struct {
	symBase
	Scope *Scope
}

// Label ist eine Sprungmarke.
type Label struct {
	symBase
	Defined bool
}

// ScopeKind unterscheidet Scopes.
type ScopeKind int

// Arten von Scopes.
const (
	UniverseScope ScopeKind = iota // System-Unit
	UnitScope                      // Interface oder Implementation einer Unit
	ProgramScope
	ProcScope
)

// Scope ist eine Symboltabelle mit Verweis auf den umschließenden Scope.
type Scope struct {
	Kind   ScopeKind
	Parent *Scope
	Proc   *Proc // Besitzer bei ProcScope
	Syms   map[string]Symbol
	Order  []Symbol
}

// NewScope erzeugt einen leeren Scope.
func NewScope(kind ScopeKind, parent *Scope, proc *Proc) *Scope {
	return &Scope{Kind: kind, Parent: parent, Proc: proc, Syms: map[string]Symbol{}}
}

// Insert trägt sym ein und liefert ein schon vorhandenes Symbol gleichen
// Namens zurück (dann wird nichts eingetragen).
func (s *Scope) Insert(sym Symbol) Symbol {
	if old, ok := s.Syms[sym.Name()]; ok {
		return old
	}
	s.Syms[sym.Name()] = sym
	s.Order = append(s.Order, sym)
	return nil
}

// Lookup sucht name in s und den umschließenden Scopes.
func (s *Scope) Lookup(name string) (Symbol, *Scope) {
	for sc := s; sc != nil; sc = sc.Parent {
		if sym, ok := sc.Syms[name]; ok {
			return sym, sc
		}
	}
	return nil, nil
}
