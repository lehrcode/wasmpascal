package check

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

var noPos source.Pos

// decls prüft Deklarationen in Reihenfolge. intf: Interface-Teil einer Unit.
func (c *checker) decls(ds []ast.Decl, intf bool) {
	for i := 0; i < len(ds); i++ {
		switch d := ds[i].(type) {
		case *ast.LabelDecl:
			for _, id := range d.Labels {
				c.declare(id, types.NewLabel(id.Name, id.Text, id.Pos()))
			}
		case *ast.ConstDecl:
			c.constDecl(d)
		case *ast.TypeDecl:
			// Ein type-Teil endet, wenn keine TypeDecl mehr folgt; erst dann
			// werden Zeiger mit Vorwärtsverweis aufgelöst.
			c.typeDecl(d)
			if i+1 == len(ds) || !isTypeDecl(ds[i+1]) {
				c.resolvePending()
			}
		case *ast.VarDecl:
			c.varDecl(d)
		case *ast.ProcDecl:
			c.procDecl(d, intf)
		}
	}
}

func isTypeDecl(d ast.Decl) bool { _, ok := d.(*ast.TypeDecl); return ok }

// ---------------------------------------------------------------------------
// Konstanten

func (c *checker) constDecl(d *ast.ConstDecl) {
	if d.Type == nil {
		x := c.expr(d.Value)
		if x.Mode != Constant {
			c.fail(d.Value.Pos(), source.ErrConstExpected, "")
		}
		c.declare(d.Name, types.NewConst(d.Name.Name, d.Name.Text, d.Name.Pos(), x.Type, x.Val))
		return
	}
	t := c.typeExpr(d.Type)
	c.resolvePending()
	if f, ok := t.(*types.File); ok {
		c.fail(d.Type.Pos(), source.ErrInvalidFileType, "%s", f)
	}
	val := c.typedConst(t, d.Value)
	v := types.NewVar(d.Name.Name, d.Name.Text, d.Name.Pos(), types.TypedConst, t)
	v.ReadOnly, v.Val, v.Owner = true, val, c.proc
	c.declare(d.Name, v)
}

// typedConst wertet den Wert einer typisierten Konstante für den Typ t aus.
func (c *checker) typedConst(t types.Type, e ast.Expr) *types.Value {
	switch t := t.(type) {
	case *types.Array:
		// Ein String passender Länge darf ein packed array of Char belegen.
		if types.Is(t.Elem, types.Char) {
			if _, isList := e.(*ast.ConstList); !isList {
				x := c.expr(e)
				if x.Mode == Constant && x.Val.Kind == types.VString && int64(len(x.Val.Str)) == t.Len() {
					elems := make([]*types.Value, t.Len())
					for i := range elems {
						elems[i] = types.IntVal(int64(x.Val.Str[i]))
					}
					return &types.Value{Kind: types.VList, Elems: elems}
				}
			}
		}
		var elems []ast.Expr
		switch e := e.(type) {
		case *ast.ConstList:
			elems = e.Elems
		default:
			elems = []ast.Expr{e} // (5) für ein Array mit einem Element
		}
		if int64(len(elems)) != t.Len() {
			c.fail(e.Pos(), source.ErrTypeMismatch, "%d elements expected", t.Len())
		}
		v := &types.Value{Kind: types.VList}
		for _, el := range elems {
			v.Elems = append(v.Elems, c.typedConst(t.Elem, el))
		}
		return v
	case *types.Record:
		rc, ok := e.(*ast.RecordConst)
		if !ok {
			c.fail(e.Pos(), source.ErrTypeMismatch, "record constant expected")
		}
		v := &types.Value{Kind: types.VRecord, Elems: make([]*types.Value, len(t.Fields))}
		last := -1
		for _, fi := range rc.Fields {
			idx := -1
			for i, f := range t.Fields {
				if f.Name == fi.Name.Name {
					idx = i
				}
			}
			if idx < 0 {
				c.fail(fi.Name.Pos(), source.ErrUnknownIdent, "%s", fi.Name.Text)
			}
			// TP-ANNAHME: Felder müssen in Deklarationsreihenfolge stehen.
			if idx <= last {
				c.fail(fi.Name.Pos(), source.ErrSyntax, "fields out of order")
			}
			last = idx
			c.info.Fields[fi.Name] = t.Fields[idx]
			v.Elems[idx] = c.typedConst(t.Fields[idx].Type, fi.Value)
		}
		return v
	}
	x := c.expr(e)
	if x.Mode != Constant {
		c.fail(e.Pos(), source.ErrConstExpected, "")
	}
	c.assignable(t, x, e)
	return convertConst(t, x.Val)
}

