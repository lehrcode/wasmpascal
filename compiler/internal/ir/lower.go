package ir

import (
	"encoding/binary"
	"math"
	"strings"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// Options steuert das Lowering.
type Options struct {
	StackSize int64
	// NoLines lässt die Vormerkung der Quelltextstellen weg (-nolines).
	NoLines bool
}

// Laufzeitfehler (TP-Nummern).
const (
	ErrDivByZero    = 200
	ErrStackOver    = 202
	ErrHeapOver     = 203
	ErrInvalidPtr   = 204
	ErrNilPointer   = 216 // WasmPascal prüft jede Dereferenzierung (PLAN.md)
	ErrInvalidFloat = 207
)

type bailout struct{ err error }

// Lower übersetzt ein geprüftes Programm samt System-Unit in die IR.
// Sprachmittel, die die Codegenerierung noch nicht beherrscht, ergeben
// Error 902 mit Angabe der Etappe.
func Lower(prog *ast.Program, system *ast.Unit, info *check.Info, opts Options) (m *Module, err error) {
	l := &lowerer{
		info:    info,
		opts:    opts,
		m:       &Module{StackSize: opts.StackSize, Sigs: map[string]*FuncSig{}},
		funcs:   map[*types.Proc]*Func{},
		vars:    map[*types.Var]*Var{},
		strs:    map[string]int64{},
		sets:    map[[4]uint64]int64{},
		files:   map[string]int{},
		copyIns: map[*Func][]copyIn{},
		hosts:   map[int]*Host{},
		table:   map[*types.Proc]int32{},
	}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			m, err = nil, b.err
		}
	}()
	name := "main"
	if prog.Name != nil {
		name = prog.Name.Text
	}
	main := &Func{Name: "main", Display: name, Pos: l.pos(prog.Pos())}
	l.m.Main = main
	l.m.Funcs = append(l.m.Funcs, main)
	// Units in der Reihenfolge ihrer Initialisierung, System zuerst
	units := info.Units
	if system != nil {
		units = append([]*check.UnitInfo{{Name: "System", AST: system, Intf: info.SystemIntf, Impl: info.SystemImpl, RTL: true}}, units...)
	}
	var bodies []*ast.ProcDecl
	var inits []*Func
	for _, u := range units {
		l.rtlMode = u.RTL
		l.declare(u.Intf, nil, u.Name)
		l.declare(u.Impl, nil, u.Name)
		bodies = append(bodies, procDecls(u.AST.Impl)...)
		if u.AST.Init != nil {
			f := &Func{Name: u.Name + ".init", Display: u.Name, Pos: l.pos(u.AST.Init.Pos()), RTL: u.RTL}
			l.m.Funcs = append(l.m.Funcs, f)
			l.body(f, nil, u.AST.Init.Stmts)
			inits = append(inits, f)
		}
	}
	l.rtlMode = false
	l.declare(info.Root, nil, "")
	bodies = append(bodies, procDecls(prog.Block.Decls)...)

	if system != nil {
		l.m.SuspendAction = l.rtl("_SuspendAction")
		if p, ok := info.SystemImpl.Syms["_goresumeaction"].(*types.Proc); ok {
			l.m.ResumeAction = l.funcs[p]
		}
		l.m.ErrorExit = l.rtl("_ErrorExit")
	}
	// Hauptprogramm: erst die Initialisierungen, am Ende _Halt(0) (im exit-Block)
	l.mainInits = inits
	l.body(main, nil, prog.Block.Body.Stmts)
	for _, d := range bodies {
		p := info.Procs[d]
		l.body(l.funcs[p], p, d.Block.Body.Stmts)
	}
	// Funktionstabelle in der Reihenfolge der ersten Verwendung
	for _, p := range l.tableProcs {
		l.m.Table = append(l.m.Table, l.funcs[p])
	}
	// Erst jetzt steht das Datensegment fest: BSS-Adressen nachtragen.
	l.m.BSSBase = types.AlignTo(DataBase+int64(len(l.m.Data)), 16)
	for _, v := range l.bss {
		v.Offset += l.m.BSSBase
	}
	return l.m, nil
}

type lowerer struct {
	bss     []*Var // globale Variablen im BSS, Offset zunächst relativ zu BSSBase
	info    *check.Info
	opts    Options
	m       *Module
	funcs   map[*types.Proc]*Func
	vars    map[*types.Var]*Var
	strs    map[string]int64
	sets    map[[4]uint64]int64
	files   map[string]int
	copyIns map[*Func][]copyIn
	hosts   map[int]*Host
	rtlMode bool // es werden Routinen einer Laufzeit-Unit angelegt
	// table: Index der Routinen in der Funktionstabelle (ab 1);
	// tableProcs in Reihenfolge der Indizes.
	table      map[*types.Proc]int32
	tableProcs []*types.Proc
	// mainInits sind die Initialisierungen der Units, die das Hauptprogramm
	// zu Beginn aufruft.
	mainInits []*Func
}

