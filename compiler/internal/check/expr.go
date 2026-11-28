package check

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

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
// alle Ganzzahltypen außer Int64 werden zu LongInt, Teilbereiche zu ihrem
// Basistyp. Int64 bleibt Int64, damit in 64 Bit gerechnet wird.
func host(t types.Type) types.Type {
	t = types.Underlying(t)
	if b, ok := t.(*types.Basic); ok && b.IsInteger() && b.Kind != types.KInt64 {
		return types.LongInt
	}
	return t
}

// isStringish meldet String, Char und WideChar (für +, Vergleiche, Zuweisung
// an String und alle Standardroutinen mit String-Argument; ein WideChar wird
// dort als UTF-8 eingesetzt).
func isStringish(t types.Type) bool {
	return types.IsString(t) || types.Is(t, types.Char) || types.Is(t, types.WideChar)
}

// charPair meldet, ob a und b (als Wirtstypen) Char und WideChar sind; sie
// sind ordinal verträglich, Char wird dabei erweitert.
func charPair(a, b types.Type) bool {
	a, b = host(a), host(b)
	return (a == types.Char && b == types.WideChar) || (a == types.WideChar && b == types.Char)
}

// isWide meldet WideChar und Teilbereiche davon.
func isWide(t types.Type) bool { return host(t) == types.WideChar }

// wideLiteral wandelt eine String-Konstante mit genau einem Zeichen in eine
// WideChar-Konstante, wenn ein WideChar erwartet wird (Zuweisung, Parameter,
// Vergleich, case-Marke, Teilbereichsgrenze). Ein Zeichen ab U+10000 ergibt
// Error 76. Ohne passende Konstante bleibt x unverändert.
func (c *checker) wideLiteral(x TypeAndValue, e ast.Expr) TypeAndValue {
	if x.Mode != Constant || !types.IsString(x.Type) || x.Val.Kind != types.VString {
		return x
	}
	s := x.Val.Str
	r, size := utf8.DecodeRuneInString(s)
	if size != len(s) || size < 2 || r == utf8.RuneError {
		return x
	}
	if r > 0xFFFF {
		c.fail(e.Pos(), source.ErrConstOutOfRange, "")
	}
	return c.record(e, TypeAndValue{Mode: Constant, Type: types.WideChar, Val: types.IntVal(int64(r))})
}