// convertConst passt einen Konstantenwert an den Zieltyp an (Ganzzahl nach
// Real, Char nach String, leere Menge).
func convertConst(t types.Type, v *types.Value) *types.Value {
	switch {
	case types.IsReal(t) && v.Kind == types.VInt:
		return types.RealVal(float64(v.Int))
	case types.IsString(t) && v.Kind == types.VInt:
		return types.StrVal(string([]byte{byte(v.Int)}))
	}
	return v
}

// ---------------------------------------------------------------------------
// Typen

func (c *checker) typeDecl(d *ast.TypeDecl) {
	t := c.typeExpr(d.Type)
	if n, ok := t.(types.Nameable); ok {
		n.SetName(d.Name.Text)
	}
	c.declare(d.Name, types.NewTypeName(d.Name.Name, d.Name.Text, d.Name.Pos(), t))
}

// resolvePending löst Zeigertypen auf, deren Basistyp beim Lesen noch nicht
// deklariert war.
func (c *checker) resolvePending() {
	for _, p := range c.pending {
		p.ptr.Elem = c.namedType(p.name, source.ErrUndefinedPtrType)
	}
	c.pending = nil
}

// namedType löst einen Typbezeichner auf; code ist der Fehler für unbekannte Namen.
func (c *checker) namedType(n *ast.NamedType, code int) types.Type {
	scope := c.scope
	if n.Unit != nil {
		u, ok := c.lookup(n.Unit).(*types.Unit)
		if !ok {
			c.fail(n.Unit.Pos(), source.ErrUnknownIdent, "%s", n.Unit.Text)
		}
		scope = u.Scope
		if sym, ok := scope.Syms[n.Name.Name]; ok {
			if tn, ok := sym.(*types.TypeName); ok {
				c.info.Uses[n.Name] = tn
				return tn.Type
			}
		}
		c.fail(n.Name.Pos(), source.ErrTypeIdentExpected, "")
	}
	sym, _ := scope.Lookup(n.Name.Name)
	if sym == nil {
		c.fail(n.Name.Pos(), code, "%s", n.Name.Text)
	}
	tn, ok := sym.(*types.TypeName)
	if !ok {
		c.fail(n.Name.Pos(), source.ErrTypeIdentExpected, "")
	}
	c.info.Uses[n.Name] = tn
	return tn.Type
}

func (c *checker) typeExpr(e ast.TypeExpr) types.Type {
	switch e := e.(type) {
	case *ast.NamedType:
		return c.namedType(e, source.ErrUnknownIdent)
	case *ast.StringType:
		if e.Len == nil {
			return types.StringType
		}
		n := c.constInt(e.Len)
		if n < 1 || n > 255 {
			c.fail(e.Len.Pos(), source.ErrInvalidStringLen, "")
		}
		return &types.String{Max: n}
	case *ast.SubrangeType:
		lo, hi := c.constExpr(e.Lo), c.constExpr(e.Hi)
		if !types.IsOrdinal(lo.Type) || !types.IsOrdinal(hi.Type) {
			c.fail(e.Pos(), source.ErrInvalidSubrangeBase, "")
		}
		base := host(lo.Type)
		if host(hi.Type) != base {
			c.fail(e.Hi.Pos(), source.ErrTypeMismatch, "")
		}
		if lo.Val.Int > hi.Val.Int {
			c.fail(e.Hi.Pos(), source.ErrLowerGtUpper, "")
		}
		if b, ok := base.(*types.Basic); ok && b.IsInteger() {
			base = types.LongInt
		}
		return &types.Subrange{Base: base, Lo: lo.Val.Int, Hi: hi.Val.Int}
	case *ast.EnumType:
		t := &types.Enum{}
		for i, id := range e.Names {
			t.Names = append(t.Names, id.Text)
			k := types.NewConst(id.Name, id.Text, id.Pos(), t, types.IntVal(int64(i)))
			k.EnumMember = true
			c.declare(id, k)
		}
		return t
	case *ast.ArrayType:
		return c.arrayType(e, 0)
	case *ast.RecordType:
		r := &types.Record{Packed: e.Packed}
		var variant int
		size, align := c.fieldList(r, e.Fields, 0, &variant, 0)
		align = max(align, 1)
		r.SetLayout(types.AlignTo(size, align), align)
		c.checkSize(e, r.Size())
		return r
	case *ast.SetType:
		elem := c.typeExpr(e.Elem)
		if !types.IsOrdinal(elem) {
			c.fail(e.Elem.Pos(), source.ErrOrdinalTypeExpected, "")
		}
		if lo, hi := types.Bounds(elem); lo < 0 || hi > 255 {
			c.fail(e.Elem.Pos(), source.ErrSetBaseRange, "")
		}
		return &types.Set{Packed: e.Packed, Elem: elem}
	case *ast.FileType:
		f := &types.File{Packed: e.Packed}
		if e.Elem != nil {
			f.Elem = c.typeExpr(e.Elem)
			if containsFile(f.Elem) {
				c.fail(e.Elem.Pos(), source.ErrInvalidFileType, "")
			}
		}
		return f
	case *ast.PointerType:
		p := &types.Pointer{}
		switch el := e.Elem.(type) {
		case *ast.StringType:
			p.Elem = types.StringType
		case *ast.NamedType:
			// Bekannte Typen sofort, sonst am Ende des type-Teils auflösen.
			if sym, _ := c.scope.Lookup(el.Name.Name); el.Unit != nil || sym != nil {
				if _, isType := sym.(*types.TypeName); isType || el.Unit != nil {
					p.Elem = c.namedType(el, source.ErrUndefinedPtrType)
					return p
				}
			}
			c.pending = append(c.pending, &pendingPointer{ptr: p, name: el})
		}
		return p
	}
	panic("check: unbekannter Typausdruck")
}

