package ast

import (
	"fmt"
	"strings"

	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
)

// Sprint gibt den Baum als eingerückten S-Ausdruck aus (für Golden-Tests).
// Bezeichner erscheinen normalisiert, Strings in Go-Schreibweise. Anweisungen,
// die unter {$I-} stehen, werden mit (io-off ...) umschlossen.
func Sprint(n Node) string {
	var b strings.Builder
	render(&b, toSx(n), 0)
	b.WriteByte('\n')
	return b.String()
}

// sx ist ein S-Ausdruck: ein Atom oder eine Liste mit optionalem Kopf.
type sx struct {
	list bool
	atom string // bei Atomen der Text, bei Listen der Kopf ("" = ohne Kopf)
	kids []sx
}

func a(s string) sx                { return sx{atom: s} }
func l(head string, kids ...sx) sx { return sx{list: true, atom: head, kids: kids} }
func la(head string, kids []sx) sx { return sx{list: true, atom: head, kids: kids} }

const lineWidth = 80

func flat(x sx) string {
	if !x.list {
		return x.atom
	}
	var parts []string
	if x.atom != "" {
		parts = append(parts, x.atom)
	}
	for _, k := range x.kids {
		parts = append(parts, flat(k))
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// render schreibt x ab der Einrückung indent. Passt x nicht in die Zeile,
// bleiben Kopf und führende Atome oben, jede weitere Liste kommt auf eine
// eigene Zeile.
func render(b *strings.Builder, x sx, indent int) {
	f := flat(x)
	if !x.list || indent+len(f) <= lineWidth {
		b.WriteString(f)
		return
	}
	b.WriteString("(" + x.atom)
	sep := x.atom != ""
	i := 0
	for ; i < len(x.kids) && !x.kids[i].list; i++ {
		if sep {
			b.WriteByte(' ')
		}
		b.WriteString(x.kids[i].atom)
		sep = true
	}
	for ; i < len(x.kids); i++ {
		if sep || i > 0 {
			b.WriteString("\n" + strings.Repeat(" ", indent+2))
		}
		render(b, x.kids[i], indent+2)
		sep = true
	}
	b.WriteByte(')')
}

func toSx(n Node) sx {
	switch n := n.(type) {
	case nil:
		return a("_")
	case Expr:
		return exprSx(n)
	case TypeExpr:
		return typeSx(n)
	case Decl:
		return declSx(n)
	case Stmt:
		return stmtSx(n)
	case *Program:
		kids := []sx{identOr(n.Name, "_")}
		if n.Params != nil {
			kids = append(kids, la("params", idents(n.Params)))
		}
		if n.Uses != nil {
			kids = append(kids, la("uses", idents(n.Uses)))
		}
		kids = append(kids, blockSx(n.Block)...)
		return la("program", kids)
	case *Unit:
		intf := []sx{}
		if n.IntfUses != nil {
			intf = append(intf, la("uses", idents(n.IntfUses)))
		}
		for _, d := range n.Intf {
			intf = append(intf, declSx(d))
		}
		impl := []sx{}
		if n.ImplUses != nil {
			impl = append(impl, la("uses", idents(n.ImplUses)))
		}
		for _, d := range n.Impl {
			impl = append(impl, declSx(d))
		}
		kids := []sx{a(n.Name.Name), la("interface", intf), la("implementation", impl)}
		if n.Init != nil {
			kids = append(kids, la("init", stmts(n.Init.Stmts)))
		}
		return la("unit", kids)
	case *Block:
		return la("block", blockSx(n))
	}
	panic(fmt.Sprintf("ast.Sprint: unbekannter Knoten %T", n))
}

func blockSx(b *Block) []sx {
	var kids []sx
	for _, d := range b.Decls {
		kids = append(kids, declSx(d))
	}
	return append(kids, stmtSx(b.Body))
}

func identOr(id *Ident, def string) sx {
	if id == nil {
		return a(def)
	}
	return a(id.Name)
}

func idents(ids []*Ident) []sx {
	out := make([]sx, len(ids))
	for i, id := range ids {
		out[i] = a(id.Name)
	}
	return out
}

func exprs(xs []Expr) []sx {
	out := make([]sx, len(xs))
	for i, x := range xs {
		out[i] = exprSx(x)
	}
	return out
}

func stmts(ss []Stmt) []sx {
	out := make([]sx, len(ss))
	for i, s := range ss {
		out[i] = stmtSx(s)
	}
	return out
}

func opName(k lexer.Kind) string { return k.String() }

func exprSx(x Expr) sx {
	switch x := x.(type) {
	case nil:
		return a("_")
	case *Ident:
		return a(x.Name)
	case *IntLit:
		return a(x.Text)
	case *RealLit:
		return a(x.Text)
	case *StrLit:
		return a(fmt.Sprintf("%q", x.Value))
	case *NilLit:
		return a("nil")
	case *SetLit:
		kids := []sx{}
		for _, e := range x.Elems {
			if e.Hi != nil {
				kids = append(kids, l("..", exprSx(e.Lo), exprSx(e.Hi)))
			} else {
				kids = append(kids, exprSx(e.Lo))
			}
		}
		return la("set", kids)
	case *UnaryExpr:
		return l(opName(x.Op), exprSx(x.X))
	case *BinaryExpr:
		return l(opName(x.Op), exprSx(x.X), exprSx(x.Y))
	case *CallExpr:
		return la("call", append([]sx{exprSx(x.Fun)}, exprs(x.Args)...))
	case *FormatArg:
		kids := []sx{exprSx(x.X), exprSx(x.Width)}
		if x.Prec != nil {
			kids = append(kids, exprSx(x.Prec))
		}
		return la("fmt", kids)
	case *IndexExpr:
		return la("index", append([]sx{exprSx(x.X)}, exprs(x.Indices)...))
	case *SelectorExpr:
		return l(".", exprSx(x.X), a(x.Sel.Name))
	case *DerefExpr:
		return l("^", exprSx(x.X))
	case *ConstList:
		return la("list", exprs(x.Elems))
	case *RecordConst:
		kids := []sx{}
		for _, f := range x.Fields {
			kids = append(kids, l(f.Name.Name, exprSx(f.Value)))
		}
		return la("record-const", kids)
	}
	panic(fmt.Sprintf("ast.Sprint: unbekannter Ausdruck %T", x))
}

func packed(p bool, name string) string {
	if p {
		return "packed-" + name
	}
	return name
}

func typeSx(t TypeExpr) sx {
	switch t := t.(type) {
	case nil:
		return a("_")
	case *NamedType:
		if t.Unit != nil {
			return a(t.Unit.Name + "." + t.Name.Name)
		}
		return a(t.Name.Name)
	case *StringType:
		if t.Len == nil {
			return a("string")
		}
		return l("string", exprSx(t.Len))
	case *SubrangeType:
		return l("..", exprSx(t.Lo), exprSx(t.Hi))
	case *EnumType:
		return la("enum", idents(t.Names))
	case *ArrayType:
		idx := make([]sx, len(t.Index))
		for i, it := range t.Index {
			idx[i] = typeSx(it)
		}
		return l(packed(t.Packed, "array"), la("", idx), typeSx(t.Elem))
	case *RecordType:
		return la(packed(t.Packed, "record"), fieldsSx(t.Fields))
	case *SetType:
		return l(packed(t.Packed, "set-of"), typeSx(t.Elem))
	case *FileType:
		if t.Elem == nil {
			return a(packed(t.Packed, "file"))
		}
		return l(packed(t.Packed, "file-of"), typeSx(t.Elem))
	case *PointerType:
		return l("^", typeSx(t.Elem))
	case *ProcType:
		head := "procedure-type"
		if t.IsFunc {
			head = "function-type"
		}
		kids := []sx{}
		if t.Params != nil {
			kids = append(kids, paramsSx(t.Params))
		}
		if t.Result != nil {
			kids = append(kids, l("result", typeSx(t.Result)))
		}
		return la(head, kids)
	}
	panic(fmt.Sprintf("ast.Sprint: unbekannter Typ %T", t))
}

func fieldsSx(fl *FieldList) []sx {
	kids := []sx{}
	for _, g := range fl.Fixed {
		kids = append(kids, l("field", la("", idents(g.Names)), typeSx(g.Type)))
	}
	if v := fl.Variant; v != nil {
		vk := []sx{identOr(v.Tag, "_"), typeSx(v.TagType)}
		for _, c := range v.Cases {
			vk = append(vk, la("", append([]sx{la("", exprs(c.Labels))}, fieldsSx(c.Fields)...)))
		}
		kids = append(kids, la("case", vk))
	}
	return kids
}

var modeNames = map[ParamMode]string{ByValue: "value", ByVar: "var", ByConst: "const"}

func declSx(d Decl) sx {
	switch d := d.(type) {
	case *LabelDecl:
		return la("label", idents(d.Labels))
	case *ConstDecl:
		if d.Type != nil {
			return l("const", a(d.Name.Name), typeSx(d.Type), exprSx(d.Value))
		}
		return l("const", a(d.Name.Name), exprSx(d.Value))
	case *TypeDecl:
		return l("type", a(d.Name.Name), typeSx(d.Type))
	case *VarDecl:
		return l("var", la("", idents(d.Names)), typeSx(d.Type))
	case *ProcDecl:
		head := "procedure"
		if d.IsFunc {
			head = "function"
		}
		kids := []sx{a(d.Name.Name)}
		if d.Params != nil {
			kids = append(kids, paramsSx(d.Params))
		}
		if d.Result != nil {
			kids = append(kids, l("result", typeSx(d.Result)))
		}
		switch {
		case d.External != nil:
			ext := []sx{a(fmt.Sprintf("%q", d.External.Module))}
			if d.External.Name != "" {
				ext = append(ext, a(fmt.Sprintf("%q", d.External.Name)))
			}
			kids = append(kids, la("external", ext))
		case d.Forward:
			kids = append(kids, a("forward"))
		case d.Block != nil:
			kids = append(kids, blockSx(d.Block)...)
		}
		return la(head, kids)
	}
	panic(fmt.Sprintf("ast.Sprint: unbekannte Deklaration %T", d))
}

func paramsSx(gs []*ParamGroup) sx {
	ps := []sx{}
	for _, g := range gs {
		ps = append(ps, l(modeNames[g.Mode], la("", idents(g.Names)), typeSx(g.Type)))
	}
	return la("params", ps)
}

func stmtSx(s Stmt) sx {
	x := stmtSx0(s)
	if base, ok := stmtBase(s); ok && !base.IOChecks {
		return l("io-off", x)
	}
	return x
}

func stmtBase(s Stmt) (StmtBase, bool) {
	switch s := s.(type) {
	case *CompoundStmt:
		return s.StmtBase, true
	case *EmptyStmt:
		return s.StmtBase, true
	case *AssignStmt:
		return s.StmtBase, true
	case *CallStmt:
		return s.StmtBase, true
	case *IfStmt:
		return s.StmtBase, true
	case *CaseStmt:
		return s.StmtBase, true
	case *WhileStmt:
		return s.StmtBase, true
	case *RepeatStmt:
		return s.StmtBase, true
	case *ForStmt:
		return s.StmtBase, true
	case *ForInStmt:
		return s.StmtBase, true
	case *WithStmt:
		return s.StmtBase, true
	case *GotoStmt:
		return s.StmtBase, true
	case *LabeledStmt:
		return s.StmtBase, true
	}
	return StmtBase{}, false
}

func stmtSx0(s Stmt) sx {
	switch s := s.(type) {
	case *CompoundStmt:
		return la("begin", stmts(s.Stmts))
	case *EmptyStmt:
		return l("empty")
	case *AssignStmt:
		return l(":=", exprSx(s.Lhs), exprSx(s.Rhs))
	case *CallStmt:
		if c, ok := s.Call.(*CallExpr); ok {
			return exprSx(c)
		}
		return l("call", exprSx(s.Call))
	case *IfStmt:
		kids := []sx{exprSx(s.Cond), stmtSx(s.Then)}
		if s.Else != nil {
			kids = append(kids, stmtSx(s.Else))
		}
		return la("if", kids)
	case *CaseStmt:
		kids := []sx{exprSx(s.X)}
		for _, arm := range s.Arms {
			ls := []sx{}
			for _, lb := range arm.Labels {
				if lb.Hi != nil {
					ls = append(ls, l("..", exprSx(lb.Lo), exprSx(lb.Hi)))
				} else {
					ls = append(ls, exprSx(lb.Lo))
				}
			}
			kids = append(kids, l("arm", la("", ls), stmtSx(arm.Body)))
		}
		if s.HasElse {
			kids = append(kids, la("else", stmts(s.Else)))
		}
		return la("case", kids)
	case *WhileStmt:
		return l("while", exprSx(s.Cond), stmtSx(s.Body))
	case *RepeatStmt:
		return la("repeat", append(stmts(s.Body), l("until", exprSx(s.Cond))))
	case *ForStmt:
		dir := "to"
		if s.Down {
			dir = "downto"
		}
		return l("for", a(s.Var.Name), exprSx(s.From), a(dir), exprSx(s.To), stmtSx(s.Body))
	case *ForInStmt:
		return l("for-in", a(s.Var.Name), exprSx(s.In), stmtSx(s.Body))
	case *WithStmt:
		return l("with", la("", exprs(s.Records)), stmtSx(s.Body))
	case *GotoStmt:
		return l("goto", a(s.Label.Name))
	case *LabeledStmt:
		return l("labeled", a(s.Label.Name), stmtSx(s.Stmt))
	}
	panic(fmt.Sprintf("ast.Sprint: unbekannte Anweisung %T", s))
}