// compatible meldet die Typverträglichkeit für Operatoren (TP: "compatible").
func compatible(a, b types.Type) bool {
	if a == b {
		return true
	}
	if types.IsInteger(a) && types.IsInteger(b) {
		return true
	}
	if types.IsOrdinal(a) && types.IsOrdinal(b) {
		return host(a) == host(b) || charPair(a, b)
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
	if isWide(dst) {
		x = c.wideLiteral(x, e)
	}
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
	case types.IsInteger(dst) && types.IsInteger(src):
		// auch Int64 ↔ kleinere Ganzzahlen (abgeschnitten wie {$R-})
		return true
	case types.IsInt64(dst) && types.Is(src, types.Comp):
		return true // Comp → Int64 über den Ganzzahlwert (Etappe 17)
	case types.IsOrdinal(dst) && types.IsOrdinal(src):
		// Char → WideChar implizit, umgekehrt nur über Char(w)
		return host(dst) == host(src) || (isWide(dst) && host(src) == types.Char)
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
	case *types.ProcType:
		if src == types.Nil {
			return true
		}
		sp, ok := src.(*types.ProcType)
		return ok && sp.IsFunc == d.IsFunc && types.SameProcSig(d.Sig, sp.Sig)
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
// Konstante). Eine parameterlose Funktionsvariable wird dabei aufgerufen,
// wie in TP 5.0; wo der prozedurale Wert selbst gemeint ist, gilt
// exprNoCall bzw. operand.
func (c *checker) expr(e ast.Expr) TypeAndValue {
	return c.autoCall(e, c.exprNoCall(e))
}

// exprNoCall ist expr ohne den Aufruf parameterloser Funktionsvariablen.
func (c *checker) exprNoCall(e ast.Expr) TypeAndValue {
	x := c.exprOrType(e)
	switch x.Mode {
	case TypeExpr:
		c.fail(e.Pos(), source.ErrInExpression, "")
	case NoValue:
		c.fail(e.Pos(), source.ErrInvalidProcRef, "")
	}
	return x
}

// autoCall macht aus einer parameterlosen Funktionsvariable einen Aufruf.
func (c *checker) autoCall(e ast.Expr, x TypeAndValue) TypeAndValue {
	pt, ok := x.Type.(*types.ProcType)
	if !ok || !pt.IsFunc || len(pt.Sig.Params) > 0 || c.info.ProcRefs[e] != nil {
		return x
	}
	// Schon ein anderer Aufruf (Funktion mit prozeduralem Ergebnis)? Ein
	// früherer autoCall desselben Knotens zählt nicht: Manche Argumente
	// werden zweimal geprüft (Write erkennt erst die Datei).
	if call := c.info.Calls[e]; call != nil && (call.Kind != CallIndirect || call.Fun != e) {
		return x
	}
	c.info.Calls[e] = &Call{Kind: CallIndirect, Sig: pt.Sig, Fun: e}
	return c.record(e, TypeAndValue{Mode: Value, Type: pt.Sig.Result})
}

// operand prüft e als Wert für ein Ziel vom Typ dst. Ist dst prozedural,
// bezeichnet ein Routinenname die Routine selbst und eine Prozedurvariable
// ihren Wert (kein Aufruf).
func (c *checker) operand(dst types.Type, e ast.Expr) TypeAndValue {
	if _, ok := dst.(*types.ProcType); !ok {
		return c.expr(e)
	}
	if p, ok := c.procRef(e); ok {
		return c.procRefValue(e, p)
	}
	return c.exprNoCall(e)
}

// procRef meldet, ob e eine Routine als Wert bezeichnet (Bezeichner oder
// Unit.Name). Erlaubt sind nur Routinen der obersten Ebene; verschachtelte
// Routinen und Standardroutinen ergeben Error 143, wie in TP.
func (c *checker) procRef(e ast.Expr) (*types.Proc, bool) {
	var sym types.Symbol
	var id *ast.Ident
	switch e := e.(type) {
	case *ast.Ident:
		if _, isField := c.withFieldLookup(e); isField {
			return nil, false
		}
		sym, _ = c.scope.Lookup(e.Name)
		id = e
	case *ast.SelectorExpr:
		s, ok := c.qualified(e)
		if !ok {
			return nil, false
		}
		sym, id = s, e.Sel
	default:
		return nil, false
	}
	switch s := sym.(type) {
	case *types.Proc:
		if s.Level > 1 {
			c.fail(id.Pos(), source.ErrInvalidProcRef, "")
		}
		c.info.Uses[id] = s
		c.info.ProcRefs[e] = s
		return s, true
	case *types.Builtin:
		c.fail(id.Pos(), source.ErrInvalidProcRef, "")
	}
	return nil, false
}

// procRefValue ist der Wert einer Routine als prozeduraler Typ.
func (c *checker) procRefValue(e ast.Expr, p *types.Proc) TypeAndValue {
	return c.record(e, TypeAndValue{Mode: Value, Type: &types.ProcType{Sig: p.Sig, IsFunc: p.IsFunc}})
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
		if e.CodePoint != 0 { // genau ein #n über 255
			return c.record(e, TypeAndValue{Mode: Constant, Type: types.WideChar, Val: types.IntVal(int64(e.CodePoint))})
		}
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
		digits := strings.TrimLeft(e.Text[1:], "0")
		if len(digits) > 8 {
			// 9 bis 16 Ziffern: Int64 (Etappe 17), $FFFFFFFFFFFFFFFF = -1
			v, err := strconv.ParseUint(e.Text[1:], 16, 64)
			if err != nil {
				c.fail(e.Pos(), source.ErrIntConst, "")
			}
			return TypeAndValue{Mode: Constant, Type: types.Int64, Val: types.IntVal(int64(v))}
		}
		v, err := strconv.ParseUint(e.Text[1:], 16, 32)
		if err != nil {
			c.fail(e.Pos(), source.ErrIntConst, "")
		}
		// TP-ANNAHME: $80000000..$FFFFFFFF ergeben negative LongInt-Werte.
		return TypeAndValue{Mode: Constant, Type: types.Integer, Val: types.IntVal(int64(int32(uint32(v))))}
	}
	v, err := strconv.ParseInt(e.Text, 10, 64)
	if err != nil {
		// TP-ANNAHME: Dezimalkonstanten über High(Int64) sind Real-Konstanten.
		f, _ := strconv.ParseFloat(e.Text, 64)
		return TypeAndValue{Mode: Constant, Type: types.Real, Val: types.RealVal(f)}
	}
	if v > types.MaxLongInt {
		return TypeAndValue{Mode: Constant, Type: types.Int64, Val: types.IntVal(v)}
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
	if !found && u.Name() == "system" && c.info.SystemIntf != nil {
		s, found = c.info.SystemIntf.Syms[e.Sel.Name]
	}
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
		// @Routine: Zeiger auf die Routine; @P mit einer Prozedurvariable:
		// ihr Wert als Pointer (TP 5.0)
		if p, ok := c.procRef(e.X); ok {
			c.procRefValue(e.X, p)
			return TypeAndValue{Mode: Value, Type: types.PointerType}
		}
		x := c.exprOrType(e.X)
		if _, isProc := x.Type.(*types.ProcType); isProc && (x.Mode == Variable || x.Mode == Value) {
			return TypeAndValue{Mode: Value, Type: types.PointerType}
		}
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
					res.Val = c.foldInt(e, res.Type, lexer.Minus, 0, x.Val.Int)
				} else {
					res.Val = types.RealVal(-x.Val.Real)
				}
			}
		}
	}
	return res
}

