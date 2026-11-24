// Package parser ist ein Recursive-Descent-Parser für Turbo Pascal 4.0 samt
// den Erweiterungen aus PLAN.md. Er bricht beim ersten Fehler ab.
package parser

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

// Parse liest ein Programm oder eine Unit. Text nach dem abschließenden
// "end." wird wie in TP nicht mehr gelesen.
func Parse(lx *lexer.Lexer) (f ast.File, err error) {
	p := &parser{lx: lx}
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(bailout)
			if !ok {
				panic(r)
			}
			f, err = nil, b.err
		}
	}()
	p.next()
	if p.tok.Kind == lexer.Unit {
		return p.unit(), nil
	}
	return p.program(), nil
}

type bailout struct{ err error }

type parser struct {
	lx     *lexer.Lexer
	tok    lexer.Token  // aktuelles Token
	peeked *lexer.Token // ein Token Vorschau, falls angefordert
}

// ---------------------------------------------------------------------------
// Token-Verwaltung und Fehler

func (p *parser) next() {
	if p.peeked != nil {
		p.tok, p.peeked = *p.peeked, nil
		return
	}
	tok, err := p.lx.Next()
	if err != nil {
		panic(bailout{err})
	}
	p.tok = tok
}

// peek liefert das Token nach dem aktuellen. Es darf nicht an Stellen benutzt
// werden, an denen das nächste Token im Typmodus gelesen werden muss.
func (p *parser) peek() lexer.Token {
	if p.peeked == nil {
		tok, err := p.lx.Next()
		if err != nil {
			panic(bailout{err})
		}
		p.peeked = &tok
	}
	return *p.peeked
}

// nextType geht weiter und liest das folgende Token im Typmodus, damit ^C dort
// als Zeiger auf C und nicht als Steuerzeichen gilt.
func (p *parser) nextType() {
	if p.peeked != nil {
		panic("parser: nextType nach peek")
	}
	p.lx.SetTypeMode(true)
	p.next()
	p.lx.SetTypeMode(false)
}

func (p *parser) fail(pos source.Pos, code int, detail string, args ...any) {
	panic(bailout{source.Errorf(pos, code, detail, args...)})
}

// failHere meldet code am aktuellen Token; am Dateiende stattdessen Error 10.
func (p *parser) failHere(code int) {
	if p.tok.Kind == lexer.EOF {
		p.fail(p.tok.Pos, source.ErrUnexpectedEOF, "")
	}
	p.fail(p.tok.Pos, code, "")
}

var expectCodes = map[lexer.Kind]int{
	lexer.Ident:          source.ErrIdentExpected,
	lexer.Semicolon:      source.ErrSemicolonExpected,
	lexer.Colon:          source.ErrColonExpected,
	lexer.Comma:          source.ErrCommaExpected,
	lexer.LParen:         source.ErrLParenExpected,
	lexer.RParen:         source.ErrRParenExpected,
	lexer.Eq:             source.ErrEqExpected,
	lexer.Assign:         source.ErrAssignExpected,
	lexer.LBrack:         source.ErrLBrackExpected,
	lexer.RBrack:         source.ErrRBrackExpected,
	lexer.Dot:            source.ErrDotExpected,
	lexer.DotDot:         source.ErrDotDotExpected,
	lexer.Begin:          source.ErrBeginExpected,
	lexer.End:            source.ErrEndExpected,
	lexer.Do:             source.ErrDoExpected,
	lexer.Of:             source.ErrOfExpected,
	lexer.Interface:      source.ErrInterfaceExpected,
	lexer.Then:           source.ErrThenExpected,
	lexer.Implementation: source.ErrImplExpected,
}

// check meldet einen Fehler, wenn das aktuelle Token nicht k ist.
func (p *parser) check(k lexer.Kind) {
	if p.tok.Kind != k {
		code, ok := expectCodes[k]
		if !ok {
			code = source.ErrSyntax
		}
		p.failHere(code)
	}
}

// expect prüft auf k und geht weiter.
func (p *parser) expect(k lexer.Kind) lexer.Token {
	p.check(k)
	tok := p.tok
	p.next()
	return tok
}

func (p *parser) got(k lexer.Kind) bool {
	if p.tok.Kind == k {
		p.next()
		return true
	}
	return false
}

