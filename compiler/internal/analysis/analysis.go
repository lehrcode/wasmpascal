// Package analysis untersucht die IR des ganzen Programms: Aufrufgraph und
// Anhaltefähigkeit (PLAN.md, Scheduler).
package analysis

import "github.com/lehrcode/wasmpascal/compiler/internal/ir"

// CallGraph ordnet jeder Funktion ihre direkten Aufrufer zu.
type CallGraph map[*ir.Func][]*ir.Func

// Callers baut den Aufrufgraph (Aufgerufener → Aufrufer) über alle Funktionen,
// einschließlich der Laufzeit-Units. Ein indirekter Aufruf (prozedurale
// Typen) gilt als Aufruf jeder Routine der Funktionstabelle mit derselben
// WASM-Signatur.
func Callers(m *ir.Module) CallGraph {
	bySig := tableBySig(m)
	g := CallGraph{}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				switch c := in.(type) {
				case ir.Call:
					g[c.Fn] = append(g[c.Fn], f)
				case ir.CallIndirect:
					for _, callee := range bySig[c.Sig] {
						g[callee] = append(g[callee], f)
					}
				}
			}
		}
	}
	return g
}

// tableBySig ordnet die Routinen der Funktionstabelle den Signaturen
// indirekter Aufrufe zu.
func tableBySig(m *ir.Module) map[*ir.FuncSig][]*ir.Func {
	bySig := map[*ir.FuncSig][]*ir.Func{}
	for _, f := range m.Table {
		if fs := m.Sigs[ir.SigKey(f.WasmSig())]; fs != nil {
			bySig[fs] = append(bySig[fs], f)
		}
	}
	return bySig
}

// MarkSuspendable setzt Func.Suspendable für jede Funktion, die direkt (über
// einen Anhaltepunkt ir.Suspend) oder transitiv anhalten kann, und
// FuncSig.Suspendable für Signaturen, über die eine anhaltefähige Routine
// indirekt aufgerufen werden kann. Alle anderen werden normal und
// strukturiert übersetzt und kosten nichts extra.
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
	for fs, fns := range tableBySig(m) {
		fs.Suspendable = false
		for _, f := range fns {
			if f.Suspendable {
				fs.Suspendable = true
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

// Reachable bestimmt die von den Wurzeln aus über ir.Call erreichbaren
// Funktionen (PLAN.md, Etappe 18). Wurzeln sind das Hauptprogramm, die vom
// Codegenerator selbst aufgerufenen Laufzeitroutinen (SuspendAction,
// ResumeAction, ErrorExit) und jede Routine der Funktionstabelle (ihre
// Adresse wird genommen; indirekte Aufrufe brauchen deshalb keine eigene
// Behandlung). Verschachtelte Routinen sind nur über ihre umschließende
// Routine erreichbar und fallen mit ihr weg.
func Reachable(m *ir.Module) map[*ir.Func]bool {
	seen := map[*ir.Func]bool{}
	var work []*ir.Func
	add := func(f *ir.Func) {
		if f != nil && !seen[f] {
			seen[f] = true
			work = append(work, f)
		}
	}
	add(m.Main)
	add(m.SuspendAction)
	add(m.ResumeAction)
	add(m.ErrorExit)
	for _, f := range m.Table {
		add(f)
	}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for _, b := range f.Blocks {
			for _, in := range b.Instrs {
				if c, ok := in.(ir.Call); ok {
					add(c.Fn)
				}
			}
		}
	}
	return seen
}

// Prune kürzt Module.Funcs auf die erreichbaren Funktionen; die Reihenfolge
// der Deklaration bleibt. Nicht erreichbare Routinen landen nicht im Modul
// (PLAN.md, Etappe 18). Die Typprüfung ist davon nicht betroffen: Sie lief
// vor dem Lowering und meldet Fehler auch in ungenutzten Routinen.
func Prune(m *ir.Module) {
	reach := Reachable(m)
	out := make([]*ir.Func, 0, len(m.Funcs))
	for _, f := range m.Funcs {
		if reach[f] {
			out = append(out, f)
		}
	}
	m.Funcs = out
}
