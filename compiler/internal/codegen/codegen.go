// Package codegen übersetzt die IR in ein WebAssembly-Modul für das Ziel
// wasi: Ablage der Variablen, Frames auf dem Schatten-Stack, strukturierter
// Kontrollfluss und Speicheraufteilung.
package codegen

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/ir"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
	"github.com/lehrcode/wasmpascal/compiler/internal/wasm"
)

// Options steuert die Codegenerierung.
type Options struct {
	// ForceFrame legt alle Pascal-Variablen in den Frame statt in WASM-Locals.
	// Das prüft, dass jede Variable auslagerbar ist (Voraussetzung für die
	// anhaltefähigen Prozeduren aus Etappe 8).
	ForceFrame bool
}

// Globale des Moduls.
const globalSP = 0 // Schatten-Stack-Zeiger

// Generate erzeugt das Binärmodul.
//
// Speicher (PLAN.md): Seite 0 bleibt frei, ab ir.DataBase liegt das
// Datensegment, danach der Schatten-Stack, der von seinem Ende nach unten wächst.
func Generate(m *ir.Module, opts Options) []byte {
	mod := &wasm.Module{}
	gen := &generator{m: m, mod: mod, opts: opts, funcIndex: map[*ir.Func]uint32{}, hostIndex: map[*ir.Host]uint32{}}

	// Importe in Reihenfolge der ersten Verwendung
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if hc, ok := in.(ir.HostCall); ok {
					if _, seen := gen.hostIndex[hc.Fn]; !seen {
						gen.hostIndex[hc.Fn] = uint32(len(mod.Imports))
						mod.Imports = append(mod.Imports, wasm.Import{
							Module: hc.Fn.Module, Name: hc.Fn.Name,
							Type: mod.TypeIndex(wasm.FuncType{Params: valTypes(hc.Fn.Params), Results: valTypes(hc.Fn.Result)}),
						})
					}
				}
			}
		}
	}
	for i, f := range m.Funcs {
		gen.funcIndex[f] = uint32(len(mod.Imports) + i)
	}

	stackBase := types.AlignTo(ir.DataBase+int64(len(m.Data)), 16)
	stackTop := stackBase + types.AlignTo(m.StackSize, 16)
	mod.MemMin = uint32((stackTop + 65535) / 65536)
	mod.Globals = []wasm.Global{{Type: wasm.I32, Mutable: true, Init: wasm.AppendS32([]byte{wasm.OpI32Const}, int32(stackTop))}}
	if len(m.Data) > 0 {
		mod.Data = []wasm.Data{{Offset: ir.DataBase, Bytes: m.Data}}
	}

	for _, f := range m.Funcs {
		mod.Funcs = append(mod.Funcs, gen.function(f))
	}
	mod.Exports = []wasm.Export{
		{Name: "memory", Kind: wasm.ExportMemory, Index: 0},
		{Name: "_start", Kind: wasm.ExportFunc, Index: gen.funcIndex[m.Main]},
	}
	return mod.Encode()
}

type generator struct {
	m         *ir.Module
	mod       *wasm.Module
	opts      Options
	funcIndex map[*ir.Func]uint32
	hostIndex map[*ir.Host]uint32
}

func valType(t ir.ValType) wasm.ValType {
	if t == ir.F64 {
		return wasm.F64
	}
	return wasm.I32
}

func valTypes(ts []ir.ValType) []wasm.ValType {
	out := make([]wasm.ValType, len(ts))
	for i, t := range ts {
		out[i] = valType(t)
	}
	return out
}

// funcGen erzeugt den Code einer Funktion.
type funcGen struct {
	g         *generator
	fn        *ir.Func
	code      wasm.Code
	locals    []wasm.ValType
	local     map[*ir.Var]uint32
	fp        uint32
	frameSize int64
}