// isDirective meldet, ob das aktuelle Token der Bezeichner name ist (für
// Standarddirektiven wie forward, die keine reservierten Wörter sind).
func (p *parser) isDirective(name string) bool {
	return p.tok.Kind == lexer.Ident && p.tok.Name == name
}

// unsupported meldet Sprachmittel, die WasmPascal bewusst weglässt.
func (p *parser) unsupported() {
	if p.tok.Kind == lexer.Inline {
		p.fail(p.tok.Pos, source.ErrNotSupported, "inline")
	}
	for _, d := range []string{"absolute", "external", "interrupt"} {
		if p.isDirective(d) {
			p.fail(p.tok.Pos, source.ErrNotSupported, "%s", d)
		}
	}
}

// ---------------------------------------------------------------------------
// Bezeichner

func (p *parser) ident() *ast.Ident {
	p.check(lexer.Ident)
	id := &ast.Ident{Base: ast.At(p.tok.Pos), Name: p.tok.Name, Text: p.tok.Text}
	p.next()
	return id
}

func (p *parser) identList() []*ast.Ident {
	ids := []*ast.Ident{p.ident()}
	for p.got(lexer.Comma) {
		ids = append(ids, p.ident())
	}
	return ids
}

// label liest eine Marke: Ziffernfolge oder Bezeichner.
func (p *parser) label() *ast.Ident {
	if p.tok.Kind == lexer.Int && p.tok.Text[0] != '$' {
		id := &ast.Ident{Base: ast.At(p.tok.Pos), Name: p.tok.Text, Text: p.tok.Text}
		p.next()
		return id
	}
	return p.ident()
}

// ---------------------------------------------------------------------------
// Programm und Unit

func (p *parser) program() *ast.Program {
	prog := &ast.Program{Base: ast.At(p.tok.Pos)}
	if p.got(lexer.Program) {
		prog.Name = p.ident()
		if p.got(lexer.LParen) {
			prog.Params = p.identList()
			p.expect(lexer.RParen)
		}
		p.expect(lexer.Semicolon)
	}
	prog.Uses = p.uses()
	prog.Block = p.block()
	p.check(lexer.Dot) // nicht weiterlesen: Text danach wird ignoriert
	return prog
}

func (p *parser) uses() []*ast.Ident {
	if !p.got(lexer.Uses) {
		return nil
	}
	ids := p.identList()
	p.expect(lexer.Semicolon)
	return ids
}

func (p *parser) unit() *ast.Unit {
	u := &ast.Unit{Base: ast.At(p.tok.Pos)}
	p.next()
	u.Name = p.ident()
	p.expect(lexer.Semicolon)
	p.expect(lexer.Interface)
	u.IntfUses = p.uses()
	u.Intf = p.decls(true)
	p.expect(lexer.Implementation)
	u.ImplUses = p.uses()
	u.Impl = p.decls(false)
	if p.tok.Kind == lexer.Begin {
		u.Init = p.compound()
	} else {
		p.expect(lexer.End)
	}
	p.check(lexer.Dot)
	return u
}

// ---------------------------------------------------------------------------
// Deklarationen

func (p *parser) block() *ast.Block {
	b := &ast.Block{Base: ast.At(p.tok.Pos)}
	b.Decls = p.decls(false)
	b.Body = p.compound()
	return b
}

// decls liest Deklarationsteile in beliebiger Reihenfolge. Im Interface-Teil
// (intf) gibt es nur Köpfe von Prozeduren und keine Labels.
func (p *parser) decls(intf bool) []ast.Decl {
	var ds []ast.Decl
	for {
		switch p.tok.Kind {
		case lexer.Label:
			if intf {
				return ds
			}
			ds = append(ds, p.labelDecl())
		case lexer.Const:
			ds = append(ds, p.constSection()...)
		case lexer.Type:
			ds = append(ds, p.typeSection()...)
		case lexer.Var:
			ds = append(ds, p.varSection()...)
		case lexer.Procedure, lexer.Function:
			ds = append(ds, p.procDecl(intf))
		default:
			return ds
		}
	}
}

func (p *parser) labelDecl() ast.Decl {
	d := &ast.LabelDecl{Base: ast.At(p.tok.Pos)}
	p.next()
	d.Labels = []*ast.Ident{p.label()}
	for p.got(lexer.Comma) {
		d.Labels = append(d.Labels, p.label())
	}
	p.expect(lexer.Semicolon)
	return d
}

