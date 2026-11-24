// Package codegen übersetzt die IR in ein WebAssembly-Modul für das Ziel
// wasi: Ablage der Variablen, Frames auf dem Schatten-Stack, Static Links,
// strukturierter Kontrollfluss, Speicheraufteilung und Laufzeitfehler.
package codegen

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/analysis"
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
	// NoLines lässt Name Section und wasmpascal.lines weg.
	NoLines bool
}

// Globale des Moduls. Die Laufzeitfehler-Globalen werden exportiert; der
// Runner und der Browser-Loader lesen sie nach einem Trap.
const (
	globalSP   = 0 // Schatten-Stack-Zeiger
	globalCode = 1 // Nummer des Laufzeitfehlers (0: echter WASM-Trap)
	globalFile = 2 // Datei der aktuellen Anweisung (Index in wasmpascal.lines)
	globalLine = 3 // Zeile der aktuellen Anweisung
	globalFunc = 4 // Funktionsindex der aktuellen Prozedur

	// Scheduler (dispatch.go)
	globalState  = 5 // 0 normal, 1 unwinding, 2 rewinding
	globalReason = 6 // Grund des Anhaltens
	globalArg    = 7 // Argument des Anhaltens
	globalResume = 8 // Ergebnis des Hosts beim Fortsetzen
)

// GlobalExports sind die Namen der exportierten Globalen.
var GlobalExports = [...]string{"", "__rt_code", "__rt_file", "__rt_line", "__rt_func",
	"__sched_state", "__sched_reason", "__sched_arg", "__sched_resume"}

