package ir

import (
	"math"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

const (
	lexAnd = lexer.And
	lexOr  = lexer.Or
	lexNot = lexer.Not
)

// value übersetzt e und passt das Ergebnis an den Zieltyp want an.
func (s *fnState) value(e ast.Expr, want types.Type) Expr {
	return s.coerce(s.expr(e), s.typeOf(e), want)
}

// coerce wandelt einen Wert vom Typ from in den Typ to: Ganzzahl nach Real,
// Kürzung auf 8 Bit bei 1-Byte-Typen (PLAN.md, Festlegungen).
func (s *fnState) coerce(x Expr, from, to types.Type) Expr {
	if types.IsReal(to) {
		if !types.IsReal(from) {
			if c, ok := x.(IntConst); ok {
				return FloatConst{V: float64(c.V)}
			}
			return Un{Op: IntToF, X: x}
		}
		return x
	}
	if from == to {
		return x
	}
	return narrow(x, to)
}

// narrow kürzt einen Ordinalwert auf die Speicherbreite von t.
func narrow(x Expr, t types.Type) Expr {
	_, mem, ok := scalar(t)
	if !ok {
		return x
	}
	switch mem {
	case MemU8:
		if c, ok := x.(IntConst); ok {
			return IntConst{V: int32(uint8(c.V))}
		}
		return Bin{Op: AndI, X: x, Y: IntConst{V: 255}}
	case MemI8:
		if c, ok := x.(IntConst); ok {
			return IntConst{V: int32(int8(c.V))}
		}
		return Un{Op: Ext8, X: x}
	}
	return x
}

// constant übersetzt einen Konstantenwert.
func constant(tv check.TypeAndValue) Expr {
	switch tv.Val.Kind {
	case types.VInt:
		return IntConst{V: int32(tv.Val.Int)}
	case types.VReal:
		return FloatConst{V: tv.Val.Real}
	case types.VNil:
		return IntConst{V: 0}
	}
	return nil
}

// expr übersetzt einen skalaren Ausdruck. Aufrufe werden als Anweisungen
// vorgezogen; der Ausdruck selbst bleibt frei von Seiteneffekten.
func (s *fnState) expr(e ast.Expr) Expr {
	tv := s.l.info.Types[e]
	if tv.Mode == check.Constant {
		if c := constant(tv); c != nil {
			return c
		}
		unsupported(e.Pos(), "string values are supported from stage 5")
	}
	if _, _, ok := scalar(tv.Type); !ok && tv.Type != nil {
		checkSupported(tv.Type, e.Pos())
	}
	switch e := e.(type) {
	case *ast.Ident:
		if call := s.l.info.Calls[e]; call != nil {
			return s.call(e, call, nil)
		}
		if v, ok := s.directVar(e); ok {
			return Get{Var: v}
		}
	case *ast.SelectorExpr:
		if call := s.l.info.Calls[e.Sel]; call != nil {
			return s.call(e.Sel, call, nil)
		}
		if v, ok := s.directVar(e); ok {
			return Get{Var: v}
		}
		unsupported(e.Pos(), "records are supported from stage 5")
	case *ast.IndexExpr:
		_, mem, _ := scalar(tv.Type)
		return Load{Addr: s.addr(e), Mem: mem}
	case *ast.CallExpr:
		call := s.l.info.Calls[e]
		if call.Kind == check.CallTypecast {
			return s.typecast(e, call.Type)
		}
		return s.call(e, call, e.Args)
	case *ast.UnaryExpr:
		return s.unary(e)
	case *ast.BinaryExpr:
		return s.binary(e)
	case *ast.DerefExpr:
		unsupported(e.Pos(), "pointers are supported from stage 5")
	case *ast.SetLit:
		unsupported(e.Pos(), "sets are supported from stage 5")
	}
	panic("ir: unerwarteter Ausdruck")
}

func (s *fnState) typecast(e *ast.CallExpr, t types.Type) Expr {
	x := s.expr(e.Args[0])
	from := s.typeOf(e.Args[0])
	if !types.IsOrdinal(t) || !types.IsOrdinal(from) {
		unsupported(e.Pos(), "this typecast is supported from stage 5")
	}
	if types.Is(t, types.Word) {
		return Bin{Op: AndI, X: x, Y: IntConst{V: 0xFFFF}}
	}
	return narrow(x, t)
}

func (s *fnState) unary(e *ast.UnaryExpr) Expr {
	t := s.typeOf(e.X)
	switch e.Op {
	case lexer.Not:
		x := s.expr(e.X)
		if types.Is(t, types.Boolean) {
			return Un{Op: EqzI, X: x}
		}
		return Bin{Op: XorI, X: x, Y: IntConst{V: -1}}
	case lexer.Minus:
		x := s.expr(e.X)
		if types.IsReal(t) {
			return Un{Op: NegF, X: x}
		}
		return Bin{Op: SubI, X: IntConst{V: 0}, Y: x}
	case lexer.Plus:
		return s.expr(e.X)
	}
	unsupported(e.Pos(), "@ is supported from stage 5")
	return nil
}

var intOps = map[lexer.Kind]Op{
	lexer.Plus: AddI, lexer.Minus: SubI, lexer.Star: MulI, lexer.Div: DivI, lexer.Mod: RemI,
	lexer.And: AndI, lexer.Or: OrI, lexer.Xor: XorI, lexer.Shl: ShlI, lexer.Shr: ShrI,
	lexer.Eq: EqI, lexer.Ne: NeI, lexer.Lt: LtI, lexer.Le: LeI, lexer.Gt: GtI, lexer.Ge: GeI,
}

var realOps = map[lexer.Kind]Op{
	lexer.Plus: AddF, lexer.Minus: SubF, lexer.Star: MulF, lexer.Slash: DivF,
	lexer.Eq: EqF, lexer.Ne: NeF, lexer.Lt: LtF, lexer.Le: LeF, lexer.Gt: GtF, lexer.Ge: GeF,
}

func (s *fnState) binary(e *ast.BinaryExpr) Expr {
	if _, ok := isBoolOp(e, s.l.info); ok {
		return s.boolValue(e)
	}
	xt, yt := s.typeOf(e.X), s.typeOf(e.Y)
	if e.Op == lexer.In {
		return s.inSet(e)
	}
	ordinal := types.IsOrdinal(xt) && types.IsOrdinal(yt)
	numeric := types.IsNumeric(xt) && types.IsNumeric(yt)
	switch {
	case ordinal && e.Op != lexer.Slash:
		x, y := s.expr(e.X), s.expr(e.Y)
		return Bin{Op: intOps[e.Op], X: x, Y: y}
	case numeric:
		x, y := s.value(e.X, types.Real), s.value(e.Y, types.Real)
		return Bin{Op: realOps[e.Op], X: x, Y: y}
	}
	unsupported(e.OpPos, "this operator is supported from stage 5")
	return nil
}

// inSet übersetzt x in [konstante Menge] als Vergleich mit den Bereichen der
// Menge; allgemeine Mengen folgen in Etappe 5.
func (s *fnState) inSet(e *ast.BinaryExpr) Expr {
	tv := s.l.info.Types[e.Y]
	if tv.Mode != check.Constant {
		unsupported(e.Y.Pos(), "sets are supported from stage 5")
	}
	x := s.temp(types.LongInt)
	s.emit(Set{Var: x, Val: s.expr(e.X)})
	var test Expr = IntConst{V: 0}
	for v := int64(0); v < 256; v++ {
		if !tv.Val.SetHas(v) {
			continue
		}
		hi := v
		for hi+1 < 256 && tv.Val.SetHas(hi+1) {
			hi++
		}
		var t Expr = Bin{Op: EqI, X: Get{Var: x}, Y: IntConst{V: int32(v)}}
		if hi > v {
			t = Bin{Op: AndI,
				X: Bin{Op: GeI, X: Get{Var: x}, Y: IntConst{V: int32(v)}},
				Y: Bin{Op: LeI, X: Get{Var: x}, Y: IntConst{V: int32(hi)}}}
		}
		test = Bin{Op: OrI, X: test, Y: t}
		v = hi
	}
	return test
}

// ---------------------------------------------------------------------------
// Adressen

// addr liefert die Adresse einer Variable, eines Elements oder eines
// Aggregatwerts (bei Funktionsergebnissen eine Hilfsvariable im Frame).
func (s *fnState) addr(e ast.Expr) Expr {
	switch e := e.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		var id *ast.Ident
		if sel, ok := e.(*ast.SelectorExpr); ok {
			id = sel.Sel
		} else {
			id = e.(*ast.Ident)
		}
		if call := s.l.info.Calls[id]; call != nil {
			return s.aggregateCall(id, call, nil)
		}
		v, ok := s.directVar(e)
		if !ok {
			unsupported(e.Pos(), "records are supported from stage 5")
		}
		if v.Scalar && v.Storage != Global && v.Storage != Indirect {
			v.Addressed = true
		}
		return Addr{Var: v}
	case *ast.IndexExpr:
		base := s.addr(e.X)
		t := s.typeOf(e.X)
		for _, ie := range e.Indices {
			arr, ok := t.(*types.Array)
			if !ok {
				unsupported(ie.Pos(), "string indexing is supported from stage 5")
			}
			size := int32(arr.Elem.Size())
			if tv := s.l.info.Types[ie]; tv.Mode == check.Constant {
				base = Bin{Op: AddI, X: base, Y: IntConst{V: int32(tv.Val.Int-arr.Lo) * size}}
			} else {
				i := Bin{Op: SubI, X: s.expr(ie), Y: IntConst{V: int32(arr.Lo)}}
				base = Bin{Op: AddI, X: base, Y: Bin{Op: MulI, X: i, Y: IntConst{V: size}}}
			}
			t = arr.Elem
		}
		return base
	case *ast.CallExpr:
		call := s.l.info.Calls[e]
		if call.Kind == check.CallProc {
			return s.aggregateCall(e, call, e.Args)
		}
	}
	unsupported(e.Pos(), "this expression is supported from stage 5")
	return nil
}

