// Package check führt die Typprüfung durch: Scopes und Symboltabellen,
// Typverträglichkeit, Konstantenauswertung und die Auflösung der
// Standardprozeduren. Wie der Parser bricht es beim ersten Fehler ab.
package check

import (
	"context"
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
	CallIndirect                 // Aufruf über eine Prozedurvariable (TP 5.0)
)

// Call beschreibt einen aufgelösten Aufruf (CallExpr, oder Ident/Selector ohne
// Klammern bei parameterlosen Aufrufen; bei CallIndirect auch IndexExpr,
// DerefExpr und andere Ausdrücke eines prozeduralen Typs).
type Call struct {
	Kind    CallKind
	Proc    *types.Proc
	Builtin *types.Builtin
	Type    types.Type // Zieltyp bei Typumwandlungen
	// Bei CallIndirect: Signatur des prozeduralen Typs und der Ausdruck, der
	// die Prozedurvariable liefert (bei Aufrufen ohne Klammern der Knoten
	// selbst).
	Sig *types.Signature
	Fun ast.Expr
}

// Result liefert den Ergebnistyp eines Aufrufs eigener Routinen (direkt
// oder indirekt), nil bei Prozeduren.
func (c *Call) Result() types.Type {
	if c.Kind == CallIndirect {
		return c.Sig.Result
	}
	return c.Proc.Sig.Result
}

// Info sammelt die Ergebnisse der Typprüfung für die späteren Stufen.
type Info struct {
	Types  map[ast.Expr]TypeAndValue
	Defs   map[*ast.Ident]types.Symbol
	Uses   map[*ast.Ident]types.Symbol
	Fields map[*ast.Ident]*types.Field // Feldzugriffe (auch implizit über with)
	Calls  map[ast.Expr]*Call
	// ProcRefs sind Bezeichner, die eine Routine als Wert bezeichnen (@P,
	// Zuweisung an eine Prozedurvariable, typisierte Konstanten).
	ProcRefs map[ast.Expr]*types.Proc
	// WithRecs ordnet Bezeichner, die über with auf ein Feld zugreifen, dem
	// Record-Ausdruck der with-Anweisung zu.
	WithRecs map[*ast.Ident]ast.Expr
	Procs    map[*ast.ProcDecl]*types.Proc
	Scopes   map[ast.Node]*types.Scope // Program/Unit/ProcDecl → Scope
	// Root ist der Scope des Programms bzw. des Interface-Teils einer Unit;
	// Impl der des Implementation-Teils.
	Root, Impl *types.Scope
	// Universe ist der Scope der vordefinierten Bezeichner.
	Universe *types.Scope
	// SystemIntf und SystemImpl sind die Scopes der System-Unit (rtl/system.pas),
	// falls sie mitgeprüft wurde.
	SystemIntf, SystemImpl *types.Scope
	// Units sind die übrigen benutzten Units in der Reihenfolge ihrer
	// Initialisierung.
	Units []*UnitInfo
}

// UnitInfo beschreibt eine geprüfte Unit.
type UnitInfo struct {
	Name       string // Schreibweise der Deklaration
	AST        *ast.Unit
	Intf, Impl *types.Scope
	RTL        bool // eingebettete Laufzeit-Unit (Crt, Dos)
}

// Config steuert die Prüfung.
type Config struct {
	// System ist die System-Unit. Sie wird vor dem Programm geprüft, ihr
	// Interface umschließt den Programm-Scope.
	System *ast.Unit
	// Units sind alle weiteren Units, die per uses erreichbar sind, nach
	// normalisiertem Namen.
	Units map[string]*ast.Unit
	// Embedded kennzeichnet Units aus rtl.FS (Laufzeit, ohne Positionspflege).
	Embedded map[*ast.Unit]bool
	// MaxStaticData begrenzt die Summe aller globalen Variablen und
	// typisierten Konstanten in Byte (Error 49); 0 heißt DefaultMaxStaticData,
	// eine negative Zahl schaltet die Grenze ab.
	MaxStaticData int64
	// Ctx bricht die Prüfung ab (geprüft je Routine); nil = nie.
	Ctx context.Context
	// GoJS: Ziel gojs; dort sind external-Routinen nur in der System-Unit
	// erlaubt (Etappe 17).
	GoJS bool
}