func (g *generator) function(f *ir.Func) *wasm.Func {
	fg := &funcGen{g: g, fn: f, local: map[*ir.Var]uint32{}}
	nParams := uint32(len(f.Params))

	// Ablage entscheiden und Frame anlegen
	var frame []*ir.Var
	for _, v := range f.Vars {
		if v.Storage != ir.Undecided {
			continue
		}
		switch {
		case v.Temp:
			v.Storage = ir.Local
		case !v.Scalar || v.CopyIn || v.Addressed || g.opts.ForceFrame:
			v.Storage = ir.Frame
		default:
			v.Storage = ir.Local
		}
		if v.Storage == ir.Frame {
			frame = append(frame, v)
		}
	}
	for _, v := range frame {
		align := int64(4)
		if v.Scalar {
			align = v.Mem.Size()
		} else {
			align = max(v.Type.Align(), 1)
		}
		v.Offset = types.AlignTo(fg.frameSize, align)
		fg.frameSize = v.Offset + v.Size()
	}
	fg.frameSize = types.AlignTo(fg.frameSize, 16)

	// Parameter und Locals
	var params []wasm.ValType
	for _, p := range f.Params {
		if p.Storage == ir.Indirect || p.CopyIn {
			params = append(params, wasm.I32)
		} else {
			params = append(params, valType(p.VT))
		}
	}
	next := nParams
	newLocal := func(t wasm.ValType) uint32 {
		fg.locals = append(fg.locals, t)
		next++
		return next - 1
	}
	if fg.frameSize > 0 {
		fg.fp = newLocal(wasm.I32)
	}
	for _, v := range f.Vars {
		switch {
		case v.Param >= 0 && (v.Storage == ir.Local || v.Storage == ir.Indirect):
			fg.local[v] = uint32(v.Param)
		case v.Storage == ir.Local:
			fg.local[v] = newLocal(valType(v.VT))
		}
	}
	var results []wasm.ValType
	if vt, ok := f.ResultVT(); ok {
		results = []wasm.ValType{valType(vt)}
	}
	typ := g.mod.TypeIndex(wasm.FuncType{Params: params, Results: results})

	// Prolog: Frame reservieren, Parameter im Frame ablegen
	c := &fg.code
	if fg.frameSize > 0 {
		c.Index(wasm.OpGlobalGet, globalSP)
		c.I32Const(int32(fg.frameSize))
		c.Op(wasm.OpI32Sub)
		c.Index(wasm.OpLocalTee, fg.fp)
		c.Index(wasm.OpGlobalSet, globalSP)
		for _, v := range frame {
			if v.Param < 0 {
				continue
			}
			if v.CopyIn {
				fg.addrOf(v)
				c.Index(wasm.OpLocalGet, uint32(v.Param))
				c.I32Const(int32(v.Type.Size()))
				c.MemoryCopy()
				continue
			}
			c.Index(wasm.OpLocalGet, fg.fp)
			c.Index(wasm.OpLocalGet, uint32(v.Param))
			fg.storeOp(v.Mem, v.Offset)
		}
	}

	s := &structurer{g: analyze(f.Entry), code: c, fg: fg}
	s.doTree(f.Entry, nil)
	c.Op(wasm.OpUnreachable, wasm.OpEnd)
	return &wasm.Func{Type: typ, Locals: fg.locals, Code: c.B}
}

// ret erzeugt den Rücksprung: Frame freigeben, Ergebnis liefern.
func (fg *funcGen) ret() {
	c := &fg.code
	if _, ok := fg.fn.ResultVT(); ok {
		fg.expr(ir.Get{Var: fg.fn.Result})
	}
	if fg.frameSize > 0 {
		c.Index(wasm.OpLocalGet, fg.fp)
		c.I32Const(int32(fg.frameSize))
		c.Op(wasm.OpI32Add)
		c.Index(wasm.OpGlobalSet, globalSP)
	}
	c.Op(wasm.OpReturn)
}

// ---------------------------------------------------------------------------
// Speicherzugriffe

func (fg *funcGen) loadOp(m ir.MemType, off int64) {
	c := &fg.code
	switch m {
	case ir.MemI8:
		c.Mem(wasm.OpI32Load8S, 0, uint32(off))
	case ir.MemU8:
		c.Mem(wasm.OpI32Load8U, 0, uint32(off))
	case ir.MemI32:
		c.Mem(wasm.OpI32Load, 2, uint32(off))
	case ir.MemF32:
		c.Mem(wasm.OpF32Load, 2, uint32(off))
		c.Op(wasm.OpF64PromoteF32)
	case ir.MemF64:
		c.Mem(wasm.OpF64Load, 3, uint32(off))
	}
}

func (fg *funcGen) storeOp(m ir.MemType, off int64) {
	c := &fg.code
	switch m {
	case ir.MemI8, ir.MemU8:
		c.Mem(wasm.OpI32Store8, 0, uint32(off))
	case ir.MemI32:
		c.Mem(wasm.OpI32Store, 2, uint32(off))
	case ir.MemF32:
		c.Op(wasm.OpF32DemoteF64)
		c.Mem(wasm.OpF32Store, 2, uint32(off))
	case ir.MemF64:
		c.Mem(wasm.OpF64Store, 3, uint32(off))
	}
}

// base legt die Basisadresse einer Speichervariable auf den Stack und liefert
// den Offset, der als Immediate dazukommt.
func (fg *funcGen) base(v *ir.Var) int64 {
	c := &fg.code
	switch v.Storage {
	case ir.Frame:
		c.Index(wasm.OpLocalGet, fg.fp)
		return v.Offset
	case ir.Global:
		c.I32Const(0)
		return v.Offset
	case ir.Indirect:
		c.Index(wasm.OpLocalGet, fg.local[v])
		return 0
	}
	panic("codegen: Variable " + v.Name + " liegt nicht im Speicher")
}

func (fg *funcGen) addrOf(v *ir.Var) {
	off := fg.base(v)
	if off != 0 {
		fg.code.I32Const(int32(off))
		fg.code.Op(wasm.OpI32Add)
	}
}

// ---------------------------------------------------------------------------
// Ausdrücke und Anweisungen