func (c *checker) arrayType(e *ast.ArrayType, dim int) types.Type {
	idx := c.typeExpr(e.Index[dim])
	if !types.IsOrdinal(idx) {
		c.fail(e.Index[dim].Pos(), source.ErrOrdinalTypeExpected, "")
	}
	var elem types.Type
	if dim+1 < len(e.Index) {
		elem = c.arrayType(e, dim+1)
	} else {
		elem = c.typeExpr(e.Elem)
	}
	lo, hi := types.Bounds(idx)
	a := &types.Array{Packed: e.Packed, Index: idx, Lo: lo, Hi: hi, Elem: elem}
	if float64(a.Len())*float64(elem.Size()) > types.MaxStructSize {
		c.fail(e.Pos(), source.ErrStructTooLarge, "")
	}
	return a
}

func (c *checker) checkSize(n ast.Node, size int64) {
	if size > types.MaxStructSize {
		c.fail(n.Pos(), source.ErrStructTooLarge, "")
	}
}

func containsFile(t types.Type) bool {
	switch t := t.(type) {
	case *types.File:
		return true
	case *types.Array:
		return containsFile(t.Elem)
	case *types.Record:
		for _, f := range t.Fields {
			if containsFile(f.Type) {
				return true
			}
		}
	}
	return false
}

// fieldList legt die Felder ab Offset off an und liefert Ende und Ausrichtung.
// Variantenzweige beginnen alle am selben Offset; das Ende ist das Maximum.
func (c *checker) fieldList(r *types.Record, fl *ast.FieldList, off int64, variant *int, cur int) (int64, int64) {
	align := int64(1)
	add := func(id *ast.Ident, t types.Type) {
		if r.Field(id.Name) != nil {
			c.fail(id.Pos(), source.ErrDuplicateIdent, "%s", id.Text)
		}
		off = types.AlignTo(off, t.Align())
		f := &types.Field{Name: id.Name, Text: id.Text, Type: t, Offset: off, Variant: cur}
		r.Fields = append(r.Fields, f)
		c.info.Fields[id] = f
		off += t.Size()
		align = max(align, t.Align())
	}
	for _, g := range fl.Fixed {
		t := c.typeExpr(g.Type)
		for _, id := range g.Names {
			add(id, t)
		}
	}
	if v := fl.Variant; v != nil {
		tagT := c.namedType(v.TagType, source.ErrUnknownIdent)
		if !types.IsOrdinal(tagT) {
			c.fail(v.TagType.Pos(), source.ErrOrdinalTypeExpected, "")
		}
		if v.Tag != nil {
			add(v.Tag, tagT)
		}
		start, end := off, off
		for _, vc := range v.Cases {
			for _, l := range vc.Labels {
				x := c.constExpr(l)
				if !compatible(tagT, x.Type) {
					c.fail(l.Pos(), source.ErrCaseConstType, "")
				}
			}
			*variant++
			e, a := c.fieldList(r, vc.Fields, start, variant, *variant)
			end = max(end, e)
			align = max(align, a)
		}
		off = end
	}
	return off, align
}

// ---------------------------------------------------------------------------
// Variablen

func (c *checker) varDecl(d *ast.VarDecl) {
	t := c.typeExpr(d.Type)
	c.resolvePending()
	kind := types.GlobalVar
	if c.proc != nil {
		kind = types.LocalVar
	}
	for _, id := range d.Names {
		v := types.NewVar(id.Name, id.Text, id.Pos(), kind, t)
		v.Owner = c.proc
		c.declare(id, v)
	}
}

