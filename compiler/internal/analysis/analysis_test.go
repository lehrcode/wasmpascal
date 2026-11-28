package analysis

import (
	"testing"

	"github.com/lehrcode/wasmpascal/compiler/internal/ir"
)

func TestMarkSuspendable(t *testing.T) {
	// main → a → b → (suspend); main → c; d ruft a, wird aber nicht aufgerufen
	mk := func(name string) *ir.Func {
		f := &ir.Func{Name: name}
		f.Entry = f.NewBlock()
		f.Entry.Term = ir.Return{}
		return f
	}
	main, a, b, c, d := mk("main"), mk("a"), mk("b"), mk("c"), mk("d")
	call := func(from, to *ir.Func) { from.Entry.Instrs = append(from.Entry.Instrs, ir.Call{Fn: to}) }
	call(main, a)
	call(a, b)
	call(main, c)
	call(d, a)
	b.Entry.Instrs = append(b.Entry.Instrs, ir.Suspend{Reason: ir.IntConst{}, Arg: ir.IntConst{}, Dst: &ir.Var{Name: "r"}})
	m := &ir.Module{Funcs: []*ir.Func{main, a, b, c, d}}
	MarkSuspendable(m)
	want := map[*ir.Func]bool{main: true, a: true, b: true, c: false, d: true}
	for f, w := range want {
		if f.Suspendable != w {
			t.Errorf("%s.Suspendable = %v, want %v", f.Name, f.Suspendable, w)
		}
	}
	if len(Callers(m)[a]) != 2 {
		t.Errorf("Aufrufer von a: %v", Callers(m)[a])
	}
}

// Ein indirekter Aufruf ist anhaltefähig, wenn eine Routine der
// Funktionstabelle mit derselben Signatur anhalten kann.
func TestMarkSuspendableIndirect(t *testing.T) {
	mk := func(name string, params ...*ir.Var) *ir.Func {
		f := &ir.Func{Name: name, Params: params}
		f.Entry = f.NewBlock()
		f.Entry.Term = ir.Return{}
		return f
	}
	i32 := func() *ir.Var { return &ir.Var{Name: "x", VT: ir.I32, Scalar: true, Param: 0} }
	main, other := mk("main"), mk("other")
	reader, compute := mk("read", i32()), mk("compute")
	reader.Entry.Instrs = append(reader.Entry.Instrs, ir.Suspend{Reason: ir.IntConst{}, Arg: ir.IntConst{}, Dst: &ir.Var{Name: "r"}})
	sigI := &ir.FuncSig{Params: []ir.ValType{ir.I32}}
	sigV := &ir.FuncSig{}
	main.Entry.Instrs = append(main.Entry.Instrs, ir.CallIndirect{Sig: sigI, Index: ir.IntConst{V: 1}, Args: []ir.Expr{ir.IntConst{}}})
	other.Entry.Instrs = append(other.Entry.Instrs, ir.CallIndirect{Sig: sigV, Index: ir.IntConst{V: 2}})
	m := &ir.Module{
		Funcs: []*ir.Func{main, other, reader, compute},
		Table: []*ir.Func{reader, compute},
		Sigs:  map[string]*ir.FuncSig{ir.SigKey(sigI.Params, nil): sigI, ir.SigKey(nil, nil): sigV},
	}
	MarkSuspendable(m)
	want := map[*ir.Func]bool{main: true, other: false, reader: true, compute: false}
	for f, w := range want {
		if f.Suspendable != w {
			t.Errorf("%s.Suspendable = %v, want %v", f.Name, f.Suspendable, w)
		}
	}
	if !sigI.Suspendable || sigV.Suspendable {
		t.Errorf("Signaturen: %v %v", sigI.Suspendable, sigV.Suspendable)
	}
}

// Erreichbarkeit (Etappe 18): Wurzeln sind Hauptprogramm, die Laufzeitroutinen
// des Codegenerators und die Funktionstabelle; Kanten sind direkte Aufrufe.
func TestReachablePrune(t *testing.T) {
	mk := func(name string) *ir.Func {
		f := &ir.Func{Name: name}
		f.Entry = f.NewBlock()
		f.Entry.Term = ir.Return{}
		return f
	}
	main, a, b, tot, tab, susp := mk("main"), mk("a"), mk("b"), mk("tot"), mk("tab"), mk("susp")
	call := func(from, to *ir.Func) { from.Entry.Instrs = append(from.Entry.Instrs, ir.Call{Fn: to}) }
	call(main, a)
	call(a, b)
	call(tot, a) // toter Rumpf ruft a auf, zählt aber nicht
	m := &ir.Module{
		Funcs:         []*ir.Func{main, a, b, tot, tab},
		Main:          main,
		SuspendAction: susp,
		Table:         []*ir.Func{tab},
	}
	reach := Reachable(m)
	for f, want := range map[*ir.Func]bool{main: true, a: true, b: true, tot: false, tab: true, susp: true} {
		if reach[f] != want {
			t.Errorf("Reachable(%s) = %v, want %v", f.Name, reach[f], want)
		}
	}
	Prune(m)
	want := []*ir.Func{main, a, b, tab}
	if len(m.Funcs) != len(want) {
		t.Fatalf("Prune: %d Funktionen, want %d", len(m.Funcs), len(want))
	}
	for i, f := range want {
		if m.Funcs[i] != f {
			t.Errorf("Prune: Funcs[%d] = %s, want %s (Reihenfolge der Deklaration)", i, m.Funcs[i].Name, f.Name)
		}
	}
}