// Generate erzeugt das Binärmodul.
//
// Speicher (PLAN.md): Seite 0 bleibt frei, ab ir.DataBase liegt das
// Datensegment, danach der Schatten-Stack, der von seinem Ende nach unten
// wächst, dahinter der Heap (rtl/system.pas, wächst mit memory.grow).
func Generate(m *ir.Module, opts Options) []byte {
	analysis.MarkSuspendable(m)
	mod := &wasm.Module{}
	g := &generator{m: m, mod: mod, opts: opts, funcIndex: map[*ir.Func]uint32{}, hostIndex: map[*ir.Host]uint32{},
		spill: map[*ir.Var]int64{}, frameSize: map[*ir.Func]int64{}, stateSlot: map[*ir.Func]int64{}}

	// Importe in Reihenfolge der ersten Verwendung
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if hc, ok := in.(ir.HostCall); ok {
					if _, seen := g.hostIndex[hc.Fn]; !seen {
						g.hostIndex[hc.Fn] = uint32(len(mod.Imports))
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
		g.funcIndex[f] = uint32(len(mod.Imports) + i)
	}

	g.stackBase = types.AlignTo(ir.DataBase+int64(len(m.Data)), 16)
	g.stackTop = g.stackBase + types.AlignTo(m.StackSize, 16)
	mod.MemMin = uint32((g.stackTop + 65535) / 65536)
	i32 := func(v int64) wasm.Global {
		return wasm.Global{Type: wasm.I32, Mutable: true, Init: wasm.AppendS32([]byte{wasm.OpI32Const}, int32(v))}
	}
	mod.Globals = []wasm.Global{i32(g.stackTop), i32(0), i32(0), i32(0), i32(0), i32(0), i32(0), i32(0), i32(0)}
	if len(m.Data) > 0 {
		mod.Data = []wasm.Data{{Offset: ir.DataBase, Bytes: m.Data}}
	}

	// Erst alle Frames anlegen, dann Code erzeugen: Zugriffe über den Static
	// Link brauchen die Offsets fremder Frames.
	for _, f := range m.Funcs {
		g.layout(f)
	}
	for _, f := range m.Funcs {
		mod.Funcs = append(mod.Funcs, g.function(f))
	}
	start := g.funcIndex[m.Main]
	if m.SuspendAction != nil {
		start = uint32(len(mod.Imports) + len(mod.Funcs))
		mod.Funcs = append(mod.Funcs, g.driver())
	}
	mod.Exports = []wasm.Export{
		{Name: "memory", Kind: wasm.ExportMemory, Index: 0},
		{Name: "_start", Kind: wasm.ExportFunc, Index: start},
	}
	for i := globalCode; i <= globalResume; i++ {
		mod.Exports = append(mod.Exports, wasm.Export{Name: GlobalExports[i], Kind: wasm.ExportGlobal, Index: uint32(i)})
	}
	if !opts.NoLines {
		mod.Customs = append(mod.Customs, g.nameSection(), g.linesSection())
	}
	return mod.Encode()
}

type generator struct {
	m                   *ir.Module
	mod                 *wasm.Module
	opts                Options
	funcIndex           map[*ir.Func]uint32
	hostIndex           map[*ir.Host]uint32
	stackBase, stackTop int64
	frameSize           map[*ir.Func]int64
	// spill: Indirect-Variablen, auf die innere Prozeduren zugreifen; ihre
	// Adresse liegt zusätzlich an diesem Offset im Frame.
	spill map[*ir.Var]int64
	// stateSlot: Offset des gesicherten Segments anhaltefähiger Funktionen.
	stateSlot map[*ir.Func]int64
}

func valType(t ir.ValType) wasm.ValType {
	switch t {
	case ir.F64:
		return wasm.F64
	case ir.I64:
		return wasm.I64
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

// layout entscheidet die Ablage der Variablen und legt den Frame an.
func (g *generator) layout(f *ir.Func) {
	size := int64(0)
	place := func(n, align int64) int64 {
		off := types.AlignTo(size, max(align, 1))
		size = off + n
		return off
	}
	for _, v := range f.Vars {
		switch v.Storage {
		case ir.Undecided:
			switch {
			case f.Suspendable:
				// alle Variablen überdauern das Anhalten im Frame
				v.Storage = ir.Frame
			case v.Temp:
				v.Storage = ir.Local
			case !v.Scalar || v.Addressed || g.opts.ForceFrame:
				v.Storage = ir.Frame
			default:
				v.Storage = ir.Local
			}
			if v.Storage == ir.Frame {
				if v.Scalar {
					v.Offset = place(v.Mem.Size(), v.Mem.Size())
				} else {
					v.Offset = place(v.Type.Size(), v.Type.Align())
				}
			}
		case ir.Indirect:
			if v.Addressed {
				g.spill[v] = place(4, 4)
			}
		}
	}
	if f.Suspendable {
		g.stateSlot[f] = place(4, 4)
	}
	if size == 0 && f.HasNested {
		size = 16 // innere Prozeduren brauchen eine Frame-Adresse
	}
	g.frameSize[f] = types.AlignTo(size, 16)
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
	st        uint32 // Zustandsvariable der Dispatch-Schleife
	loopDepth uint32 // Tiefe des Schleifenkopfs im aktuellen Segment
}

func (g *generator) function(f *ir.Func) *wasm.Func {
	fg := &funcGen{g: g, fn: f, local: map[*ir.Var]uint32{}, frameSize: g.frameSize[f]}
	nParams := uint32(len(f.Params))

	var params []wasm.ValType
	for _, p := range f.Params {
		if p.Storage == ir.Indirect {
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
	if isDispatch(f) {
		fg.st = newLocal(wasm.I32)
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

	c := &fg.code
	if !g.opts.NoLines && !f.RTL {
		c.I32Const(int32(g.funcIndex[f]))
		c.Index(wasm.OpGlobalSet, globalFunc)
	}
	// Prolog: Frame reservieren (mit Prüfung auf Überlauf), Parameter ablegen
	if fg.frameSize > 0 {
		c.Index(wasm.OpGlobalGet, globalSP)
		c.I32Const(int32(fg.frameSize))
		c.Op(wasm.OpI32Sub)
		c.Index(wasm.OpLocalTee, fg.fp)
		c.I32Const(int32(g.stackBase))
		c.Op(wasm.OpI32LtU)
		c.Block(wasm.OpIf, wasm.BlockEmpty)
		fg.trap(ir.Trap{Code: ir.ErrStackOver, Pos: f.Pos, KeepPos: f.RTL})
		c.Op(wasm.OpEnd)
		c.Index(wasm.OpLocalGet, fg.fp)
		c.Index(wasm.OpGlobalSet, globalSP)
		if f.Suspendable {
			// Beim Fortsetzen stehen die Parameter schon (womöglich verändert) im Frame.
			c.Index(wasm.OpGlobalGet, globalState)
			c.I32Const(stateRewinding)
			c.Op(wasm.OpI32Ne)
			c.Block(wasm.OpIf, wasm.BlockEmpty)
		}
		for _, v := range f.Vars {
			switch {
			case v.Param >= 0 && v.Storage == ir.Frame:
				c.Index(wasm.OpLocalGet, fg.fp)
				c.Index(wasm.OpLocalGet, uint32(v.Param))
				fg.storeOp(v.Mem, v.Offset)
			case v.Storage == ir.Indirect && v.Addressed:
				c.Index(wasm.OpLocalGet, fg.fp)
				c.Index(wasm.OpLocalGet, uint32(v.Param))
				c.Mem(wasm.OpI32Store, 2, uint32(g.spill[v]))
			}
		}
		if f.Suspendable {
			c.Op(wasm.OpEnd)
		}
	}

	if isDispatch(f) {
		fg.dispatch()
	} else {
		s := &structurer{g: analyze(f.Entry), code: c, fg: fg}
		s.doTree(f.Entry, nil)
	}
	c.Op(wasm.OpUnreachable, wasm.OpEnd)
	return &wasm.Func{Type: typ, Locals: fg.locals, Code: c.B}
}

// ret erzeugt den Rücksprung: Ergebnis laden, Frame freigeben.
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

// trap bricht mit einem geprüften Laufzeitfehler ab.
func (fg *funcGen) trap(t ir.Trap) {
	c := &fg.code
	if t.CodeVar != nil {
		fg.expr(ir.Get{Var: t.CodeVar})
	} else {
		c.I32Const(int32(t.Code))
	}
	c.Index(wasm.OpGlobalSet, globalCode)
	if !fg.g.opts.NoLines && !t.KeepPos {
		fg.setPos(t.Pos)
	}
	c.Op(wasm.OpUnreachable)
}

// setPos merkt Datei, Zeile und Funktion für Laufzeitfehler vor.
func (fg *funcGen) setPos(pos ir.Pos) {
	c := &fg.code
	c.I32Const(int32(pos.File))
	c.Index(wasm.OpGlobalSet, globalFile)
	c.I32Const(int32(pos.Line))
	c.Index(wasm.OpGlobalSet, globalLine)
	c.I32Const(int32(fg.g.funcIndex[fg.fn]))
	c.Index(wasm.OpGlobalSet, globalFunc)
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

// frameBase legt die Frame-Adresse der Funktion target auf den Stack: die
// eigene oder, über die Kette der Static Links, die einer umschließenden.
func (fg *funcGen) frameBase(target *ir.Func) {
	c := &fg.code
	if target == fg.fn {
		c.Index(wasm.OpLocalGet, fg.fp)
		return
	}
	sl := fg.fn.StaticLink
	c.Index(wasm.OpLocalGet, fg.fp)
	c.Mem(wasm.OpI32Load, 2, uint32(sl.Offset)) // Frame von Parent
	for p := fg.fn.Parent; p != target; p = p.Parent {
		c.Mem(wasm.OpI32Load, 2, uint32(p.StaticLink.Offset))
	}
}

// base legt die Basisadresse einer Speichervariable auf den Stack und liefert
// den Offset, der als Immediate dazukommt.
func (fg *funcGen) base(v *ir.Var) int64 {
	c := &fg.code
	if v.Owner != nil && v.Owner != fg.fn {
		fg.frameBase(v.Owner)
		switch v.Storage {
		case ir.Frame:
			return v.Offset
		case ir.Indirect:
			c.Mem(wasm.OpI32Load, 2, uint32(fg.g.spill[v]))
			return 0
		}
		panic("codegen: " + v.Name + " ist für innere Prozeduren nicht erreichbar")
	}
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
	ir.ExtS64:   {wasm.OpI64ExtendI32S},
	ir.ExtU64:   {wasm.OpI64ExtendI32U},
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
		if v.Storage == ir.Local && (v.Owner == nil || v.Owner == fg.fn) {
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
	case ir.FrameBase:
		fg.frameBase(e.F)
	case ir.HeapBase:
		c.I32Const(int32(fg.g.stackTop))
	case ir.MemSize:
		c.Op(wasm.OpMemorySize, 0)
	case ir.SetCmp:
		// Gleichheit: (A xor B) == 0; Teilmenge: (A andnot B) == 0, je 2 × 16 Byte
		op := wasm.SimdV128Xor
		if e.Op == ir.SetSub {
			op = wasm.SimdV128AndNot
		}
		for half := uint32(0); half < 32; half += 16 {
			fg.expr(e.A)
			c.SIMDMem(wasm.SimdV128Load, 0, half)
			fg.expr(e.B)
			c.SIMDMem(wasm.SimdV128Load, 0, half)
			c.SIMD(op)
		}
		c.SIMD(wasm.SimdV128Or)
		c.SIMD(wasm.SimdV128AnyTrue)
		c.Op(wasm.OpI32Eqz)
	default:
		panic("codegen: unbekannter Ausdruck")
	}
}

// set weist einer Variable den Wert zu, den push auf den Stack legt.
func (fg *funcGen) set(v *ir.Var, push func()) {
	c := &fg.code
	if v.Storage == ir.Local && (v.Owner == nil || v.Owner == fg.fn) {
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
		fg.expr(in.N)
		c.MemoryCopy()
	case ir.Fill:
		fg.expr(in.Dst)
		fg.expr(in.Val)
		fg.expr(in.N)
		c.MemoryFill()
	case ir.SetOp:
		op := map[ir.SetOpKind]uint32{ir.SetUnion: wasm.SimdV128Or, ir.SetInter: wasm.SimdV128And, ir.SetDiff: wasm.SimdV128AndNot}[in.Op]
		for half := uint32(0); half < 32; half += 16 {
			fg.expr(in.Dst)
			fg.expr(in.A)
			c.SIMDMem(wasm.SimdV128Load, 0, half)
			fg.expr(in.B)
			c.SIMDMem(wasm.SimdV128Load, 0, half)
			c.SIMD(op)
			c.SIMDMem(wasm.SimdV128Store, 0, half)
		}
	case ir.Grow:
		fg.set(in.Dst, func() {
			fg.expr(in.Pages)
			c.Op(wasm.OpMemoryGrow, 0)
		})
	case ir.Line:
		if !fg.g.opts.NoLines {
			c.I32Const(int32(in.Pos.File))
			c.Index(wasm.OpGlobalSet, globalFile)
			c.I32Const(int32(in.Pos.Line))
			c.Index(wasm.OpGlobalSet, globalLine)
		}
	case ir.Call:
		_, hasResult := in.Fn.ResultVT()
		fg.call(in.Args, in.Dst, func() { c.Index(wasm.OpCall, fg.g.funcIndex[in.Fn]) }, hasResult)
		if !fg.g.opts.NoLines && !fg.fn.RTL {
			fg.setPos(in.Pos) // nach der Rückkehr gilt wieder die eigene Stelle
		}
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

// ---------------------------------------------------------------------------
// Custom Sections

// nameSection erzeugt die standardisierte Name Section (Funktionsnamen).
func (g *generator) nameSection() wasm.Custom {
	var names []byte
	names = wasm.AppendU32(names, uint32(len(g.m.Funcs)))
	for _, f := range g.m.Funcs {
		names = wasm.AppendU32(names, g.funcIndex[f])
		names = appendName(names, f.Name)
	}
	b := []byte{1}
	b = wasm.AppendU32(b, uint32(len(names)))
	return wasm.Custom{Name: "name", Bytes: append(b, names...)}
}

// linesSection erzeugt wasmpascal.lines: Version, Dateinamen und je Funktion
// Index, Anzeigename, Datei und Zeile der Deklaration. Der Runner übersetzt
// damit die Laufzeitfehler-Globalen in eine Meldung.
func (g *generator) linesSection() wasm.Custom {
	b := wasm.AppendU32(nil, 1)
	b = wasm.AppendU32(b, uint32(len(g.m.Files)))
	for _, f := range g.m.Files {
		b = appendName(b, f)
	}
	b = wasm.AppendU32(b, uint32(len(g.m.Funcs)))
	for _, f := range g.m.Funcs {
		b = wasm.AppendU32(b, g.funcIndex[f])
		b = appendName(b, f.Display)
		b = wasm.AppendU32(b, uint32(f.Pos.File))
		b = wasm.AppendU32(b, uint32(f.Pos.Line))
	}
	return wasm.Custom{Name: "wasmpascal.lines", Bytes: b}
}

func appendName(b []byte, s string) []byte {
	b = wasm.AppendU32(b, uint32(len(s)))
	return append(b, s...)
}
