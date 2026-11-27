package ir

import (
	"math"
	"strings"

	"github.com/lehrcode/wasmpascal/compiler/internal/abi"
	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// Laufzeitfehler aus der Mathematik.
const errInvalidFloat = 207

// pointee liefert die Größe des Typs, auf den ein Zeiger zeigt.
func pointee(t types.Type) int64 {
	if p, ok := t.(*types.Pointer); ok && p.Elem != nil {
		return p.Elem.Size()
	}
	return 0
}

// sysVar liefert die Adresse einer Variable der System-Unit (Input, Output).
func (s *fnState) sysVar(name string) Expr {
	v := s.l.info.SystemIntf.Syms[strings.ToLower(name)].(*types.Var)
	return Addr{Var: s.l.vars[v]}
}

// wasiHost liefert den Import zu abi.WASI[i].
func (l *lowerer) wasiHost(i int) *Host {
	if h, ok := l.hosts[i]; ok {
		return h
	}
	w := abi.Intrinsics[i]
	h := &Host{Module: w.Module, Name: w.Import}
	for _, p := range w.Params {
		if p == abi.I32 {
			h.Params = append(h.Params, I32)
		} else {
			h.Params = append(h.Params, I64)
		}
	}
	if w.Result {
		h.Result = []ValType{I32}
	}
	l.hosts[i] = h
	return h
}

// ioCheck prüft nach einer E/A-Routine unter {$I+} das Ergebnis und bricht
// gegebenenfalls mit dessen Nummer ab (wie TP).
func (s *fnState) ioCheck() {
	if !s.io || s.rtl {
		return
	}
	r := s.temp(types.Integer)
	s.callRtl("_IOResult", r)
	trap, ok := s.fn.NewBlock(), s.fn.NewBlock()
	trap.Term = Trap{CodeVar: r, Pos: s.at}
	s.branch(Get{Var: r}, trap, ok)
	s.cur = ok
}

// rtlValue ruft eine Laufzeitfunktion mit skalarem Ergebnis auf.
func (s *fnState) rtlValue(name string, t types.Type, args ...Expr) Expr {
	r := s.temp(t)
	s.callRtl(name, r, args...)
	return Get{Var: r}
}

// rtlString ruft eine Laufzeitfunktion mit String-Ergebnis auf und liefert
// die Adresse des Ergebnisses.
func (s *fnState) rtlString(name string, args ...Expr) Expr {
	t := s.temp(types.StringType)
	s.callRtl(name, nil, append([]Expr{Addr{Var: t}}, args...)...)
	return Addr{Var: t}
}

// fileArg liefert die Datei eines E/A-Aufrufs (Adresse, Typ) und die übrigen
// Argumente; ohne Dateiargument die Standarddatei def.
func (s *fnState) fileArg(args []ast.Expr, def string) (Expr, *types.File, []ast.Expr) {
	if len(args) > 0 {
		if _, isFmt := args[0].(*ast.FormatArg); !isFmt {
			if f, ok := s.typeOf(args[0]).(*types.File); ok {
				return s.keep(s.designator(args[0])), f, args[1:]
			}
		}
	}
	return s.sysVar(def), types.Text, args
}

// valueAddr liefert die Adresse eines Werts vom Typ t (für Write auf
// typisierte Dateien); Ausdrücke landen in einer Hilfsvariable.
func (s *fnState) valueAddr(e ast.Expr, t types.Type) Expr {
	if _, _, ok := scalar(t); ok {
		tmp := s.temp(t)
		tmp.Temp = false // braucht eine Adresse
		tmp.Addressed = true
		s.emit(Set{Var: tmp, Val: s.value(e, t)})
		return Addr{Var: tmp}
	}
	return s.addr(e)
}

// builtin übersetzt eine Standardroutine und liefert gegebenenfalls ihr Ergebnis.
func (s *fnState) builtin(at ast.Node, b *types.Builtin, args []ast.Expr) Expr {
	arg := func(i int) Expr { return s.expr(args[i]) }
	if b.ID >= check.BWasiBase {
		i := int(b.ID - check.BWasiBase)
		w := abi.Intrinsics[i]
		var in []Expr
		for j, a := range args {
			x := s.expr(a)
			switch w.Params[j] {
			case abi.I64S:
				x = Un{Op: ExtS64, X: x}
			case abi.I64U:
				x = Un{Op: ExtU64, X: x}
			}
			in = append(in, x)
		}
		if !w.Result {
			s.emit(HostCall{Fn: s.l.wasiHost(i), Args: in})
			if w.Import == "proc_exit" {
				s.terminate(Unreachable{})
			}
			return nil
		}
		r := s.temp(types.LongInt)
		s.emit(HostCall{Fn: s.l.wasiHost(i), Args: in, Dst: r})
		return Get{Var: r}
	}

	switch b.ID {
	// ------------------------------------------------------------ E/A
	case check.BWrite, check.BWriteLn:
		s.write(args, b.ID == check.BWriteLn)
		return nil
	case check.BRead, check.BReadLn:
		s.read(args, b.ID == check.BReadLn)
		return nil
	case check.BAssign:
		s.callRtl("_Assign", nil, s.designator(args[0]), s.strAddr(args[1]))
		return nil
	case check.BReset, check.BRewrite:
		f := s.typeOf(args[0]).(*types.File)
		fa := s.designator(args[0])
		switch {
		case f.Text && b.ID == check.BReset:
			s.callRtl("_ResetText", nil, fa)
		case f.Text:
			s.callRtl("_RewriteText", nil, fa)
		default:
			var size Expr = IntConst{V: 128} // untypisiert: Satzgröße 128 wie TP
			if f.Elem != nil {
				size = IntConst{V: int32(f.Elem.Size())}
			} else if len(args) == 2 {
				size = arg(1)
			}
			name := "_ResetFile"
			if b.ID == check.BRewrite {
				name = "_RewriteFile"
			}
			s.callRtl(name, nil, fa, size)
		}
		s.ioCheck()
		return nil
	case check.BAppend:
		s.callRtl("_AppendText", nil, s.designator(args[0]))
		s.ioCheck()
		return nil
	case check.BClose:
		name := "_CloseFile"
		if s.typeOf(args[0]).(*types.File).Text {
			name = "_CloseText"
		}
		s.callRtl(name, nil, s.designator(args[0]))
		s.ioCheck()
		return nil
	case check.BFlush:
		s.callRtl("_Flush", nil, s.designator(args[0]))
		s.ioCheck()
		return nil
	case check.BErase:
		s.callRtl("_Erase", nil, s.designator(args[0]))
		s.ioCheck()
		return nil
	case check.BRename:
		s.callRtl("_Rename", nil, s.designator(args[0]), s.strAddr(args[1]))
		s.ioCheck()
		return nil
	case check.BSeek:
		s.callRtl("_Seek", nil, s.designator(args[0]), arg(1))
		s.ioCheck()
		return nil
	case check.BTruncate:
		s.callRtl("_Truncate", nil, s.designator(args[0]))
		s.ioCheck()
		return nil
	case check.BFilePos, check.BFileSize:
		name := "_FilePos"
		if b.ID == check.BFileSize {
			name = "_FileSize"
		}
		r := s.rtlValue(name, types.LongInt, s.designator(args[0]))
		s.ioCheck()
		return r
	case check.BEof, check.BEoln, check.BSeekEof, check.BSeekEoln:
		f, ft, _ := s.fileArg(args, "Input")
		name := map[types.BuiltinID]string{check.BEof: "_EofText", check.BEoln: "_Eoln",
			check.BSeekEof: "_SeekEof", check.BSeekEoln: "_SeekEoln"}[b.ID]
		if !ft.Text {
			name = "_EofFile"
		}
		r := s.rtlValue(name, types.Boolean, f)
		s.ioCheck()
		return r
	case check.BBlockRead, check.BBlockWrite:
		name := "_BlockRead"
		if b.ID == check.BBlockWrite {
			name = "_BlockWrite"
		}
		want := IntConst{V: 0}
		if len(args) == 4 {
			want = IntConst{V: 1}
		}
		r := s.rtlValue(name, types.LongInt, s.designator(args[0]), s.designator(args[1]), arg(2), want)
		if len(args) == 4 {
			s.storeTo(args[3], s.coerce(r, types.LongInt, s.typeOf(args[3])))
		}
		s.ioCheck()
		return nil
	case check.BIOResult:
		return s.rtlValue("_IOResult", types.Integer)
	case check.BChDir, check.BMkDir, check.BRmDir:
		name := map[types.BuiltinID]string{check.BChDir: "_ChDir", check.BMkDir: "_MkDir", check.BRmDir: "_RmDir"}[b.ID]
		s.callRtl(name, nil, s.strAddr(args[0]))
		s.ioCheck()
		return nil
	case check.BGetDir:
		dir := s.rtlString("_GetDir", arg(0))
		s.assignString(args[1], dir)
		s.ioCheck()
		return nil

	// ------------------------------------------------------------ Str, Val
	case check.BStr:
		x, width, prec := args[0], Expr(IntConst{V: 0}), Expr(IntConst{V: -1})
		if f, ok := x.(*ast.FormatArg); ok {
			x, width = f.X, s.expr(f.Width)
			if f.Prec != nil {
				prec = s.expr(f.Prec)
			}
		} else if types.IsReal(s.typeOf(x)) {
			width = IntConst{V: -1}
		}
		var str Expr
		if types.IsReal(s.typeOf(x)) {
			str = s.rtlString("_RealStr", s.value(x, types.Real), width, prec)
		} else {
			str = s.rtlString("_IntStr", s.expr(x), width)
		}
		s.assignString(args[1], str)
		return nil
	case check.BVal:
		code := s.temp(types.Integer)
		code.Temp, code.Addressed = false, true
		t := s.typeOf(args[1])
		var r Expr
		if types.IsReal(t) {
			r = s.rtlValue("_ValReal", types.Real, s.strAddr(args[0]), Addr{Var: code})
		} else {
			r = s.rtlValue("_ValInt", types.LongInt, s.strAddr(args[0]), Addr{Var: code})
		}
		from := types.Type(types.LongInt)
		if types.IsReal(t) {
			from = types.Real
		}
		s.storeTo(args[1], s.coerce(r, from, t))
		s.storeTo(args[2], s.coerce(Get{Var: code}, types.Integer, s.typeOf(args[2])))
		return nil

	// ------------------------------------------------------------ Ablauf
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
		if v, ok := s.directVar(args[0]); ok && v.Scalar {
			s.emit(Set{Var: v, Val: narrow(Bin{Op: op, X: Get{Var: v}, Y: n}, t)})
			return nil
		}
		a := s.keep(s.designator(args[0]))
		s.emit(Store{Addr: a, Mem: memOf(t), Val: Bin{Op: op, X: Load{Addr: a, Mem: memOf(t)}, Y: n}})
		return nil
	case check.BHalt:
		var code Expr = IntConst{V: 0}
		if len(args) == 1 {
			code = arg(0)
		}
		s.callRtl("_Halt", nil, code)
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
	case check.BGoSetSP:
		s.emit(SetHostGlobal{G: GlobalGoSP, Val: arg(0)})
		return nil
	case check.BGoArgc:
		return HostGlobal{G: GlobalArgc}
	case check.BGoArgv:
		return HostGlobal{G: GlobalArgv}
	case check.BSuspend:
		r := s.temp(types.LongInt)
		s.emit(Suspend{Reason: arg(0), Arg: arg(1), Dst: r})
		return Get{Var: r}
	case check.BRunError:
		code := s.temp(types.LongInt)
		s.emit(Set{Var: code, Val: arg(0)})
		s.terminate(Trap{CodeVar: code, Pos: s.at, KeepPos: s.rtl})
		return nil
	case check.BRunErrorTP:
		// Bit 16 kennzeichnet die Nummer als gesetzt, damit auch
		// RunError(0) kein „echter“ WASM-Trap (Nummer 0) ist; der Host
		// nimmt nur die unteren 16 Bit.
		var n Expr = IntConst{V: 0}
		if len(args) == 1 {
			n = Bin{Op: AndI, X: arg(0), Y: IntConst{V: 0xFFFF}}
		}
		code := s.temp(types.LongInt)
		s.emit(Set{Var: code, Val: Bin{Op: OrI, X: n, Y: IntConst{V: 0x10000}}})
		s.terminate(Trap{CodeVar: code, Pos: s.at, KeepPos: s.rtl})
		return nil
	case check.BParamCount:
		return s.rtlValue("_ParamCount", types.Integer)
	case check.BParamStr:
		return s.rtlString("_ParamStr", arg(0))
	case check.BRandomize:
		s.callRtl("_Randomize", nil)
		return nil
	case check.BRandom:
		if len(args) == 0 {
			return s.rtlValue("_RandomReal", types.Real)
		}
		return s.rtlValue("_Random", types.LongInt, arg(0))

	// ------------------------------------------------------------ Heap
	case check.BNew, check.BGetMem:
		var n Expr = IntConst{V: int32(pointee(s.typeOf(args[0])))}
		if b.ID == check.BGetMem {
			n = arg(1)
		}
		p := s.temp(types.LongInt)
		s.callRtl("_GetMem", p, n)
		s.check(Un{Op: EqzI, X: Get{Var: p}}, ErrHeapOver)
		s.storeTo(args[0], Get{Var: p})
		return nil
	case check.BDispose, check.BFreeMem:
		var n Expr = IntConst{V: int32(pointee(s.typeOf(args[0])))}
		if b.ID == check.BFreeMem {
			n = arg(1)
		}
		p := s.keep(arg(0))
		s.check(Un{Op: EqzI, X: p}, ErrInvalidPtr)
		s.callRtl("_FreeMem", nil, p, n)
		return nil
	case check.BMark:
		s.callRtl("_Mark", nil, s.designator(args[0]))
		return nil
	case check.BRelease:
		s.callRtl("_Release", nil, arg(0))
		return nil
	case check.BMemAvail:
		return s.rtlValue("_MemAvail", types.LongInt)
	case check.BMaxAvail:
		return s.rtlValue("_MaxAvail", types.LongInt)

	// ------------------------------------------------------------ Speicher
	case check.BFillChar:
		s.emit(Fill{Dst: s.designator(args[0]), Val: arg(2), N: arg(1)})
		return nil
	case check.BMove:
		s.emit(Copy{Dst: s.designator(args[1]), Src: s.designator(args[0]), N: arg(2)})
		return nil
	case check.BAddr, check.BOfs:
		if p := s.l.info.ProcRefs[args[0]]; p != nil {
			return IntConst{V: s.l.tableIndex(p)}
		}
		if _, ok := s.typeOf(args[0]).(*types.ProcType); ok && b.ID == check.BAddr {
			return s.procValue(args[0])
		}
		return s.place(args[0])
	case check.BPtr:
		return arg(1)
	case check.BMemGrow:
		r := s.temp(types.LongInt)
		s.emit(Grow{Pages: arg(0), Dst: r})
		return Get{Var: r}
	case check.BMemSize:
		return MemSize{}
	case check.BHeapBase:
		return HeapBase{}

	// ------------------------------------------------------------ Strings
	case check.BLength:
		if types.Is(s.typeOf(args[0]), types.Char) {
			return IntConst{V: 1}
		}
		return Load{Addr: s.strAddr(args[0]), Mem: MemU8}
	case check.BPos:
		return s.rtlValue("_StrPos", types.Integer, s.strAddr(args[0]), s.strAddr(args[1]))
	case check.BDelete:
		s.callRtl("_StrDelete", nil, s.designator(args[0]), arg(1), arg(2))
		return nil
	case check.BInsert:
		limit := s.typeOf(args[1]).(*types.String).Max
		s.callRtl("_StrInsert", nil, s.strAddr(args[0]), s.designator(args[1]), arg(2), IntConst{V: int32(limit)})
		return nil
	case check.BUpCase:
		return s.rtlValue("_UpCase", types.Char, arg(0))

	// ------------------------------------------------------------ Ordinal
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
		g := s.keep(arg(0))
		return Bin{Op: OrI,
			X: Bin{Op: AndI, X: g, Y: IntConst{V: ^0xFFFF}},
			Y: Bin{Op: OrI,
				X: Bin{Op: ShlI, X: Bin{Op: AndI, X: g, Y: IntConst{V: 255}}, Y: IntConst{V: 8}},
				Y: Bin{Op: AndI, X: Bin{Op: ShrI, X: g, Y: IntConst{V: 8}}, Y: IntConst{V: 255}}}}

	// ------------------------------------------------------------ Arithmetik
	case check.BSqr:
		x := s.keep(arg(0))
		if types.IsReal(s.typeOf(args[0])) {
			return Bin{Op: MulF, X: x, Y: x}
		}
		return Bin{Op: MulI, X: x, Y: x}
	case check.BAbs:
		if types.IsReal(s.typeOf(args[0])) {
			return Un{Op: AbsF, X: arg(0)}
		}
		x := s.temp(types.LongInt)
		s.emit(Set{Var: x, Val: arg(0)})
		s.ifElse(Bin{Op: LtI, X: Get{Var: x}, Y: IntConst{V: 0}},
			func() { s.emit(Set{Var: x, Val: Bin{Op: SubI, X: IntConst{V: 0}, Y: Get{Var: x}}}) },
			func() {})
		return Get{Var: x}
	case check.BSqrt:
		x := s.keep(s.value(args[0], types.Real))
		s.check(Bin{Op: LtF, X: x, Y: FloatConst{V: 0}}, errInvalidFloat)
		return Un{Op: SqrtF, X: x}
	case check.BInt:
		return Un{Op: TruncF, X: s.value(args[0], types.Real)}
	case check.BFrac:
		x := s.keep(s.value(args[0], types.Real))
		return Bin{Op: SubF, X: x, Y: Un{Op: TruncF, X: x}}
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
	case check.BSin, check.BCos, check.BArcTan, check.BExp, check.BLn:
		name := map[types.BuiltinID]string{check.BSin: "_Sin", check.BCos: "_Cos", check.BArcTan: "_ArcTan",
			check.BExp: "_Exp", check.BLn: "_Ln"}[b.ID]
		return s.rtlValue(name, types.Real, s.value(args[0], types.Real))
	}
	panic("ir: Standardroutine " + b.Text() + " nicht übersetzt")
}

// assignString weist die String-Adresse src der String-Variable dst zu.
func (s *fnState) assignString(dst ast.Expr, src Expr) {
	limit := s.typeOf(dst).(*types.String).Max
	s.callRtl("_StrAssign", nil, s.designator(dst), src, IntConst{V: int32(limit)})
}

// storeTo schreibt einen skalaren Wert in die Variable oder das Element e.
func (s *fnState) storeTo(e ast.Expr, val Expr) {
	if v, ok := s.directVar(e); ok && v.Scalar {
		s.emit(Set{Var: v, Val: val})
		return
	}
	s.emit(Store{Addr: s.designator(e), Mem: memOf(s.typeOf(e)), Val: val})
}

// write übersetzt Write/WriteLn auf Text- oder typisierte Dateien.
func (s *fnState) write(args []ast.Expr, ln bool) {
	f, ft, rest := s.fileArg(args, "Output")
	if !ft.Text {
		for _, a := range rest {
			s.callRtl("_WriteRec", nil, f, s.valueAddr(a, ft.Elem))
		}
		s.ioCheck()
		return
	}
	for _, a := range rest {
		var width, prec Expr = IntConst{V: 0}, IntConst{V: -1}
		x := a
		fa, isFmt := a.(*ast.FormatArg)
		if isFmt {
			x = fa.X
			width = s.expr(fa.Width)
			if fa.Prec != nil {
				prec = s.expr(fa.Prec)
			}
		}
		t := s.typeOf(x)
		switch {
		case types.IsString(t):
			s.callRtl("_WriteStr", nil, f, s.strAddr(x), width)
		case types.IsReal(t):
			if !isFmt {
				width = IntConst{V: -1}
			}
			s.callRtl("_WriteReal", nil, f, s.value(x, types.Real), width, prec)
		case types.Is(t, types.Char):
			s.callRtl("_WriteChar", nil, f, s.expr(x), width)
		case types.Is(t, types.WideChar):
			s.callRtl("_WriteWideChar", nil, f, s.expr(x), width)
		case types.Is(t, types.Boolean):
			s.callRtl("_WriteBool", nil, f, s.expr(x), width)
		default:
			s.callRtl("_WriteInt", nil, f, s.expr(x), width)
		}
	}
	if ln {
		s.callRtl("_WriteLn", nil, f)
	}
	s.callRtl("_WriteEnd", nil, f)
	s.ioCheck()
}

// read übersetzt Read/ReadLn von Text- oder typisierten Dateien.
func (s *fnState) read(args []ast.Expr, ln bool) {
	f, ft, rest := s.fileArg(args, "Input")
	if !ft.Text {
		for _, a := range rest {
			s.callRtl("_ReadRec", nil, f, s.designator(a))
		}
		s.ioCheck()
		return
	}
	for _, a := range rest {
		t := s.typeOf(a)
		switch {
		case types.IsString(t):
			s.callRtl("_ReadStr", nil, f, s.designator(a), IntConst{V: int32(t.(*types.String).Max)})
		case types.IsReal(t):
			s.storeTo(a, s.rtlValue("_ReadReal", types.Real, f))
		case types.Is(t, types.Char):
			s.storeTo(a, s.rtlValue("_ReadChar", types.Char, f))
		case types.Is(t, types.WideChar):
			s.storeTo(a, s.coerce(s.rtlValue("_ReadWideChar", types.LongInt, f), types.LongInt, t))
		default:
			s.storeTo(a, s.coerce(s.rtlValue("_ReadInt", types.LongInt, f), types.LongInt, t))
		}
	}
	if ln {
		s.callRtl("_ReadLn", nil, f)
	}
	s.ioCheck()
}
