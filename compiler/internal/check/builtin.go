package check

import (
	"strings"

	"github.com/lehrcode/wasmpascal/compiler/internal/abi"
	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// builtin prüft den Aufruf einer Standardprozedur oder -funktion. args ist nil
// bei einem Aufruf ohne Klammern; rparen ist dann leer.
func (c *checker) builtin(id *ast.Ident, b *types.Builtin, args []ast.Expr, rparen source.Pos) TypeAndValue {
	noParens := args == nil
	count := func(lo, hi int) {
		if noParens && lo > 0 {
			c.fail(id.Pos(), source.ErrLParenExpected, "")
		}
		c.argCount(args, rparen, lo, hi)
	}
	val := func(t types.Type) TypeAndValue { return TypeAndValue{Mode: Value, Type: t} }
	none := TypeAndValue{Mode: NoValue}

	switch b.ID {
	// ---------------------------------------------------------------- E/A
	case BWrite, BWriteLn:
		c.write(args, b.ID == BWriteLn)
		return none
	case BRead, BReadLn:
		c.read(args, b.ID == BReadLn)
		return none

	// ---------------------------------------------------------------- Ordinal
	case BInc, BDec:
		count(1, 2)
		x := c.varArg(args[0])
		if !types.IsOrdinal(x.Type) {
			c.fail(args[0].Pos(), source.ErrOrdinalVarExpected, "")
		}
		if len(args) == 2 {
			c.intExpr(args[1])
		}
		return none

	// ---------------------------------------------------------------- Heap
	case BNew, BDispose:
		count(1, 1)
		x := c.varArg(args[0])
		if p, ok := x.Type.(*types.Pointer); !ok || p.Elem == nil {
			c.fail(args[0].Pos(), source.ErrPointerVarExpected, "")
		}
		return none
	case BGetMem, BFreeMem:
		count(2, 2)
		c.pointerVar(args[0])
		c.intExpr(args[1])
		return none
	case BMark, BRelease:
		count(1, 1)
		c.pointerVar(args[0])
		return none

	// ---------------------------------------------------------------- Ablauf
	case BHalt:
		count(0, 1)
		if len(args) == 1 {
			c.intExpr(args[0])
		}
		return none
	case BExit:
		count(0, 0)
		return none
	case BBreak, BContinue:
		count(0, 0)
		if c.loops == 0 {
			c.fail(id.Pos(), source.ErrBreakOutside, "")
		}
		return none
	case BRandomize:
		count(0, 0)
		return none

	// ---------------------------------------------------------------- Dateien
	case BAssign, BRename:
		count(2, 2)
		c.fileVar(args[0])
		c.stringExpr(args[1])
		return none
	case BReset, BRewrite:
		count(1, 2)
		f := c.fileVar(args[0])
		if len(args) == 2 {
			if f.Text || f.Elem != nil {
				c.fail(args[1].Pos(), source.ErrRParenExpected, "")
			}
			c.intExpr(args[1])
		}
		return none
	case BAppend, BFlush:
		count(1, 1)
		c.textVar(args[0])
		return none
	case BClose, BErase:
		count(1, 1)
		c.fileVar(args[0])
		return none
	case BTruncate:
		count(1, 1)
		c.binaryFileVar(args[0])
		return none
	case BSeek:
		count(2, 2)
		c.binaryFileVar(args[0])
		c.intExpr(args[1])
		return none
	case BBlockRead, BBlockWrite:
		count(3, 4)
		if f := c.fileVar(args[0]); f.Text || f.Elem != nil {
			c.fail(args[0].Pos(), source.ErrInvalidFileType, "")
		}
		if b.ID == BBlockRead {
			c.varArg(args[1])
		} else if x := c.expr(args[1]); x.Mode != Variable {
			c.fail(args[1].Pos(), source.ErrVarIdentExpected, "")
		}
		c.intExpr(args[2])
		if len(args) == 4 {
			c.intVar(args[3])
		}
		return none
	case BChDir, BMkDir, BRmDir:
		count(1, 1)
		c.stringExpr(args[0])
		return none
	case BGetDir:
		count(2, 2)
		c.intExpr(args[0])
		c.stringVar(args[1])
		return none

	// ---------------------------------------------------------------- Strings
	case BDelete:
		count(3, 3)
		c.stringVar(args[0])
		c.intExpr(args[1])
		c.intExpr(args[2])
		return none
	case BInsert:
		count(3, 3)
		c.stringExpr(args[0])
		c.stringVar(args[1])
		c.intExpr(args[2])
		return none
	case BStr:
		count(2, 2)
		c.formatted(args[0], func(x TypeAndValue, e ast.Expr) {
			if !types.IsNumeric(x.Type) {
				c.fail(e.Pos(), source.ErrNumExprExpected, "")
			}
		})
		c.stringVar(args[1])
		return none
	case BVal:
		count(3, 3)
		c.stringExpr(args[0])
		if x := c.varArg(args[1]); !types.IsNumeric(x.Type) {
			c.fail(args[1].Pos(), source.ErrNumExprExpected, "")
		}
		c.intVar(args[2])
		return none

	// ---------------------------------------------------------------- Speicher
	case BFillChar:
		count(3, 3)
		c.varArg(args[0])
		c.intExpr(args[1])
		if x := c.expr(args[2]); !types.IsOrdinal(x.Type) {
			c.fail(args[2].Pos(), source.ErrOrdinalExprExpected, "")
		}
		return none
	case BMove:
		count(3, 3)
		if x := c.expr(args[0]); x.Mode != Variable {
			c.fail(args[0].Pos(), source.ErrVarIdentExpected, "")
		}
		c.varArg(args[1])
		c.intExpr(args[2])
		return none

	// ---------------------------------------------------------------- Arithmetik
	case BAbs, BSqr:
		count(1, 1)
		x := c.numExpr(args[0])
		if types.IsInteger(x.Type) {
			res := val(intResult(x.Type, x.Type))
			if b.ID == BAbs && x.Mode == Constant {
				v := x.Val.Int
				if v < 0 {
					v = -v
				}
				res.Mode, res.Val = Constant, c.checkInt(args[0], v)
			}
			return res
		}
		if b.ID == BAbs && x.Mode == Constant {
			v := x.Val.Real
			if v < 0 {
				v = -v
			}
			return TypeAndValue{Mode: Constant, Type: types.Real, Val: types.RealVal(v)}
		}
		return val(types.Real)
	case BArcTan, BCos, BSin, BExp, BLn, BSqrt, BFrac, BInt:
		count(1, 1)
		c.numExpr(args[0])
		return val(types.Real)
	case BTrunc, BRound:
		count(1, 1)
		x := c.numExpr(args[0])
		if x.Mode == Constant {
			v := x.Val.AsReal()
			if b.ID == BRound {
				// Round rundet wie TP kaufmännisch (0.5 von null weg).
				if v < 0 {
					v -= 0.5
				} else {
					v += 0.5
				}
			}
			return TypeAndValue{Mode: Constant, Type: types.LongInt, Val: c.checkInt(args[0], int64(v))}
		}
		return val(types.LongInt)
	case BRandom:
		count(0, 1)
		if len(args) == 1 {
			c.intExpr(args[0])
			return val(types.Integer)
		}
		return val(types.Real)
	case BPi:
		count(0, 0)
		return val(types.Real)

	// ---------------------------------------------------------------- Ordinal (Funktionen)
	case BChr:
		count(1, 1)
		x := c.intExpr(args[0])
		if x.Mode == Constant {
			if x.Val.Int < 0 || x.Val.Int > 255 {
				c.fail(args[0].Pos(), source.ErrConstOutOfRange, "")
			}
			return TypeAndValue{Mode: Constant, Type: types.Char, Val: x.Val}
		}
		return val(types.Char)
	case BOrd:
		count(1, 1)
		x := c.ordExpr(args[0])
		if x.Mode == Constant {
			return TypeAndValue{Mode: Constant, Type: types.LongInt, Val: x.Val}
		}
		return val(types.LongInt)
	case BOdd:
		count(1, 1)
		x := c.intExpr(args[0])
		if x.Mode == Constant {
			return TypeAndValue{Mode: Constant, Type: types.Boolean, Val: types.BoolVal(x.Val.Int%2 != 0)}
		}
		return val(types.Boolean)
	case BSucc, BPred:
		count(1, 1)
		x := c.ordExpr(args[0])
		if x.Mode == Constant {
			v := x.Val.Int + 1
			if b.ID == BPred {
				v = x.Val.Int - 1
			}
			if lo, hi := types.Bounds(x.Type); v < lo || v > hi {
				c.fail(args[0].Pos(), source.ErrConstOutOfRange, "")
			}
			return TypeAndValue{Mode: Constant, Type: x.Type, Val: types.IntVal(v)}
		}
		return val(x.Type)
	case BHi, BLo:
		count(1, 1)
		x := c.intExpr(args[0])
		if x.Mode == Constant {
			v := x.Val.Int & 255
			if b.ID == BHi {
				v = (x.Val.Int >> 8) & 255
			}
			return TypeAndValue{Mode: Constant, Type: types.Byte, Val: types.IntVal(v)}
		}
		return val(types.Byte)
	case BSwap:
		count(1, 1)
		x := c.intExpr(args[0])
		t := intResult(x.Type, x.Type)
		if x.Mode == Constant {
			// TP-ANNAHME: Swap tauscht die beiden unteren Bytes, wie bei 16 Bit.
			v := x.Val.Int
			v = v&^0xFFFF | (v&0xFF)<<8 | (v>>8)&0xFF
			return TypeAndValue{Mode: Constant, Type: t, Val: types.IntVal(int64(int32(v)))}
		}
		return val(t)
	case BHigh, BLow:
		count(1, 1)
		return c.lowHigh(args[0], b.ID == BHigh)
	case BSizeOf:
		count(1, 1)
		x := c.exprOrType(args[0])
		if x.Type == untyped || x.Mode == NoValue {
			c.fail(args[0].Pos(), source.ErrVarIdentExpected, "")
		}
		return TypeAndValue{Mode: Constant, Type: types.Integer, Val: types.IntVal(x.Type.Size())}
	case BUpCase:
		count(1, 1)
		x := c.expr(args[0])
		if !types.Is(x.Type, types.Char) {
			c.fail(args[0].Pos(), source.ErrTypeMismatch, "")
		}
		if x.Mode == Constant {
			v := x.Val.Int
			if v >= 'a' && v <= 'z' {
				v -= 32
			}
			return TypeAndValue{Mode: Constant, Type: types.Char, Val: types.IntVal(v)}
		}
		return val(types.Char)

	// ---------------------------------------------------------------- Strings (Funktionen)
	case BLength:
		count(1, 1)
		x := c.stringExpr(args[0])
		if x.Mode == Constant {
			return TypeAndValue{Mode: Constant, Type: types.Integer, Val: types.IntVal(int64(len(strValue(x))))}
		}
		return val(types.Integer)
	case BCopy:
		count(3, 3)
		c.stringExpr(args[0])
		c.intExpr(args[1])
		c.intExpr(args[2])
		return val(types.StringType)
	case BConcat:
		if noParens || len(args) < 1 {
			count(1, 1)
		}
		var sb strings.Builder
		allConst := true
		for _, a := range args {
			x := c.stringExpr(a)
			if x.Mode == Constant {
				sb.WriteString(strValue(x))
			} else {
				allConst = false
			}
		}
		if allConst {
			return TypeAndValue{Mode: Constant, Type: types.StringType, Val: types.StrVal(sb.String())}
		}
		return val(types.StringType)
	case BPos:
		count(2, 2)
		c.stringExpr(args[0])
		c.stringExpr(args[1])
		return val(types.Integer)
	case BParamStr:
		count(1, 1)
		c.intExpr(args[0])
		return val(types.StringType)

	// ---------------------------------------------------------------- Dateien (Funktionen)
	case BEof:
		count(0, 1)
		if len(args) == 1 {
			c.fileVar(args[0])
		}
		return val(types.Boolean)
	case BEoln, BSeekEof, BSeekEoln:
		count(0, 1)
		if len(args) == 1 {
			c.textVar(args[0])
		}
		return val(types.Boolean)
	case BFilePos, BFileSize:
		count(1, 1)
		c.binaryFileVar(args[0])
		return val(types.LongInt)
	case BIOResult, BParamCount:
		count(0, 0)
		return val(types.Integer)
	case BMemAvail, BMaxAvail:
		count(0, 0)
		return val(types.LongInt)

	// ---------------------------------------------------------------- Intrinsics
	case BMemGrow:
		count(1, 1)
		c.intExpr(args[0])
		return val(types.LongInt)
	case BMemSize, BHeapBase:
		count(0, 0)
		return val(types.LongInt)
	case BRunError:
		count(1, 1)
		c.intExpr(args[0])
		return none
	case BGoSetSP:
		count(1, 1)
		c.intExpr(args[0])
		return none
	case BGoArgc, BGoArgv:
		count(0, 0)
		return val(types.LongInt)
	case BSuspend:
		count(2, 2)
		c.intExpr(args[0])
		c.intExpr(args[1])
		return val(types.LongInt)

	// ---------------------------------------------------------------- Adressen
	case BPtr:
		count(2, 2)
		c.intExpr(args[0])
		c.intExpr(args[1])
		return val(types.PointerType)
	case BAddr, BOfs, BSeg:
		count(1, 1)
		x := c.expr(args[0])
		if x.Mode != Variable {
			c.fail(args[0].Pos(), source.ErrVarIdentExpected, "")
		}
		switch b.ID {
		case BAddr:
			if x.ReadOnly {
				c.fail(args[0].Pos(), source.ErrInvalidAt, "")
			}
			return val(types.PointerType)
		case BOfs:
			return val(types.LongInt)
		}
		// Seg ist immer 0 (PLAN.md, Adressfunktionen).
		return TypeAndValue{Mode: Constant, Type: types.Word, Val: types.IntVal(0)}
	}
	if b.ID >= BWasiBase {
		w := abi.Intrinsics[b.ID-BWasiBase]
		count(len(w.Params), len(w.Params))
		for _, a := range args {
			c.intExpr(a)
		}
		if w.Result {
			return val(types.LongInt)
		}
		return none
	}
	panic("check: unbekannte Standardroutine " + b.Text())
}

// ---------------------------------------------------------------------------
// Argumentprüfungen

func (c *checker) intExpr(e ast.Expr) TypeAndValue {
	x := c.expr(e)
	if !types.IsInteger(x.Type) {
		c.fail(e.Pos(), source.ErrIntExprExpected, "")
	}
	return x
}

func (c *checker) numExpr(e ast.Expr) TypeAndValue {
	x := c.expr(e)
	if !types.IsNumeric(x.Type) {
		c.fail(e.Pos(), source.ErrNumExprExpected, "")
	}
	return x
}

func (c *checker) ordExpr(e ast.Expr) TypeAndValue {
	x := c.expr(e)
	if !types.IsOrdinal(x.Type) {
		c.fail(e.Pos(), source.ErrOrdinalExprExpected, "")
	}
	return x
}

func (c *checker) stringExpr(e ast.Expr) TypeAndValue {
	x := c.expr(e)
	if !isStringish(x.Type) {
		c.fail(e.Pos(), source.ErrTypeMismatch, "string expected")
	}
	return x
}

func (c *checker) stringVar(e ast.Expr) {
	if x := c.varArg(e); !types.IsString(x.Type) {
		c.fail(e.Pos(), source.ErrStringVarExpected, "")
	}
}

func (c *checker) intVar(e ast.Expr) {
	if x := c.varArg(e); !types.IsInteger(x.Type) {
		c.fail(e.Pos(), source.ErrIntVarExpected, "")
	}
}

func (c *checker) pointerVar(e ast.Expr) {
	if _, ok := c.varArg(e).Type.(*types.Pointer); !ok {
		c.fail(e.Pos(), source.ErrPointerVarExpected, "")
	}
}

func (c *checker) fileVar(e ast.Expr) *types.File {
	x := c.expr(e)
	f, ok := x.Type.(*types.File)
	if !ok || x.Mode != Variable {
		c.fail(e.Pos(), source.ErrFileVarExpected, "")
	}
	return f
}

func (c *checker) textVar(e ast.Expr) {
	if !c.fileVar(e).Text {
		c.fail(e.Pos(), source.ErrInvalidFileType, "")
	}
}

func (c *checker) binaryFileVar(e ast.Expr) {
	if c.fileVar(e).Text {
		c.fail(e.Pos(), source.ErrInvalidFileType, "")
	}
}

// formatted prüft ein Argument mit optionaler Formatangabe x:breite:nachkomma.
// Nachkommastellen sind nur bei Real-Werten erlaubt.
func (c *checker) formatted(e ast.Expr, check func(TypeAndValue, ast.Expr)) {
	f, ok := e.(*ast.FormatArg)
	if !ok {
		check(c.expr(e), e)
		return
	}
	x := c.expr(f.X)
	c.record(f, x)
	check(x, f.X)
	c.intExpr(f.Width)
	if f.Prec != nil {
		if !types.IsReal(x.Type) {
			c.fail(f.Prec.Pos(), source.ErrInvalidFormat, "")
		}
		c.intExpr(f.Prec)
	}
}

// fileArg erkennt eine Dateivariable als erstes Argument von Read/Write.
func (c *checker) fileArg(args []ast.Expr) (*types.File, []ast.Expr) {
	if len(args) == 0 {
		return types.Text, args
	}
	if _, isFmt := args[0].(*ast.FormatArg); isFmt {
		return types.Text, args
	}
	x := c.exprOrType(args[0])
	if f, ok := x.Type.(*types.File); ok {
		if x.Mode != Variable {
			c.fail(args[0].Pos(), source.ErrFileVarExpected, "")
		}
		return f, args[1:]
	}
	return types.Text, args
}

func (c *checker) write(args []ast.Expr, ln bool) {
	f, rest := c.fileArg(args)
	switch {
	case f.Text:
		for _, a := range rest {
			c.formatted(a, func(x TypeAndValue, e ast.Expr) {
				switch {
				case types.IsInteger(x.Type), types.IsReal(x.Type), isStringish(x.Type), types.Is(x.Type, types.Boolean):
				default:
					c.fail(e.Pos(), source.ErrCannotReadWrite, "")
				}
			})
		}
	case ln || f.Elem == nil:
		c.fail(args[0].Pos(), source.ErrInvalidFileType, "")
	default:
		for _, a := range rest {
			c.assignable(f.Elem, c.expr(a), a)
		}
	}
}

func (c *checker) read(args []ast.Expr, ln bool) {
	f, rest := c.fileArg(args)
	switch {
	case f.Text:
		for _, a := range rest {
			x := c.varArg(a)
			switch {
			case types.IsInteger(x.Type), types.IsReal(x.Type), isStringish(x.Type):
			default:
				c.fail(a.Pos(), source.ErrCannotReadWrite, "")
			}
		}
	case ln || f.Elem == nil:
		c.fail(args[0].Pos(), source.ErrInvalidFileType, "")
	default:
		for _, a := range rest {
			if x := c.varArg(a); x.Type != f.Elem {
				c.fail(a.Pos(), source.ErrTypeMismatch, "")
			}
		}
	}
}

// lowHigh wertet Low(x)/High(x) für Ordinaltypen, Arrays und Strings aus;
// das Ergebnis ist immer eine Konstante (Erweiterung aus TP 7).
func (c *checker) lowHigh(e ast.Expr, high bool) TypeAndValue {
	x := c.exprOrType(e)
	pick := func(lo, hi int64) int64 {
		if high {
			return hi
		}
		return lo
	}
	switch t := x.Type.(type) {
	case *types.Array:
		return TypeAndValue{Mode: Constant, Type: t.Index, Val: types.IntVal(pick(t.Lo, t.Hi))}
	case *types.String:
		return TypeAndValue{Mode: Constant, Type: types.Integer, Val: types.IntVal(pick(0, t.Max))}
	}
	if !types.IsOrdinal(x.Type) || x.Mode == NoValue {
		c.fail(e.Pos(), source.ErrOrdinalTypeExpected, "")
	}
	lo, hi := types.Bounds(x.Type)
	return TypeAndValue{Mode: Constant, Type: x.Type, Val: types.IntVal(pick(lo, hi))}
}