// intResult ist der Ergebnistyp ganzzahliger Arithmetik: Int64, wenn ein
// Operand Int64 ist, sonst wie bisher 32 Bit.
func intResult(a, b types.Type) types.Type {
	switch {
	case types.IsInt64(a) || types.IsInt64(b):
		return types.Int64
	case types.Is(a, types.LongInt) || types.Is(b, types.LongInt):
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

// foldInt faltet a op b für den Ergebnistyp t: bei Int64 in 64 Bit mit
// Überlaufprüfung, sonst im LongInt-Bereich (Error 76).
func (c *checker) foldInt(e ast.Node, t types.Type, op lexer.Kind, a, b int64) *types.Value {
	if !types.IsInt64(t) {
		switch op {
		case lexer.Plus:
			return c.checkInt(e, a+b)
		case lexer.Minus:
			return c.checkInt(e, a-b)
		}
		return c.checkInt(e, a*b)
	}
	x, y := big.NewInt(a), big.NewInt(b)
	switch op {
	case lexer.Plus:
		x.Add(x, y)
	case lexer.Minus:
		x.Sub(x, y)
	default:
		x.Mul(x, y)
	}
	if !x.IsInt64() {
		c.fail(e.Pos(), source.ErrConstOutOfRange, "")
	}
	return types.IntVal(x.Int64())
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
				res.Val = c.foldInt(at, res.Type, e.Op, x.Val.Int, y.Val.Int)
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
				switch {
				case e.Op == lexer.Div && types.IsInt64(res.Type):
					if a == math.MinInt64 && b == -1 {
						c.fail(e.OpPos, source.ErrConstOutOfRange, "")
					}
					res.Val = types.IntVal(a / b)
				case e.Op == lexer.Div:
					res.Val = c.checkInt(at, a/b)
				case b == -1:
					res.Val = types.IntVal(0) // a mod -1, auch für Low(Int64)
				default:
					res.Val = types.IntVal(a % b)
				}
			case lexer.Shl:
				if types.IsInt64(res.Type) {
					res.Val = types.IntVal(int64(uint64(a) << (uint64(b) & 63)))
				} else {
					res.Val = types.IntVal(int64(int32(uint32(a) << (uint64(b) & 31))))
				}
			default:
				if types.IsInt64(res.Type) {
					res.Val = types.IntVal(int64(uint64(a) >> (uint64(b) & 63)))
				} else {
					res.Val = types.IntVal(int64(int32(uint32(a) >> (uint64(b) & 31))))
				}
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
		if !types.IsOrdinal(x.Type) || ys == nil || (ys.Elem != nil && host(x.Type) != host(ys.Elem) && !charPair(x.Type, ys.Elem)) {
			bad()
		}
		res.Type = types.Boolean
		if both {
			res.Val = types.BoolVal(y.Val.SetHas(x.Val.Int))
		}
	case lexer.Eq, lexer.Ne, lexer.Lt, lexer.Le, lexer.Gt, lexer.Ge:
		res.Type = types.Boolean
		// Ein Literal mit einem Zeichen gegenüber einem WideChar wird selbst
		// WideChar; verglichen wird dann ordinal (PLAN.md, Etappe 14).
		if isWide(x.Type) {
			y = c.wideLiteral(y, e.Y)
		} else if isWide(y.Type) {
			x = c.wideLiteral(x, e.X)
		}
		both = x.Mode == Constant && y.Mode == Constant
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
		if types.Is(x.Type, types.WideChar) {
			return WideString(x.Val.Int)
		}
		return string([]byte{byte(x.Val.Int)})
	}
	return x.Val.Str
}

// WideString liefert einen WideChar-Wert als UTF-8; Surrogate werden wie zur
// Laufzeit zu U+FFFD, damit nie ungültiges UTF-8 entsteht.
func WideString(v int64) string {
	r := rune(v & 0xFFFF)
	if r >= 0xD800 && r <= 0xDFFF {
		r = utf8.RuneError
	}
	return string(r)
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
