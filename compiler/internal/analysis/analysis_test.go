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