// aggregateCall ruft eine Funktion mit Aggregat-Ergebnis auf; das Ergebnis
// landet in einer Hilfsvariable, deren Adresse zurückkommt.
func (s *fnState) aggregateCall(at ast.Node, call *check.Call, args []ast.Expr) Expr {
	tmp := s.temp(call.Proc.Sig.Result)
	s.emitCall(at, call.Proc, args, tmp)
	return Addr{Var: tmp}
}

// ---------------------------------------------------------------------------
// Aufrufe

// call übersetzt einen Aufruf in Wertposition.
func (s *fnState) call(at ast.Node, call *check.Call, args []ast.Expr) Expr {
	if call.Kind == check.CallBuiltin {
		return s.builtin(at, call.Builtin, args, true)
	}
	res := call.Proc.Sig.Result
	tmp := s.temp(res)
	s.emitCall(at, call.Proc, args, tmp)
	return Get{Var: tmp}
}

// emitCall erzeugt den Aufruf; dst nimmt das Ergebnis auf (bei Aggregaten
// über die versteckte Ergebnisadresse) oder ist nil.
func (s *fnState) emitCall(at ast.Node, p *types.Proc, args []ast.Expr, dst *Var) {
	f := s.l.funcs[p]
	var in []Expr
	if f.Result != nil && !f.Result.Scalar {
		if dst == nil {
			dst = s.temp(p.Sig.Result)
		}
		in = append(in, Addr{Var: dst})
		dst = nil
	}
	for i, prm := range p.Sig.Params {
		a := args[i]
		switch {
		case prm.Mode == types.ByVar || prm.Type == nil:
			in = append(in, s.addr(a))
		default:
			if _, _, ok := scalar(prm.Type); ok {
				in = append(in, s.value(a, prm.Type))
			} else {
				in = append(in, s.addr(a))
			}
		}
	}
	s.emit(Call{Fn: f, Args: in, Dst: dst})
}

