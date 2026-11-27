package source

import "strings"

// Defines ist die Menge der bedingten Symbole ({$DEFINE}), unabhängig von
// Groß-/Kleinschreibung.
type Defines map[string]bool

// NewDefines erzeugt eine Symbolmenge mit den vordefinierten Symbolen und syms.
func NewDefines(syms ...string) Defines {
	d := Defines{}
	for _, s := range Predefined {
		d.Define(s)
	}
	for _, s := range syms {
		d.Define(s)
	}
	return d
}

// Predefined sind die Symbole, die in jeder Übersetzung definiert sind.
// VER50 wie in Turbo Pascal 5.0 (bis Etappe 15 VER40); WASMPASCAL
// kennzeichnet diesen Compiler. MSDOS und CPU86 fehlen bewusst, weil die
// DOS-spezifischen Teile nicht existieren.
var Predefined = []string{"VER50", "WASMPASCAL"}

// Define setzt ein Symbol.
func (d Defines) Define(s string) { d[strings.ToUpper(s)] = true }

// Undef entfernt ein Symbol.
func (d Defines) Undef(s string) { delete(d, strings.ToUpper(s)) }

// Defined meldet, ob ein Symbol gesetzt ist.
func (d Defines) Defined(s string) bool { return d[strings.ToUpper(s)] }

// Clone kopiert die Menge; jede Unit bekommt ihre eigene.
func (d Defines) Clone() Defines {
	c := make(Defines, len(d))
	for k, v := range d {
		c[k] = v
	}
	return c
}

// Conditionals verwaltet die Verschachtelung von {$IFxxx}/{$ELSE}/{$ENDIF}.
type Conditionals struct {
	stack []condFrame
}

type condFrame struct {
	pos    Pos
	outer  bool // war der umgebende Bereich aktiv?
	cond   bool
	inElse bool
}

func (f condFrame) active() bool {
	if f.inElse {
		return f.outer && !f.cond
	}
	return f.outer && f.cond
}

// Active meldet, ob Quelltext an der aktuellen Stelle übersetzt wird.
func (c *Conditionals) Active() bool {
	return len(c.stack) == 0 || c.stack[len(c.stack)-1].active()
}

// If beginnt einen bedingten Bereich. In einem inaktiven Bereich ist cond egal.
func (c *Conditionals) If(pos Pos, cond bool) {
	c.stack = append(c.stack, condFrame{pos: pos, outer: c.Active(), cond: cond})
}

// Else wechselt in den Else-Zweig; ohne offenes {$IFxxx} oder bei doppeltem
// {$ELSE} ist das ein Fehler.
func (c *Conditionals) Else(pos Pos) error {
	if len(c.stack) == 0 || c.stack[len(c.stack)-1].inElse {
		// TP-ANNAHME: fehlplatzierte Bedingungsdirektiven melden Error 17.
		return Errorf(pos, ErrInvalidDirective, "misplaced {$ELSE}")
	}
	c.stack[len(c.stack)-1].inElse = true
	return nil
}

// EndIf schließt den innersten bedingten Bereich.
func (c *Conditionals) EndIf(pos Pos) error {
	if len(c.stack) == 0 {
		return Errorf(pos, ErrInvalidDirective, "misplaced {$ENDIF}")
	}
	c.stack = c.stack[:len(c.stack)-1]
	return nil
}

// Open liefert die Position des innersten offenen {$IFxxx}, falls es eines gibt.
func (c *Conditionals) Open() (Pos, bool) {
	if len(c.stack) == 0 {
		return Pos{}, false
	}
	return c.stack[len(c.stack)-1].pos, true
}
