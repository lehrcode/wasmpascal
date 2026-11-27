package ir

import (
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

func add(a Expr, off int64) Expr {
	if off == 0 {
		return a
	}
	if c, ok := a.(IntConst); ok {
		return IntConst{V: c.V + int32(off)}
	}
	return Bin{Op: AddI, X: a, Y: IntConst{V: int32(off)}}
}

func memOf(t types.Type) MemType {
	_, m, _ := scalar(t)
	return m
}

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
	if !types.IsOrdinal(t) {
		return x
	}
	switch memOf(t) {
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
	case MemU16:
		if c, ok := x.(IntConst); ok {
			return IntConst{V: int32(uint16(c.V))}
		}
		return Bin{Op: AndI, X: x, Y: IntConst{V: 0xFFFF}}
	case MemI16:
		if c, ok := x.(IntConst); ok {
			return IntConst{V: int32(int16(c.V))}
		}
		return Un{Op: Ext16, X: x}
	}
	return x
}

// constant übersetzt einen skalaren Konstantenwert.
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
	}
	if p := s.l.info.ProcRefs[e]; p != nil { // Routine als Wert
		return IntConst{V: s.l.tableIndex(p)}
	}
	switch e := e.(type) {
	case *ast.Ident:
		if call := s.l.info.Calls[e]; call != nil {
			return s.call(e, call, nil)
		}
		if v, ok := s.directVar(e); ok && v.Scalar {
			return Get{Var: v}
		}
		return Load{Addr: s.designator(e), Mem: memOf(tv.Type)}
	case *ast.SelectorExpr:
		if call := s.l.info.Calls[e.Sel]; call != nil {
			return s.call(e.Sel, call, nil)
		}
		if call := s.l.info.Calls[e]; call != nil { // Funktionsvariable R.F
			return s.call(e, call, nil)
		}
		if v, ok := s.directVar(e); ok && v.Scalar {
			return Get{Var: v}
		}
		return Load{Addr: s.designator(e), Mem: memOf(tv.Type)}
	case *ast.IndexExpr, *ast.DerefExpr:
		if call := s.l.info.Calls[e]; call != nil { // parameterlose Funktionsvariable
			return s.call(e, call, nil)
		}
		return Load{Addr: s.designator(e), Mem: memOf(tv.Type)}
	case *ast.CallExpr:
		call := s.l.info.Calls[e]
		if call.Kind == check.CallTypecast {
			return s.typecast(e, tv)
		}
		return s.call(e, call, e.Args)
	case *ast.UnaryExpr:
		return s.unary(e)
	case *ast.BinaryExpr:
		return s.binary(e)
	}
	panic("ir: unerwarteter skalarer Ausdruck")
}

func (s *fnState) typecast(e *ast.CallExpr, tv check.TypeAndValue) Expr {
	from := s.typeOf(e.Args[0])
	if _, _, ok := scalar(from); !ok || tv.Mode == check.Variable && !(types.IsOrdinal(tv.Type) && types.IsOrdinal(from)) {
		// Variablenumwandlung: an derselben Adresse mit dem Zieltyp lesen
		return Load{Addr: s.designator(e.Args[0]), Mem: memOf(tv.Type)}
	}
	x := s.expr(e.Args[0])
	switch {
	case types.IsOrdinal(tv.Type):
		return narrow(x, tv.Type)
	}
	return x // Zeiger ↔ Zeiger, Zeiger ↔ LongInt
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
	// @x; @Routine und @P (Prozedurvariable) liefern den Tabellenindex
	if p := s.l.info.ProcRefs[e.X]; p != nil {
		return IntConst{V: s.l.tableIndex(p)}
	}
	if _, ok := t.(*types.ProcType); ok {
		return s.procValue(e.X)
	}
	return s.designator(e.X)
}

// procValue liefert den Wert einer Prozedurvariable (Tabellenindex), ohne
// sie aufzurufen, auch wenn der Ausdruck selbst ein Aufruf ohne Klammern ist.
func (s *fnState) procValue(e ast.Expr) Expr {
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr:
		if s.l.info.Fields[selIdent(e)] == nil {
			if v, ok := s.directVar(e); ok && v.Scalar {
				return Get{Var: v}
			}
		}
		return Load{Addr: s.place(e), Mem: MemI32}
	case *ast.IndexExpr, *ast.DerefExpr:
		return Load{Addr: s.place(e), Mem: MemI32}
	}
	return s.expr(e)
}