func (p *parser) constSection() []ast.Decl {
	p.next()
	var ds []ast.Decl
	for first := true; first || p.tok.Kind == lexer.Ident; first = false {
		d := &ast.ConstDecl{Base: ast.At(p.tok.Pos)}
		d.Name = p.ident()
		if p.tok.Kind == lexer.Colon {
			p.nextType()
			d.Type = p.typeExpr()
			p.expect(lexer.Eq)
			d.Value = p.typedConst()
		} else {
			p.expect(lexer.Eq)
			d.Value = p.expr()
		}
		p.expect(lexer.Semicolon)
		ds = append(ds, d)
	}
	return ds
}

func (p *parser) typeSection() []ast.Decl {
	p.next()
	var ds []ast.Decl
	for first := true; first || p.tok.Kind == lexer.Ident; first = false {
		d := &ast.TypeDecl{Base: ast.At(p.tok.Pos)}
		d.Name = p.ident()
		p.check(lexer.Eq)
		p.nextType()
		d.Type = p.typeExpr()
		p.expect(lexer.Semicolon)
		ds = append(ds, d)
	}
	return ds
}

func (p *parser) varSection() []ast.Decl {
	p.next()
	var ds []ast.Decl
	for first := true; first || p.tok.Kind == lexer.Ident; first = false {
		d := &ast.VarDecl{Base: ast.At(p.tok.Pos)}
		d.Names = p.identList()
		p.check(lexer.Colon)
		p.nextType()
		d.Type = p.typeExpr()
		p.unsupported() // absolute
		p.expect(lexer.Semicolon)
		ds = append(ds, d)
	}
	return ds
}

func (p *parser) procDecl(intf bool) ast.Decl {
	d := &ast.ProcDecl{Base: ast.At(p.tok.Pos), IsFunc: p.tok.Kind == lexer.Function}
	p.next()
	d.Name = p.ident()
	if p.tok.Kind == lexer.LParen {
		d.Params = p.params()
	}
	if d.IsFunc && p.got(lexer.Colon) {
		d.Result = p.paramType()
	}
	p.expect(lexer.Semicolon)
	if intf {
		return d
	}
	p.unsupported() // external, interrupt, inline
	if p.isDirective("forward") {
		p.next()
		p.expect(lexer.Semicolon)
		d.Forward = true
		return d
	}
	d.Block = p.block()
	p.expect(lexer.Semicolon)
	return d
}

func (p *parser) params() []*ast.ParamGroup {
	p.expect(lexer.LParen)
	var gs []*ast.ParamGroup
	for {
		g := &ast.ParamGroup{Base: ast.At(p.tok.Pos)}
		switch {
		case p.got(lexer.Var):
			g.Mode = ast.ByVar
		case p.got(lexer.Const):
			g.Mode = ast.ByConst
		}
		g.Names = p.identList()
		if g.Mode == ast.ByValue || p.tok.Kind == lexer.Colon {
			p.expect(lexer.Colon)
			g.Type = p.paramType()
		}
		gs = append(gs, g)
		if !p.got(lexer.Semicolon) {
			break
		}
	}
	p.expect(lexer.RParen)
	return gs
}

// paramType liest den Typ eines Parameters oder Funktionsergebnisses: nur
// Typbezeichner, string oder file.
func (p *parser) paramType() ast.TypeExpr {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case lexer.StringKw:
		p.next()
		return &ast.StringType{Base: ast.At(pos)}
	case lexer.File:
		p.next()
		return &ast.FileType{Base: ast.At(pos)}
	case lexer.Ident:
		return p.namedType()
	}
	p.failHere(source.ErrTypeIdentExpected)
	return nil
}

func (p *parser) namedType() *ast.NamedType {
	t := &ast.NamedType{Base: ast.At(p.tok.Pos), Name: p.ident()}
	if p.got(lexer.Dot) {
		t.Unit, t.Name = t.Name, p.ident()
	}
	return t
}

// ---------------------------------------------------------------------------
// Typen