var binOps = map[ir.Op]byte{
	ir.AddI: wasm.OpI32Add, ir.SubI: wasm.OpI32Sub, ir.MulI: wasm.OpI32Mul, ir.DivI: wasm.OpI32DivS,
	ir.RemI: wasm.OpI32RemS, ir.AndI: wasm.OpI32And, ir.OrI: wasm.OpI32Or, ir.XorI: wasm.OpI32Xor,
	ir.ShlI: wasm.OpI32Shl, ir.ShrI: wasm.OpI32ShrU,
	ir.EqI: wasm.OpI32Eq, ir.NeI: wasm.OpI32Ne, ir.LtI: wasm.OpI32LtS, ir.LeI: wasm.OpI32LeS,
	ir.GtI: wasm.OpI32GtS, ir.GeI: wasm.OpI32GeS,
	ir.AddF: wasm.OpF64Add, ir.SubF: wasm.OpF64Sub, ir.MulF: wasm.OpF64Mul, ir.DivF: wasm.OpF64Div,
	ir.EqF: wasm.OpF64Eq, ir.NeF: wasm.OpF64Ne, ir.LtF: wasm.OpF64Lt, ir.LeF: wasm.OpF64Le,
	ir.GtF: wasm.OpF64Gt, ir.GeF: wasm.OpF64Ge,
}

var unOps = map[ir.Op][]byte{
	ir.EqzI:     {wasm.OpI32Eqz},
	ir.NegF:     {wasm.OpF64Neg},
	ir.AbsF:     {wasm.OpF64Abs},
	ir.SqrtF:    {wasm.OpF64Sqrt},
	ir.TruncF:   {wasm.OpF64Trunc},
	ir.IntToF:   {wasm.OpF64ConvertI32S},
	ir.FToInt:   {wasm.OpI32TruncF64S},
	ir.Ext8:     {wasm.OpI32Extend8S},
	ir.RoundF32: {wasm.OpF32DemoteF64, wasm.OpF64PromoteF32},
}

func (fg *funcGen) expr(e ir.Expr) {
	c := &fg.code
	switch e := e.(type) {
	case ir.IntConst:
		c.I32Const(e.V)
	case ir.FloatConst:
		c.F64Const(e.V)
	case ir.Get:
		v := e.Var
		if v.Storage == ir.Local {
			c.Index(wasm.OpLocalGet, fg.local[v])
			return
		}
		off := fg.base(v)
		fg.loadOp(v.Mem, off)
	case ir.Addr:
		fg.addrOf(e.Var)
	case ir.Load:
		fg.expr(e.Addr)
		fg.loadOp(e.Mem, e.Off)
	case ir.Bin:
		fg.expr(e.X)
		fg.expr(e.Y)
		c.Op(binOps[e.Op])
	case ir.Un:
		fg.expr(e.X)
		c.Op(unOps[e.Op]...)
	default:
		panic("codegen: unbekannter Ausdruck")
	}
}

// set weist einer Variable den Wert auf dem Stack zu; push legt ihn ab.
func (fg *funcGen) set(v *ir.Var, push func()) {
	c := &fg.code
	if v.Storage == ir.Local {
		push()
		if v.Mem == ir.MemF32 {
			c.Op(unOps[ir.RoundF32]...)
		}
		c.Index(wasm.OpLocalSet, fg.local[v])
		return
	}
	off := fg.base(v)
	push()
	fg.storeOp(v.Mem, off)
}

func (fg *funcGen) instr(in ir.Instr) {
	c := &fg.code
	switch in := in.(type) {
	case ir.Set:
		fg.set(in.Var, func() { fg.expr(in.Val) })
	case ir.Store:
		fg.expr(in.Addr)
		fg.expr(in.Val)
		fg.storeOp(in.Mem, in.Off)
	case ir.Copy:
		fg.expr(in.Dst)
		fg.expr(in.Src)
		c.I32Const(int32(in.Size))
		c.MemoryCopy()
	case ir.Call:
		fg.call(in.Args, in.Dst, func() { c.Index(wasm.OpCall, fg.g.funcIndex[in.Fn]) }, func() bool {
			_, ok := in.Fn.ResultVT()
			return ok
		}())
	case ir.HostCall:
		fg.call(in.Args, in.Dst, func() { c.Index(wasm.OpCall, fg.g.hostIndex[in.Fn]) }, len(in.Fn.Result) > 0)
	default:
		panic("codegen: unbekannte Anweisung")
	}
}

func (fg *funcGen) call(args []ir.Expr, dst *ir.Var, emit func(), hasResult bool) {
	push := func() {
		for _, a := range args {
			fg.expr(a)
		}
		emit()
	}
	switch {
	case dst != nil && hasResult:
		if dst.Storage == ir.Local {
			fg.set(dst, push)
			return
		}
		// Ergebnis über einen Hilfslocal in den Speicher schreiben
		push()
		tmp := uint32(len(fg.fn.Params)) + uint32(len(fg.locals))
		fg.locals = append(fg.locals, valType(dst.VT))
		fg.code.Index(wasm.OpLocalSet, tmp)
		fg.set(dst, func() { fg.code.Index(wasm.OpLocalGet, tmp) })
	case hasResult:
		push()
		fg.code.Op(wasm.OpDrop)
	default:
		push()
	}
}
