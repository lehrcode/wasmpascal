// Package check führt die Typprüfung durch: Scopes und Symboltabellen,
// Typverträglichkeit, Konstantenauswertung und die Auflösung der
// Standardprozeduren. Wie der Parser bricht es beim ersten Fehler ab.
package check

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// Mode beschreibt, was ein Ausdruck bezeichnet.
type Mode int

// Arten von Ausdrücken.
const (
	NoValue  Mode = iota // Prozeduraufruf ohne Wert
	Value                // berechneter Wert
	Constant             // Konstante (Val gesetzt)
	Variable             // adressierbar
	TypeExpr             // Typ (für Typumwandlung, SizeOf, Low/High)
)

// TypeAndValue ist das Ergebnis der Prüfung eines Ausdrucks.
type TypeAndValue struct {
	Mode     Mode
	Type     types.Type
	Val      *types.Value
	ReadOnly bool
}

// CallKind unterscheidet Aufrufe.
type CallKind int

// Arten von Aufrufen.
const (
	CallProc     CallKind = iota // eigene Prozedur oder Funktion
	CallBuiltin                  // Standardprozedur/-funktion
	CallTypecast                 // Typumwandlung T(x)
)

// Call beschreibt einen aufgelösten Aufruf (CallExpr, oder Ident/Selector ohne
// Klammern bei parameterlosen Aufrufen).
type Call struct {
	Kind    CallKind
	Proc    *types.Proc
	Builtin *types.Builtin
	Type    types.Type // Zieltyp bei Typumwandlungen
}

// Info sammelt die Ergebnisse der Typprüfung für die späteren Stufen.
type Info struct {
	Types  map[ast.Expr]TypeAndValue
	Defs   map[*ast.Ident]types.Symbol
	Uses   map[*ast.Ident]types.Symbol
	Fields map[*ast.Ident]*types.Field // Feldzugriffe (auch implizit über with)
	Calls  map[ast.Expr]*Call
	// WithRecs ordnet Bezeichner, die über with auf ein Feld zugreifen, dem
	// Record-Ausdruck der with-Anweisung zu.
	WithRecs map[*ast.Ident]ast.Expr
	Procs    map[*ast.ProcDecl]*types.Proc
	Scopes   map[ast.Node]*types.Scope // Program/Unit/ProcDecl → Scope
	// Root ist der Scope des Programms bzw. des Interface-Teils einer Unit;
	// Impl der des Implementation-Teils.
	Root, Impl *types.Scope
	// Universe ist der Scope der System-Unit.
	Universe *types.Scope
}

// Config steuert die Prüfung.
type Config struct{}

// Check prüft ein Programm oder eine Unit.
func Check(file ast.File, conf Config) (info *Info, err error) {
	c := &checker{
		info: &Info{
			Types:    map[ast.Expr]TypeAndValue{},
			Defs:     map[*ast.Ident]types.Symbol{},
			Uses:     map[*ast.Ident]types.Symbol{},
			Fields:   map[*ast.Ident]*types.Field{},
			Calls:    map[ast.Expr]*Call{},
			WithRecs: map[*ast.Ident]ast.Expr{},
			Procs:    map[*ast.ProcDecl]*types.Proc{},
			Scopes:   map[ast.Node]*types.Scope{},
		},
	}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			info, err = nil, b.err
		}
	}()
	c.info.Universe = newUniverse()
	switch f := file.(type) {
	case *ast.Program:
		c.program(f)
	case *ast.Unit:
		c.unit(f)
	}
	return c.info, nil
}

type bailout struct{ err error }

// withEntry ist ein Record, dessen Felder über with sichtbar sind.
type withEntry struct {
	rec      *types.Record
	expr     ast.Expr
	readOnly bool
}