// copyIn ist ein Wertparameter vom Aggregattyp: Der Aufgerufene kopiert ihn
// beim Eintritt von der übergebenen Adresse in seinen Frame.
type copyIn struct {
	dst, src *Var
}

func unsupported(pos source.Pos, what string) {
	panic(bailout{source.Errorf(pos, source.ErrNotSupported, "%s", what)})
}

func (l *lowerer) pos(p source.Pos) Pos {
	i, ok := l.files[p.File]
	if !ok {
		i = len(l.m.Files)
		l.files[p.File] = i
		l.m.Files = append(l.m.Files, p.File)
	}
	return Pos{File: i, Line: p.Line}
}

// procDecls sammelt alle Prozedurdeklarationen mit Rumpf, auch verschachtelte.
func procDecls(ds []ast.Decl) []*ast.ProcDecl {
	var out []*ast.ProcDecl
	for _, d := range ds {
		if pd, ok := d.(*ast.ProcDecl); ok && pd.Block != nil {
			out = append(out, pd)
			out = append(out, procDecls(pd.Block.Decls)...)
		}
	}
	return out
}

// rtl liefert eine Routine der System-Unit.
func (l *lowerer) rtl(name string) *Func {
	for _, s := range []*types.Scope{l.info.SystemImpl, l.info.SystemIntf} {
		if s == nil {
			continue
		}
		if p, ok := s.Syms[strings.ToLower(name)].(*types.Proc); ok {
			return l.funcs[p]
		}
	}
	panic("ir: Laufzeitroutine " + name + " fehlt in rtl/system.pas")
}

// ---------------------------------------------------------------------------
// Typen und Daten

// scalar liefert Wert- und Speicherdarstellung eines skalaren Typs.
func scalar(t types.Type) (vt ValType, mem MemType, ok bool) {
	switch t.(type) {
	case *types.Pointer, types.NilType, *types.ProcType:
		return I32, MemI32, true
	}
	switch {
	case types.IsReal(t):
		if types.Is(t, types.Single) {
			return F64, MemF32, true
		}
		return F64, MemF64, true
	case types.IsOrdinal(t):
		lo, _ := types.Bounds(t)
		switch t.Size() {
		case 1:
			if lo < 0 {
				return I32, MemI8, true
			}
			return I32, MemU8, true
		case 2:
			if lo < 0 {
				return I32, MemI16, true
			}
			return I32, MemU16, true
		}
		return I32, MemI32, true
	}
	return 0, 0, false
}

// checkSupported meldet Typen, die erst in späteren Etappen übersetzt werden.
func checkSupported(t types.Type, pos source.Pos) {
	switch t := t.(type) {
	case *types.Array:
		checkSupported(t.Elem, pos)
	case *types.Record:
		for _, f := range t.Fields {
			checkSupported(f.Type, pos)
		}
	}
}

func newVar(name string, t types.Type) *Var {
	v := &Var{Name: name, Type: t, Param: -1}
	if t != nil {
		v.VT, v.Mem, v.Scalar = scalar(t)
	}
	return v
}

// addrVar ist ein Hilfswert, der eine Adresse hält.
func addrVar(name string) *Var {
	return &Var{Name: name, Type: types.LongInt, Scalar: true, VT: I32, Mem: MemI32, Param: -1}
}

// allocBSS reserviert Platz für eine nicht initialisierte globale Variable
// und liefert den Offset relativ zum Beginn des BSS (Lower trägt die Basis
// am Ende nach).
func (l *lowerer) allocBSS(size, align int64) int64 {
	off := types.AlignTo(l.m.BSSSize, max(align, 1))
	l.m.BSSSize = off + size
	return off
}

// alloc reserviert Platz im Datensegment und liefert die absolute Adresse.
func (l *lowerer) alloc(size, align int64, init []byte) int64 {
	off := types.AlignTo(int64(len(l.m.Data)), max(align, 1))
	if need := off + size; need > int64(len(l.m.Data)) {
		l.m.Data = append(l.m.Data, make([]byte, need-int64(len(l.m.Data)))...)
	}
	copy(l.m.Data[off:], init)
	return DataBase + off
}

// stringAddr legt eine Stringkonstante als ShortString (Längenbyte + Bytes)
// im Datensegment ab.
func (l *lowerer) stringAddr(s string) int64 {
	if a, ok := l.strs[s]; ok {
		return a
	}
	a := l.alloc(int64(len(s))+1, 1, append([]byte{byte(len(s))}, s...))
	l.strs[s] = a
	return a
}