// DefaultMaxStaticData ist die Voreinstellung für Config.MaxStaticData.
const DefaultMaxStaticData = 256 << 20

// Check prüft ein Programm oder eine Unit.
func Check(file ast.File, conf Config) (info *Info, err error) {
	c := &checker{
		info: &Info{
			Types:    map[ast.Expr]TypeAndValue{},
			Defs:     map[*ast.Ident]types.Symbol{},
			Uses:     map[*ast.Ident]types.Symbol{},
			Fields:   map[*ast.Ident]*types.Field{},
			Calls:    map[ast.Expr]*Call{},
			ProcRefs: map[ast.Expr]*types.Proc{},
			WithRecs: map[*ast.Ident]ast.Expr{},
			Procs:    map[*ast.ProcDecl]*types.Proc{},
			Scopes:   map[ast.Node]*types.Scope{},
		},
		maxStaticData: conf.MaxStaticData,
		ctx:           conf.Ctx,
		goJS:          conf.GoJS,
		imports:       map[string]string{},
	}
	if c.maxStaticData == 0 {
		c.maxStaticData = DefaultMaxStaticData
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
	outer := c.info.Universe
	if conf.System != nil {
		c.inSystem = true
		c.info.SystemIntf, c.info.SystemImpl = c.unit(conf.System, outer)
		c.inSystem = false
		outer = c.info.SystemIntf
	}
	c.units = map[string]*unitState{}
	for name, u := range conf.Units {
		c.units[name] = &unitState{info: &UnitInfo{Name: u.Name.Text, AST: u, RTL: conf.Embedded[u]}}
	}
	var roots []*ast.Ident
	switch f := file.(type) {
	case *ast.Program:
		c.program(f, outer)
		roots = f.Uses
	case *ast.Unit:
		st := &unitState{info: &UnitInfo{Name: f.Name.Text, AST: f}}
		c.units[f.Name.Name] = st
		c.checkImpl(st)
		c.info.Root, c.info.Impl = st.info.Intf, st.info.Impl
		roots = append(append([]*ast.Ident{}, f.IntfUses...), f.ImplUses...)
	}
	// Implementation-Teile in der Reihenfolge der Initialisierung
	for _, st := range c.initOrder(roots) {
		c.checkImpl(st)
		if st.info.AST != file {
			c.info.Units = append(c.info.Units, st.info)
		}
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

	unitIntf, unitImpl *types.Scope // Scopes der Unit, die gerade geprüft wird
	units              map[string]*unitState

	staticData, maxStaticData int64 // statische Daten bisher und Grenze (<0: keine)
	ctx                       context.Context

	goJS     bool              // Ziel gojs
	inSystem bool              // die System-Unit wird geprüft
	imports  map[string]string // Modul + "\x00" + Name → WASM-Signatur der external-Routinen
}

// canceled bricht die Prüfung ab, wenn ctx abgelaufen ist; Check liefert
// dann ctx.Err().
func (c *checker) canceled() {
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			panic(bailout{err})
		}
	}
}

// addStatic zählt size Byte statischer Daten (globale Variable oder
// typisierte Konstante) und meldet Error 49 an pos, wenn die Grenze
// überschritten ist – bevor ir dafür Speicher belegt.
func (c *checker) addStatic(pos source.Pos, size int64) {
	c.staticData += size
	if c.maxStaticData >= 0 && c.staticData > c.maxStaticData {
		c.fail(pos, source.ErrDataTooLarge, "")
	}
}