func (s *fnState) callStmt(e ast.Expr) {
	var at *ast.Ident
	var args []ast.Expr
	key := e
	switch c := e.(type) {
	case *ast.Ident:
		at = c
	case *ast.SelectorExpr:
		at, key = c.Sel, c.Sel
	case *ast.CallExpr:
		args = c.Args
		at, _ = c.Fun.(*ast.Ident)
		if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
			at = sel.Sel
		}
	}
	call := s.l.info.Calls[key]
	if call.Kind == check.CallBuiltin {
		s.builtin(at, call.Builtin, args, false)
		return
	}
	s.emitCall(e, call.Proc, args, nil)
}

// ---------------------------------------------------------------------------
// Standardroutinen

func (s *fnState) host(h *Host, dst *Var, args ...Expr) {
	s.emit(HostCall{Fn: h, Args: args, Dst: dst})
}

// builtin übersetzt eine Standardroutine. Ist wantValue gesetzt, wird ihr
// Ergebnis geliefert.
func (s *fnState) builtin(at ast.Node, b *types.Builtin, args []ast.Expr, wantValue bool) Expr {
	arg := func(i int) Expr { return s.expr(args[i]) }
	switch b.ID {
	case check.BWrite, check.BWriteLn:
		s.write(args, b.ID == check.BWriteLn)
		return nil
	case check.BInc, check.BDec:
		op := AddI
		if b.ID == check.BDec {
			op = SubI
		}
		var n Expr = IntConst{V: 1}
		if len(args) == 2 {
			n = arg(1)
		}
		t := s.typeOf(args[0])
		if v, ok := s.directVar(args[0]); ok {
			s.emit(Set{Var: v, Val: narrow(Bin{Op: op, X: Get{Var: v}, Y: n}, t)})
			return nil
		}
		a := s.temp(types.LongInt)
		s.emit(Set{Var: a, Val: s.addr(args[0])})
		_, mem, _ := scalar(t)
		s.emit(Store{Addr: Get{Var: a}, Mem: mem, Val: Bin{Op: op, X: Load{Addr: Get{Var: a}, Mem: mem}, Y: n}})
		return nil
	case check.BHalt:
		var code Expr = IntConst{V: 0}
		if len(args) == 1 {
			code = arg(0)
		}
		s.host(HostProcExit, nil, code)
		s.terminate(Unreachable{})
		return nil
	case check.BExit:
		s.jump(s.exit)
		return nil
	case check.BBreak:
		s.jump(s.breaks[len(s.breaks)-1])
		return nil
	case check.BContinue:
		s.jump(s.continues[len(s.continues)-1])
		return nil
	case check.BOrd:
		return arg(0)
	case check.BChr:
		return Bin{Op: AndI, X: arg(0), Y: IntConst{V: 255}}
	case check.BSucc, check.BPred:
		op := AddI
		if b.ID == check.BPred {
			op = SubI
		}
		return Bin{Op: op, X: arg(0), Y: IntConst{V: 1}}
	case check.BOdd:
		return Bin{Op: AndI, X: arg(0), Y: IntConst{V: 1}}
	case check.BHi:
		return Bin{Op: AndI, X: Bin{Op: ShrI, X: arg(0), Y: IntConst{V: 8}}, Y: IntConst{V: 255}}
	case check.BLo:
		return Bin{Op: AndI, X: arg(0), Y: IntConst{V: 255}}
	case check.BSwap:
		x := s.temp(types.LongInt)
		s.emit(Set{Var: x, Val: arg(0)})
		g := Get{Var: x}
		return Bin{Op: OrI,
			X: Bin{Op: AndI, X: g, Y: IntConst{V: ^0xFFFF}},
			Y: Bin{Op: OrI,
				X: Bin{Op: ShlI, X: Bin{Op: AndI, X: g, Y: IntConst{V: 255}}, Y: IntConst{V: 8}},
				Y: Bin{Op: AndI, X: Bin{Op: ShrI, X: g, Y: IntConst{V: 8}}, Y: IntConst{V: 255}}}}
	case check.BSqr:
		x := s.temp(s.typeOf(args[0]))
		s.emit(Set{Var: x, Val: arg(0)})
		if types.IsReal(s.typeOf(args[0])) {
			return Bin{Op: MulF, X: Get{Var: x}, Y: Get{Var: x}}
		}
		return Bin{Op: MulI, X: Get{Var: x}, Y: Get{Var: x}}
	case check.BAbs:
		t := s.typeOf(args[0])
		if types.IsReal(t) {
			return Un{Op: AbsF, X: arg(0)}
		}
		x := s.temp(types.LongInt)
		s.emit(Set{Var: x, Val: arg(0)})
		s.ifElse(Bin{Op: LtI, X: Get{Var: x}, Y: IntConst{V: 0}},
			func() { s.emit(Set{Var: x, Val: Bin{Op: SubI, X: IntConst{V: 0}, Y: Get{Var: x}}}) },
			func() {})
		return Get{Var: x}
	case check.BSqrt:
		return Un{Op: SqrtF, X: s.value(args[0], types.Real)}
	case check.BTrunc:
		return Un{Op: FToInt, X: s.value(args[0], types.Real)}
	case check.BRound:
		// Round rundet wie TP halbe Werte von null weg.
		x := s.temp(types.Real)
		s.emit(Set{Var: x, Val: s.value(args[0], types.Real)})
		r := s.temp(types.LongInt)
		s.ifElse(Bin{Op: LtF, X: Get{Var: x}, Y: FloatConst{V: 0}},
			func() {
				s.emit(Set{Var: r, Val: Un{Op: FToInt, X: Bin{Op: SubF, X: Get{Var: x}, Y: FloatConst{V: 0.5}}}})
			},
			func() {
				s.emit(Set{Var: r, Val: Un{Op: FToInt, X: Bin{Op: AddF, X: Get{Var: x}, Y: FloatConst{V: 0.5}}}})
			})
		return Get{Var: r}
	case check.BPi:
		return FloatConst{V: math.Pi}
	case check.BUpCase:
		c := s.temp(types.Char)
		s.emit(Set{Var: c, Val: arg(0)})
		isLower := Bin{Op: AndI,
			X: Bin{Op: GeI, X: Get{Var: c}, Y: IntConst{V: 'a'}},
			Y: Bin{Op: LeI, X: Get{Var: c}, Y: IntConst{V: 'z'}}}
		return Bin{Op: SubI, X: Get{Var: c}, Y: Bin{Op: MulI, X: isLower, Y: IntConst{V: 32}}}
	}
	unsupported(at.Pos(), b.Text()+" is supported from stage 6")
	return nil
}

