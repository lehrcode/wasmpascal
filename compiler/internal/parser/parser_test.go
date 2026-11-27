package parser

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

var update = flag.Bool("update", false, "Golden-Files (*.ast) neu schreiben")

func parseFS(loader *source.Loader, name string) (ast.File, error) {
	lx, err := lexer.New(loader, name, source.NewDefines())
	if err != nil {
		return nil, err
	}
	return Parse(lx)
}

func parseString(src string) (ast.File, error) {
	return parseFS(source.NewLoader(fstest.MapFS{"t.pas": {Data: []byte(src)}}, nil), "t.pas")
}

// TestGolden parst jedes testdata/*.pas und vergleicht den gedruckten Baum
// mit testdata/*.ast.
func TestGolden(t *testing.T) {
	files, err := filepath.Glob("testdata/*.pas")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 30 {
		t.Errorf("nur %d Golden-Tests, Etappe 2 verlangt mindestens 30", len(files))
	}
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(strings.TrimSuffix(name, ".pas"), func(t *testing.T) {
			f, err := parseFS(source.NewLoader(os.DirFS("testdata"), nil), name)
			if err != nil {
				t.Fatal(err)
			}
			got := ast.Sprint(f)
			golden := strings.TrimSuffix(file, ".pas") + ".ast"
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (mit -update anlegen)", err)
			}
			if got != string(want) {
				t.Errorf("Baum weicht von %s ab:\n--- erhalten ---\n%s", golden, got)
			}
		})
	}
}