func (p *parser) typeExpr() ast.TypeExpr {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case lexer.Caret:
		p.next()
		if p.tok.Kind == lexer.StringKw {
			p.next()
			return &ast.PointerType{Base: ast.At(pos), Elem: &ast.StringType{Base: ast.At(pos)}}
		}
		if p.tok.Kind != lexer.Ident {
			p.failHere(source.ErrTypeIdentExpected)
		}
		return &ast.PointerType{Base: ast.At(pos), Elem: p.namedType()}
	case lexer.LParen:
		p.next()
		t := &ast.EnumType{Base: ast.At(pos), Names: p.identList()}
		p.expect(lexer.RParen)
		return t
	case lexer.Packed:
		p.next()
		switch p.tok.Kind {
		case lexer.Array, lexer.Record, lexer.Set, lexer.File:
		default:
			p.failHere(source.ErrInType)
		}
		t := p.typeExpr()
		switch t := t.(type) {
		case *ast.ArrayType:
			t.Packed, t.At = true, pos
		case *ast.RecordType:
			t.Packed, t.At = true, pos
		case *ast.SetType:
			t.Packed, t.At = true, pos
		case *ast.FileType:
			t.Packed, t.At = true, pos
		}
		return t
	case lexer.Array:
		p.next()
		t := &ast.ArrayType{Base: ast.At(pos)}
		p.expect(lexer.LBrack)
		t.Index = []ast.TypeExpr{p.typeExpr()}
		for p.got(lexer.Comma) {
			t.Index = append(t.Index, p.typeExpr())
		}
		p.expect(lexer.RBrack)
		p.check(lexer.Of)
		p.nextType()
		t.Elem = p.typeExpr()
		return t
	case lexer.Record:
		p.next()
		t := &ast.RecordType{Base: ast.At(pos), Fields: p.fieldList()}
		p.expect(lexer.End)
		return t
	case lexer.Set:
		p.next()
		p.expect(lexer.Of)
		return &ast.SetType{Base: ast.At(pos), Elem: p.typeExpr()}
	case lexer.File:
		p.next()
		t := &ast.FileType{Base: ast.At(pos)}
		if p.tok.Kind == lexer.Of {
			p.nextType()
			t.Elem = p.typeExpr()
		}
		return t
	case lexer.StringKw:
		p.next()
		t := &ast.StringType{Base: ast.At(pos)}
		if p.got(lexer.LBrack) {
			t.Len = p.expr()
			p.expect(lexer.RBrack)
		}
		return t
	case lexer.Ident:
		// Typbezeichner (auch Unit.Typ) oder Teilbereich, der mit einem
		// Konstantenbezeichner beginnt. Grenzen ohne Vergleichsoperatoren,
		// damit "const A: T = ..." und "R: 1..9 = 5" richtig enden.
		x := p.simpleExpr(nil)
		if p.got(lexer.DotDot) {
			return &ast.SubrangeType{Base: ast.At(pos), Lo: x, Hi: p.simpleExpr(nil)}
		}
		switch x := x.(type) {
		case *ast.Ident:
			return &ast.NamedType{Base: x.Base, Name: x}
		case *ast.SelectorExpr:
			if u, ok := x.X.(*ast.Ident); ok {
				return &ast.NamedType{Base: u.Base, Unit: u, Name: x.Sel}
			}
		}
		p.fail(pos, source.ErrInType, "")
	case lexer.EOF:
		p.failHere(source.ErrInType)
	}
	// Teilbereich mit Konstante: 1..10, -5..5, 'a'..'z'
	lo := p.simpleExpr(nil)
	p.expect(lexer.DotDot)
	return &ast.SubrangeType{Base: ast.At(pos), Lo: lo, Hi: p.simpleExpr(nil)}
}

// fieldList liest Felder bis end bzw. ")" (Variantenzweig).
func (p *parser) fieldList() *ast.FieldList {
	fl := &ast.FieldList{}
	for p.tok.Kind == lexer.Ident {
		g := &ast.FieldGroup{Base: ast.At(p.tok.Pos), Names: p.identList()}
		p.check(lexer.Colon)
		p.nextType()
		g.Type = p.typeExpr()
		fl.Fixed = append(fl.Fixed, g)
		if !p.got(lexer.Semicolon) {
			break
		}
	}
	if p.tok.Kind == lexer.Case {
		fl.Variant = p.variantPart()
	}
	return fl
}