// setAddr legt eine konstante Menge im Datensegment ab.
func (l *lowerer) setConstAddr(v [4]uint64) int64 {
	if a, ok := l.sets[v]; ok {
		return a
	}
	b := make([]byte, 32)
	for i, w := range v {
		binary.LittleEndian.PutUint64(b[i*8:], w)
	}
	a := l.alloc(32, 4, b)
	l.sets[v] = a
	return a
}

// tableIndex liefert den Index einer Routine in der Funktionstabelle und
// trägt sie bei der ersten Verwendung ein. Weil der Index hier schon
// feststeht, kann er direkt in Code und Datensegment stehen.
func (l *lowerer) tableIndex(p *types.Proc) int32 {
	if i, ok := l.table[p]; ok {
		return i
	}
	l.tableProcs = append(l.tableProcs, p)
	i := int32(len(l.tableProcs))
	l.table[p] = i
	return i
}

// funcSig liefert die WASM-Signatur eines prozeduralen Typs, nach denselben
// Regeln wie newFunc für Routinen der obersten Ebene.
func (l *lowerer) funcSig(sig *types.Signature) *FuncSig {
	var params, results []ValType
	if sig.Result != nil {
		if vt, _, ok := scalar(sig.Result); ok {
			results = []ValType{vt}
		} else {
			params = append(params, I32) // Adresse des Ergebnisses
		}
	}
	for _, p := range sig.Params {
		vt, _, ok := scalar(p.Type)
		if p.Mode == types.ByVar || p.Type == nil || !ok {
			vt = I32
		}
		params = append(params, vt)
	}
	key := SigKey(params, results)
	fs, ok := l.m.Sigs[key]
	if !ok {
		fs = &FuncSig{Params: params, Results: results}
		l.m.Sigs[key] = fs
	}
	return fs
}

// encode schreibt einen Konstantenwert im Speicherformat des Typs t.
func (l *lowerer) encode(t types.Type, v *types.Value) []byte {
	b := make([]byte, t.Size())
	l.put(b, t, v)
	return b
}

func (l *lowerer) put(b []byte, t types.Type, v *types.Value) {
	if v == nil {
		return
	}
	if v.Kind == types.VProc {
		binary.LittleEndian.PutUint32(b, uint32(l.tableIndex(v.Proc)))
		return
	}
	switch t := t.(type) {
	case *types.Array:
		es := t.Elem.Size()
		if v.Kind == types.VString { // packed array of Char aus einem String
			copy(b, v.Str)
			return
		}
		for i, e := range v.Elems {
			l.put(b[int64(i)*es:], t.Elem, e)
		}
		return
	case *types.Record:
		for i, e := range v.Elems {
			f := t.Fields[i]
			l.put(b[f.Offset:], f.Type, e)
		}
		return
	case *types.String:
		s := v.Str
		if v.Kind == types.VInt {
			s = string([]byte{byte(v.Int)})
		}
		b[0] = byte(len(s))
		copy(b[1:], s)
		return
	case *types.Set:
		for i, w := range v.Set {
			binary.LittleEndian.PutUint64(b[i*8:], w)
		}
		return
	}
	_, mem, _ := scalar(t)
	switch mem {
	case MemI8, MemU8:
		b[0] = byte(v.Int)
	case MemI16, MemU16:
		binary.LittleEndian.PutUint16(b, uint16(v.Int))
	case MemI32:
		binary.LittleEndian.PutUint32(b, uint32(v.Int))
	case MemF32:
		binary.LittleEndian.PutUint32(b, math.Float32bits(float32(v.AsReal())))
	case MemF64:
		binary.LittleEndian.PutUint64(b, math.Float64bits(v.AsReal()))
	}
}

// ---------------------------------------------------------------------------
// Deklarationen

// declare legt globale Variablen und typisierte Konstanten an und erzeugt für
// jede Prozedur eine Func. parent ist die umschließende Funktion.
func (l *lowerer) declare(s *types.Scope, parent *Func, prefix string) {
	for _, sym := range s.Order {
		switch sym := sym.(type) {
		case *types.Var:
			switch sym.Kind {
			case types.GlobalVar:
				checkSupported(sym.Type, sym.Pos())
				v := newVar(sym.Text(), sym.Type)
				v.Storage = Global
				v.Offset = l.allocBSS(sym.Type.Size(), sym.Type.Align())
				l.bss = append(l.bss, v)
				l.vars[sym] = v
				l.m.Globals = append(l.m.Globals, v)
			case types.TypedConst:
				checkSupported(sym.Type, sym.Pos())
				v := newVar(sym.Text(), sym.Type)
				v.Storage = Global
				v.Init = l.encode(sym.Type, sym.Val)
				v.Offset = l.alloc(sym.Type.Size(), sym.Type.Align(), v.Init)
				l.vars[sym] = v
				l.m.Globals = append(l.m.Globals, v)
			}
		case *types.Proc:
			if _, done := l.funcs[sym]; done {
				continue // Interface-Kopf, bereits angelegt
			}
			name := sym.Text()
			if prefix != "" {
				name = prefix + "." + name
			}
			l.newFunc(sym, parent, name)
		}
	}
}

