package ir

import (
	"encoding/binary"
	"math"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// Options steuert das Lowering.
type Options struct {
	StackSize int64
}

type bailout struct{ err error }

// Lower übersetzt ein geprüftes Programm in die IR. Sprachmittel, die die
// Codegenerierung noch nicht beherrscht, ergeben Error 902 mit Angabe der
// Etappe.
func Lower(prog *ast.Program, info *check.Info, opts Options) (m *Module, err error) {
	l := &lowerer{
		info:    info,
		m:       &Module{StackSize: opts.StackSize},
		funcs:   map[*types.Proc]*Func{},
		globals: map[*types.Var]*Var{},
		strs:    map[string]int64{},
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
	main := &Func{Name: "main"}
	l.m.Main = main
	l.m.Funcs = append(l.m.Funcs, main)
	l.declare(info.Root, "")
	l.body(main, nil, prog.Block.Body)
	for _, d := range l.procDecls(prog.Block.Decls) {
		l.procBody(d)
	}
	return l.m, nil
}

type lowerer struct {
	info    *check.Info
	m       *Module
	funcs   map[*types.Proc]*Func
	globals map[*types.Var]*Var
	strs    map[string]int64
}

func unsupported(pos source.Pos, what string) {
	panic(bailout{source.Errorf(pos, source.ErrNotSupported, "%s", what)})
}

// procDecls sammelt alle Prozedurdeklarationen mit Rumpf, auch verschachtelte.
func (l *lowerer) procDecls(ds []ast.Decl) []*ast.ProcDecl {
	var out []*ast.ProcDecl
	for _, d := range ds {
		if pd, ok := d.(*ast.ProcDecl); ok && pd.Block != nil {
			out = append(out, pd)
			out = append(out, l.procDecls(pd.Block.Decls)...)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Typen und Daten

// scalar liefert Wert- und Speicherdarstellung eines skalaren Typs.
func scalar(t types.Type) (vt ValType, mem MemType, ok bool) {
	switch {
	case types.IsReal(t):
		if types.Is(t, types.Single) {
			return F64, MemF32, true
		}
		return F64, MemF64, true
	case types.IsOrdinal(t):
		if t.Size() == 1 {
			if lo, _ := types.Bounds(t); lo < 0 {
				return I32, MemI8, true
			}
			return I32, MemU8, true
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
		unsupported(pos, "records are supported from stage 5")
	case *types.String:
		unsupported(pos, "string variables are supported from stage 5")
	case *types.Set:
		unsupported(pos, "set variables are supported from stage 5")
	case *types.Pointer, types.NilType:
		unsupported(pos, "pointers are supported from stage 5")
	case *types.File:
		unsupported(pos, "files are supported from stage 6")
	}
}

func newVar(name string, t types.Type) *Var {
	v := &Var{Name: name, Type: t, Param: -1}
	if t != nil {
		v.VT, v.Mem, v.Scalar = scalar(t)
	}
	return v
}

// alloc reserviert Platz im Datensegment und liefert die absolute Adresse.
func (l *lowerer) alloc(size, align int64, init []byte) int64 {
	off := types.AlignTo(int64(len(l.m.Data)), max(align, 1))
	need := off + size
	for int64(len(l.m.Data)) < need {
		l.m.Data = append(l.m.Data, 0)
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

// encode schreibt einen Konstantenwert im Speicherformat des Typs t.
func encode(t types.Type, v *types.Value) []byte {
	b := make([]byte, t.Size())
	put(b, t, v)
	return b
}

func put(b []byte, t types.Type, v *types.Value) {
	if v == nil {
		return
	}
	switch t := t.(type) {
	case *types.Array:
		es := t.Elem.Size()
		for i, e := range v.Elems {
			put(b[int64(i)*es:], t.Elem, e)
		}
		return
	case *types.Record:
		for i, e := range v.Elems {
			f := t.Fields[i]
			put(b[f.Offset:], f.Type, e)
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
// jede Prozedur eine Func. prefix ist der Name der umschließenden Prozedur.
func (l *lowerer) declare(s *types.Scope, prefix string) {
	for _, sym := range s.Order {
		switch sym := sym.(type) {
		case *types.Var:
			switch sym.Kind {
			case types.GlobalVar:
				checkSupported(sym.Type, sym.Pos())
				v := newVar(sym.Text(), sym.Type)
				v.Storage = Global
				v.Offset = l.alloc(sym.Type.Size(), sym.Type.Align(), nil)
				l.globals[sym] = v
				l.m.Globals = append(l.m.Globals, v)
			case types.TypedConst:
				checkSupported(sym.Type, sym.Pos())
				v := newVar(sym.Text(), sym.Type)
				v.Storage = Global
				v.Init = encode(sym.Type, sym.Val)
				v.Offset = l.alloc(sym.Type.Size(), sym.Type.Align(), v.Init)
				l.globals[sym] = v
				l.m.Globals = append(l.m.Globals, v)
			}
		case *types.Proc:
			name := sym.Text()
			if prefix != "" {
				name = prefix + "." + name
			}
			l.newFunc(sym, name)
			if sym.Scope != nil {
				l.declare(sym.Scope, name)
			}
		}
	}
}

// newFunc legt die Func einer Prozedur mit ihren Parametern an.
func (l *lowerer) newFunc(p *types.Proc, name string) *Func {
	f := &Func{Name: name, Proc: p}
	l.funcs[p] = f
	l.m.Funcs = append(l.m.Funcs, f)
	param := func(v *Var) {
		v.Param = len(f.Params)
		f.Params = append(f.Params, v)
		f.NewVar(v)
	}
	if p.IsFunc {
		checkSupported(p.Sig.Result, p.Pos())
		r := newVar("Result", p.Sig.Result)
		if !r.Scalar {
			r.Storage = Indirect
			param(r)
		} else {
			f.NewVar(r)
		}
		f.Result = r
	}
	for _, prm := range p.Sig.Params {
		if prm.Type != nil {
			checkSupported(prm.Type, p.Pos())
		}
		v := newVar(prm.Name, prm.Type)
		switch {
		case prm.Mode == types.ByVar || prm.Type == nil:
			v.Storage = Indirect
		case !v.Scalar && prm.Mode == types.ByConst:
			v.Storage = Indirect
		case !v.Scalar:
			v.CopyIn = true
		}
		param(v)
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
	vars      map[*types.Var]*Var
	exit      *Block
	breaks    []*Block
	continues []*Block
	tmp       int
}

func (l *lowerer) procBody(d *ast.ProcDecl) {
	p := l.info.Procs[d]
	l.body(l.funcs[p], p, d.Block.Body)
}

func (l *lowerer) body(f *Func, p *types.Proc, body *ast.CompoundStmt) {
	s := &fnState{l: l, fn: f, proc: p, vars: map[*types.Var]*Var{}}
	if p != nil {
		i := 0
		if f.Result != nil {
			s.vars[p.Result] = f.Result
			if !f.Result.Scalar {
				i = 1
			}
		}
		for _, prm := range p.Sig.Params {
			s.vars[prm.Var] = f.Params[i]
			i++
		}
		for _, sym := range p.Scope.Order {
			if v, ok := sym.(*types.Var); ok && v.Kind == types.LocalVar {
				checkSupported(v.Type, v.Pos())
				s.vars[v] = f.NewVar(newVar(v.Text(), v.Type))
			}
		}
	}
	f.Entry = f.NewBlock()
	s.cur = f.Entry
	s.exit = f.NewBlock()
	s.exit.Term = Return{}
	s.compound(body)
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

func (s *fnState) temp(t types.Type) *Var {
	s.tmp++
	v := newVar("t"+itoa(s.tmp), t)
	if v.Scalar {
		v.Temp = true
	}
	return s.fn.NewVar(v)
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

// variable liefert die IR-Variable zu einer Pascal-Variable.
func (s *fnState) variable(v *types.Var, pos source.Pos) *Var {
	if g, ok := s.l.globals[v]; ok {
		return g
	}
	if iv, ok := s.vars[v]; ok {
		return iv
	}
	if v.Kind == types.BuiltinVar {
		unsupported(pos, v.Text()+" is supported from stage 6")
	}
	unsupported(pos, "access to locals of an enclosing procedure is supported from stage 5")
	return nil
}

// ---------------------------------------------------------------------------
// Anweisungen

func (s *fnState) compound(c *ast.CompoundStmt) {
	for _, st := range c.Stmts {
		s.stmt(st)
	}
}

func (s *fnState) stmt(st ast.Stmt) {
	switch st := st.(type) {
	case *ast.EmptyStmt:
	case *ast.CompoundStmt:
		s.compound(st)
	case *ast.LabeledStmt:
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
		unsupported(st.Pos(), "with is supported from stage 5")
	case *ast.GotoStmt:
		unsupported(st.Pos(), "goto is supported from stage 8")
	default:
		panic("ir: unbekannte Anweisung")
	}
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
	// Zuweisung an den Funktionsnamen
	if id, ok := st.Lhs.(*ast.Ident); ok {
		if p, ok := s.l.info.Uses[id].(*types.Proc); ok {
			if p != s.proc {
				unsupported(id.Pos(), "assigning the result of an enclosing function is supported from stage 5")
			}
			s.assignTo(s.fn.Result, st.Rhs)
			return
		}
	}
	t := s.typeOf(st.Lhs)
	if _, _, ok := scalar(t); !ok {
		s.emit(Copy{Dst: s.addr(st.Lhs), Src: s.addr(st.Rhs), Size: t.Size()})
		return
	}
	val := s.value(st.Rhs, t)
	s.store(st.Lhs, val)
}

// assignTo weist einer IR-Variable zu.
func (s *fnState) assignTo(v *Var, rhs ast.Expr) {
	if !v.Scalar {
		s.emit(Copy{Dst: Addr{Var: v}, Src: s.addr(rhs), Size: v.Type.Size()})
		return
	}
	s.emit(Set{Var: v, Val: s.value(rhs, v.Type)})
}

// store schreibt einen skalaren Wert in eine Variable oder ein Element.
func (s *fnState) store(lhs ast.Expr, val Expr) {
	if v, ok := s.directVar(lhs); ok {
		s.emit(Set{Var: v, Val: val})
		return
	}
	_, mem, _ := scalar(s.typeOf(lhs))
	s.emit(Store{Addr: s.addr(lhs), Mem: mem, Val: val})
}

// directVar liefert die Variable, wenn e ein einfacher Variablenbezeichner ist.
func (s *fnState) directVar(e ast.Expr) (*Var, bool) {
	var id *ast.Ident
	switch e := e.(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return nil, false
	}
	if _, isField := s.l.info.Fields[id]; isField {
		unsupported(id.Pos(), "records are supported from stage 5")
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
	s.branch(Bin{Op: cmp, X: Get{Var: v}, Y: Get{Var: end}}, body, exit)
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

func (s *fnState) forIn(st *ast.ForInStmt) {
	v := s.variable(s.l.info.Uses[st.Var].(*types.Var), st.Var.Pos())
	arr, ok := s.typeOf(st.In).(*types.Array)
	if !ok {
		unsupported(st.In.Pos(), "for-in over strings and sets is supported from stage 5")
	}
	base := s.temp(types.LongInt)
	s.emit(Set{Var: base, Val: s.addr(st.In)})
	idx := s.temp(types.LongInt)
	s.emit(Set{Var: idx, Val: IntConst{V: 0}})
	head, body, next, exit := s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock(), s.fn.NewBlock()
	s.jump(head)
	s.cur = head
	s.branch(Bin{Op: LtI, X: Get{Var: idx}, Y: IntConst{V: int32(arr.Len())}}, body, exit)
	s.cur = body
	elemAddr := Bin{Op: AddI, X: Get{Var: base}, Y: Bin{Op: MulI, X: Get{Var: idx}, Y: IntConst{V: int32(arr.Elem.Size())}}}
	if v.Scalar {
		_, mem, _ := scalar(arr.Elem)
		s.emit(Set{Var: v, Val: s.coerce(Load{Addr: elemAddr, Mem: mem}, arr.Elem, v.Type)})
	} else {
		s.emit(Copy{Dst: Addr{Var: v}, Src: elemAddr, Size: arr.Elem.Size()})
	}
	s.loop(exit, next, func() { s.stmt(st.Body) })
	s.jump(next)
	s.cur = next
	s.emit(Set{Var: idx, Val: Bin{Op: AddI, X: Get{Var: idx}, Y: IntConst{V: 1}}})
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

// ifElse verzweigt über eine Bedingung (für Abs, Round).
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