func (p *parser) variantPart() *ast.VariantPart {
	v := &ast.VariantPart{Base: ast.At(p.tok.Pos)}
	p.next()
	if p.tok.Kind == lexer.Ident && p.peek().Kind == lexer.Colon {
		v.Tag = p.ident()
		p.next()
	}
	if p.tok.Kind != lexer.Ident {
		p.failHere(source.ErrTypeIdentExpected)
	}
	v.TagType = p.namedType()
	p.expect(lexer.Of)
	for p.tok.Kind != lexer.End && p.tok.Kind != lexer.RParen {
		c := &ast.VariantCase{Base: ast.At(p.tok.Pos)}
		c.Labels = []ast.Expr{p.expr()}
		for p.got(lexer.Comma) {
			c.Labels = append(c.Labels, p.expr())
		}
		p.expect(lexer.Colon)
		p.expect(lexer.LParen)
		c.Fields = p.fieldList()
		p.expect(lexer.RParen)
		v.Cases = append(v.Cases, c)
		if !p.got(lexer.Semicolon) {
			break
		}
	}
	return v
}

// typedConst liest den Wert einer typisierten Konstante: Ausdruck,
// Array-Konstante (a, b, c) oder Record-Konstante (x: 1; y: 2).
func (p *parser) typedConst() ast.Expr {
	if p.tok.Kind != lexer.LParen {
		return p.expr()
	}
	pos := p.tok.Pos
	p.next()
	if p.tok.Kind == lexer.Ident && p.peek().Kind == lexer.Colon {
		rc := &ast.RecordConst{Base: ast.At(pos)}
		for p.tok.Kind == lexer.Ident {
			fi := &ast.FieldInit{Base: ast.At(p.tok.Pos), Name: p.ident()}
			p.expect(lexer.Colon)
			fi.Value = p.typedConst()
			rc.Fields = append(rc.Fields, fi)
			if !p.got(lexer.Semicolon) {
				break
			}
		}
		p.expect(lexer.RParen)
		return rc
	}
	first := p.typedConst()
	if p.tok.Kind == lexer.Comma {
		cl := &ast.ConstList{Base: ast.At(pos), Elems: []ast.Expr{first}}
		for p.got(lexer.Comma) {
			cl.Elems = append(cl.Elems, p.typedConst())
		}
		p.expect(lexer.RParen)
		return cl
	}
	p.expect(lexer.RParen)
	// Geklammerter Ausdruck wie (1 + 2) * 3: mit den Operatoren weiterlesen.
	return p.exprFrom(first)
}

// ---------------------------------------------------------------------------
// Anweisungen

func (p *parser) stmtBase() ast.StmtBase {
	return ast.StmtBase{Base: ast.At(p.tok.Pos), IOChecks: p.tok.IOChecks}
}

func (p *parser) compound() *ast.CompoundStmt {
	s := &ast.CompoundStmt{StmtBase: p.stmtBase()}
	p.expect(lexer.Begin)
	s.Stmts = p.stmtList()
	p.endOfList(lexer.End)
	return s
}

// stmtList liest Anweisungen, getrennt durch ";".
func (p *parser) stmtList() []ast.Stmt {
	list := []ast.Stmt{p.stmt()}
	for p.got(lexer.Semicolon) {
		if p.tok.Kind == lexer.Else {
			p.fail(p.tok.Pos, source.ErrInStatement, "")
		}
		list = append(list, p.stmt())
	}
	return list
}

// endOfList erwartet das Ende einer Anweisungsliste (end oder until). Steht
// dort etwas anderes, fehlt in TP-Manier ein ";".
func (p *parser) endOfList(k lexer.Kind) {
	if p.tok.Kind != k {
		p.failHere(source.ErrSemicolonExpected)
	}
	p.next()
}