// newFunc legt die Func einer Prozedur mit Parametern und Locals an.
func (l *lowerer) newFunc(p *types.Proc, parent *Func, name string) *Func {
	f := &Func{Name: name, Display: p.Text(), Proc: p, Parent: parent, Pos: l.pos(p.Pos()), RTL: l.rtlMode}
	l.funcs[p] = f
	l.m.Funcs = append(l.m.Funcs, f)
	param := func(v *Var) *Var {
		v.Param = len(f.Params)
		f.Params = append(f.Params, v)
		return f.NewVar(v)
	}
	if parent != nil {
		parent.HasNested = true
		sl := addrVar("$link")
		sl.Addressed = true // liegt immer im Frame
		f.StaticLink = param(sl)
	}
	if p.IsFunc {
		checkSupported(p.Sig.Result, p.Pos())
		r := newVar("Result", p.Sig.Result)
		if r.Scalar {
			f.NewVar(r)
		} else {
			r.Storage = Indirect
			param(r)
		}
		f.Result = r
		l.vars[p.Result] = r
	}
	for _, prm := range p.Sig.Params {
		if prm.Type != nil {
			checkSupported(prm.Type, p.Pos())
		}
		v := newVar(prm.Name, prm.Type)
		switch {
		case prm.Mode == types.ByVar || prm.Type == nil || (!v.Scalar && prm.Mode == types.ByConst):
			v.Storage = Indirect
			param(v)
		case !v.Scalar:
			// Wertparameter vom Aggregattyp: Adresse kommt herein, Kopie im Frame
			src := param(addrVar("$" + prm.Name))
			f.NewVar(v)
			l.copyIns[f] = append(l.copyIns[f], copyIn{dst: v, src: src})
		default:
			param(v)
		}
		l.vars[prm.Var] = v
	}
	if p.Scope != nil {
		for _, sym := range p.Scope.Order {
			if v, ok := sym.(*types.Var); ok && v.Kind == types.LocalVar {
				checkSupported(v.Type, v.Pos())
				l.vars[v] = f.NewVar(newVar(v.Text(), v.Type))
			}
		}
		l.declare(p.Scope, f, name)
	}
	return f
}

// ---------------------------------------------------------------------------
// Rümpfe

// fnState ist der Zustand beim Übersetzen eines Rumpfs.
type fnState struct {
	l         *lowerer
	fn        *Func
	proc      *types.Proc
	cur       *Block
	exit      *Block
	breaks    []*Block
	continues []*Block
	withs     map[ast.Expr]*Var // with-Record → Hilfswert mit seiner Adresse
	at        Pos               // aktuelle Quelltextstelle
	io        bool              // {$I+} gilt für die aktuelle Anweisung
	rtl       bool              // Code der System-Unit (ohne Positionspflege)
	labels    map[*types.Label]*Block
	tmp       int
}

// body übersetzt einen Rumpf.
func (l *lowerer) body(f *Func, p *types.Proc, stmts []ast.Stmt) {
	s := &fnState{l: l, fn: f, proc: p, withs: map[ast.Expr]*Var{}, at: f.Pos, rtl: f.RTL, labels: map[*types.Label]*Block{}}
	f.Entry = f.NewBlock()
	s.cur = f.Entry
	s.exit = f.NewBlock()
	if f == l.m.Main && l.info.SystemImpl != nil {
		// Programmende wie Halt(0): Exit-Prozeduren, Ausgabe leeren, beenden.
		// TP-ANNAHME: Wie in TP endet das Programm mit Halt(0); ein im
		// Hauptprogramm gesetzter ExitCode zählt also nicht, nur einer aus
		// einer Exit-Prozedur.
		saved := s.cur
		s.cur = s.exit
		s.callRtl("_Halt", nil, IntConst{V: 0})
		s.cur = saved
	}
	s.exit.Term = Return{}
	if f == l.m.Main {
		for _, init := range l.mainInits {
			s.emit(Call{Fn: init, Pos: f.Pos})
		}
	}
	for _, ci := range l.copyIns[f] {
		if st, ok := ci.dst.Type.(*types.String); ok {
			s.callRtl("_StrAssign", nil, Addr{Var: ci.dst}, Get{Var: ci.src}, IntConst{V: int32(st.Max)})
		} else {
			s.emit(Copy{Dst: Addr{Var: ci.dst}, Src: Get{Var: ci.src}, N: IntConst{V: int32(ci.dst.Type.Size())}})
		}
	}
	for _, st := range stmts {
		s.stmt(st)
	}
	s.jump(s.exit)
}

