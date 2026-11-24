package types

import (
	"fmt"
	"strconv"
	"strings"
)

// ValueKind unterscheidet Konstantenwerte.
type ValueKind int

// Arten von Konstantenwerten.
const (
	VNone   ValueKind = iota
	VInt              // Ordinalwerte: Ganzzahlen, Char, Boolean, Aufzählungen
	VReal             // Gleitkomma
	VString           // Bytes eines Strings
	VSet              // Bitmenge über 0..255
	VNil              // nil
	VList             // Array-Konstante (typisierte Konstanten)
	VRecord           // Record-Konstante; Elems in Feldreihenfolge, nil = nicht belegt
)

// Value ist der Wert einer Konstante.
type Value struct {
	Kind  ValueKind
	Int   int64
	Real  float64
	Str   string
	Set   [4]uint64
	Elems []*Value
}

// IntVal erzeugt einen Ordinalwert.
func IntVal(v int64) *Value { return &Value{Kind: VInt, Int: v} }

// RealVal erzeugt einen Gleitkommawert.
func RealVal(v float64) *Value { return &Value{Kind: VReal, Real: v} }

// StrVal erzeugt einen Stringwert.
func StrVal(s string) *Value { return &Value{Kind: VString, Str: s} }

// BoolVal erzeugt einen Booleschen Wert.
func BoolVal(b bool) *Value {
	if b {
		return IntVal(1)
	}
	return IntVal(0)
}

// SetHas meldet, ob v in der Menge liegt.
func (v *Value) SetHas(e int64) bool {
	return e >= 0 && e < 256 && v.Set[e/64]&(1<<(e%64)) != 0
}

// SetAdd fügt e zur Menge hinzu.
func (v *Value) SetAdd(e int64) { v.Set[e/64] |= 1 << (e % 64) }

// AsReal liefert einen numerischen Wert als float64.
func (v *Value) AsReal() float64 {
	if v.Kind == VInt {
		return float64(v.Int)
	}
	return v.Real
}

// Format schreibt den Wert passend zum Typ t.
func (v *Value) Format(t Type) string {
	switch v.Kind {
	case VInt:
		return FormatOrd(t, v.Int)
	case VReal:
		return strconv.FormatFloat(v.Real, 'g', -1, 64)
	case VString:
		return fmt.Sprintf("%q", v.Str)
	case VNil:
		return "nil"
	case VSet:
		var elemT Type
		if s, ok := t.(*Set); ok {
			elemT = s.Elem
		}
		var parts []string
		for e := int64(0); e < 256; e++ {
			if v.SetHas(e) {
				parts = append(parts, FormatOrd(elemT, e))
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case VList:
		var elemT Type
		if a, ok := t.(*Array); ok {
			elemT = a.Elem
		}
		parts := make([]string, len(v.Elems))
		for i, e := range v.Elems {
			parts[i] = e.Format(elemT)
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case VRecord:
		r, _ := t.(*Record)
		var parts []string
		for i, e := range v.Elems {
			if e == nil {
				continue
			}
			var ft Type
			name := fmt.Sprint(i)
			if r != nil {
				ft, name = r.Fields[i].Type, r.Fields[i].Text
			}
			parts = append(parts, name+": "+e.Format(ft))
		}
		return "(" + strings.Join(parts, "; ") + ")"
	}
	return "?"
}