func (p *parser) stmt() ast.Stmt {
	base := p.stmtBase()
	if p.tok.Kind == lexer.Int || (p.tok.Kind == lexer.Ident && p.peek().Kind == lexer.Colon) {
		s := &ast.LabeledStmt{StmtBase: base, Label: p.label()}
		p.expect(lexer.Colon)
		s.Stmt = p.stmt()
		return s
	}
	switch p.tok.Kind {
	case lexer.Begin:
		return p.compound()
	case lexer.If:
		p.next()
		s := &ast.IfStmt{StmtBase: base, Cond: p.expr()}
		p.expect(lexer.Then)
		s.Then = p.stmt()
		if p.got(lexer.Else) {
			s.Else = p.stmt()
		}
		return s
	case lexer.Case:
		return p.caseStmt(base)
	case lexer.While:
		p.next()
		s := &ast.WhileStmt{StmtBase: base, Cond: p.expr()}
		p.expect(lexer.Do)
		s.Body = p.stmt()
		return s
	case lexer.Repeat:
		p.next()
		s := &ast.RepeatStmt{StmtBase: base, Body: p.stmtList()}
		p.endOfList(lexer.Until)
		s.Cond = p.expr()
		return s
	case lexer.For:
		p.next()
		v := p.ident()
		if p.got(lexer.In) {
			s := &ast.ForInStmt{StmtBase: base, Var: v, In: p.expr()}
			p.expect(lexer.Do)
			s.Body = p.stmt()
			return s
		}
		p.expect(lexer.Assign)
		s := &ast.ForStmt{StmtBase: base, Var: v, From: p.expr()}
		switch p.tok.Kind {
		case lexer.To:
		case lexer.Downto:
			s.Down = true
		default:
			p.failHere(source.ErrToDowntoExpected)
		}
		p.next()
		s.To = p.expr()
		p.expect(lexer.Do)
		s.Body = p.stmt()
		return s
	case lexer.With:
		p.next()
		s := &ast.WithStmt{StmtBase: base, Records: []ast.Expr{p.designator()}}
		for p.got(lexer.Comma) {
			s.Records = append(s.Records, p.designator())
		}
		p.expect(lexer.Do)
		s.Body = p.stmt()
		return s
	case lexer.Goto:
		p.next()
		return &ast.GotoStmt{StmtBase: base, Label: p.label()}
	case lexer.Ident:
		x := p.designator()
		if p.got(lexer.Assign) {
			return &ast.AssignStmt{StmtBase: base, Lhs: x, Rhs: p.expr()}
		}
		switch x.(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr:
			return &ast.CallStmt{StmtBase: base, Call: x}
		}
		p.failHere(source.ErrAssignExpected)
	case lexer.Semicolon, lexer.End, lexer.Until, lexer.Else:
		return &ast.EmptyStmt{StmtBase: base}
	case lexer.Inline:
		p.unsupported()
	}
	p.failHere(source.ErrInStatement)
	return nil
}

func (p *parser) caseStmt(base ast.StmtBase) ast.Stmt {
	p.next()
	s := &ast.CaseStmt{StmtBase: base, X: p.expr()}
	p.expect(lexer.Of)
	for p.tok.Kind != lexer.End && p.tok.Kind != lexer.Else {
		arm := &ast.CaseArm{Base: ast.At(p.tok.Pos)}
		for {
			l := ast.CaseLabel{Lo: p.expr()}
			if p.got(lexer.DotDot) {
				l.Hi = p.expr()
			}
			arm.Labels = append(arm.Labels, l)
			if !p.got(lexer.Comma) {
				break
			}
		}
		p.expect(lexer.Colon)
		arm.Body = p.stmt()
		s.Arms = append(s.Arms, arm)
		if !p.got(lexer.Semicolon) {
			break
		}
	}
	if p.got(lexer.Else) {
		s.HasElse = true
		s.Else = p.stmtList()
	}
	p.endOfList(lexer.End)
	return s
}

// ---------------------------------------------------------------------------
// Ausdrücke

func (p *parser) expr() ast.Expr { return p.exprFrom(nil) }

// exprFrom liest einen Ausdruck; ist f nicht nil, ist es der bereits gelesene
// erste Faktor.
func (p *parser) exprFrom(f ast.Expr) ast.Expr {
	x := p.simpleExpr(f)
	switch p.tok.Kind {
	case lexer.Eq, lexer.Ne, lexer.Lt, lexer.Le, lexer.Gt, lexer.Ge, lexer.In:
		op, pos := p.tok.Kind, p.tok.Pos
		p.next()
		x = &ast.BinaryExpr{Base: ast.At(x.Pos()), OpPos: pos, Op: op, X: x, Y: p.simpleExpr(nil)}
	}
	return x
}

