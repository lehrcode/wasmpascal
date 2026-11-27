package check

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// ---------------------------------------------------------------------------
// Aufrufe

// callee löst den aufgerufenen Namen auf, ohne ihn als Ausdruck zu bewerten.
func (c *checker) callee(fun ast.Expr) (types.Symbol, *ast.Ident) {
	switch f := fun.(type) {
	case *ast.Ident:
		if _, isField := c.withFieldLookup(f); isField {
			return nil, f
		}
		return c.lookup(f), f
	case *ast.SelectorExpr:
		if sym, ok := c.qualified(f); ok {
			return sym, f.Sel
		}
	}
	return nil, nil
}

func (c *checker) call(e *ast.CallExpr, stmt bool) TypeAndValue {
	sym, id := c.callee(e.Fun)
	switch s := sym.(type) {
	case *types.Proc:
		c.info.Calls[e] = &Call{Kind: CallProc, Proc: s}
		c.args(e, s.Sig.Params)
		if s.IsFunc {
			return TypeAndValue{Mode: Value, Type: s.Sig.Result}
		}
		return TypeAndValue{Mode: NoValue}
	case *types.Builtin:
		c.info.Calls[e] = &Call{Kind: CallBuiltin, Builtin: s}
		return c.builtin(id, s, e.Args, e.Rparen)
	case *types.TypeName:
		c.info.Calls[e] = &Call{Kind: CallTypecast, Type: s.Type}
		return c.typecast(e, s.Type)
	case *types.Unit, *types.Label, *types.Const:
		c.fail(e.Fun.Pos(), source.ErrInvalidProcRef, "")
	}
	// Aufruf über eine Prozedurvariable (TP 5.0)
	x := c.exprNoCall(e.Fun)
	pt, ok := x.Type.(*types.ProcType)
	if !ok || (x.Mode != Variable && x.Mode != Value) {
		c.fail(e.Fun.Pos(), source.ErrProcVarExpected, "")
	}
	c.info.Calls[e] = &Call{Kind: CallIndirect, Sig: pt.Sig, Fun: e.Fun}
	c.args(e, pt.Sig.Params)
	if pt.IsFunc {
		return TypeAndValue{Mode: Value, Type: pt.Sig.Result}
	}
	return TypeAndValue{Mode: NoValue}
}

// argCount prüft die Zahl der Argumente wie TP: zu viele ergibt `")" expected`
// beim ersten überzähligen, zu wenige `"," expected` vor der Klammer.
func (c *checker) argCount(args []ast.Expr, rparen source.Pos, lo, hi int) {
	if len(args) > hi {
		c.fail(args[hi].Pos(), source.ErrRParenExpected, "")
	}
	if len(args) < lo {
		c.fail(rparen, source.ErrCommaExpected, "")
	}
}

func (c *checker) args(e *ast.CallExpr, params []*types.Param) {
	c.argCount(e.Args, e.Rparen, len(params), len(params))
	for i, a := range e.Args {
		p := params[i]
		switch {
		case p.Mode == types.ByVar:
			x := c.varArg(a)
			if p.Type != nil && x.Type != p.Type {
				c.fail(a.Pos(), source.ErrTypeMismatch, "")
			}
		case p.Type == nil: // untypisierter const-Parameter
			x := c.expr(a)
			if x.Mode != Variable {
				c.fail(a.Pos(), source.ErrVarIdentExpected, "")
			}
		default:
			c.assignable(p.Type, c.operand(p.Type, a), a)
		}
	}
}

// varArg prüft ein Argument für einen var-Parameter: eine beschreibbare Variable.
func (c *checker) varArg(a ast.Expr) TypeAndValue {
	x := c.exprNoCall(a)
	if x.Mode != Variable {
		c.fail(a.Pos(), source.ErrVarIdentExpected, "")
	}
	if x.ReadOnly {
		c.fail(a.Pos(), source.ErrInvalidVarRef, "")
	}
	return x
}

// typecast prüft T(x): Wertumwandlung zwischen Ordinaltypen, zwischen Zeigern
// sowie zwischen Zeigern und 4-Byte-Ganzzahlen; sonst Variablenumwandlung bei
// gleicher Größe.
func (c *checker) typecast(e *ast.CallExpr, t types.Type) TypeAndValue {
	c.argCount(e.Args, e.Rparen, 1, 1)
	a := e.Args[0]
	x := c.expr(a)
	if x.Mode == Variable && (x.Type == untyped || (x.Type.Size() == t.Size() && !types.IsOrdinal(t) && !types.IsOrdinal(x.Type))) {
		return TypeAndValue{Mode: Variable, Type: t, ReadOnly: x.ReadOnly}
	}
	_, tPtr := t.(*types.Pointer)
	_, xPtr := x.Type.(*types.Pointer)
	xPtr = xPtr || x.Type == types.Nil
	// Pointer ↔ prozeduraler Typ wie Zeiger untereinander (TP 5.0: TProc(P))
	if _, ok := t.(*types.ProcType); ok {
		tPtr = true
	}
	if _, ok := x.Type.(*types.ProcType); ok {
		xPtr = true
	}
	switch {
	case types.IsOrdinal(t) && types.IsOrdinal(x.Type):
		if x.Mode == Constant {
			return TypeAndValue{Mode: Constant, Type: t, Val: types.IntVal(truncate(t, x.Val.Int))}
		}
		if x.Mode == Variable && x.Type.Size() == t.Size() {
			return TypeAndValue{Mode: Variable, Type: t, ReadOnly: x.ReadOnly}
		}
		return TypeAndValue{Mode: Value, Type: t}
	case tPtr && xPtr,
		tPtr && types.IsInteger(x.Type) && x.Type.Size() == 4,
		xPtr && types.IsInteger(t) && t.Size() == 4:
		return TypeAndValue{Mode: Value, Type: t}
	case x.Mode == Variable && x.Type.Size() == t.Size():
		return TypeAndValue{Mode: Variable, Type: t, ReadOnly: x.ReadOnly}
	}
	c.fail(e.Pos(), source.ErrInvalidTypecast, "")
	return TypeAndValue{}
}

