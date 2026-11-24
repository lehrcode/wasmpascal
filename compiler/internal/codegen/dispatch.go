package codegen

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/ir"
	"github.com/lehrcode/wasmpascal/compiler/internal/wasm"
)

// Dispatch-Schleife (PLAN.md, Scheduler und goto): Der Rumpf wird nicht
// strukturiert, sondern als loop mit br_table über eine Zustandsvariable $st
// erzeugt. Jeder Basisblock ist ein Segment; in anhaltefähigen Funktionen
// beginnt zusätzlich vor jedem Aufruf einer anhaltefähigen Funktion und vor
// jedem Anhaltepunkt ein neues Segment, damit genau dort fortgesetzt werden
// kann.
//
// Abwickeln: Nach einem solchen Aufruf prüft die Funktion den Zustand
// state = 1 (unwinding). Dann sichert sie $st im Frame und kehrt sofort
// zurück, ohne den Frame freizugeben. Fortsetzen: Der Treiber setzt
// state = 2 (rewinding), den Schatten-Stack zurück und ruft das
// Hauptprogramm erneut. Weil die Aufrufkette dieselbe ist, liegen alle Frames
// wieder an denselben Adressen. Jede Funktion lädt $st aus ihrem Frame,
// springt zum gesicherten Segment und ruft dort erneut auf, bis der
// Anhaltepunkt erreicht ist; er setzt state = 0 und übernimmt das Ergebnis.

// Zustände des Schedulers (Global globalState).
const (
	stateNormal    = 0
	stateUnwinding = 1
	stateRewinding = 2
)

type segment struct {
	block      *ir.Block
	start, end int // Anweisungen [start, end)
	last       bool
}

// isDispatch meldet, ob eine Funktion als Dispatch-Schleife übersetzt wird.
func isDispatch(f *ir.Func) bool { return f.Suspendable || f.HasGoto }

// resumePoint meldet Anweisungen, vor denen eine anhaltefähige Funktion ein
// neues Segment beginnt.
func resumePoint(in ir.Instr) bool {
	switch in := in.(type) {
	case ir.Suspend:
		return true
	case ir.Call:
		return in.Fn.Suspendable
	}
	return false
}

// segments zerlegt die erreichbaren Blöcke in Segmente; first liefert das
// erste Segment jedes Blocks. Der Eintrittsblock ist Segment 0.
func segments(f *ir.Func) (segs []segment, first map[*ir.Block]int) {
	first = map[*ir.Block]int{}
	var order []*ir.Block
	seen := map[*ir.Block]bool{}
	var visit func(b *ir.Block)
	visit = func(b *ir.Block) {
		seen[b] = true
		order = append(order, b)
		for _, s := range b.Succs() {
			if !seen[s] {
				visit(s)
			}
		}
	}
	visit(f.Entry)
	for _, b := range order {
		first[b] = len(segs)
		start := 0
		for i, in := range b.Instrs {
			if i > start && f.Suspendable && resumePoint(in) {
				segs = append(segs, segment{block: b, start: start, end: i})
				start = i
			}
		}
		segs = append(segs, segment{block: b, start: start, end: len(b.Instrs), last: true})
	}
	return segs, first
}

// dispatch erzeugt den Rumpf als Dispatch-Schleife.
func (fg *funcGen) dispatch() {
	c := &fg.code
	segs, first := segments(fg.fn)
	n := len(segs)

	// $st: beim Fortsetzen aus dem Frame, sonst Segment 0
	if fg.fn.Suspendable {
		c.Index(wasm.OpGlobalGet, globalState)
		c.I32Const(stateRewinding)
		c.Op(wasm.OpI32Eq)
		c.Block(wasm.OpIf, wasm.BlockEmpty)
		c.Index(wasm.OpLocalGet, fg.fp)
		c.Mem(wasm.OpI32Load, 2, uint32(fg.g.stateSlot[fg.fn]))
		c.Index(wasm.OpLocalSet, fg.st)
		c.Op(wasm.OpElse)
		c.I32Const(0)
		c.Index(wasm.OpLocalSet, fg.st)
		c.Op(wasm.OpEnd)
	} else {
		c.I32Const(0)
		c.Index(wasm.OpLocalSet, fg.st)
	}

	c.Block(wasm.OpLoop, wasm.BlockEmpty)
	for i := 0; i < n; i++ {
		c.Block(wasm.OpBlock, wasm.BlockEmpty)
	}
	c.Index(wasm.OpLocalGet, fg.st)
	c.B = append(c.B, wasm.OpBrTable)
	c.B = wasm.AppendU32(c.B, uint32(n))
	for i := 0; i < n; i++ {
		c.B = wasm.AppendU32(c.B, uint32(i))
	}
	c.B = wasm.AppendU32(c.B, uint32(n-1))

	for k, seg := range segs {
		c.Op(wasm.OpEnd) // schließt Block k; dahinter steht Segment k
		fg.loopDepth = uint32(n - 1 - k)
		fg.segment(k, seg, first)
	}
	c.Op(wasm.OpEnd) // loop
}

