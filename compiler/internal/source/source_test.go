package source

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestPosAndError(t *testing.T) {
	if got := (Pos{File: "a.pas", Line: 2, Col: 5}).String(); got != "a.pas(2,5)" {
		t.Errorf("Pos = %q", got)
	}
	if got := Errorf(Pos{File: "a.pas", Line: 1, Col: 1}, ErrFileNotFound, "%s", "x.inc").Error(); got != "a.pas(1,1): Error 15: File not found (x.inc)" {
		t.Errorf("Error = %q", got)
	}
	if got := Errorf(Pos{File: "a.pas"}, 999, "").Error(); got != "a.pas: Error 999: Error 999" {
		t.Errorf("unbekannte Nummer: %q", got)
	}
}

func TestLoader(t *testing.T) {
	fsys := fstest.MapFS{
		"prog.pas":         {Data: []byte("\xef\xbb\xbfprogram")},
		"src/Lib/Util.INC": {Data: []byte("u")},
		"src/lib.inc":      {Data: []byte("exakt")},
		"src/LIB.INC":      {Data: []byte("anders")},
		"inc/common.inc":   {Data: []byte("c")},
		"bad.pas":          {Data: []byte("äö\n\xff")},
	}
	l := NewLoader(fsys, []string{"inc"})

	f, err := l.Load("prog.pas")
	if err != nil || string(f.Data) != "program" {
		t.Fatalf("Load = %q, %v", f.Data, err)
	}
	if f2, _ := l.Load("prog.pas"); f2 != f {
		t.Error("Load lädt erneut statt aus dem Cache")
	}

	var se *Error
	if _, err := l.Load("bad.pas"); !errors.As(err, &se) || se.Code != ErrInvalidUTF8 || se.Pos.Line != 2 || se.Pos.Col != 1 {
		t.Errorf("ungültiges UTF-8: %v", err)
	}
	if _, err := l.Load("fehlt.pas"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("fehlende Datei: %v", err)
	}

	resolve := []struct{ name, dir, want string }{
		{`lib\util.inc`, "src", "src/Lib/Util.INC"},
		{"LIB/UTIL.inc", "src", "src/Lib/Util.INC"},
		{"lib.inc", "src", "src/lib.inc"},
		{"Lib.Inc", "src", "src/LIB.INC"}, // erster Treffer in der (sortierten) Verzeichnisliste
		{"common.inc", "src", "inc/common.inc"},
		{"../inc/common.inc", "src", "inc/common.inc"},
	}
	for _, r := range resolve {
		got, err := l.Resolve(r.name, r.dir)
		if err != nil || got != r.want {
			t.Errorf("Resolve(%q, %q) = %q, %v; want %q", r.name, r.dir, got, err, r.want)
		}
	}
	for _, name := range []string{"", "/abs.inc", `C:\x.inc`} {
		if _, err := l.Resolve(name, "."); !IsInvalidName(err) {
			t.Errorf("Resolve(%q): %v, want ungültiger Name", name, err)
		}
	}
	if _, err := l.Resolve("nix.inc", "src"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Resolve(nix.inc): %v", err)
	}
	if _, err := l.Resolve("lib", "src"); err == nil {
		t.Error("Resolve findet ein Verzeichnis")
	}
}

func TestFilePosition(t *testing.T) {
	f := &File{Name: "x.pas", Data: []byte("ab\näöc\n")}
	for off, want := range map[int]string{0: "x.pas(1,1)", 3: "x.pas(2,1)", 7: "x.pas(2,3)", 100: "x.pas(3,1)"} {
		if got := f.Position(off).String(); got != want {
			t.Errorf("Position(%d) = %s, want %s", off, got, want)
		}
	}
}

func TestDefines(t *testing.T) {
	d := NewDefines("Debug")
	if !d.Defined("DEBUG") || !d.Defined("ver50") || !d.Defined("WasmPascal") || d.Defined("MSDOS") {
		t.Errorf("Defines = %v", d)
	}
	c := d.Clone()
	c.Undef("debug")
	if !d.Defined("debug") || c.Defined("debug") {
		t.Error("Clone teilt Zustand")
	}
}

func TestConditionals(t *testing.T) {
	var c Conditionals
	p := Pos{File: "a.pas", Line: 1, Col: 1}
	if !c.Active() {
		t.Fatal("anfangs inaktiv")
	}
	c.If(p, false) // inaktiv
	c.If(p, true)  // bleibt inaktiv, weil außen inaktiv
	if c.Active() {
		t.Error("innerer Bereich aktiv trotz inaktivem äußeren")
	}
	if err := c.Else(p); err != nil || c.Active() {
		t.Errorf("Else innen: %v, aktiv=%v", err, c.Active())
	}
	_ = c.EndIf(p)
	if err := c.Else(p); err != nil || !c.Active() {
		t.Errorf("Else außen: %v, aktiv=%v", err, c.Active())
	}
	if err := c.Else(p); err == nil {
		t.Error("doppeltes Else ohne Fehler")
	}
	if _, open := c.Open(); !open {
		t.Error("Open meldet keinen offenen Bereich")
	}
	_ = c.EndIf(p)
	if err := c.EndIf(p); err == nil {
		t.Error("EndIf ohne If ohne Fehler")
	}
}

// Bidi-Steuerzeichen sind im ganzen Quelltext verboten (Error 908).
func TestBidiChars(t *testing.T) {
	for _, tt := range []struct{ data, want string }{
		{"a { ‮ } b", "p.pas(1,5): Error 908: Bidirectional control character (U+202E; in strings use #$202E)"},
		{"x := 'ab⁦';\n", "p.pas(1,9): Error 908: Bidirectional control character (U+2066; in strings use #$2066)"},
		{"\n\n  ‪", "p.pas(3,3): Error 908: Bidirectional control character (U+202A; in strings use #$202A)"},
	} {
		l := NewLoader(fstest.MapFS{"p.pas": {Data: []byte(tt.data)}}, nil)
		_, err := l.Load("p.pas")
		if err == nil || err.Error() != tt.want {
			t.Errorf("%q: %v, want %s", tt.data, err, tt.want)
		}
	}
	// U+200E (Links-nach-rechts-Marke) gehört nicht dazu.
	l := NewLoader(fstest.MapFS{"p.pas": {Data: []byte("'‎'")}}, nil)
	if _, err := l.Load("p.pas"); err != nil {
		t.Errorf("U+200E: %v", err)
	}
}
