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
	lies, rechne := mk("lies", i32()), mk("rechne")
	lies.Entry.Instrs = append(lies.Entry.Instrs, ir.Suspend{Reason: ir.IntConst{}, Arg: ir.IntConst{}, Dst: &ir.Var{Name: "r"}})
	sigI := &ir.FuncSig{Params: []ir.ValType{ir.I32}}
	sigV := &ir.FuncSig{}
	main.Entry.Instrs = append(main.Entry.Instrs, ir.CallIndirect{Sig: sigI, Index: ir.IntConst{V: 1}, Args: []ir.Expr{ir.IntConst{}}})
	other.Entry.Instrs = append(other.Entry.Instrs, ir.CallIndirect{Sig: sigV, Index: ir.IntConst{V: 2}})
	m := &ir.Module{
		Funcs: []*ir.Func{main, other, lies, rechne},
		Table: []*ir.Func{lies, rechne},
		Sigs:  map[string]*ir.FuncSig{ir.SigKey(sigI.Params, nil): sigI, ir.SigKey(nil, nil): sigV},
	}
	MarkSuspendable(m)
	want := map[*ir.Func]bool{main: true, other: false, lies: true, rechne: false}
	for f, w := range want {
		if f.Suspendable != w {
			t.Errorf("%s.Suspendable = %v, want %v", f.Name, f.Suspendable, w)
		}
	}
	if !sigI.Suspendable || sigV.Suspendable {
		t.Errorf("Signaturen: %v %v", sigI.Suspendable, sigV.Suspendable)
	}
}
