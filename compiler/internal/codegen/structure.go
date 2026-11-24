package codegen

import (
	"fmt"
	"sort"

	"github.com/lehrcode/wasmpascal/compiler/internal/ir"
	"github.com/lehrcode/wasmpascal/compiler/internal/wasm"
)

// Strukturierter Kontrollfluss nach N. Ramsey, "Beyond Relooper: Recursive
// Translation of Unstructured Control Flow to Structured Control Flow"
// (ICFP 2022). Voraussetzung ist ein reduzierbarer Graph; das gilt für alle
// Prozeduren ohne goto. Prozeduren mit goto und anhaltefähige Prozeduren
// bekommen ab Etappe 8 eine Dispatch-Schleife.

type cfg struct {
	order    []*ir.Block       // umgekehrte Postordnung der erreichbaren Blöcke
	rpo      map[*ir.Block]int // Position in order
	idom     map[*ir.Block]*ir.Block
	children map[*ir.Block][]*ir.Block // Dominatorbaum
	header   map[*ir.Block]bool        // Ziel einer Rückwärtskante
	merge    map[*ir.Block]bool        // mindestens zwei Vorwärtskanten
}

func analyze(entry *ir.Block) *cfg {
	g := &cfg{
		rpo:      map[*ir.Block]int{},
		idom:     map[*ir.Block]*ir.Block{},
		children: map[*ir.Block][]*ir.Block{},
		header:   map[*ir.Block]bool{},
		merge:    map[*ir.Block]bool{},
	}
	// Postordnung per Tiefensuche
	seen := map[*ir.Block]bool{}
	var post []*ir.Block
	var dfs func(b *ir.Block)
	dfs = func(b *ir.Block) {
		seen[b] = true
		for _, s := range b.Succs() {
			if !seen[s] {
				dfs(s)
			}
		}
		post = append(post, b)
	}
	dfs(entry)
	for i := len(post) - 1; i >= 0; i-- {
		g.rpo[post[i]] = len(g.order)
		g.order = append(g.order, post[i])
	}
	preds := map[*ir.Block][]*ir.Block{}
	for _, b := range g.order {
		for _, s := range b.Succs() {
			preds[s] = append(preds[s], b)
		}
	}
	// Dominatoren nach Cooper, Harvey, Kennedy
	g.idom[entry] = entry
	intersect := func(a, b *ir.Block) *ir.Block {
		for a != b {
			for g.rpo[a] > g.rpo[b] {
				a = g.idom[a]
			}
			for g.rpo[b] > g.rpo[a] {
				b = g.idom[b]
			}
		}
		return a
	}
	for changed := true; changed; {
		changed = false
		for _, b := range g.order[1:] {
			var nd *ir.Block
			for _, p := range preds[b] {
				if g.idom[p] == nil {
					continue
				}
				if nd == nil {
					nd = p
				} else {
					nd = intersect(p, nd)
				}
			}
			if g.idom[b] != nd {
				g.idom[b] = nd
				changed = true
			}
		}
	}
	for _, b := range g.order[1:] {
		g.children[g.idom[b]] = append(g.children[g.idom[b]], b)
	}
	// Rückwärtskanten und Zusammenführungen
	for _, b := range g.order {
		forward := 0
		for _, p := range preds[b] {
			if g.rpo[b] <= g.rpo[p] {
				if !g.dominates(b, p) {
					panic(fmt.Sprintf("codegen: irreduzibler Kontrollfluss bei Block %d", b.ID))
				}
				g.header[b] = true
			} else {
				forward++
			}
		}
		g.merge[b] = forward >= 2
	}
	return g
}

func (g *cfg) dominates(a, b *ir.Block) bool {
	for {
		if a == b {
			return true
		}
		if b == g.idom[b] {
			return false
		}
		b = g.idom[b]
	}
}

// Kontexteinträge: umschließende WASM-Konstrukte von innen nach außen.
type ctxKind int

const (
	ctxIf     ctxKind = iota
	ctxLoop           // loop mit Kopf block (Ziel für Rückwärtssprünge)
	ctxBlockF         // block, auf den block folgt (Ziel für Vorwärtssprünge)
)

type ctxEntry struct {
	kind  ctxKind
	block *ir.Block
}

type structurer struct {
	g    *cfg
	code *wasm.Code
	fg   *funcGen
}

func (s *structurer) depth(ctx []ctxEntry, kind ctxKind, b *ir.Block) uint32 {
	for i := len(ctx) - 1; i >= 0; i-- {
		if ctx[i].kind == kind && ctx[i].block == b {
			return uint32(len(ctx) - 1 - i)
		}
	}
	panic("codegen: Sprungziel nicht im Kontext")
}

func push(ctx []ctxEntry, e ctxEntry) []ctxEntry {
	out := make([]ctxEntry, len(ctx)+1)
	copy(out, ctx)
	out[len(ctx)] = e
	return out
}

func (s *structurer) doTree(x *ir.Block, ctx []ctxEntry) {
	var merges []*ir.Block
	for _, c := range s.g.children[x] {
		if s.g.merge[c] {
			merges = append(merges, c)
		}
	}
	sort.Slice(merges, func(i, j int) bool { return s.g.rpo[merges[i]] > s.g.rpo[merges[j]] })
	if s.g.header[x] {
		s.code.Block(wasm.OpLoop, wasm.BlockEmpty)
		s.nodeWithin(x, merges, push(ctx, ctxEntry{ctxLoop, x}))
		s.code.Op(wasm.OpEnd)
		return
	}
	s.nodeWithin(x, merges, ctx)
}

func (s *structurer) nodeWithin(x *ir.Block, merges []*ir.Block, ctx []ctxEntry) {
	if len(merges) > 0 {
		y := merges[0]
		s.code.Block(wasm.OpBlock, wasm.BlockEmpty)
		s.nodeWithin(x, merges[1:], push(ctx, ctxEntry{ctxBlockF, y}))
		s.code.Op(wasm.OpEnd)
		s.doTree(y, ctx)
		return
	}
	for _, in := range x.Instrs {
		s.fg.instr(in)
	}
	switch t := x.Term.(type) {
	case ir.Jump:
		s.doBranch(x, t.To, ctx)
	case ir.Branch:
		s.fg.expr(t.Cond)
		s.code.Block(wasm.OpIf, wasm.BlockEmpty)
		inner := push(ctx, ctxEntry{kind: ctxIf})
		s.doBranch(x, t.Then, inner)
		s.code.Op(wasm.OpElse)
		s.doBranch(x, t.Else, inner)
		s.code.Op(wasm.OpEnd)
	case ir.Return:
		s.fg.ret()
	case ir.Unreachable:
		s.code.Op(wasm.OpUnreachable)
	default:
		panic(fmt.Sprintf("codegen: Block %d ohne Abschluss", x.ID))
	}
}

func (s *structurer) doBranch(from, to *ir.Block, ctx []ctxEntry) {
	switch {
	case s.g.rpo[to] <= s.g.rpo[from]:
		s.code.Index(wasm.OpBr, s.depth(ctx, ctxLoop, to))
	case s.g.merge[to]:
		s.code.Index(wasm.OpBr, s.depth(ctx, ctxBlockF, to))
	default:
		s.doTree(to, ctx)
	}
}
