// Package analysis untersucht die IR des ganzen Programms: Aufrufgraph und
// Anhaltefähigkeit (PLAN.md, Scheduler).
package analysis

import "github.com/lehrcode/wasmpascal/compiler/internal/ir"

// CallGraph ordnet jeder Funktion ihre direkten Aufrufer zu.
type CallGraph map[*ir.Func][]*ir.Func

// Callers baut den Aufrufgraph (Aufgerufener → Aufrufer) über alle Funktionen,
// einschließlich der Laufzeit-Units.
func Callers(m *ir.Module) CallGraph {
	g := CallGraph{}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if c, ok := in.(ir.Call); ok {
					g[c.Fn] = append(g[c.Fn], f)
				}
			}
		}
	}
	return g
}

// MarkSuspendable setzt Func.Suspendable für jede Funktion, die direkt (über
// einen Anhaltepunkt ir.Suspend) oder transitiv anhalten kann. Alle anderen
// werden normal und strukturiert übersetzt und kosten nichts extra.
func MarkSuspendable(m *ir.Module) {
	var work []*ir.Func
	for _, f := range m.Funcs {
		f.Suspendable = false
		if hasSuspend(f) {
			f.Suspendable = true
			work = append(work, f)
		}
	}
	callers := Callers(m)
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for _, c := range callers[f] {
			if !c.Suspendable {
				c.Suspendable = true
				work = append(work, c)
			}
		}
	}
}

func hasSuspend(f *ir.Func) bool {
	for _, b := range f.Blocks {
		for _, in := range b.Instrs {
			if _, ok := in.(ir.Suspend); ok {
				return true
			}
		}
	}
	return false
}