type checker struct {
	info  *Info
	scope *types.Scope
	proc  *types.Proc // aktuelle Prozedur, nil auf oberster Ebene

	withs    []withEntry
	loops    int                  // Tiefe der Schleifen (Break/Continue)
	readOnly map[*types.Var]bool  // for-in-Variablen im Rumpf
	pending  []*pendingPointer    // Zeigertypen mit Vorwärtsverweis im type-Teil
	enclosed map[*types.Proc]bool // Funktionen, deren Rumpf gerade geprüft wird
}

type pendingPointer struct {
	ptr  *types.Pointer
	name *ast.NamedType
}

func (c *checker) fail(pos source.Pos, code int, detail string, args ...any) {
	panic(bailout{source.Errorf(pos, code, detail, args...)})
}

// ---------------------------------------------------------------------------
// Scopes

func (c *checker) openScope(kind types.ScopeKind, proc *types.Proc) *types.Scope {
	c.scope = types.NewScope(kind, c.scope, proc)
	return c.scope
}

func (c *checker) closeScope() { c.scope = c.scope.Parent }

// declare trägt sym im aktuellen Scope ein; doppelte Namen sind Error 4.
func (c *checker) declare(id *ast.Ident, sym types.Symbol) {
	if c.scope.Insert(sym) != nil {
		c.fail(id.Pos(), source.ErrDuplicateIdent, "%s", id.Text)
	}
	if id != nil {
		c.info.Defs[id] = sym
	}
}

// lookup sucht einen Bezeichner; unbekannte Namen sind Error 3.
func (c *checker) lookup(id *ast.Ident) types.Symbol {
	sym, _ := c.scope.Lookup(id.Name)
	if sym == nil {
		c.fail(id.Pos(), source.ErrUnknownIdent, "%s", id.Text)
	}
	if b, ok := sym.(*types.Builtin); ok && b.Unsupported {
		c.fail(id.Pos(), source.ErrNotSupported, "%s", id.Text)
	}
	c.info.Uses[id] = sym
	return sym
}

// ---------------------------------------------------------------------------
// Programm und Unit

func (c *checker) program(p *ast.Program) {
	c.scope = c.info.Universe
	root := c.openScope(types.ProgramScope, nil)
	c.info.Root = root
	c.info.Scopes[p] = root
	c.uses(p.Uses)
	c.block(p.Block)
}

func (c *checker) unit(u *ast.Unit) {
	c.scope = c.info.Universe
	intf := c.openScope(types.UnitScope, nil)
	c.info.Root = intf
	c.info.Scopes[u] = intf
	c.uses(u.IntfUses)
	c.decls(u.Intf, true)
	impl := c.openScope(types.UnitScope, nil)
	c.info.Impl = impl
	c.uses(u.ImplUses)
	c.decls(u.Impl, false)
	c.checkForwards(intf)
	c.checkForwards(impl)
	if u.Init != nil {
		c.compound(u.Init)
	}
}

// uses löst Unit-Namen auf. Stand Etappe 3 gibt es nur die System-Unit;
// Mehrdatei-Programme folgen in Etappe 7.
func (c *checker) uses(ids []*ast.Ident) {
	for _, id := range ids {
		if id.Name == "system" {
			c.info.Uses[id] = c.info.Universe.Syms["system"]
			continue
		}
		c.fail(id.Pos(), source.ErrFileNotFound, "%s.pas", id.Text)
	}
}

func (c *checker) block(b *ast.Block) {
	c.decls(b.Decls, false)
	c.checkForwards(c.scope)
	c.compound(b.Body)
	c.checkLabels(c.scope)
}

// checkForwards meldet Prozeduren, die forward oder im Interface deklariert,
// aber nie definiert wurden.
func (c *checker) checkForwards(s *types.Scope) {
	for _, sym := range s.Order {
		if p, ok := sym.(*types.Proc); ok && p.Forward && !p.Defined {
			c.fail(p.Pos(), source.ErrUndefinedForward, "%s", p.Text())
		}
	}
}

// checkLabels ist ein Platzhalter: Bis Etappe 8 ist goto ohnehin ein Fehler.
func (c *checker) checkLabels(*types.Scope) {}