// goSeg springt zum Segment j über den Schleifenkopf.
func (fg *funcGen) goSeg(j int, extra uint32) {
	c := &fg.code
	c.I32Const(int32(j))
	c.Index(wasm.OpLocalSet, fg.st)
	c.Index(wasm.OpBr, fg.loopDepth+extra)
}

func (fg *funcGen) segment(k int, seg segment, first map[*ir.Block]int) {
	c := &fg.code
	for i := seg.start; i < seg.end; i++ {
		switch in := seg.block.Instrs[i].(type) {
		case ir.Suspend:
			fg.suspend(in)
		case ir.Call:
			fg.instr(in)
			if in.Fn.Suspendable && fg.fn.Suspendable {
				// abwickeln, wenn der Aufgerufene angehalten hat
				c.Index(wasm.OpGlobalGet, globalState)
				c.I32Const(stateUnwinding)
				c.Op(wasm.OpI32Eq)
				c.Block(wasm.OpIf, wasm.BlockEmpty)
				fg.unwind()
				c.Op(wasm.OpEnd)
			}
		default:
			fg.instr(in)
		}
	}
	if !seg.last {
		fg.goSeg(k+1, 0)
		return
	}
	switch t := seg.block.Term.(type) {
	case ir.Jump:
		fg.goSeg(first[t.To], 0)
	case ir.Branch:
		fg.expr(t.Cond)
		c.Block(wasm.OpIf, wasm.BlockEmpty)
		fg.goSeg(first[t.Then], 1)
		c.Op(wasm.OpElse)
		fg.goSeg(first[t.Else], 1)
		c.Op(wasm.OpEnd)
		c.Op(wasm.OpUnreachable)
	case ir.Return:
		fg.ret()
	case ir.Trap:
		fg.trap(t)
	default:
		c.Op(wasm.OpUnreachable)
	}
}

// unwind sichert $st im Frame und kehrt zurück, ohne den Frame freizugeben.
func (fg *funcGen) unwind() {
	c := &fg.code
	c.Index(wasm.OpLocalGet, fg.fp)
	c.Index(wasm.OpLocalGet, fg.st)
	c.Mem(wasm.OpI32Store, 2, uint32(fg.g.stateSlot[fg.fn]))
	if vt, ok := fg.fn.ResultVT(); ok {
		if vt == ir.F64 {
			c.F64Const(0)
		} else {
			c.I32Const(0)
		}
	}
	c.Op(wasm.OpReturn)
}

// suspend übersetzt einen Anhaltepunkt.
func (fg *funcGen) suspend(in ir.Suspend) {
	c := &fg.code
	c.Index(wasm.OpGlobalGet, globalState)
	c.I32Const(stateRewinding)
	c.Op(wasm.OpI32Eq)
	c.Block(wasm.OpIf, wasm.BlockEmpty)
	// fortgesetzt: Ergebnis des Hosts übernehmen
	c.I32Const(stateNormal)
	c.Index(wasm.OpGlobalSet, globalState)
	fg.set(in.Dst, func() { c.Index(wasm.OpGlobalGet, globalResume) })
	c.Op(wasm.OpElse)
	// anhalten: Grund und Argument für den Host, dann abwickeln
	fg.expr(in.Reason)
	c.Index(wasm.OpGlobalSet, globalReason)
	fg.expr(in.Arg)
	c.Index(wasm.OpGlobalSet, globalArg)
	c.I32Const(stateUnwinding)
	c.Index(wasm.OpGlobalSet, globalState)
	fg.unwind()
	c.Op(wasm.OpEnd)
}