// ---------------------------------------------------------------------------
// Prozeduren

func (c *checker) signature(d *ast.ProcDecl) *types.Signature {
	sig := &types.Signature{}
	for _, g := range d.Params {
		var t types.Type
		if g.Type != nil {
			t = c.typeExpr(g.Type)
			if _, isFile := t.(*types.File); isFile && g.Mode != ast.ByVar {
				c.fail(g.Type.Pos(), source.ErrFilesMustBeVar, "")
			}
		}
		for _, id := range g.Names {
			p := &types.Param{Name: id.Text, Mode: types.ParamMode(g.Mode), Type: t}
			p.Var = types.NewVar(id.Name, id.Text, id.Pos(), types.ParamVar, t)
			p.Var.Mode = p.Mode
			p.Var.ReadOnly = p.Mode == types.ByConst
			sig.Params = append(sig.Params, p)
		}
	}
	if d.Result != nil {
		// Wie in Delphi 1.0 sind alle Ergebnistypen außer Dateien erlaubt,
		// also auch Records, Arrays und Mengen.
		sig.Result = c.typeExpr(d.Result)
		if containsFile(sig.Result) {
			c.fail(d.Result.Pos(), source.ErrInvalidResultType, "")
		}
	}
	return sig
}

func sameSig(a, b *types.Signature) bool {
	if len(a.Params) != len(b.Params) || a.Result != b.Result {
		return false
	}
	for i := range a.Params {
		p, q := a.Params[i], b.Params[i]
		if p.Mode != q.Mode || p.Type != q.Type || lower(p.Name) != lower(q.Name) {
			return false
		}
	}
	return true
}

func (c *checker) procDecl(d *ast.ProcDecl, intf bool) {
	var p *types.Proc
	// Definition einer forward- oder Interface-Prozedur?
	if old, sc := c.scope.Lookup(d.Name.Name); old != nil && d.Block != nil &&
		(sc == c.scope || (c.info.Impl == c.scope && sc == c.info.Root)) {
		if op, ok := old.(*types.Proc); ok && op.Forward && !op.Defined && op.IsFunc == d.IsFunc {
			p = op
			if d.Params != nil || d.Result != nil {
				if !sameSig(c.signature(d), p.Sig) {
					c.fail(d.Name.Pos(), source.ErrHeaderMismatch, "%s", d.Name.Text)
				}
			}
			c.info.Defs[d.Name] = p
		}
	}
	if p == nil {
		p = types.NewProc(d.Name.Name, d.Name.Text, d.Name.Pos(), d.IsFunc)
		p.Parent = c.proc
		p.Level = 1
		if c.proc != nil {
			p.Level = c.proc.Level + 1
		}
		p.Sig = c.signature(d)
		if d.IsFunc && p.Sig.Result == nil {
			c.fail(d.Name.Pos(), source.ErrColonExpected, "")
		}
		c.declare(d.Name, p)
	}
	if d.Params != nil {
		i := 0
		for _, g := range d.Params {
			for _, id := range g.Names {
				if i < len(p.Sig.Params) {
					c.info.Defs[id] = p.Sig.Params[i].Var
				}
				i++
			}
		}
	}
	c.info.Procs[d] = p
	if intf || d.Forward {
		p.Forward = true
		return
	}
	p.Defined = true
	c.procBody(p, d)
}

func (c *checker) procBody(p *types.Proc, d *ast.ProcDecl) {
	saveProc, saveWiths, saveLoops := c.proc, c.withs, c.loops
	c.proc, c.withs, c.loops = p, nil, 0
	scope := c.openScope(types.ProcScope, p)
	p.Scope = scope
	c.info.Scopes[d] = scope
	if p.IsFunc {
		r := types.NewVar("result", "Result", d.Name.Pos(), types.ResultVar, p.Sig.Result)
		r.Owner = p
		p.Result = r
		scope.Insert(r)
	}
	for _, prm := range p.Sig.Params {
		prm.Var.Owner = p
		if scope.Insert(prm.Var) != nil {
			c.fail(prm.Var.Pos(), source.ErrDuplicateIdent, "%s", prm.Var.Text())
		}
	}
	if c.enclosed == nil {
		c.enclosed = map[*types.Proc]bool{}
	}
	c.enclosed[p] = true
	c.block(d.Block)
	delete(c.enclosed, p)
	c.closeScope()
	c.proc, c.withs, c.loops = saveProc, saveWiths, saveLoops
}