// truncate schneidet einen Ordinalwert bei einer Typumwandlung auf die Breite
// des Zieltyps zu.
func truncate(t types.Type, v int64) int64 {
	lo, _ := types.Bounds(t)
	switch {
	case t.Size() == 1 && lo < 0:
		return int64(int8(v))
	case t.Size() == 1:
		return int64(uint8(v))
	case t.Size() == 2 && lo < 0:
		return int64(int16(v))
	case t.Size() == 2:
		return int64(uint16(v))
	}
	return int64(int32(v))
}

// ---------------------------------------------------------------------------
// Anweisungen

func (c *checker) compound(s *ast.CompoundStmt) {
	for _, st := range s.Stmts {
		c.stmt(st)
	}
}

func (c *checker) cond(e ast.Expr) {
	if x := c.expr(e); !types.Is(x.Type, types.Boolean) {
		c.fail(e.Pos(), source.ErrBoolExprExpected, "")
	}
}

func (c *checker) loopBody(s ast.Stmt) {
	c.loops++
	c.stmt(s)
	c.loops--
}

func (c *checker) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.EmptyStmt:
	case *ast.CompoundStmt:
		c.compound(s)
	case *ast.AssignStmt:
		c.assign(s)
	case *ast.CallStmt:
		// Aufrufe eigener Funktionen dürfen als Anweisung stehen, das Ergebnis
		// wird verworfen (wie {$X+} in TP 6/7 und Delphi, hier immer an). Wie
		// dort gilt das nicht für die Standardfunktionen der System-Unit.
		x := c.exprOrType(s.Call)
		if pt, ok := x.Type.(*types.ProcType); ok && (x.Mode == Variable || x.Mode == Value) && c.info.Calls[s.Call] == nil {
			// parameterlose Prozedurvariable als Anweisung: P;
			if len(pt.Sig.Params) > 0 {
				c.fail(s.End, source.ErrLParenExpected, "")
			}
			c.info.Calls[s.Call] = &Call{Kind: CallIndirect, Sig: pt.Sig, Fun: s.Call}
			c.record(s.Call, TypeAndValue{Mode: Value, Type: pt.Sig.Result})
			break
		}
		switch s.Call.(type) {
		case *ast.IndexExpr, *ast.DerefExpr:
			c.fail(s.End, source.ErrAssignExpected, "")
		}
		if x.Mode != NoValue && !c.isProcCall(s.Call) {
			c.fail(s.Call.Pos(), source.ErrInvalidVarRef, "")
		}
	case *ast.IfStmt:
		c.cond(s.Cond)
		c.stmt(s.Then)
		if s.Else != nil {
			c.stmt(s.Else)
		}
	case *ast.CaseStmt:
		x := c.expr(s.X)
		if !types.IsOrdinal(x.Type) {
			c.fail(s.X.Pos(), source.ErrOrdinalExprExpected, "")
		}
		for _, arm := range s.Arms {
			for _, l := range arm.Labels {
				for _, e := range []ast.Expr{l.Lo, l.Hi} {
					if e == nil {
						continue
					}
					k := c.constExpr(e)
					if isWide(x.Type) {
						k = c.wideLiteral(k, e)
					}
					if !types.IsOrdinal(k.Type) || !compatible(x.Type, k.Type) {
						c.fail(e.Pos(), source.ErrCaseConstType, "")
					}
				}
			}
			c.stmt(arm.Body)
		}
		for _, st := range s.Else {
			c.stmt(st)
		}
	case *ast.WhileStmt:
		c.cond(s.Cond)
		c.loopBody(s.Body)
	case *ast.RepeatStmt:
		c.loops++
		for _, st := range s.Body {
			c.stmt(st)
		}
		c.loops--
		c.cond(s.Cond)
	case *ast.ForStmt:
		v := c.forVar(s.Var)
		if !types.IsOrdinal(v.Type) {
			c.fail(s.Var.Pos(), source.ErrOrdinalVarExpected, "")
		}
		c.assignable(v.Type, c.expr(s.From), s.From)
		c.assignable(v.Type, c.expr(s.To), s.To)
		c.loopBody(s.Body)
	case *ast.ForInStmt:
		c.forIn(s)
	case *ast.WithStmt:
		n := len(c.withs)
		for _, r := range s.Records {
			x := c.expr(r)
			rec, ok := x.Type.(*types.Record)
			if !ok || x.Mode != Variable {
				c.fail(r.Pos(), source.ErrRecordVarExpected, "")
			}
			c.withs = append(c.withs, withEntry{rec: rec, expr: r, readOnly: x.ReadOnly})
		}
		c.stmt(s.Body)
		c.withs = c.withs[:n]
	case *ast.GotoStmt:
		l := c.labelSym(s.Label)
		if !l.Used {
			l.Used, l.UsedAt = true, s.Label.Pos()
		}
	case *ast.LabeledStmt:
		c.label(s.Label)
		c.stmt(s.Stmt)
	default:
		panic("check: unbekannte Anweisung")
	}
}