func TestSnippets(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"zeigertyp gegen steuerzeichen",
			"type P = ^T; var q: ^T; a: array[1..2] of ^T; const C = ^T; begin case c of ^M: ; end; x := p^ end.",
			`(program _
  (type p (^ t))
  (var (q) (^ t))
  (var (a) (array ((.. 1 2)) (^ t)))
  (const c "\x14")
  (begin (case c (arm ("\r") (empty))) (:= x (^ p))))`},
		{"typisierte konstanten mit =",
			"const A: TArr = (1, 2, 3); R: 1..9 = 5; P: TPoint = (x: 1; y: -2); " +
				"M: array[1..2, 1..2] of Integer = ((1, 2), (3, 4)); E: Integer = (1 + 2) * 3; begin end.",
			`(program _
  (const a tarr (list 1 2 3))
  (const r (.. 1 9) 5)
  (const p tpoint (record-const (x 1) (y (- 2))))
  (const m (array ((.. 1 2) (.. 1 2)) integer) (list (list 1 2) (list 3 4)))
  (const e integer (* (+ 1 2) 3))
  (begin (empty)))`},
		{"vorrang (or und + gleichrangig, linksassoziativ)",
			"begin x := not a and b or c * -d + e div 2 shl 1 = f; y := -a * b in [1, 3..5] end.",
			`(program _
  (begin
    (:= x (= (+ (or (and (not a) b) (* c (- d))) (shl (div e 2) 1)) f))
    (:= y (in (* (- a) b) (set 1 (.. 3 5))))))`},
		{"nur ein vergleichsoperator", "begin x := a = b = c end.", ""},
		{"io-schalter",
			"begin {$I-} Reset(f); {$I+} if IOResult <> 0 then Halt end.",
			`(program _ (begin (io-off (call reset f)) (if (<> ioresult 0) (call halt))))`},
		{"text nach end. wird ignoriert",
			"begin end. ä ' {",
			`(program _ (begin (empty)))`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := parseString(tt.src)
			if tt.want == "" {
				if err == nil {
					t.Fatal("kein Fehler")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(ast.Sprint(f)); got != tt.want {
				t.Errorf("\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	tests := []struct{ src, want string }{
		{"program P begin end.", `t.pas(1,11): Error 85: ";" expected`},
		{"begin x := 1\n  y := 2 end.", `t.pas(2,3): Error 85: ";" expected`},
		{"begin if x x := 1 end.", "t.pas(1,12): Error 57: THEN expected"},
		{"begin while x x end.", "t.pas(1,15): Error 50: DO expected"},
		{"begin for i := 1 til 10 do end.", "t.pas(1,18): Error 58: TO or DOWNTO expected"},
		{"begin for i = 1 to 10 do end.", `t.pas(1,13): Error 91: ":=" expected`},
		{"begin case x 1: end end.", "t.pas(1,14): Error 54: OF expected"},
		{"begin x := ; end.", "t.pas(1,12): Error 42: Error in expression"},
		{"begin x := 1; else end.", "t.pas(1,15): Error 113: Error in statement"},
		{"begin (x) end.", "t.pas(1,7): Error 113: Error in statement"},
		{"begin a[1] 1 end.", `t.pas(1,12): Error 91: ":=" expected`},
		{"begin x := (1, 2) end.", `t.pas(1,14): Error 89: ")" expected`},
		{"begin repeat x := 1 until end.", "t.pas(1,27): Error 42: Error in expression"},
		{"begin with do end.", "t.pas(1,12): Error 2: Identifier expected"},
		{"begin end", "t.pas(1,10): Error 10: Unexpected end of file"},
		{"begin end;", `t.pas(1,10): Error 94: "." expected`},
		{"begin x := 1", "t.pas(1,13): Error 10: Unexpected end of file"},
		{"x := 1; begin end.", "t.pas(1,1): Error 36: BEGIN expected"},
		{"var x Integer; begin end.", `t.pas(1,7): Error 86: ":" expected`},
		{"type T Integer; begin end.", `t.pas(1,8): Error 90: "=" expected`},
		{"const c: Integer; begin end.", `t.pas(1,17): Error 90: "=" expected`},
		{"type a = array[1..2] Integer; begin end.", "t.pas(1,22): Error 54: OF expected"},
		{"type p = ^1; begin end.", "t.pas(1,11): Error 12: Type identifier expected"},
		{"type p = packed Integer; begin end.", "t.pas(1,17): Error 21: Error in type"},
		{"type p = a + 1; begin end.", "t.pas(1,10): Error 21: Error in type"},
		{"procedure P(x); begin end; begin end.", `t.pas(1,14): Error 86: ":" expected`},
		{"procedure P(x: string[5]); begin end; begin end.", `t.pas(1,22): Error 89: ")" expected`},
		{"procedure; begin end.", "t.pas(1,10): Error 2: Identifier expected"},
		{"var object: Integer; begin end.", "t.pas(1,5): Error 2: Identifier expected"},
		{"var x: Integer absolute y; begin end.", "t.pas(1,16): Error 902: Not supported by WasmPascal (absolute)"},
		{"procedure P; external; begin end.", "t.pas(1,14): Error 902: Not supported by WasmPascal (external)"},
		{"procedure P; interrupt; begin end; begin end.", "t.pas(1,14): Error 902: Not supported by WasmPascal (interrupt)"},
		{"begin inline($90) end.", "t.pas(1,7): Error 902: Not supported by WasmPascal (inline)"},
		{"unit U; begin end.", "t.pas(1,9): Error 55: INTERFACE expected"},
		{"unit U; interface var x: Integer; begin end.", "t.pas(1,35): Error 73: IMPLEMENTATION expected"},
		{"unit U; interface implementation", "t.pas(1,33): Error 10: Unexpected end of file"},
		{"unit U; interface implementation end", "t.pas(1,37): Error 10: Unexpected end of file"},
		{"begin x := 'offen\nend.", "t.pas(1,12): Error 8: String constant exceeds line"},
	}
	for _, tt := range tests {
		_, err := parseString(tt.src)
		var se *source.Error
		if !errors.As(err, &se) {
			t.Errorf("%q: err = %v, want *source.Error", tt.src, err)
			continue
		}
		if got := se.Error(); got != tt.want {
			t.Errorf("%q:\n got: %s\nwant: %s", tt.src, got, tt.want)
		}
	}
}
