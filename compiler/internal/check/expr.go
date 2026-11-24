package check

import (
	"math"
	"strconv"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// untypedT ist der Typ untypisierter var-/const-Parameter. Mit ihm passt keine
// Regel außer Typumwandlung und Weitergabe an untypisierte Parameter.
type untypedT struct{}

func (untypedT) String() string { return "untyped" }
func (untypedT) Size() int64    { return 0 }
func (untypedT) Align() int64   { return 1 }

var untyped types.Type = untypedT{}

// host liefert den Wirtstyp eines Ordinaltyps für Verträglichkeitsprüfungen:
// alle Ganzzahltypen werden zu LongInt, Teilbereiche zu ihrem Basistyp.
func host(t types.Type) types.Type {
	t = types.Underlying(t)
	if b, ok := t.(*types.Basic); ok && b.IsInteger() {
		return types.LongInt
	}
	return t
}

// isStringish meldet String und Char (für +, Vergleiche, Zuweisung an String).
func isStringish(t types.Type) bool { return types.IsString(t) || types.Is(t, types.Char) }

// compatible meldet die Typverträglichkeit für Operatoren (TP: "compatible").
func compatible(a, b types.Type) bool {
	if a == b {
		return true
	}
	if types.IsOrdinal(a) && types.IsOrdinal(b) {
		return host(a) == host(b)
	}
	if types.IsNumeric(a) && types.IsNumeric(b) {
		return true
	}
	if isStringish(a) && isStringish(b) {
		return true
	}
	if sa, ok := a.(*types.Set); ok {
		if sb, ok := b.(*types.Set); ok {
			return sa.Elem == nil || sb.Elem == nil || host(sa.Elem) == host(sb.Elem)
		}
	}
	return pointersCompatible(a, b)
}

func pointersCompatible(a, b types.Type) bool {
	if a == types.Nil {
		a, b = b, a
	}
	pa, ok := a.(*types.Pointer)
	if !ok {
		return false
	}
	if b == types.Nil {
		return true
	}
	pb, ok := b.(*types.Pointer)
	return ok && (pa == pb || pa.Elem == nil || pb.Elem == nil || pa.Elem == pb.Elem)
}

// assignable prüft die Zuweisungsverträglichkeit von x an den Typ dst und
// meldet sonst Error 26 bzw. 76 für Konstanten außerhalb des Wertebereichs.
func (c *checker) assignable(dst types.Type, x TypeAndValue, e ast.Expr) {
	if !c.isAssignable(dst, x) {
		c.fail(e.Pos(), source.ErrTypeMismatch, "")
	}
	if x.Mode == Constant && types.IsOrdinal(dst) && x.Val.Kind == types.VInt {
		if lo, hi := types.Bounds(dst); x.Val.Int < lo || x.Val.Int > hi {
			c.fail(e.Pos(), source.ErrConstOutOfRange, "")
		}
	}
	if x.Mode == Constant && x.Val.Kind == types.VSet {
		if s, ok := dst.(*types.Set); ok && s.Elem != nil {
			lo, hi := types.Bounds(s.Elem)
			for v := int64(0); v < 256; v++ {
				if x.Val.SetHas(v) && (v < lo || v > hi) {
					c.fail(e.Pos(), source.ErrConstOutOfRange, "")
				}
			}
		}
	}
}

func (c *checker) isAssignable(dst types.Type, x TypeAndValue) bool {
	src := x.Type
	if _, isFile := dst.(*types.File); isFile || src == untyped || dst == untyped {
		return false
	}
	if dst == src {
		return true
	}
	switch {
	case types.IsOrdinal(dst) && types.IsOrdinal(src):
		return host(dst) == host(src)
	case types.IsReal(dst):
		return types.IsNumeric(src)
	case types.IsString(dst):
		return isStringish(src)
	}
	switch d := dst.(type) {
	case *types.Array:
		// packed array[1..n] of Char := 'n Zeichen'
		return d.Packed && types.Is(d.Elem, types.Char) && x.Mode == Constant &&
			x.Val.Kind == types.VString && int64(len(x.Val.Str)) == d.Len()
	case *types.Set:
		s, ok := src.(*types.Set)
		return ok && (s.Elem == nil || d.Elem == nil || host(s.Elem) == host(d.Elem))
	case *types.Pointer:
		return pointersCompatible(d, src)
	}
	return false
}

// ---------------------------------------------------------------------------
// Einstieg

func (c *checker) record(e ast.Expr, x TypeAndValue) TypeAndValue {
	c.info.Types[e] = x
	return x
}

// expr prüft einen Ausdruck, der einen Wert liefern muss (auch Variable oder
// Konstante).
func (c *checker) expr(e ast.Expr) TypeAndValue {
	x := c.exprOrType(e)
	switch x.Mode {
	case TypeExpr:
		c.fail(e.Pos(), source.ErrInExpression, "")
	case NoValue:
		c.fail(e.Pos(), source.ErrInvalidProcRef, "")
	}
	return x
}

// constExpr prüft einen Konstantenausdruck.
func (c *checker) constExpr(e ast.Expr) TypeAndValue {
	x := c.expr(e)
	if x.Mode != Constant {
		c.fail(e.Pos(), source.ErrConstExpected, "")
	}
	return x
}

// constInt prüft einen ganzzahligen Konstantenausdruck.
func (c *checker) constInt(e ast.Expr) int64 {
	x := c.constExpr(e)
	if !types.IsInteger(x.Type) {
		c.fail(e.Pos(), source.ErrIntConstExpected, "")
	}
	return x.Val.Int
}

// exprOrType prüft einen Ausdruck, der auch ein Typ sein darf.
func (c *checker) exprOrType(e ast.Expr) TypeAndValue {
	switch e := e.(type) {
	case *ast.IntLit:
		return c.record(e, c.intLit(e))
	case *ast.RealLit:
		v, err := strconv.ParseFloat(e.Text, 64)
		if err != nil || math.IsInf(v, 0) {
			c.fail(e.Pos(), source.ErrRealConst, "")
		}
		return c.record(e, TypeAndValue{Mode: Constant, Type: types.Real, Val: types.RealVal(v)})
	case *ast.StrLit:
		return c.record(e, strConst(e.Value))
	case *ast.NilLit:
		return c.record(e, TypeAndValue{Mode: Constant, Type: types.Nil, Val: &types.Value{Kind: types.VNil}})
	case *ast.Ident:
		return c.record(e, c.ident(e))
	case *ast.SelectorExpr:
		return c.record(e, c.selector(e))
	case *ast.IndexExpr:
		return c.record(e, c.index(e))
	case *ast.DerefExpr:
		x := c.expr(e.X)
		p, ok := x.Type.(*types.Pointer)
		if !ok || p.Elem == nil {
			c.fail(e.X.Pos(), source.ErrPointerVarExpected, "")
		}
		return c.record(e, TypeAndValue{Mode: Variable, Type: p.Elem})
	case *ast.CallExpr:
		return c.record(e, c.call(e, false))
	case *ast.UnaryExpr:
		return c.record(e, c.unary(e))
	case *ast.BinaryExpr:
		return c.record(e, c.binary(e))
	case *ast.SetLit:
		return c.record(e, c.setLit(e))
	case *ast.FormatArg:
		c.fail(e.Pos(), source.ErrInvalidFormat, "")
	}
	c.fail(e.Pos(), source.ErrInExpression, "")
	return TypeAndValue{}
}

func strConst(s string) TypeAndValue {
	// Ein Literal mit genau einem Byte ist ein Char, sonst ein String (PLAN.md).
	if len(s) == 1 {
		return TypeAndValue{Mode: Constant, Type: types.Char, Val: types.IntVal(int64(s[0]))}
	}
	return TypeAndValue{Mode: Constant, Type: types.StringType, Val: types.StrVal(s)}
}

func (c *checker) intLit(e *ast.IntLit) TypeAndValue {
	if e.Text[0] == '$' {
		v, err := strconv.ParseUint(e.Text[1:], 16, 32)
		if err != nil {
			c.fail(e.Pos(), source.ErrIntConst, "")
		}
		// TP-ANNAHME: $80000000..$FFFFFFFF ergeben negative LongInt-Werte.
		return TypeAndValue{Mode: Constant, Type: types.Integer, Val: types.IntVal(int64(int32(uint32(v))))}
	}
	v, err := strconv.ParseInt(e.Text, 10, 64)
	if err != nil || v > types.MaxLongInt {
		// TP-ANNAHME: Dezimalkonstanten über MaxLongInt sind Real-Konstanten.
		f, _ := strconv.ParseFloat(e.Text, 64)
		return TypeAndValue{Mode: Constant, Type: types.Real, Val: types.RealVal(f)}
	}
	return TypeAndValue{Mode: Constant, Type: types.Integer, Val: types.IntVal(v)}
}

// ---------------------------------------------------------------------------
// Bezeichner

// withField sucht name in den Records offener with-Anweisungen (innerste zuerst).
func (c *checker) withField(id *ast.Ident) (TypeAndValue, bool) {
	for i := len(c.withs) - 1; i >= 0; i-- {
		w := c.withs[i]
		if f := w.rec.Field(id.Name); f != nil {
			c.info.Fields[id] = f
			c.info.WithRecs[id] = w.expr
			return TypeAndValue{Mode: Variable, Type: f.Type, ReadOnly: w.readOnly}, true
		}
	}
	return TypeAndValue{}, false
}

func (c *checker) ident(id *ast.Ident) TypeAndValue {
	if x, ok := c.withField(id); ok {
		return x
	}
	return c.symbolOperand(id, c.lookup(id))
}

// symbolOperand bewertet ein aufgelöstes Symbol als Ausdruck.
func (c *checker) symbolOperand(id *ast.Ident, sym types.Symbol) TypeAndValue {
	switch s := sym.(type) {
	case *types.Const:
		return TypeAndValue{Mode: Constant, Type: s.Type, Val: s.Val}
	case *types.TypeName:
		return TypeAndValue{Mode: TypeExpr, Type: s.Type}
	case *types.Var:
		t := s.Type
		if t == nil {
			t = untyped
		}
		return TypeAndValue{Mode: Variable, Type: t, ReadOnly: s.ReadOnly || c.readOnly[s]}
	case *types.Proc:
		// Funktionsaufruf ohne Klammern.
		c.info.Calls[id] = &Call{Kind: CallProc, Proc: s}
		if len(s.Sig.Params) > 0 {
			c.fail(id.Pos(), source.ErrLParenExpected, "")
		}
		if !s.IsFunc {
			return TypeAndValue{Mode: NoValue}
		}
		return TypeAndValue{Mode: Value, Type: s.Sig.Result}
	case *types.Builtin:
		c.info.Calls[id] = &Call{Kind: CallBuiltin, Builtin: s}
		return c.builtin(id, s, nil, noPos)
	case *types.Unit:
		c.fail(id.Pos(), source.ErrSyntax, "unit name %s used as value", id.Text)
	case *types.Label:
		c.fail(id.Pos(), source.ErrUnknownIdent, "%s", id.Text)
	}
	panic("check: unbekanntes Symbol")
}

// qualified löst Unit.Name auf, falls X ein Unit-Name ist.
func (c *checker) qualified(e *ast.SelectorExpr) (types.Symbol, bool) {
	id, ok := e.X.(*ast.Ident)
	if !ok {
		return nil, false
	}
	if _, isWith := c.withFieldLookup(id); isWith {
		return nil, false
	}
	sym, _ := c.scope.Lookup(id.Name)
	u, ok := sym.(*types.Unit)
	if !ok {
		return nil, false
	}
	c.info.Uses[id] = u
	s, found := u.Scope.Syms[e.Sel.Name]
	if !found {
		c.fail(e.Sel.Pos(), source.ErrUnknownIdent, "%s", e.Sel.Text)
	}
	if b, ok := s.(*types.Builtin); ok && b.Unsupported {
		c.fail(e.Sel.Pos(), source.ErrNotSupported, "%s", e.Sel.Text)
	}
	c.info.Uses[e.Sel] = s
	return s, true
}

func (c *checker) withFieldLookup(id *ast.Ident) (*types.Field, bool) {
	for i := len(c.withs) - 1; i >= 0; i-- {
		if f := c.withs[i].rec.Field(id.Name); f != nil {
			return f, true
		}
	}
	return nil, false
}

func (c *checker) selector(e *ast.SelectorExpr) TypeAndValue {
	if sym, ok := c.qualified(e); ok {
		return c.symbolOperand(e.Sel, sym)
	}
	x := c.expr(e.X)
	r, ok := x.Type.(*types.Record)
	if !ok {
		c.fail(e.X.Pos(), source.ErrRecordVarExpected, "")
	}
	f := r.Field(e.Sel.Name)
	if f == nil {
		c.fail(e.Sel.Pos(), source.ErrUnknownIdent, "%s", e.Sel.Text)
	}
	c.info.Fields[e.Sel] = f
	mode := x.Mode
	if mode == Constant {
		mode = Value
	}
	return TypeAndValue{Mode: mode, Type: f.Type, ReadOnly: x.ReadOnly}
}

func (c *checker) index(e *ast.IndexExpr) TypeAndValue {
	x := c.expr(e.X)
	t := x.Type
	for _, ie := range e.Indices {
		switch a := t.(type) {
		case *types.Array:
			i := c.expr(ie)
			c.assignable(a.Index, i, ie)
			t = a.Elem
		case *types.String:
			i := c.expr(ie)
			if !types.IsInteger(i.Type) {
				c.fail(ie.Pos(), source.ErrIntExprExpected, "")
			}
			if i.Mode == Constant && (i.Val.Int < 0 || i.Val.Int > a.Max) {
				c.fail(ie.Pos(), source.ErrConstOutOfRange, "")
			}
			t = types.Char
		default:
			c.fail(ie.Pos(), source.ErrTypeMismatch, "array or string expected")
		}
	}
	mode := x.Mode
	if mode == Constant {
		mode = Value
	}
	return TypeAndValue{Mode: mode, Type: t, ReadOnly: x.ReadOnly}
}

// ---------------------------------------------------------------------------
// Operatoren

func (c *checker) unary(e *ast.UnaryExpr) TypeAndValue {
	if e.Op == lexer.At {
		x := c.exprOrType(e.X)
		if x.Mode != Variable || x.ReadOnly {
			c.fail(e.X.Pos(), source.ErrInvalidAt, "")
		}
		return TypeAndValue{Mode: Value, Type: types.PointerType}
	}
	x := c.expr(e.X)
	res := TypeAndValue{Mode: Value, Type: x.Type}
	switch e.Op {
	case lexer.Not:
		switch {
		case types.Is(x.Type, types.Boolean):
			res.Type = types.Boolean
			if x.Mode == Constant {
				res.Mode, res.Val = Constant, types.BoolVal(x.Val.Int == 0)
			}
		case types.IsInteger(x.Type):
			res.Type = intResult(x.Type, x.Type)
			if x.Mode == Constant {
				res.Mode, res.Val = Constant, types.IntVal(^x.Val.Int)
			}
		default:
			c.fail(e.X.Pos(), source.ErrOperandTypes, "")
		}
	case lexer.Minus, lexer.Plus:
		switch {
		case types.IsInteger(x.Type):
			res.Type = intResult(x.Type, x.Type)
		case types.IsReal(x.Type):
			res.Type = types.Real
		default:
			c.fail(e.X.Pos(), source.ErrOperandTypes, "")
		}
		if x.Mode == Constant {
			res.Mode, res.Val = Constant, x.Val
			if e.Op == lexer.Minus {
				if x.Val.Kind == types.VInt {
					res.Val = c.checkInt(e, -x.Val.Int)
				} else {
					res.Val = types.RealVal(-x.Val.Real)
				}
			}
		}
	}
	return res
}

// intResult ist der Ergebnistyp ganzzahliger Arithmetik.
func intResult(a, b types.Type) types.Type {
	if types.Is(a, types.LongInt) || types.Is(b, types.LongInt) {
		return types.LongInt
	}
	return types.Integer
}

// checkInt prüft ein gefaltetes Ganzzahlergebnis auf den LongInt-Bereich.
func (c *checker) checkInt(e ast.Node, v int64) *types.Value {
	if v < types.MinLongInt || v > types.MaxLongInt {
		c.fail(e.Pos(), source.ErrConstOutOfRange, "")
	}
	return types.IntVal(v)
}

func (c *checker) binary(e *ast.BinaryExpr) TypeAndValue {
	x, y := c.expr(e.X), c.expr(e.Y)
	both := x.Mode == Constant && y.Mode == Constant
	res := TypeAndValue{Mode: Value}
	bad := func() { c.fail(e.OpPos, source.ErrOperandTypes, "") }
	at := opNode{e.OpPos}
	xs, ys := asSet(x.Type), asSet(y.Type)

	switch e.Op {
	case lexer.Plus, lexer.Minus, lexer.Star:
		switch {
		case e.Op == lexer.Plus && isStringish(x.Type) && isStringish(y.Type):
			res.Type = types.StringType
			if both {
				res.Val = types.StrVal(strValue(x) + strValue(y))
			}
		case xs != nil && ys != nil:
			if !compatible(x.Type, y.Type) {
				bad()
			}
			res.Type = mergeSet(xs, ys)
			if both {
				v := &types.Value{Kind: types.VSet}
				for i := range v.Set {
					switch e.Op {
					case lexer.Plus:
						v.Set[i] = x.Val.Set[i] | y.Val.Set[i]
					case lexer.Minus:
						v.Set[i] = x.Val.Set[i] &^ y.Val.Set[i]
					default:
						v.Set[i] = x.Val.Set[i] & y.Val.Set[i]
					}
				}
				res.Val = v
			}
		case types.IsInteger(x.Type) && types.IsInteger(y.Type):
			res.Type = intResult(x.Type, y.Type)
			if both {
				a, b := x.Val.Int, y.Val.Int
				switch e.Op {
				case lexer.Plus:
					res.Val = c.checkInt(at, a+b)
				case lexer.Minus:
					res.Val = c.checkInt(at, a-b)
				default:
					res.Val = c.checkInt(at, a*b)
				}
			}
		case types.IsNumeric(x.Type) && types.IsNumeric(y.Type):
			res.Type = types.Real
			if both {
				a, b := x.Val.AsReal(), y.Val.AsReal()
				switch e.Op {
				case lexer.Plus:
					res.Val = types.RealVal(a + b)
				case lexer.Minus:
					res.Val = types.RealVal(a - b)
				default:
					res.Val = types.RealVal(a * b)
				}
			}
		default:
			bad()
		}
	case lexer.Slash:
		if !types.IsNumeric(x.Type) || !types.IsNumeric(y.Type) {
			bad()
		}
		res.Type = types.Real
		if both {
			if y.Val.AsReal() == 0 {
				c.fail(e.OpPos, source.ErrDivisionByZero, "")
			}
			res.Val = types.RealVal(x.Val.AsReal() / y.Val.AsReal())
		}
	case lexer.Div, lexer.Mod, lexer.Shl, lexer.Shr:
		if !types.IsInteger(x.Type) || !types.IsInteger(y.Type) {
			bad()
		}
		res.Type = intResult(x.Type, y.Type)
		if both {
			a, b := x.Val.Int, y.Val.Int
			switch e.Op {
			case lexer.Div, lexer.Mod:
				if b == 0 {
					c.fail(e.OpPos, source.ErrDivisionByZero, "")
				}
				if e.Op == lexer.Div {
					res.Val = c.checkInt(at, a/b)
				} else {
					res.Val = types.IntVal(a % b)
				}
			case lexer.Shl:
				res.Val = types.IntVal(int64(int32(uint32(a) << (uint64(b) & 31))))
			default:
				res.Val = types.IntVal(int64(int32(uint32(a) >> (uint64(b) & 31))))
			}
		}
	case lexer.And, lexer.Or, lexer.Xor:
		switch {
		case types.Is(x.Type, types.Boolean) && types.Is(y.Type, types.Boolean):
			res.Type = types.Boolean
		case types.IsInteger(x.Type) && types.IsInteger(y.Type):
			res.Type = intResult(x.Type, y.Type)
		default:
			bad()
		}
		if both {
			a, b := x.Val.Int, y.Val.Int
			switch e.Op {
			case lexer.And:
				res.Val = types.IntVal(a & b)
			case lexer.Or:
				res.Val = types.IntVal(a | b)
			default:
				res.Val = types.IntVal(a ^ b)
			}
		}
	case lexer.In:
		if !types.IsOrdinal(x.Type) || ys == nil || (ys.Elem != nil && host(x.Type) != host(ys.Elem)) {
			bad()
		}
		res.Type = types.Boolean
		if both {
			res.Val = types.BoolVal(y.Val.SetHas(x.Val.Int))
		}
	case lexer.Eq, lexer.Ne, lexer.Lt, lexer.Le, lexer.Gt, lexer.Ge:
		res.Type = types.Boolean
		if !compatible(x.Type, y.Type) {
			bad()
		}
		ordered := e.Op == lexer.Lt || e.Op == lexer.Gt
		switch {
		case types.IsOrdinal(x.Type) || types.IsNumeric(x.Type) || isStringish(x.Type):
		case xs != nil:
			if ordered {
				bad()
			}
		case pointersCompatible(x.Type, y.Type):
			if e.Op != lexer.Eq && e.Op != lexer.Ne {
				bad()
			}
		default:
			bad()
		}
		if both {
			res.Val = types.BoolVal(compareConst(e.Op, x, y))
		}
	default:
		bad()
	}
	if both && res.Val != nil {
		res.Mode = Constant
	}
	return res
}

// opNode liefert die Operatorposition für Fehler in gefalteten Ausdrücken.
type opNode struct{ pos source.Pos }

func (o opNode) Pos() source.Pos { return o.pos }

func asSet(t types.Type) *types.Set { s, _ := t.(*types.Set); return s }

func mergeSet(a, b *types.Set) types.Type {
	if a.Elem == nil {
		return b
	}
	return a
}

func strValue(x TypeAndValue) string {
	if x.Val.Kind == types.VInt {
		return string([]byte{byte(x.Val.Int)})
	}
	return x.Val.Str
}

// compareConst vergleicht zwei verträgliche Konstanten.
func compareConst(op lexer.Kind, x, y TypeAndValue) bool {
	var cmp int
	switch {
	case x.Val.Kind == types.VSet:
		sub := func(a, b *types.Value) bool { // a ⊆ b
			for i := range a.Set {
				if a.Set[i]&^b.Set[i] != 0 {
					return false
				}
			}
			return true
		}
		switch op {
		case lexer.Eq:
			return x.Val.Set == y.Val.Set
		case lexer.Ne:
			return x.Val.Set != y.Val.Set
		case lexer.Le:
			return sub(x.Val, y.Val)
		default:
			return sub(y.Val, x.Val)
		}
	case x.Val.Kind == types.VNil || y.Val.Kind == types.VNil:
		cmp = 0
		if x.Val.Kind != y.Val.Kind {
			cmp = 1
		}
	case isStringish(x.Type) && isStringish(y.Type) && (x.Val.Kind == types.VString || y.Val.Kind == types.VString):
		a, b := strValue(x), strValue(y)
		switch {
		case a < b:
			cmp = -1
		case a > b:
			cmp = 1
		}
	case x.Val.Kind == types.VInt && y.Val.Kind == types.VInt:
		switch {
		case x.Val.Int < y.Val.Int:
			cmp = -1
		case x.Val.Int > y.Val.Int:
			cmp = 1
		}
	default:
		a, b := x.Val.AsReal(), y.Val.AsReal()
		switch {
		case a < b:
			cmp = -1
		case a > b:
			cmp = 1
		}
	}
	switch op {
	case lexer.Eq:
		return cmp == 0
	case lexer.Ne:
		return cmp != 0
	case lexer.Lt:
		return cmp < 0
	case lexer.Le:
		return cmp <= 0
	case lexer.Gt:
		return cmp > 0
	}
	return cmp >= 0
}

func (c *checker) setLit(e *ast.SetLit) TypeAndValue {
	res := TypeAndValue{Mode: Constant, Val: &types.Value{Kind: types.VSet}}
	var elem types.Type
	add := func(x TypeAndValue, ex ast.Expr) {
		if !types.IsOrdinal(x.Type) {
			c.fail(ex.Pos(), source.ErrOrdinalExprExpected, "")
		}
		if elem == nil {
			elem = host(x.Type)
		} else if host(x.Type) != elem {
			c.fail(ex.Pos(), source.ErrTypeMismatch, "")
		}
		if x.Mode == Constant && (x.Val.Int < 0 || x.Val.Int > 255) {
			c.fail(ex.Pos(), source.ErrConstOutOfRange, "")
		}
		if x.Mode != Constant {
			res.Mode = Value
		}
	}
	for _, el := range e.Elems {
		lo := c.expr(el.Lo)
		add(lo, el.Lo)
		if el.Hi == nil {
			if lo.Mode == Constant {
				res.Val.SetAdd(lo.Val.Int)
			}
			continue
		}
		hi := c.expr(el.Hi)
		add(hi, el.Hi)
		if lo.Mode == Constant && hi.Mode == Constant {
			for v := lo.Val.Int; v <= hi.Val.Int; v++ {
				res.Val.SetAdd(v)
			}
		}
	}
	if res.Mode != Constant {
		res.Val = nil
	}
	res.Type = &types.Set{Elem: elem}
	return res
}
