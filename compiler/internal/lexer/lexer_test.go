package lexer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

// lexAll zerlegt main.pas aus files (oder src, wenn files nil ist) vollständig.
func lexAll(t *testing.T, src string, files fstest.MapFS, setup func(*Lexer)) ([]Token, *Lexer, error) {
	t.Helper()
	if files == nil {
		files = fstest.MapFS{}
	}
	if _, ok := files["main.pas"]; !ok {
		files["main.pas"] = &fstest.MapFile{Data: []byte(src)}
	}
	l, err := New(source.NewLoader(files, []string{"lib"}), "main.pas", source.NewDefines("OPT"))
	if err != nil {
		return nil, nil, err
	}
	if setup != nil {
		setup(l)
	}
	var toks []Token
	for {
		tok, err := l.Next()
		if err != nil {
			return toks, l, err
		}
		if tok.Kind == EOF {
			return toks, l, nil
		}
		toks = append(toks, tok)
		if len(toks) > 1000 {
			t.Fatal("Endlosschleife im Lexer?")
		}
	}
}

// show formatiert ein Token kompakt für Vergleiche.
func show(tok Token) string {
	switch tok.Kind {
	case Ident:
		return "id:" + tok.Name
	case Int:
		return "int:" + tok.Text
	case Real:
		return "real:" + tok.Text
	case String:
		return fmt.Sprintf("str:%q", tok.Value)
	}
	return tok.Kind.String()
}

func showAll(toks []Token) string {
	parts := make([]string, len(toks))
	for i, tok := range toks {
		parts[i] = show(tok)
	}
	return strings.Join(parts, " ")
}