func (s *fnState) emit(in Instr) { s.cur.Instrs = append(s.cur.Instrs, in) }

// terminate schließt den aktuellen Block ab und beginnt einen neuen, der nur
// erreichbar ist, wenn jemand dorthin springt (sonst entfällt er).
func (s *fnState) terminate(t Term) {
	s.cur.Term = t
	s.cur = s.fn.NewBlock()
}

func (s *fnState) jump(to *Block) { s.terminate(Jump{To: to}) }

func (s *fnState) branch(c Expr, then, els *Block) {
	s.terminate(Branch{Cond: c, Then: then, Else: els})
}

// check bricht mit Laufzeitfehler code ab, wenn fail ungleich 0 ist.
func (s *fnState) check(fail Expr, code int) {
	trap, ok := s.fn.NewBlock(), s.fn.NewBlock()
	trap.Term = Trap{Code: code, Pos: s.at, KeepPos: s.rtl}
	s.branch(fail, trap, ok)
	s.cur = ok
}

func (s *fnState) temp(t types.Type) *Var {
	s.tmp++
	v := newVar("t"+itoa(s.tmp), t)
	if v.Scalar {
		v.Temp = true
	}
	return s.fn.NewVar(v)
}

// tempAddr ist ein Hilfswert für eine Adresse.
func (s *fnState) tempAddr() *Var {
	s.tmp++
	v := addrVar("a" + itoa(s.tmp))
	v.Temp = true
	return s.fn.NewVar(v)
}