// driver erzeugt _start für wasi: Es ruft das Hauptprogramm, bis es nicht
// mehr anhält; nach jedem Anhalten führt die Laufzeit (_SuspendAction) die
// Aktion aus, dann wird fortgesetzt.
func (g *generator) driver() *wasm.Func {
	var c wasm.Code
	c.Block(wasm.OpLoop, wasm.BlockEmpty)
	c.Index(wasm.OpCall, g.funcIndex[g.m.Main])
	c.Index(wasm.OpGlobalGet, globalState)
	c.I32Const(stateUnwinding)
	c.Op(wasm.OpI32Eq)
	c.Block(wasm.OpIf, wasm.BlockEmpty)
	c.I32Const(stateNormal)
	c.Index(wasm.OpGlobalSet, globalState)
	c.Index(wasm.OpGlobalGet, globalReason)
	c.Index(wasm.OpGlobalGet, globalArg)
	c.Index(wasm.OpCall, g.funcIndex[g.m.SuspendAction])
	c.Index(wasm.OpGlobalSet, globalResume)
	c.I32Const(stateRewinding)
	c.Index(wasm.OpGlobalSet, globalState)
	c.I32Const(int32(g.stackTop))
	c.Index(wasm.OpGlobalSet, globalSP)
	c.Index(wasm.OpBr, 1)
	c.Op(wasm.OpEnd, wasm.OpEnd, wasm.OpEnd)
	return &wasm.Func{Type: g.mod.TypeIndex(wasm.FuncType{}), Code: c.B}
}

// gojsExports erzeugt die Schnittstelle von GOOS=js GOARCH=wasm für
// wasm_exec.js: run(argc, argv) startet das Programm, resume() setzt es nach
// einem Timer oder einer Eingabe fort, getsp() liefert den Go-Stackzeiger der
// Laufzeit, mem ist der Speicher. Beide kehren zurück, wenn das Programm
// anhält; die Laufzeit (_SuspendAction) hat dann einen Timer gesetzt, eine
// Eingabe angefordert oder das Programm mit wasmExit beendet.
func (g *generator) gojsExports() {
	m, mod := g.m, g.mod
	// Nach dem Lauf: angehalten → Aktion; fertig → wie Halt(0)
	after := func(c *wasm.Code) {
		c.Index(wasm.OpGlobalGet, globalState)
		c.I32Const(stateUnwinding)
		c.Op(wasm.OpI32Eq)
		c.Block(wasm.OpIf, wasm.BlockEmpty)
		c.I32Const(stateNormal)
		c.Index(wasm.OpGlobalSet, globalState)
		c.Index(wasm.OpGlobalGet, globalReason)
		c.Index(wasm.OpGlobalGet, globalArg)
		c.Index(wasm.OpCall, g.funcIndex[m.SuspendAction])
		c.Op(wasm.OpDrop)
		c.Op(wasm.OpElse)
		c.I32Const(suspendHalt)
		c.I32Const(0)
		c.Index(wasm.OpCall, g.funcIndex[m.SuspendAction])
		c.Op(wasm.OpDrop)
		c.Op(wasm.OpEnd)
	}
	var run wasm.Code
	run.Index(wasm.OpLocalGet, 0)
	run.Index(wasm.OpGlobalSet, globalArgc)
	run.Index(wasm.OpLocalGet, 1)
	run.Index(wasm.OpGlobalSet, globalArgv)
	run.Index(wasm.OpCall, g.funcIndex[m.Main])
	after(&run)
	run.Op(wasm.OpEnd)

	var resume wasm.Code
	if m.ResumeAction != nil {
		resume.Index(wasm.OpCall, g.funcIndex[m.ResumeAction])
		resume.Index(wasm.OpGlobalSet, globalResume)
	}
	resume.I32Const(stateRewinding)
	resume.Index(wasm.OpGlobalSet, globalState)
	resume.I32Const(int32(g.stackTop))
	resume.Index(wasm.OpGlobalSet, globalSP)
	resume.Index(wasm.OpCall, g.funcIndex[m.Main])
	after(&resume)
	resume.Op(wasm.OpEnd)

	var getsp wasm.Code
	getsp.Index(wasm.OpGlobalGet, globalGoSP)
	getsp.Op(wasm.OpEnd)

	base := uint32(len(mod.Imports) + len(mod.Funcs))
	mod.Funcs = append(mod.Funcs,
		&wasm.Func{Type: mod.TypeIndex(wasm.FuncType{Params: []wasm.ValType{wasm.I32, wasm.I32}}), Code: run.B},
		&wasm.Func{Type: mod.TypeIndex(wasm.FuncType{}), Code: resume.B},
		&wasm.Func{Type: mod.TypeIndex(wasm.FuncType{Results: []wasm.ValType{wasm.I32}}), Code: getsp.B},
	)
	mod.Exports = []wasm.Export{
		{Name: "mem", Kind: wasm.ExportMemory, Index: 0},
		{Name: "run", Kind: wasm.ExportFunc, Index: base},
		{Name: "resume", Kind: wasm.ExportFunc, Index: base + 1},
		{Name: "getsp", Kind: wasm.ExportFunc, Index: base + 2},
	}
}

// suspendHalt ist der Grund „Programm beendet“ (rtl: SuspendHalt).
const suspendHalt = 4