// write übersetzt Write/WriteLn auf die Standardausgabe über die vorläufigen
// Hostfunktionen.
func (s *fnState) write(args []ast.Expr, ln bool) {
	for i, a := range args {
		var width, prec Expr = IntConst{V: 0}, IntConst{V: -1}
		x := a
		if f, ok := a.(*ast.FormatArg); ok {
			x = f.X
			width = s.expr(f.Width)
			if f.Prec != nil {
				prec = s.expr(f.Prec)
			}
		} else if i == 0 {
			if _, isFile := s.typeOf(a).(*types.File); isFile {
				unsupported(a.Pos(), "files are supported from stage 6")
			}
		}
		tv := s.l.info.Types[x]
		t := tv.Type
		switch {
		case tv.Mode == check.Constant && tv.Val.Kind == types.VString:
			addr := s.l.stringAddr(tv.Val.Str)
			s.host(HostWriteStr, nil, IntConst{V: int32(addr + 1)}, IntConst{V: int32(len(tv.Val.Str))}, width)
		case types.IsReal(t):
			if _, isFmt := a.(*ast.FormatArg); !isFmt {
				width = IntConst{V: -1}
			}
			s.host(HostWriteReal, nil, s.value(x, types.Real), width, prec)
		case types.Is(t, types.Char):
			s.host(HostWriteChar, nil, s.expr(x), width)
		case types.Is(t, types.Boolean):
			s.host(HostWriteBool, nil, s.expr(x), width)
		case types.IsInteger(t):
			s.host(HostWriteInt, nil, s.expr(x), width)
		default:
			unsupported(x.Pos(), "writing this type is supported from stage 5")
		}
	}
	if ln {
		s.host(HostWriteLn, nil)
	}
}