func TestTokens(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"leer", "", ""},
		{"leerraum", " \t\r\n\f ", ""},
		{"bezeichner", "Foo _bar x1 Hello_World ABC abc", "id:foo id:_bar id:x1 id:hello_world id:abc id:abc"},
		{"direktiven sind bezeichner", "absolute forward external interrupt asm library exports Result Break",
			"id:absolute id:forward id:external id:interrupt id:asm id:library id:exports id:result id:break"},
		{"symbole", "+ - * / = <> < <= > >= [ ] (. .) . .. , ( ) : ; := @",
			"+ - * / = <> < <= > >= [ ] [ ] . .. , ( ) : ; := @"},
		{"symbole ohne leerraum", "a:=b+c;x<>y<=z>=w..v", "id:a := id:b + id:c ; id:x <> id:y <= id:z >= id:w .. id:v"},
		{"doppelter schrägstrich ist kein kommentar", "a//b", "id:a / / id:b"},
		{"ganzzahlen", "0 123 $FF $abCD 2147483648", "int:0 int:123 int:$FF int:$abCD int:2147483648"},
		{"reals", "1.5 1e10 1.5E-3 2E+5 0.0", "real:1.5 real:1e10 real:1.5E-3 real:2E+5 real:0.0"},
		{"bereich", "1..10 'a'..'z'", `int:1 .. int:10 str:"a" .. str:"z"`},
		{"punkt-klammer nach zahl", "a[1.)", "id:a [ int:1 ]"},
		{"zahl gefolgt von punkt", "end.", "end ."},
		{"strings", `'abc' '' 'don''t' ''''`, `str:"abc" str:"" str:"don't" str:"'"`},
		{"zeichenkonstanten", "#65 #$41 #0 #255", `str:"A" str:"A" str:"\x00" str:"\xff"`},
		{"verkettung", "'a'#13#10'b'", `str:"a\r\nb"`},
		{"steuerzeichen", "x := ^M; y := ^m; z := ^[; w := ^@", `id:x := str:"\r" ; id:y := str:"\r" ; id:z := str:"\x1b" ; id:w := str:"\x00"`},
		{"steuerzeichen in verkettung", "'x'^M^J#7", `str:"x\r\n\a"`},
		{"case-marke", "case c of ^M: ;", `case id:c of str:"\r" : ;`},
		{"utf-8 im string", "'Größe' 'ä'", `str:"Größe" str:"ä"`},
		{"utf-8 im kommentar", "{ Größe } (* ä *) x", "id:x"},
		{"zeiger dereferenzieren", "p^ p^^ a[i]^ f(x)^.next", "id:p ^ id:p ^ ^ id:a [ id:i ] ^ id:f ( id:x ) ^ . id:next"},
		{"zeigertyp mit langem namen", "type P = ^Integer;", "type id:p = ^ id:integer ;"},
		{"kommentare", "a {x} b (* y *) c { (* } d (* { *) e {}(**)f", "id:a id:b id:c id:d id:e id:f"},
		{"mehrzeiliger kommentar", "a {\n\n} b", "id:a id:b"},
		{"ctrl-z beendet die datei", "a b\x1a c d", "id:a id:b"},
		{"bom", "\xef\xbb\xbfprogram", "program"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, _, err := lexAll(t, tt.src, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := showAll(toks); got != tt.want {
				t.Errorf("\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestKeywords(t *testing.T) {
	kws := Keywords()
	for _, w := range []string{"object", "constructor", "destructor", "inherited", "string", "inline", "unit", "uses", "xor", "shl", "shr"} {
		if _, ok := kws[w]; !ok {
			t.Errorf("%q fehlt unter den reservierten Wörtern", w)
		}
	}
	if len(kws) != 48 {
		t.Errorf("%d reservierte Wörter, erwartet 44 aus TP 4.0 + 4 für OOP", len(kws))
	}
	for word, kind := range kws {
		for _, spelling := range []string{word, strings.ToUpper(word), strings.ToUpper(word[:1]) + word[1:]} {
			toks, _, err := lexAll(t, spelling, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(toks) != 1 || toks[0].Kind != kind || !kind.IsKeyword() || toks[0].Text != spelling || toks[0].Name != word {
				t.Errorf("%q: %+v", spelling, toks)
			}
		}
	}
}

func TestLongIdentifier(t *testing.T) {
	long := strings.Repeat("a", 70)
	toks, _, err := lexAll(t, long+" "+strings.Repeat("a", 63)+"b", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Text != long || len(toks[0].Name) != MaxIdentLen || toks[0].Name != toks[1].Name {
		t.Errorf("signifikante Länge: %q / %q", toks[0].Name, toks[1].Name)
	}
}

func TestTypeMode(t *testing.T) {
	// Ohne Typmodus ist ^T nach "=" ein Steuerzeichen (#20), im Typmodus ein Zeiger.
	toks, _, err := lexAll(t, "P = ^T;", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := showAll(toks); got != `id:p = str:"\x14" ;` {
		t.Errorf("ohne Typmodus: %s", got)
	}
	toks, _, err = lexAll(t, "P = ^T;", nil, func(l *Lexer) { l.SetTypeMode(true) })
	if err != nil {
		t.Fatal(err)
	}
	if got := showAll(toks); got != "id:p = ^ id:t ;" {
		t.Errorf("mit Typmodus: %s", got)
	}
}

func TestPositions(t *testing.T) {
	src := "program X;\n  'Größe' y\n{ä\nä} z (*\n*)\r\n\tw"
	toks, _, err := lexAll(t, src, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"main.pas(1,1)", "main.pas(1,9)", "main.pas(1,10)", "main.pas(2,3)", "main.pas(2,11)", "main.pas(4,4)", "main.pas(6,2)"}
	if len(toks) != len(want) {
		t.Fatalf("%d Tokens: %s", len(toks), showAll(toks))
	}
	for i, tok := range toks {
		if got := tok.Pos.String(); got != want[i] {
			t.Errorf("%s: Position %s, want %s", show(tok), got, want[i])
		}
	}
}

func TestDirectives(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"define/ifdef", "{$DEFINE X} {$IFDEF X} a {$ELSE} b {$ENDIF} c", "id:a id:c"},
		{"ifndef", "{$IFNDEF X} a {$ELSE} b {$ENDIF}", "id:a"},
		{"groß/klein egal", "{$define Foo}{$ifdef FOO}a{$endif}", "id:a"},
		{"undef", "{$DEFINE X}{$UNDEF X}{$IFDEF X}a{$ELSE}b{$ENDIF}", "id:b"},
		{"vordefiniert", "{$IFDEF VER40}a{$ENDIF}{$IFDEF WASMPASCAL}b{$ENDIF}{$IFDEF MSDOS}c{$ENDIF}", "id:a id:b"},
		{"aus den optionen", "{$IFDEF opt}a{$ENDIF}", "id:a"},
		{"verschachtelt inaktiv", "{$IFDEF NOPE} {$IFDEF VER40} a {$ELSE} b {$ENDIF} c {$ELSE} d {$ENDIF}", "id:d"},
		{"verschachtelt aktiv", "{$IFDEF VER40} {$IFDEF NOPE} a {$ELSE} b {$ENDIF} c {$ENDIF}", "id:b id:c"},
		{"define im inaktiven bereich", "{$IFDEF NOPE}{$DEFINE Y}{$ENDIF}{$IFDEF Y}a{$ENDIF}", ""},
		{"paren-kommentar", "(*$DEFINE Z*)(*$IFDEF Z*)a(*$ENDIF*)", "id:a"},
		{"inaktiver bereich mit strings und fehlern", "{$IFDEF NOPE} x := '{'; ä ? 'offen\n {$ENDIF} y", "id:y"},
		{"ifopt", "{$IFOPT I+}a{$ENDIF}{$I-}{$IFOPT I+}b{$ELSE}c{$ENDIF}{$IFOPT R-}d{$ENDIF}", "id:a id:c id:d"},
		{"unbekannte direktiven", "{$N+}{$X foo}{$L datei.obj}{$R+,S-} a {$B+ kommentar}", "id:a"},
		{"kommentar mit leerzeichen vor $", "{ $IFDEF NOPE} a", "id:a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, _, err := lexAll(t, tt.src, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := showAll(toks); got != tt.want {
				t.Errorf("\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestIOChecks(t *testing.T) {
	toks, _, err := lexAll(t, "a {$I-} b {$I+} c (*$R+,I-*) d", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []bool
	for _, tok := range toks {
		got = append(got, tok.IOChecks)
	}
	if fmt.Sprint(got) != "[true false true false]" {
		t.Errorf("IOChecks = %v", got)
	}
}

func TestStackSize(t *testing.T) {
	for src, want := range map[string]int{
		"a":                        0,
		"{$M 16384,0,655360} a":    16384,
		"{$M $4000}":               0x4000,
		"{$M 1000000, 0, 0}{$M 5}": 5,
	} {
		_, l, err := lexAll(t, src, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if l.StackSize() != want {
			t.Errorf("%q: StackSize = %d, want %d", src, l.StackSize(), want)
		}
	}
}

func TestInclude(t *testing.T) {
	files := fstest.MapFS{
		"main.pas":        {Data: []byte("a {$I teil} b {$I SUB\\INNER.INC} c {$I global.inc} d")},
		"teil.pas":        {Data: []byte("x\ny")},
		"sub/inner.inc":   {Data: []byte("{$I tief.inc} i")},
		"sub/tief.inc":    {Data: []byte("t {$DEFINE AUSINC}")},
		"lib/global.inc":  {Data: []byte("{$IFDEF AUSINC} g {$ENDIF}")},
		"lib/ignored.pas": {Data: []byte("nicht gelesen")},
	}
	toks, _, err := lexAll(t, "", files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := showAll(toks), "id:a id:x id:y id:b id:t id:i id:c id:g id:d"; got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
	var where []string
	for _, tok := range toks {
		where = append(where, tok.Pos.String())
	}
	want := "main.pas(1,1) teil.pas(1,1) teil.pas(2,1) main.pas(1,13) sub/tief.inc(1,1) sub/inner.inc(1,15) main.pas(1,34) lib/global.inc(1,17) main.pas(1,52)"
	if got := strings.Join(where, " "); got != want {
		t.Errorf("Positionen\n got: %s\nwant: %s", got, want)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
		src   string
		want  string // vollständige Meldung
	}{
		{"offener string", nil, "x := 'abc", "main.pas(1,6): Error 8: String constant exceeds line"},
		{"string über zeilenende", nil, "'abc\n'", "main.pas(1,1): Error 8: String constant exceeds line"},
		{"offener kommentar", nil, "a\n{ abc", "main.pas(2,1): Error 10: Unexpected end of file (unterminated comment)"},
		{"offener paren-kommentar", nil, "(* abc *", "main.pas(1,1): Error 10: Unexpected end of file (unterminated comment)"},
		{"leeres #", nil, "x #;", "main.pas(1,3): Error 7: Error in integer constant"},
		{"# zu groß", nil, "'a'#65536", "main.pas(1,4): Error 76: Constant out of range"},
		{"# surrogat", nil, "x #$D800", "main.pas(1,3): Error 76: Constant out of range (surrogate)"},
		{"# surrogat ende", nil, "#$DFFF", "main.pas(1,1): Error 76: Constant out of range (surrogate)"},
		{"# noncharacter fffe", nil, "#$FFFE", "main.pas(1,1): Error 76: Constant out of range (noncharacter)"},
		{"# noncharacter ffff", nil, "#65535", "main.pas(1,1): Error 76: Constant out of range (noncharacter)"},
		{"# noncharacter fdd0", nil, "'a'#$FDD0", "main.pas(1,4): Error 76: Constant out of range (noncharacter)"},
		{"esc im literal", nil, "x := 'a\x1bb'", "main.pas(1,8): Error 907: Invalid character in string literal (control character U+001B; use #27)"},
		{"nul im literal", nil, "'\x00'", "main.pas(1,2): Error 907: Invalid character in string literal (control character U+0000; use #0)"},
		{"noncharacter im literal", nil, "'ä\uFFFE'", "main.pas(1,3): Error 907: Invalid character in string literal (noncharacter U+FFFE)"},
		{"noncharacter fdd0 im literal", nil, "'\uFDD0'", "main.pas(1,2): Error 907: Invalid character in string literal (noncharacter U+FDD0)"},
		{"bidi im include", fstest.MapFS{"main.pas": {Data: []byte("x {$I b.inc}")}, "b.inc": {Data: []byte("\n{ \u202e }")}}, "",
			"b.inc(2,3): Error 908: Bidirectional control character (U+202E; in strings use #$202E)"},
		{"noncharacter ebene 16", nil, "'\U0010FFFF'", "main.pas(1,2): Error 907: Invalid character in string literal (noncharacter U+10FFFF)"},
		{"#$ ohne ziffern", nil, "#$", "main.pas(1,1): Error 7: Error in integer constant"},
		{"$ ohne ziffern", nil, "$G", "main.pas(1,1): Error 7: Error in integer constant"},
		{"exponent fehlt", nil, "1e", "main.pas(1,1): Error 6: Error in real constant"},
		{"exponent nach vorzeichen fehlt", nil, "x 1.5e+", "main.pas(1,3): Error 6: Error in real constant"},
		{"unzulässiges zeichen", nil, "x ! y", `main.pas(1,3): Error 5: Syntax error (invalid character '!')`},
		{"umlaut im bezeichner", nil, "Größe := 1", `main.pas(1,3): Error 901: Invalid character; identifiers and symbols must be ASCII ('ö')`},
		{"else ohne if", nil, "{$ELSE}", "main.pas(1,1): Error 17: Invalid compiler directive (misplaced {$ELSE})"},
		{"endif ohne if", nil, "a\n  {$ENDIF}", "main.pas(2,3): Error 17: Invalid compiler directive (misplaced {$ENDIF})"},
		{"doppeltes else", nil, "{$IFDEF X}{$ELSE}{$ELSE}", "main.pas(1,18): Error 17: Invalid compiler directive (misplaced {$ELSE})"},
		{"endif fehlt", nil, "{$IFDEF X}\na", "main.pas(2,2): Error 10: Unexpected end of file (missing {$ENDIF} for directive at 1,1)"},
		{"define ohne symbol", nil, "{$DEFINE}", "main.pas(1,1): Error 17: Invalid compiler directive (conditional symbol expected)"},
		{"ifopt unbekannt", nil, "{$IFOPT Q+}", "main.pas(1,1): Error 17: Invalid compiler directive (unknown switch Q)"},
		{"m ohne zahl", nil, "{$M x}", "main.pas(1,1): Error 17: Invalid compiler directive (stack size expected)"},
		{"include fehlt", nil, "a {$I fehlt}", "main.pas(1,3): Error 15: File not found (fehlt.pas)"},
		{"include ohne namen", nil, "{$I}", "main.pas(1,1): Error 14: Invalid file name"},
		{"include mit laufwerk", nil, `{$I C:\TP\X.INC}`, `main.pas(1,1): Error 14: Invalid file name (C:\TP\X.INC)`},
		{"rekursives include", fstest.MapFS{"main.pas": {Data: []byte("{$I main.pas}")}}, "", "main.pas(1,1): Error 9: Too many nested files (circular include of main.pas)"},
		{"zyklus über zwei dateien", fstest.MapFS{
			"main.pas": {Data: []byte("{$I a.inc}")},
			"a.inc":    {Data: []byte("x\n{$I b.inc}")},
			"b.inc":    {Data: []byte("  {$I A.INC}")},
		}, "", "b.inc(1,3): Error 9: Too many nested files (circular include of a.inc)"},
		{"ungültiges utf-8", nil, "ab\n c\xff", "main.pas(2,3): Error 900: Source file is not valid UTF-8; convert it with: iconv -f CP437 -t UTF-8"},
		{"ungültiges utf-8 im include", fstest.MapFS{"main.pas": {Data: []byte("{$I alt.inc}")}, "alt.inc": {Data: []byte("x := '\x84';")}}, "",
			"alt.inc(1,7): Error 900: Source file is not valid UTF-8; convert it with: iconv -f CP437 -t UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := lexAll(t, tt.src, tt.files, nil)
			var se *source.Error
			if !errors.As(err, &se) {
				t.Fatalf("err = %v, want *source.Error", err)
			}
			if got := se.Error(); got != tt.want {
				t.Errorf("\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestMissingMainFile(t *testing.T) {
	_, err := New(source.NewLoader(fstest.MapFS{}, nil), "weg.pas", source.NewDefines())
	if err == nil || err.Error() != "weg.pas: Error 15: File not found" {
		t.Errorf("err = %v", err)
	}
}

// Zulässige Zeichen und #n-Werte rund um die neuen Grenzen (Etappe 14).
func TestStringCharsAllowed(t *testing.T) {
	tests := []struct {
		src, value string
		code       rune // CodePoint des Tokens, 0 = keiner
	}{
		{"'a\tb'", "a\tb", 0},      // Tabulator erlaubt
		{"#27'[0m'", "\x1b[0m", 0}, // Steuerzeichen über #n
		{"^[", "\x1b", 0},          // und über ^X
		{"'a​b'", "a​b", 0},        // Null-Breite-Zeichen erlaubt
		{"#256", "Ā", 0x100},       // WideChar-Konstante
		{"#$20AC", "€", 0x20AC},    //
		{"#$D7FF", "퟿", 0xD7FF},    // direkt vor den Surrogaten
		{"#$E000", "", 0xE000},    // direkt dahinter
		{"#$FFFD", "�", 0xFFFD},    // größter Wert vor den Noncharacters
		{"'x'#$20AC", "x€", 0},     // gemischt: String mit UTF-8
		{"#$20AC#$20AC", "€€", 0},  // zwei Zeichen: String
		{"#255", "\xff", 0},        // bis 255 ein Byte (Char)
	}
	for _, tt := range tests {
		toks, _, err := lexAll(t, tt.src, nil, nil)
		if err != nil {
			t.Errorf("%q: %v", tt.src, err)
			continue
		}
		if len(toks) != 1 || toks[0].Kind != String || toks[0].Value != tt.value || toks[0].CodePoint != tt.code {
			t.Errorf("%q: %+v, want Value %q CodePoint %#x", tt.src, toks, tt.value, tt.code)
		}
	}
	// Steuerzeichen und Noncharacters in Kommentaren sind erlaubt.
	if _, _, err := lexAll(t, "{ \x07 ￾ } x", nil, nil); err != nil {
		t.Errorf("Kommentar: %v", err)
	}
}