// keep legt einen Ausdruck in einem Hilfswert ab, damit er nur einmal
// ausgewertet wird.
func (s *fnState) keep(e Expr) Expr {
	switch e.(type) {
	case IntConst, FloatConst, Get, HeapBase:
		return e
	}
	t := types.Type(types.LongInt)
	if e.VT() == F64 {
		t = types.Real
	}
	v := s.temp(t)
	s.emit(Set{Var: v, Val: e})
	return Get{Var: v}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

// variable liefert die IR-Variable zu einer Pascal-Variable. Gehört sie einer
// umschließenden Prozedur, muss sie im Frame liegen (Zugriff über Static Link).
func (s *fnState) variable(v *types.Var, pos source.Pos) *Var {
	iv, ok := s.l.vars[v]
	if !ok {
		panic("ir: Variable " + v.Name() + " nicht angelegt")
	}
	if iv.Owner != nil && iv.Owner != s.fn {
		iv.Addressed = true
	}
	return iv
}

// ---------------------------------------------------------------------------
// Anweisungen

func (s *fnState) stmt(st ast.Stmt) {
	if _, isCompound := st.(*ast.CompoundStmt); !isCompound {
		s.at = s.l.pos(st.Pos())
		s.io = ioChecks(st)
		if !s.l.opts.NoLines && !s.rtl {
			s.emit(Line{Pos: s.at})
		}
	}
	switch st := st.(type) {
	case *ast.EmptyStmt:
	case *ast.CompoundStmt:
		for _, b := range st.Stmts {
			s.stmt(b)
		}
	case *ast.LabeledStmt:
		b := s.labelBlock(st.Label)
		s.jump(b)
		s.cur = b
		s.stmt(st.Stmt)
	case *ast.AssignStmt:
		s.assign(st)
	case *ast.CallStmt:
		s.callStmt(st.Call)
	case *ast.IfStmt:
		then, join := s.fn.NewBlock(), s.fn.NewBlock()
		els := join
		if st.Else != nil {
			els = s.fn.NewBlock()
		}
		s.cond(st.Cond, then, els)
		s.cur = then
		s.stmt(st.Then)
		s.jump(join)
		if st.Else != nil {
			s.cur = els
			s.stmt(st.Else)
			s.jump(join)
		}
		s.cur = join
	case *ast.WhileStmt:
		head, body, exit := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
		s.jump(head)
		s.cur = head
		s.cond(st.Cond, body, exit)
		s.cur = body
		s.loop(exit, head, func() { s.stmt(st.Body) })
		s.jump(head)
		s.cur = exit
	case *ast.RepeatStmt:
		body, test, exit := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
		s.jump(body)
		s.cur = body
		s.loop(exit, test, func() {
			for _, b := range st.Body {
				s.stmt(b)
			}
		})
		s.jump(test)
		s.cur = test
		s.cond(st.Cond, exit, body)
		s.cur = exit
	case *ast.ForStmt:
		s.forStmt(st)
	case *ast.ForInStmt:
		s.forIn(st)
	case *ast.CaseStmt:
		s.caseStmt(st)
	case *ast.WithStmt:
		for _, r := range st.Records {
			a := s.tempAddr()
			s.emit(Set{Var: a, Val: s.designator(r)})
			s.withs[r] = a
		}
		s.stmt(st.Body)
	case *ast.GotoStmt:
		s.fn.HasGoto = true
		s.jump(s.labelBlock(st.Label))
	default:
		panic("ir: unbekannte Anweisung")
	}
}

// ioChecks liefert den Zustand von {$I+}/{$I-} am Beginn einer Anweisung.
func ioChecks(st ast.Stmt) bool {
	switch st := st.(type) {
	case *ast.EmptyStmt:
		return st.IOChecks
	case *ast.AssignStmt:
		return st.IOChecks
	case *ast.CallStmt:
		return st.IOChecks
	case *ast.IfStmt:
		return st.IOChecks
	case *ast.CaseStmt:
		return st.IOChecks
	case *ast.WhileStmt:
		return st.IOChecks
	case *ast.RepeatStmt:
		return st.IOChecks
	case *ast.ForStmt:
		return st.IOChecks
	case *ast.ForInStmt:
		return st.IOChecks
	case *ast.WithStmt:
		return st.IOChecks
	case *ast.GotoStmt:
		return st.IOChecks
	case *ast.LabeledStmt:
		return st.IOChecks
	}
	return true
}

// labelBlock liefert den Block, an dem eine Marke beginnt.
func (s *fnState) labelBlock(id *ast.Ident) *Block {
	l := s.l.info.Uses[id].(*types.Label)
	b, ok := s.labels[l]
	if !ok {
		b = s.fn.NewBlock()
		s.labels[l] = b
	}
	return b
}

func (s *fnState) loop(brk, cont *Block, body func()) {
	s.breaks = append(s.breaks, brk)
	s.continues = append(s.continues, cont)
	body()
	s.breaks = s.breaks[:len(s.breaks)-1]
	s.continues = s.continues[:len(s.continues)-1]
}

func (s *fnState) typeOf(e ast.Expr) types.Type { return s.l.info.Types[e].Type }

func (s *fnState) assign(st *ast.AssignStmt) {
	// Zuweisung an den Funktionsnamen (auch einer umschließenden Funktion)
	if id, ok := st.Lhs.(*ast.Ident); ok {
		if p, ok := s.l.info.Uses[id].(*types.Proc); ok {
			r := s.variable(p.Result, id.Pos())
			s.assignTo(r.Type, Addr{Var: r}, r, st.Rhs)
			return
		}
	}
	t := s.typeOf(st.Lhs)
	if v, ok := s.directVar(st.Lhs); ok && v.Scalar {
		s.emit(Set{Var: v, Val: s.value(st.Rhs, t)})
		return
	}
	if _, mem, ok := scalar(t); ok {
		val := s.value(st.Rhs, t)
		s.emit(Store{Addr: s.designator(st.Lhs), Mem: mem, Val: val})
		return
	}
	s.assignTo(t, s.designator(st.Lhs), nil, st.Rhs)
}

// assignTo weist rhs an ein Ziel vom Typ t zu: skalar über v (falls gesetzt)
// oder an die Adresse dst.
func (s *fnState) assignTo(t types.Type, dst Expr, v *Var, rhs ast.Expr) {
	if _, mem, ok := scalar(t); ok {
		if v != nil && v.Scalar {
			s.emit(Set{Var: v, Val: s.value(rhs, t)})
		} else {
			s.emit(Store{Addr: dst, Mem: mem, Val: s.value(rhs, t)})
		}
		return
	}
	switch tt := t.(type) {
	case *types.String:
		s.callRtl("_StrAssign", nil, dst, s.strAddr(rhs), IntConst{V: int32(tt.Max)})
	case *types.Set:
		s.emit(Copy{Dst: dst, Src: s.setAddr(rhs), N: IntConst{V: 32}})
	case *types.Array:
		if tv := s.l.info.Types[rhs]; tv.Mode == check.Constant && tv.Val.Kind == types.VString {
			a := s.l.stringAddr(tv.Val.Str)
			s.emit(Copy{Dst: dst, Src: IntConst{V: int32(a + 1)}, N: IntConst{V: int32(len(tv.Val.Str))}})
			return
		}
		s.emit(Copy{Dst: dst, Src: s.addr(rhs), N: IntConst{V: int32(t.Size())}})
	default:
		s.emit(Copy{Dst: dst, Src: s.addr(rhs), N: IntConst{V: int32(t.Size())}})
	}
}

// directVar liefert die Variable, wenn e ein einfacher Variablenbezeichner
// (ohne with-Feld) ist.
func (s *fnState) directVar(e ast.Expr) (*Var, bool) {
	var id *ast.Ident
	switch e := e.(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		if _, isField := s.l.info.Fields[e.Sel]; isField {
			return nil, false
		}
		id = e.Sel
	default:
		return nil, false
	}
	if _, isField := s.l.info.Fields[id]; isField {
		return nil, false
	}
	v, ok := s.l.info.Uses[id].(*types.Var)
	if !ok {
		return nil, false
	}
	return s.variable(v, id.Pos()), true
}

func (s *fnState) forStmt(st *ast.ForStmt) {
	v := s.variable(s.l.info.Uses[st.Var].(*types.Var), st.Var.Pos())
	from := s.temp(v.Type)
	s.emit(Set{Var: from, Val: s.value(st.From, v.Type)})
	end := s.temp(v.Type)
	s.emit(Set{Var: end, Val: s.value(st.To, v.Type)})
	s.emit(Set{Var: v, Val: Get{Var: from}})
	body, incr, step, exit := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
	cmp, inc := LeI, AddI
	if st.Down {
		cmp, inc = GeI, SubI
	}
	s.branch(Bin{Op: cmp, X: Get{Var: from}, Y: Get{Var: end}}, body, exit)
	s.cur = body
	s.loop(exit, incr, func() { s.stmt(st.Body) })
	s.jump(incr)
	s.cur = incr
	s.branch(Bin{Op: EqI, X: Get{Var: v}, Y: Get{Var: end}}, exit, step)
	s.cur = step
	s.emit(Set{Var: v, Val: Bin{Op: inc, X: Get{Var: v}, Y: IntConst{V: 1}}})
	s.jump(body)
	s.cur = exit
}

// forIn läuft über Array-Elemente, String-Zeichen oder Mengenelemente.
func (s *fnState) forIn(st *ast.ForInStmt) {
	v := s.variable(s.l.info.Uses[st.Var].(*types.Var), st.Var.Pos())
	coll := s.typeOf(st.In)
	idx := s.temp(types.LongInt)
	end := s.temp(types.LongInt)
	base := s.tempAddr()
	var elemT types.Type
	var elemAddr func() Expr
	var skip func() Expr // nicht 0: Element überspringen (Mengen)
	decode := false      // String zeichenweise (WideChar-Laufvariable, UTF8Next)
	switch t := coll.(type) {
	case *types.Array:
		s.emit(Set{Var: base, Val: s.designator(st.In)})
		s.emit(Set{Var: idx, Val: IntConst{V: 0}})
		s.emit(Set{Var: end, Val: IntConst{V: int32(t.Len() - 1)}})
		elemT = t.Elem
		elemAddr = func() Expr {
			return Bin{Op: AddI, X: Get{Var: base}, Y: Bin{Op: MulI, X: Get{Var: idx}, Y: IntConst{V: int32(t.Elem.Size())}}}
		}
	case *types.Set:
		// Über eine Kopie der Menge, damit der Rumpf sie ändern darf.
		cp := s.temp(t)
		s.emit(Copy{Dst: Addr{Var: cp}, Src: s.setAddr(st.In), N: IntConst{V: 32}})
		s.emit(Set{Var: base, Val: Addr{Var: cp}})
		lo, hi := int64(0), int64(255)
		if t.Elem != nil {
			lo, hi = types.Bounds(t.Elem)
		}
		s.emit(Set{Var: idx, Val: IntConst{V: int32(max(lo, 0))}})
		s.emit(Set{Var: end, Val: IntConst{V: int32(min(hi, 255))}})
		elemT = v.Type
		skip = func() Expr { return Un{Op: EqzI, X: s.bitTest(Get{Var: base}, Get{Var: idx})} }
	default: // String oder Char
		s.emit(Set{Var: base, Val: s.strAddr(st.In)})
		s.emit(Set{Var: idx, Val: IntConst{V: 1}})
		s.emit(Set{Var: end, Val: Load{Addr: Get{Var: base}, Mem: MemU8}})
		elemT = types.Char
		elemAddr = func() Expr { return Bin{Op: AddI, X: Get{Var: base}, Y: Get{Var: idx}} }
		decode = types.Is(v.Type, types.WideChar)
	}
	head, body, next, exit := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
	s.jump(head)
	s.cur = head
	s.branch(Bin{Op: LeI, X: Get{Var: idx}, Y: Get{Var: end}}, body, exit)
	s.cur = body
	if skip != nil {
		take := s.fn.NewBlock()
		s.branch(skip(), next, take)
		s.cur = take
		s.emit(Set{Var: v, Val: Get{Var: idx}})
	} else if decode {
		w := s.rtlValue("UTF8Next", types.WideChar, Get{Var: base}, Addr{Var: s.addressed(idx)})
		s.emit(Set{Var: v, Val: s.coerce(w, types.WideChar, v.Type)})
	} else if v.Scalar {
		_, mem, _ := scalar(elemT)
		s.emit(Set{Var: v, Val: s.coerce(Load{Addr: elemAddr(), Mem: mem}, elemT, v.Type)})
	} else {
		s.emit(Copy{Dst: Addr{Var: v}, Src: elemAddr(), N: IntConst{V: int32(elemT.Size())}})
	}
	s.loop(exit, next, func() { s.stmt(st.Body) })
	s.jump(next)
	s.cur = next
	if !decode { // UTF8Next hat den Index schon weitergesetzt
		s.emit(Set{Var: idx, Val: Bin{Op: AddI, X: Get{Var: idx}, Y: IntConst{V: 1}}})
	}
	s.jump(head)
	s.cur = exit
}

func (s *fnState) caseStmt(st *ast.CaseStmt) {
	sel := s.temp(types.LongInt)
	s.emit(Set{Var: sel, Val: s.expr(st.X)})
	join := s.fn.NewBlock()
	for _, arm := range st.Arms {
		var test Expr
		for _, lb := range arm.Labels {
			lo := s.l.info.Types[lb.Lo].Val.Int
			var t Expr = Bin{Op: EqI, X: Get{Var: sel}, Y: IntConst{V: int32(lo)}}
			if lb.Hi != nil {
				hi := s.l.info.Types[lb.Hi].Val.Int
				t = Bin{Op: AndI,
					X: Bin{Op: GeI, X: Get{Var: sel}, Y: IntConst{V: int32(lo)}},
					Y: Bin{Op: LeI, X: Get{Var: sel}, Y: IntConst{V: int32(hi)}}}
			}
			if test == nil {
				test = t
			} else {
				test = Bin{Op: OrI, X: test, Y: t}
			}
		}
		body, next := s.fn.NewBlock(), s.fn.NewBlock()
		s.branch(test, body, next)
		s.cur = body
		s.stmt(arm.Body)
		s.jump(join)
		s.cur = next
	}
	for _, e := range st.Else {
		s.stmt(e)
	}
	s.jump(join)
	s.cur = join
}

// ---------------------------------------------------------------------------
// Bedingungen (Kurzschlussauswertung, wie {$B-})

func isBoolOp(e ast.Expr, info *check.Info) (*ast.BinaryExpr, bool) {
	b, ok := e.(*ast.BinaryExpr)
	if !ok || (b.Op != lexAnd && b.Op != lexOr) {
		return nil, false
	}
	return b, types.Is(info.Types[b.X].Type, types.Boolean)
}

// cond übersetzt eine Bedingung als Sprung nach then oder els und schließt
// den aktuellen Block ab.
func (s *fnState) cond(e ast.Expr, then, els *Block) {
	if tv := s.l.info.Types[e]; tv.Mode == check.Constant {
		if tv.Val.Int != 0 {
			s.jump(then)
		} else {
			s.jump(els)
		}
		return
	}
	if b, ok := isBoolOp(e, s.l.info); ok {
		mid := s.fn.NewBlock()
		if b.Op == lexAnd {
			s.cond(b.X, mid, els)
		} else {
			s.cond(b.X, then, mid)
		}
		s.cur = mid
		s.cond(b.Y, then, els)
		return
	}
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == lexNot {
		s.cond(u.X, els, then)
		return
	}
	s.branch(s.expr(e), then, els)
}

// boolValue berechnet and/or als Wert über Sprünge (Kurzschluss).
func (s *fnState) boolValue(e ast.Expr) Expr {
	t := s.temp(types.Boolean)
	then, els, join := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
	s.cond(e, then, els)
	s.cur = then
	s.emit(Set{Var: t, Val: IntConst{V: 1}})
	s.jump(join)
	s.cur = els
	s.emit(Set{Var: t, Val: IntConst{V: 0}})
	s.jump(join)
	s.cur = join
	return Get{Var: t}
}

// ifElse verzweigt über eine Bedingung.
func (s *fnState) ifElse(c Expr, then, els func()) {
	tb, eb, join := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
	s.branch(c, tb, eb)
	s.cur = tb
	then()
	s.jump(join)
	s.cur = eb
	els()
	s.jump(join)
	s.cur = join
}