// selIdent liefert den Bezeichner eines Ident bzw. den Selektor eines
// SelectorExpr.
func selIdent(e ast.Expr) *ast.Ident {
	if sel, ok := e.(*ast.SelectorExpr); ok {
		return sel.Sel
	}
	id, _ := e.(*ast.Ident)
	return id
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

func isSet(t types.Type) bool { _, ok := t.(*types.Set); return ok }

func (s *fnState) binary(e *ast.BinaryExpr) Expr {
	if _, ok := isBoolOp(e, s.l.info); ok {
		return s.boolValue(e)
	}
	if e.Op == lexer.In {
		return s.inSet(e)
	}
	xt, yt := s.typeOf(e.X), s.typeOf(e.Y)
	switch {
	case isSet(xt):
		a, b := s.setAddr(e.X), s.setAddr(e.Y)
		switch e.Op {
		case lexer.Eq:
			return SetCmp{Op: SetEq, A: a, B: b}
		case lexer.Ne:
			return Un{Op: EqzI, X: SetCmp{Op: SetEq, A: a, B: b}}
		case lexer.Le:
			return SetCmp{Op: SetSub, A: a, B: b}
		default: // >=
			return SetCmp{Op: SetSub, A: b, B: a}
		}
	case types.IsString(xt) || types.IsString(yt):
		r := s.temp(types.Integer)
		s.callRtl("_StrCmp", r, s.strAddr(e.X), s.strAddr(e.Y))
		return Bin{Op: intOps[e.Op], X: Get{Var: r}, Y: IntConst{V: 0}}
	case types.IsOrdinal(xt) && types.IsOrdinal(yt) && e.Op != lexer.Slash,
		!types.IsNumeric(xt) && !types.IsNumeric(yt): // Zeiger
		x, y := s.expr(e.X), s.expr(e.Y)
		if e.Op == lexer.Div || e.Op == lexer.Mod {
			if c, ok := y.(IntConst); !ok || c.V == 0 {
				y = s.keep(y)
				s.check(Un{Op: EqzI, X: y}, ErrDivByZero)
			}
		}
		return Bin{Op: intOps[e.Op], X: x, Y: y}
	default:
		x, y := s.value(e.X, types.Real), s.value(e.Y, types.Real)
		if e.Op == lexer.Slash {
			if c, ok := y.(FloatConst); !ok || c.V == 0 {
				y = s.keep(y)
				s.check(Bin{Op: EqF, X: y, Y: FloatConst{V: 0}}, ErrDivByZero)
			}
		}
		return Bin{Op: realOps[e.Op], X: x, Y: y}
	}
}

// bitTest prüft das Element x (0..255) in der Menge an Adresse set.
func (s *fnState) bitTest(set, x Expr) Expr {
	b := Load{Addr: Bin{Op: AddI, X: set, Y: Bin{Op: ShrI, X: x, Y: IntConst{V: 3}}}, Mem: MemU8}
	return Bin{Op: AndI, X: Bin{Op: ShrI, X: b, Y: Bin{Op: AndI, X: x, Y: IntConst{V: 7}}}, Y: IntConst{V: 1}}
}

// inSet übersetzt x in Menge.
func (s *fnState) inSet(e *ast.BinaryExpr) Expr {
	x := s.keep(s.expr(e.X))
	if tv := s.l.info.Types[e.Y]; tv.Mode == check.Constant {
		// konstante Menge: Vergleich mit ihren Bereichen
		var test Expr = IntConst{V: 0}
		for v := int64(0); v < 256; v++ {
			if !tv.Val.SetHas(v) {
				continue
			}
			hi := v
			for hi+1 < 256 && tv.Val.SetHas(hi+1) {
				hi++
			}
			var t Expr = Bin{Op: EqI, X: x, Y: IntConst{V: int32(v)}}
			if hi > v {
				t = Bin{Op: AndI,
					X: Bin{Op: GeI, X: x, Y: IntConst{V: int32(v)}},
					Y: Bin{Op: LeI, X: x, Y: IntConst{V: int32(hi)}}}
			}
			test = Bin{Op: OrI, X: test, Y: t}
			v = hi
		}
		return test
	}
	set := s.keep(s.setAddr(e.Y))
	inRange := Un{Op: EqzI, X: Bin{Op: AndI, X: x, Y: IntConst{V: ^255}}}
	return Bin{Op: AndI, X: inRange, Y: s.bitTest(set, Bin{Op: AndI, X: x, Y: IntConst{V: 255}})}
}

// ---------------------------------------------------------------------------
// Adressen

// addr liefert die Adresse eines Aggregatwerts: Strings und Mengen über
// strAddr/setAddr (Ausdrücke landen in Hilfsvariablen), sonst designator.
func (s *fnState) addr(e ast.Expr) Expr {
	switch s.typeOf(e).(type) {
	case *types.String:
		return s.strAddr(e)
	case *types.Set:
		return s.setAddr(e)
	}
	return s.designator(e)
}

// designator liefert die Adresse einer Variable, eines Feldes, Elements oder
// dereferenzierten Zeigers, einer Typumwandlung einer Variable oder eines
// Aggregat-Funktionsergebnisses (in einer Hilfsvariable).
func (s *fnState) designator(e ast.Expr) Expr {
	switch e := e.(type) {
	case *ast.Ident, *ast.IndexExpr, *ast.DerefExpr:
		if call := s.l.info.Calls[e]; call != nil {
			return s.aggregateCall(call, nil)
		}
	case *ast.SelectorExpr:
		if _, isField := s.l.info.Fields[e.Sel]; !isField {
			if call := s.l.info.Calls[e.Sel]; call != nil {
				return s.aggregateCall(call, nil)
			}
		}
		if call := s.l.info.Calls[e]; call != nil {
			return s.aggregateCall(call, nil)
		}
	}
	return s.place(e)
}

// place ist designator ohne Aufrufe ohne Klammern: die Adresse der
// Variable selbst (auch einer Prozedurvariable).
func (s *fnState) place(e ast.Expr) Expr {
	switch e := e.(type) {
	case *ast.Ident:
		if f, ok := s.l.info.Fields[e]; ok { // Feld über with
			return add(Get{Var: s.withs[s.l.info.WithRecs[e]]}, f.Offset)
		}
		v, _ := s.directVar(e)
		return Addr{Var: s.addressed(v)}
	case *ast.SelectorExpr:
		if f, ok := s.l.info.Fields[e.Sel]; ok {
			return add(s.addr(e.X), f.Offset)
		}
		v, _ := s.directVar(e)
		return Addr{Var: s.addressed(v)}
	case *ast.IndexExpr:
		base := s.addr(e.X)
		t := s.typeOf(e.X)
		for _, ie := range e.Indices {
			if _, ok := t.(*types.String); ok {
				base = Bin{Op: AddI, X: base, Y: s.expr(ie)}
				t = types.Char
				continue
			}
			arr := t.(*types.Array)
			size := arr.Elem.Size()
			if tv := s.l.info.Types[ie]; tv.Mode == check.Constant {
				base = add(base, (tv.Val.Int-arr.Lo)*size)
			} else {
				i := Bin{Op: SubI, X: s.expr(ie), Y: IntConst{V: int32(arr.Lo)}}
				base = Bin{Op: AddI, X: base, Y: Bin{Op: MulI, X: i, Y: IntConst{V: int32(size)}}}
			}
			t = arr.Elem
		}
		return base
	case *ast.DerefExpr:
		p := s.keep(s.expr(e.X))
		s.check(Un{Op: EqzI, X: p}, ErrNilPointer)
		return p
	case *ast.CallExpr:
		call := s.l.info.Calls[e]
		switch call.Kind {
		case check.CallProc, check.CallIndirect:
			return s.aggregateCall(call, e.Args)
		case check.CallTypecast:
			return s.designator(e.Args[0])
		}
	}
	panic("ir: Ausdruck hat keine Adresse")
}

// addressed markiert eine skalare Variable als adressiert (sie muss dann im
// Speicher liegen).
func (s *fnState) addressed(v *Var) *Var {
	if v.Scalar && v.Storage != Global && v.Storage != Indirect {
		v.Addressed = true
	}
	return v
}

// strAddr liefert die Adresse eines String-Werts; Zeichen und Ausdrücke
// landen in Hilfsvariablen.
func (s *fnState) strAddr(e ast.Expr) Expr {
	tv := s.l.info.Types[e]
	if tv.Mode == check.Constant {
		if tv.Val.Kind == types.VString {
			return IntConst{V: int32(s.l.stringAddr(tv.Val.Str))}
		}
		if types.Is(tv.Type, types.WideChar) {
			return IntConst{V: int32(s.l.stringAddr(check.WideString(tv.Val.Int)))}
		}
		return IntConst{V: int32(s.l.stringAddr(string([]byte{byte(tv.Val.Int)})))}
	}
	if types.Is(tv.Type, types.WideChar) { // als UTF-8
		return s.rtlString("_WideCharToStr", s.expr(e))
	}
	if types.Is(tv.Type, types.Char) {
		t := s.temp(types.StringType)
		s.emit(Store{Addr: Addr{Var: t}, Mem: MemU8, Val: IntConst{V: 1}})
		s.emit(Store{Addr: Addr{Var: t}, Off: 1, Mem: MemU8, Val: s.expr(e)})
		return Addr{Var: t}
	}
	switch e := e.(type) {
	case *ast.BinaryExpr: // Verkettung
		t := s.temp(types.StringType)
		s.callRtl("_StrConcat", nil, Addr{Var: t}, s.strAddr(e.X), s.strAddr(e.Y))
		return Addr{Var: t}
	case *ast.CallExpr:
		if call := s.l.info.Calls[e]; call.Kind == check.CallBuiltin {
			switch call.Builtin.ID {
			case check.BCopy:
				t := s.temp(types.StringType)
				s.callRtl("_StrCopy", nil, Addr{Var: t}, s.strAddr(e.Args[0]), s.expr(e.Args[1]), s.expr(e.Args[2]))
				return Addr{Var: t}
			case check.BConcat:
				acc := s.strAddr(e.Args[0])
				for _, a := range e.Args[1:] {
					t := s.temp(types.StringType)
					s.callRtl("_StrConcat", nil, Addr{Var: t}, acc, s.strAddr(a))
					acc = Addr{Var: t}
				}
				return acc
			}
			return s.builtin(e, call.Builtin, e.Args) // ParamStr usw.
		}
	}
	return s.designator(e)
}

// setAddr liefert die Adresse eines Mengenwerts.
func (s *fnState) setAddr(e ast.Expr) Expr {
	tv := s.l.info.Types[e]
	if tv.Mode == check.Constant {
		return IntConst{V: int32(s.l.setConstAddr(tv.Val.Set))}
	}
	switch e := e.(type) {
	case *ast.SetLit:
		t := s.temp(tv.Type)
		dst := Addr{Var: t}
		s.emit(Fill{Dst: dst, Val: IntConst{V: 0}, N: IntConst{V: 32}})
		for _, el := range e.Elems {
			lo := s.expr(el.Lo)
			if el.Hi != nil {
				s.callRtl("_SetRange", nil, dst, lo, s.expr(el.Hi))
				continue
			}
			x := s.keep(Bin{Op: AndI, X: lo, Y: IntConst{V: 255}})
			byteAddr := s.keep(Bin{Op: AddI, X: dst, Y: Bin{Op: ShrI, X: x, Y: IntConst{V: 3}}})
			bit := Bin{Op: ShlI, X: IntConst{V: 1}, Y: Bin{Op: AndI, X: x, Y: IntConst{V: 7}}}
			s.emit(Store{Addr: byteAddr, Mem: MemU8, Val: Bin{Op: OrI, X: Load{Addr: byteAddr, Mem: MemU8}, Y: bit}})
		}
		return dst
	case *ast.BinaryExpr:
		t := s.temp(tv.Type)
		op := map[lexer.Kind]SetOpKind{lexer.Plus: SetUnion, lexer.Star: SetInter, lexer.Minus: SetDiff}[e.Op]
		s.emit(SetOp{Op: op, Dst: Addr{Var: t}, A: s.setAddr(e.X), B: s.setAddr(e.Y)})
		return Addr{Var: t}
	}
	return s.designator(e)
}

// aggregateCall ruft eine Funktion mit Aggregat-Ergebnis auf; das Ergebnis
// landet in einer Hilfsvariable, deren Adresse zurückkommt.
func (s *fnState) aggregateCall(call *check.Call, args []ast.Expr) Expr {
	tmp := s.temp(call.Result())
	s.emitCallOf(call, args, tmp)
	return Addr{Var: tmp}
}

// ---------------------------------------------------------------------------
// Aufrufe

// call übersetzt einen Aufruf in Wertposition.
func (s *fnState) call(at ast.Node, call *check.Call, args []ast.Expr) Expr {
	if call.Kind == check.CallBuiltin {
		return s.builtin(at, call.Builtin, args)
	}
	tmp := s.temp(call.Result())
	s.emitCallOf(call, args, tmp)
	return Get{Var: tmp}
}

// emitCallOf ruft eine eigene Routine direkt oder über eine
// Prozedurvariable auf.
func (s *fnState) emitCallOf(call *check.Call, args []ast.Expr, dst *Var) {
	if call.Kind == check.CallIndirect {
		s.emitIndirect(call, args, dst)
		return
	}
	s.emitCall(call.Proc, args, dst)
}

// emitIndirect erzeugt einen Aufruf über eine Prozedurvariable. Der
// Tabellenindex kommt in einen Hilfswert, damit ein Fortsetzen nach dem
// Anhalten dieselbe Routine trifft (PLAN.md, Etappe 16); nil ist
// Runtime error 216.
func (s *fnState) emitIndirect(call *check.Call, args []ast.Expr, dst *Var) {
	idx := s.temp(types.LongInt)
	s.emit(Set{Var: idx, Val: s.procValue(call.Fun)})
	s.check(Un{Op: EqzI, X: Get{Var: idx}}, ErrNilPointer)
	var in []Expr
	if r := call.Sig.Result; r != nil {
		if _, _, ok := scalar(r); !ok {
			if dst == nil {
				dst = s.temp(r)
			}
			in = append(in, Addr{Var: dst})
			dst = nil
		}
	}
	for i, prm := range call.Sig.Params {
		in = append(in, s.arg(args[i], prm.Mode, prm.Type))
	}
	s.emit(CallIndirect{Sig: s.l.funcSig(call.Sig), Index: Get{Var: idx}, Args: in, Dst: dst, Pos: s.at})
}

// emitCall erzeugt den Aufruf; dst nimmt das Ergebnis auf (bei Aggregaten
// über die versteckte Ergebnisadresse) oder ist nil.
func (s *fnState) emitCall(p *types.Proc, args []ast.Expr, dst *Var) {
	f := s.l.funcs[p]
	var in []Expr
	if f.Parent != nil {
		in = append(in, FrameBase{F: f.Parent})
	}
	if f.Result != nil && !f.Result.Scalar {
		if dst == nil {
			dst = s.temp(p.Sig.Result)
		}
		in = append(in, Addr{Var: dst})
		dst = nil
	}
	for i, prm := range p.Sig.Params {
		in = append(in, s.arg(args[i], prm.Mode, prm.Type))
	}
	s.emit(Call{Fn: f, Args: in, Dst: dst, Pos: s.at})
}

// arg übersetzt ein Argument für einen Parameter der Art mode und des Typs t.
func (s *fnState) arg(a ast.Expr, mode types.ParamMode, t types.Type) Expr {
	if mode == types.ByVar || t == nil {
		return s.place(a)
	}
	if _, _, ok := scalar(t); ok {
		return s.value(a, t)
	}
	switch t.(type) { // nach dem Parametertyp: auch ein Char wird zum String
	case *types.String:
		return s.strAddr(a)
	case *types.Set:
		return s.setAddr(a)
	}
	return s.addr(a)
}

// callRtl ruft eine Routine der System-Unit mit fertigen Argumenten auf.
func (s *fnState) callRtl(name string, dst *Var, args ...Expr) {
	s.emit(Call{Fn: s.l.rtl(name), Args: args, Dst: dst, Pos: s.at})
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
	if call == nil && key != e { // Prozedurvariable R.P als Anweisung
		call = s.l.info.Calls[e]
	}
	if call.Kind == check.CallBuiltin {
		s.builtin(at, call.Builtin, args)
		return
	}
	s.emitCallOf(call, args, nil)
}
