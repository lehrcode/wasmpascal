package check

import (
	"fmt"
	"strings"

	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// Dump gibt die deklarierten Symbole mit Typen, Größen, Feld-Offsets und
// Konstantenwerten aus (für Golden-Tests).
func Dump(info *Info) string {
	var b strings.Builder
	d := dumper{b: &b}
	d.scope(info.Root, 0)
	if info.Impl != nil {
		b.WriteString("implementation\n")
		d.scope(info.Impl, 0)
	}
	return b.String()
}

type dumper struct{ b *strings.Builder }

func (d dumper) line(indent int, format string, args ...any) {
	d.b.WriteString(strings.Repeat("  ", indent))
	fmt.Fprintf(d.b, format, args...)
	d.b.WriteByte('\n')
}

func text(s types.Symbol) string {
	if t, ok := s.(interface{ Text() string }); ok {
		return t.Text()
	}
	return s.Name()
}

func (d dumper) scope(s *types.Scope, indent int) {
	for _, sym := range s.Order {
		switch sym := sym.(type) {
		case *types.Const:
			if sym.EnumMember {
				continue
			}
			d.line(indent, "const %s = %s: %s", text(sym), sym.Val.Format(sym.Type), sym.Type)
		case *types.TypeName:
			d.typeDecl(indent, sym)
		case *types.Var:
			d.variable(indent, sym)
		case *types.Label:
			d.line(indent, "label %s", text(sym))
		case *types.Proc:
			kind := "procedure"
			if sym.IsFunc {
				kind = "function"
			}
			d.line(indent, "%s %s%s [level %d]", kind, text(sym), sym.Sig, sym.Level)
			if sym.Scope != nil {
				d.scope(sym.Scope, indent+1)
			}
		}
	}
}

func (d dumper) variable(indent int, v *types.Var) {
	t := "untyped"
	size := ""
	if v.Type != nil {
		t = v.Type.String()
		size = fmt.Sprintf(" (size %d)", v.Type.Size())
	}
	switch v.Kind {
	case types.TypedConst:
		d.line(indent, "const %s: %s = %s%s", text(v), t, v.Val.Format(v.Type), size)
	case types.ParamVar:
		mode := map[types.ParamMode]string{types.ByValue: "", types.ByVar: "var ", types.ByConst: "const "}[v.Mode]
		d.line(indent, "param %s%s: %s%s", mode, text(v), t, size)
	case types.ResultVar:
		d.line(indent, "result %s: %s%s", text(v), t, size)
	default:
		d.line(indent, "var %s: %s%s", text(v), t, size)
	}
}

func (d dumper) typeDecl(indent int, tn *types.TypeName) {
	t := tn.Type
	def := structure(t)
	d.line(indent, "type %s = %s (size %d, align %d)", text(tn), def, t.Size(), t.Align())
	if r, ok := t.(*types.Record); ok {
		for _, f := range r.Fields {
			variant := ""
			if f.Variant > 0 {
				variant = fmt.Sprintf(" [variant %d]", f.Variant)
			}
			d.line(indent+1, "%s: %s @%d%s", f.Text, f.Type, f.Offset, variant)
		}
	}
}

// structure beschreibt einen Typ ohne seinen eigenen Namen (Bestandteile
// erscheinen mit Namen).
func structure(t types.Type) string {
	switch t := t.(type) {
	case *types.Enum:
		return "(" + strings.Join(t.Names, ", ") + ")"
	case *types.Subrange:
		return fmt.Sprintf("%s..%s", types.FormatOrd(t.Base, t.Lo), types.FormatOrd(t.Base, t.Hi))
	case *types.String:
		if t.Max == 255 {
			return "string"
		}
		return fmt.Sprintf("string[%d]", t.Max)
	case *types.Set:
		return "set of " + t.Elem.String()
	case *types.Array:
		p := ""
		if t.Packed {
			p = "packed "
		}
		return fmt.Sprintf("%sarray[%s] of %s", p, t.Index, t.Elem)
	case *types.Record:
		if t.Packed {
			return "packed record"
		}
		return "record"
	case *types.Pointer:
		if t.Elem == nil {
			return "Pointer"
		}
		return "^" + t.Elem.String()
	case *types.File:
		switch {
		case t.Text:
			return "Text"
		case t.Elem == nil:
			return "file"
		}
		return "file of " + t.Elem.String()
	}
	return t.String()
}