// unitState ist der Prüfstand einer Unit.
type unitState struct {
	info  *UnitInfo
	state int // 0 neu, 1 Interface in Arbeit, 2 Interface fertig, 3 fertig
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

func (c *checker) program(p *ast.Program, outer *types.Scope) {
	c.scope = outer
	root := c.openScope(types.ProgramScope, nil)
	c.info.Root = root
	c.info.Scopes[p] = root
	c.uses(p.Uses)
	c.block(p.Block)
}

// unit prüft eine Unit und liefert die Scopes von Interface und Implementation.
func (c *checker) unit(u *ast.Unit, outer *types.Scope) (intf, impl *types.Scope) {
	c.scope = outer
	intf = c.openScope(types.UnitScope, nil)
	c.info.Scopes[u] = intf
	c.uses(u.IntfUses)
	c.decls(u.Intf, true)
	impl = c.openScope(types.UnitScope, nil)
	c.unitIntf, c.unitImpl = intf, impl
	c.uses(u.ImplUses)
	c.decls(u.Impl, false)
	c.checkForwards(intf)
	c.checkForwards(impl)
	if u.Init != nil {
		c.compound(u.Init)
	}
	c.unitIntf, c.unitImpl = nil, nil
	return intf, impl
}

// uses bindet Units ein: Ihr Interface wird (falls nötig) geprüft, ihr Name
// deklariert und ihr Interface-Scope importiert.
func (c *checker) uses(ids []*ast.Ident) {
	for _, id := range ids {
		if id.Name == "system" {
			c.info.Uses[id] = c.info.Universe.Syms["system"]
			continue
		}
		st, ok := c.units[id.Name]
		if !ok {
			c.fail(id.Pos(), source.ErrFileNotFound, "%s.pas", id.Text)
		}
		c.intf(st, id)
		c.declare(id, types.NewUnit(id.Text, st.info.Intf))
		c.scope.Imports = append(c.scope.Imports, st.info.Intf)
	}
}

// intf prüft den Interface-Teil einer Unit. Ist er gerade in Arbeit, liegt ein
// Zyklus in den uses-Listen der Interfaces vor (Error 68).
func (c *checker) intf(st *unitState, at *ast.Ident) {
	switch st.state {
	case 1:
		c.fail(at.Pos(), source.ErrCircularUnit, "%s", at.Text)
	case 2, 3:
		return
	}
	st.state = 1
	saved := c.scope
	c.scope = c.info.Universe
	if c.info.SystemIntf != nil {
		c.scope = c.info.SystemIntf
	}
	st.info.Intf = c.openScope(types.UnitScope, nil)
	c.info.Scopes[st.info.AST] = st.info.Intf
	c.uses(st.info.AST.IntfUses)
	c.decls(st.info.AST.Intf, true)
	c.scope = saved
	st.state = 2
}

// checkImpl prüft den Implementation-Teil und die Initialisierung einer Unit.
// Hier sind gegenseitige uses erlaubt, weil alle Interfaces schon feststehen.
func (c *checker) checkImpl(st *unitState) {
	if st.state == 3 {
		return
	}
	c.intf(st, st.info.AST.Name)
	st.state = 3
	saved := c.scope
	c.scope = st.info.Intf
	impl := c.openScope(types.UnitScope, nil)
	st.info.Impl = impl
	c.unitIntf, c.unitImpl = st.info.Intf, impl
	c.uses(st.info.AST.ImplUses)
	c.decls(st.info.AST.Impl, false)
	c.checkForwards(st.info.Intf)
	c.checkForwards(impl)
	if st.info.AST.Init != nil {
		c.compound(st.info.AST.Init)
	}
	c.unitIntf, c.unitImpl = nil, nil
	c.scope = saved
}

// initOrder bestimmt die Reihenfolge der Initialisierung: Tiefensuche über
// die uses-Listen (erst Interface, dann Implementation), Units nach den
// Units, die sie benutzen.
func (c *checker) initOrder(roots []*ast.Ident) []*unitState {
	var order []*unitState
	seen := map[*unitState]bool{}
	var visit func(ids []*ast.Ident)
	visit = func(ids []*ast.Ident) {
		for _, id := range ids {
			st := c.units[id.Name]
			if st == nil || seen[st] {
				continue
			}
			seen[st] = true
			visit(st.info.AST.IntfUses)
			visit(st.info.AST.ImplUses)
			order = append(order, st)
		}
	}
	visit(roots)
	return order
}

func (c *checker) block(b *ast.Block) {
	c.canceled()
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

// checkLabels meldet Marken, zu denen gesprungen wird, die aber im
// Anweisungsteil nicht vorkommen (Error 82).
func (c *checker) checkLabels(s *types.Scope) {
	for _, sym := range s.Order {
		if l, ok := sym.(*types.Label); ok && l.Used && !l.Defined {
			c.fail(l.UsedAt, source.ErrUndefinedLabel, "%s", l.Text())
		}
	}
}