// isProcCall meldet, ob e ein Aufruf einer eigenen Prozedur oder Funktion ist.
func (c *checker) isProcCall(e ast.Expr) bool {
	key := e
	if sel, ok := e.(*ast.SelectorExpr); ok {
		key = sel.Sel
	}
	call := c.info.Calls[key]
	return call != nil && (call.Kind == CallProc || call.Kind == CallIndirect)
}

func (c *checker) assign(s *ast.AssignStmt) {
	var dst types.Type
	// Zuweisung an den Funktionsnamen setzt das Ergebnis.
	if id, ok := s.Lhs.(*ast.Ident); ok {
		if _, isField := c.withFieldLookup(id); !isField {
			if p, ok := c.lookup(id).(*types.Proc); ok && p.IsFunc && c.enclosed[p] {
				dst = p.Sig.Result
				c.record(id, TypeAndValue{Mode: Variable, Type: dst})
			}
		}
	}
	if dst == nil {
		x := c.exprOrType(s.Lhs)
		switch {
		case x.Mode != Variable:
			c.fail(s.Lhs.Pos(), source.ErrVarIdentExpected, "")
		case x.ReadOnly:
			c.fail(s.Lhs.Pos(), source.ErrIllegalAssignment, "")
		}
		dst = x.Type
	}
	c.assignable(dst, c.operand(dst, s.Rhs), s.Rhs)
}

// forVar prüft die Laufvariable: eine einfache, beschreibbare Variable des
// aktuellen Blocks.
func (c *checker) forVar(id *ast.Ident) *types.Var {
	if _, isField := c.withFieldLookup(id); isField {
		c.fail(id.Pos(), source.ErrInvalidForVar, "")
	}
	v, ok := c.lookup(id).(*types.Var)
	if !ok || v.ReadOnly || c.readOnly[v] || v.Owner != c.proc ||
		(v.Kind != types.LocalVar && v.Kind != types.GlobalVar && !(v.Kind == types.ParamVar && v.Mode == types.ByValue)) {
		c.fail(id.Pos(), source.ErrInvalidForVar, "")
	}
	c.record(id, TypeAndValue{Mode: Variable, Type: v.Type})
	return v
}

func (c *checker) forIn(s *ast.ForInStmt) {
	v := c.forVar(s.Var)
	x := c.expr(s.In)
	var elem types.Type
	switch t := x.Type.(type) {
	case *types.Array:
		elem = t.Elem
	case *types.String:
		// Mit WideChar-Laufvariable läuft die Schleife über die Zeichen
		// (UTF-8 dekodiert), sonst über die Bytes.
		elem = types.Char
		if isWide(v.Type) {
			elem = types.WideChar
		}
	case *types.Set:
		elem = t.Elem
		if elem == nil {
			elem = v.Type
		}
	default:
		switch {
		case types.Is(x.Type, types.Char):
			elem = types.Char
		case types.Is(x.Type, types.WideChar):
			elem = types.WideChar
		default:
			c.fail(s.In.Pos(), source.ErrTypeMismatch, "array, string or set expected")
		}
	}
	if !c.isAssignable(v.Type, TypeAndValue{Mode: Value, Type: elem}) {
		c.fail(s.Var.Pos(), source.ErrTypeMismatch, "")
	}
	if c.readOnly == nil {
		c.readOnly = map[*types.Var]bool{}
	}
	c.readOnly[v] = true
	c.loopBody(s.Body)
	delete(c.readOnly, v)
}

// labelSym löst eine Marke auf; sie muss im aktuellen Block deklariert sein
// (TP erlaubt keine Sprünge aus einer Prozedur heraus).
func (c *checker) labelSym(id *ast.Ident) *types.Label {
	sym, sc := c.scope.Lookup(id.Name)
	l, ok := sym.(*types.Label)
	switch {
	case !ok:
		c.fail(id.Pos(), source.ErrUnknownIdent, "%s", id.Text)
	case sc != c.scope:
		c.fail(id.Pos(), source.ErrLabelNotInBlock, "")
	}
	c.info.Uses[id] = l
	return l
}

func (c *checker) label(id *ast.Ident) {
	l := c.labelSym(id)
	if l.Defined {
		c.fail(id.Pos(), source.ErrLabelDefined, "")
	}
	l.Defined = true
}