func (p *parser) simpleExpr(f ast.Expr) ast.Expr {
	x := p.term(f)
	for {
		switch p.tok.Kind {
		case lexer.Plus, lexer.Minus, lexer.Or, lexer.Xor:
			op, pos := p.tok.Kind, p.tok.Pos
			p.next()
			x = &ast.BinaryExpr{Base: ast.At(x.Pos()), OpPos: pos, Op: op, X: x, Y: p.term(nil)}
		default:
			return x
		}
	}
}

func (p *parser) term(f ast.Expr) ast.Expr {
	x := f
	if x == nil {
		x = p.factor()
	}
	for {
		switch p.tok.Kind {
		case lexer.Star, lexer.Slash, lexer.Div, lexer.Mod, lexer.And, lexer.Shl, lexer.Shr:
			op, pos := p.tok.Kind, p.tok.Pos
			p.next()
			x = &ast.BinaryExpr{Base: ast.At(x.Pos()), OpPos: pos, Op: op, X: x, Y: p.factor()}
		default:
			return x
		}
	}
}

// factor liest einen Faktor. Vorzeichen binden wie not am stärksten, damit
// wie in TP auch a * -b erlaubt ist.
func (p *parser) factor() ast.Expr {
	pos := p.tok.Pos
	switch p.tok.Kind {
	case lexer.Int:
		x := &ast.IntLit{Base: ast.At(pos), Text: p.tok.Text}
		p.next()
		return x
	case lexer.Real:
		x := &ast.RealLit{Base: ast.At(pos), Text: p.tok.Text}
		p.next()
		return x
	case lexer.String:
		x := &ast.StrLit{Base: ast.At(pos), Value: p.tok.Value}
		p.next()
		return x
	case lexer.Nil:
		p.next()
		return &ast.NilLit{Base: ast.At(pos)}
	case lexer.LBrack:
		return p.setLit()
	case lexer.LParen:
		p.next()
		x := p.expr()
		p.expect(lexer.RParen)
		return x
	case lexer.Not, lexer.Minus, lexer.Plus, lexer.At:
		op := p.tok.Kind
		p.next()
		return &ast.UnaryExpr{Base: ast.At(pos), Op: op, X: p.factor()}
	case lexer.Ident:
		return p.designator()
	}
	p.failHere(source.ErrInExpression)
	return nil
}

func (p *parser) setLit() ast.Expr {
	s := &ast.SetLit{Base: ast.At(p.tok.Pos)}
	p.next()
	if p.got(lexer.RBrack) {
		return s
	}
	for {
		e := ast.SetElem{Lo: p.expr()}
		if p.got(lexer.DotDot) {
			e.Hi = p.expr()
		}
		s.Elems = append(s.Elems, e)
		if !p.got(lexer.Comma) {
			break
		}
	}
	p.expect(lexer.RBrack)
	return s
}

// designator liest Bezeichner mit Selektoren: a.b, a[i], p^, f(x).
func (p *parser) designator() ast.Expr {
	var x ast.Expr = p.ident()
	start := x.Pos()
	for {
		switch p.tok.Kind {
		case lexer.Dot:
			p.next()
			x = &ast.SelectorExpr{Base: ast.At(start), X: x, Sel: p.ident()}
		case lexer.LBrack:
			p.next()
			ix := &ast.IndexExpr{Base: ast.At(start), X: x, Indices: []ast.Expr{p.expr()}}
			for p.got(lexer.Comma) {
				ix.Indices = append(ix.Indices, p.expr())
			}
			p.expect(lexer.RBrack)
			x = ix
		case lexer.Caret:
			p.next()
			x = &ast.DerefExpr{Base: ast.At(start), X: x}
		case lexer.LParen:
			p.next()
			c := &ast.CallExpr{Base: ast.At(start), Fun: x, Args: []ast.Expr{p.arg()}}
			for p.got(lexer.Comma) {
				c.Args = append(c.Args, p.arg())
			}
			c.Rparen = p.tok.Pos
			p.expect(lexer.RParen)
			x = c
		default:
			return x
		}
	}
}

// arg liest ein Aufrufargument, gegebenenfalls mit Formatangabe x:w:d.
func (p *parser) arg() ast.Expr {
	pos := p.tok.Pos
	x := p.expr()
	if !p.got(lexer.Colon) {
		return x
	}
	f := &ast.FormatArg{Base: ast.At(pos), X: x, Width: p.expr()}
	if p.got(lexer.Colon) {
		f.Prec = p.expr()
	}
	return f
}
