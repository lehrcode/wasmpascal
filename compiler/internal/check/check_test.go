package check

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
	"github.com/lehrcode/wasmpascal/compiler/internal/parser"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

var update = flag.Bool("update", false, "Golden-Files (*.sym) neu schreiben")

func checkFile(loader *source.Loader, name string) (*Info, error) {
	lx, err := lexer.New(loader, name, source.NewDefines())
	if err != nil {
		return nil, err
	}
	f, err := parser.Parse(lx)
	if err != nil {
		return nil, err
	}
	return Check(f, Config{})
}

func checkString(src string) (*Info, error) {
	return checkFile(source.NewLoader(fstest.MapFS{"t.pas": {Data: []byte(src)}}, nil), "t.pas")
}

// TestParserPrograms prüft, dass die Lehrbuchprogramme der Parser-Tests auch
// die Typprüfung bestehen. Ausgenommen sind Programme mit Units (Etappe 7)
// und goto (Etappe 8).
func TestParserPrograms(t *testing.T) {
	skip := map[string]string{
		"17_unit.pas":         "uses Crt/Dos",
		"21_goto.pas":         "goto",
		"30_zahlenraten.pas":  "uses Crt",
		"31_qualifiziert.pas": "uses Crt/Dos",
		"34_ohne_kopf.pas":    "uses Crt",
	}
	files, _ := filepath.Glob("../parser/testdata/*.pas")
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(strings.TrimSuffix(name, ".pas"), func(t *testing.T) {
			if why, ok := skip[name]; ok {
				t.Skip(why)
			}
			if _, err := checkFile(source.NewLoader(os.DirFS("../parser/testdata"), nil), name); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestGolden prüft testdata/*.pas und vergleicht die Symboltabelle mit *.sym.
func TestGolden(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.pas")
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(strings.TrimSuffix(name, ".pas"), func(t *testing.T) {
			info, err := checkFile(source.NewLoader(os.DirFS("testdata"), nil), name)
			if err != nil {
				t.Fatal(err)
			}
			got := Dump(info)
			golden := strings.TrimSuffix(file, ".pas") + ".sym"
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
				t.Errorf("Symbole weichen von %s ab:\n--- erhalten ---\n%s", golden, got)
			}
		})
	}
}

func errorOf(t *testing.T, err error) string {
	t.Helper()
	var se *source.Error
	if !errors.As(err, &se) {
		return "kein Compilerfehler: " + func() string {
			if err == nil {
				return "<nil>"
			}
			return err.Error()
		}()
	}
	return se.Error()
}

// exprType prüft decls + "if SizeOf(expr) > 0 then;" und liefert Typ und
// gegebenenfalls Konstantenwert von expr.
func exprType(t *testing.T, decls, expr string) string {
	t.Helper()
	src := "program T;\n" + decls + "\nbegin\n  if SizeOf(" + expr + ") > 0 then\nend."
	lx, _ := lexer.New(source.NewLoader(fstest.MapFS{"t.pas": {Data: []byte(src)}}, nil), "t.pas", source.NewDefines())
	f, err := parser.Parse(lx)
	if err != nil {
		return "Syntaxfehler: " + err.Error()
	}
	info, err := Check(f, Config{})
	if err != nil {
		return err.Error()
	}
	body := f.(*ast.Program).Block.Body
	e := body.Stmts[0].(*ast.IfStmt).Cond.(*ast.BinaryExpr).X.(*ast.CallExpr).Args[0]
	tv := info.Types[e]
	if tv.Mode == Constant {
		return tv.Type.String() + " = " + tv.Val.Format(tv.Type)
	}
	return tv.Type.String()
}

const exprDecls = `
type
  TFarbe = (Rot, Gruen, Blau);
  TRec = record b: Byte; i: Integer; r: Real end;
  PNode = ^TNode;
  TNode = record wert: Integer; next: PNode end;
  TFeld = array[2..7] of Char;
var
  x: Byte; l: LongInt; r: Real; s: string[10]; a: TFeld; p: PNode; f: TFarbe;
function Fn(n: Integer): Real; begin Fn := n end;
function Knoten: TNode; begin Result.wert := 1 end;
function Feld: TFeld; begin end;
`

func TestExprTypes(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"1 + 2 * 3", "Integer = 7"},
		{"7 div 2 + 7 mod -2", "Integer = 4"},
		{"1 / 2", "Real = 0.5"},
		{"2.5E1 - 1", "Real = 24"},
		{"'a'", "Char = 'a'"},
		{"'ab'", `string = "ab"`},
		{"'a' + 'b'", `string = "ab"`},
		{"#65#66", `string = "AB"`},
		{"''", `string = ""`},
		{"'Größe'", `string = "Größe"`},
		{"Length('Größe')", "Integer = 7"},
		{"$FF", "Integer = 255"},
		{"$FFFFFFFF", "Integer = -1"},
		{"3000000000", "Real = 3e+09"},
		{"MaxInt", "Integer = 2147483647"},
		{"[1, 3..5]", "set of LongInt = [1, 3, 4, 5]"},
		{"['a'..'c']", "set of Char = ['a', 'b', 'c']"},
		{"5 in [1..10]", "Boolean = True"},
		{"Ord('A') + 1", "LongInt = 66"},
		{"Chr(66)", "Char = 'B'"},
		{"Succ(Rot)", "TFarbe = Gruen"},
		{"High(TFarbe)", "TFarbe = Blau"},
		{"Low(s)", "Integer = 0"},
		{"High(s)", "Integer = 10"},
		{"High(a)", "2..7 = 7"},
		{"SizeOf(TRec)", "Integer = 16"},
		{"SizeOf(s)", "Integer = 256"},
		{"SizeOf(TFarbe)", "Integer = 1"},
		{"x + 1", "Integer"},
		{"x + l", "LongInt"},
		{"r * 2", "Real"},
		{"Fn(3)", "Real"},
		{"Knoten", "TNode"},
		{"Knoten.next", "PNode"},
		{"Feld[3]", "Char"},
		{"p^", "TNode"},
		{"p^.next", "PNode"},
		{"s[1]", "Char"},
		{"a[3]", "Char"},
		{"@x", "Pointer"},
		{"not True", "Boolean = False"},
		{"not 5", "Integer = -6"},
		{"Byte(300)", "Byte = 44"},
		{"ShortInt(200)", "ShortInt = -56"},
		{"Word(-1)", "Word = 65535"},
		{"TFarbe(2)", "TFarbe = Blau"},
		{"Abs(-5)", "Integer = 5"},
		{"Trunc(3.7)", "LongInt = 3"},
		{"Round(-2.5)", "LongInt = -3"},
		{"Hi($1234) + Lo($1234)", "Integer = 70"},
		{"Swap($1234)", "Integer = 13330"},
		{"Odd(3)", "Boolean = True"},
		{"'abc' < 'abd'", "Boolean = True"},
		{"Copy(s, 1, 2)", "string"},
		{"Concat('a', 'bc', #100)", `string = "abcd"`},
		{"UpCase('q')", "Char = 'Q'"},
		{"Pi", "Real"},
		{"Random", "Real"},
		{"Random(10)", "Integer"},
		{"Seg(x)", "Word = 0"},
		{"1 shl 4 + -1 shr 28", "Integer = 31"},
		{"f = Rot", "Boolean"},
		{"p = nil", "Boolean"},
		{"[1, 2] <= [1..3]", "Boolean = True"},
	}
	for _, tt := range tests {
		if got := exprType(t, exprDecls, tt.expr); got != tt.want {
			t.Errorf("%s: %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestErrors(t *testing.T) {
	tests := []struct{ name, src, want string }{
		// Bezeichner
		{"unbekannt", "begin x := 1 end.", "t.pas(1,7): Error 3: Unknown identifier (x)"},
		{"doppelt", "var a: Integer; A: Char; begin end.", "t.pas(1,17): Error 4: Duplicate identifier (A)"},
		{"doppelt param/lokal", "procedure P(x: Integer); var x: Byte; begin end; begin end.", "t.pas(1,30): Error 4: Duplicate identifier (x)"},
		{"vor deklaration", "var a: Integer; begin a := b end. var b: Integer;", "t.pas(1,28): Error 3: Unknown identifier (b)"},
		{"result doppelt", "function F: Integer; var Result: Integer; begin end; begin end.", "t.pas(1,26): Error 4: Duplicate identifier (Result)"},
		{"result als parameter", "function F(result: Integer): Integer; begin end; begin end.", "t.pas(1,12): Error 4: Duplicate identifier (result)"},
		{"nicht unterstützt", "var p: Pointer; begin p := Ptr(CSeg, 0) end.", "t.pas(1,32): Error 902: Not supported by WasmPascal (CSeg)"},
		{"mem", "begin Mem[0] := 1 end.", "t.pas(1,7): Error 902: Not supported by WasmPascal (Mem)"},
		{"unit fehlt", "uses Crt; begin end.", "t.pas(1,6): Error 15: File not found (Crt.pas)"},
		// Typen
		{"typbezeichner erwartet", "var x: Integer; y: x; begin end.", "t.pas(1,20): Error 12: Type identifier expected"},
		{"stringlänge", "var s: string[256]; begin end.", "t.pas(1,15): Error 25: Invalid string length"},
		{"stringlänge 0", "var s: string[0]; begin end.", "t.pas(1,15): Error 25: Invalid string length"},
		{"bereich verkehrt", "type T = 10..1; begin end.", "t.pas(1,14): Error 28: Lower bound greater than upper bound"},
		{"bereich gemischt", "type T = 1..'z'; begin end.", "t.pas(1,13): Error 26: Type mismatch"},
		{"bereich real", "type T = 1.5..2; begin end.", "t.pas(1,10): Error 27: Invalid subrange base type"},
		{"menge zu groß", "type T = set of Integer; begin end.", "t.pas(1,17): Error 23: Set base type out of range"},
		{"menge negativ", "type T = set of -1..5; begin end.", "t.pas(1,17): Error 23: Set base type out of range"},
		{"menge real", "type T = set of Real; begin end.", "t.pas(1,17): Error 29: Ordinal type expected"},
		{"index real", "type T = array[Real] of Byte; begin end.", "t.pas(1,16): Error 29: Ordinal type expected"},
		{"zu groß", "type T = array[Integer] of Byte; begin end.", "t.pas(1,10): Error 22: Structure too large"},
		{"2 GB ok, 4 GB nicht", "type T = array[0..$3FFFFFFF] of Word; begin end.", "t.pas(1,10): Error 22: Structure too large"},
		{"zeiger offen", "type P = ^TFehlt; begin end.", "t.pas(1,11): Error 19: Undefined type in pointer definition (TFehlt)"},
		{"datei in datei", "type F = file of Text; begin end.", "t.pas(1,18): Error 63: Invalid file type"},
		{"feld doppelt", "type R = record a: Byte; case b: Byte of 1: (a: Char) end; begin end.", "t.pas(1,46): Error 4: Duplicate identifier (a)"},
		{"variante falscher typ", "type R = record case Boolean of 'x': () end; begin end.", "t.pas(1,33): Error 74: Constant and case types do not match"},
		{"ergebnistyp datei", "function F: Text; begin end; begin end.", "t.pas(1,13): Error 34: Invalid function result type"},
		{"ergebnistyp record mit datei", "type R = record f: Text end; function F: R; begin end; begin end.", "t.pas(1,42): Error 34: Invalid function result type"},
		{"datei als wertparameter", "procedure P(f: Text); begin end; begin end.", "t.pas(1,16): Error 126: Files must be var parameters"},
		// Konstanten
		{"keine konstante", "var v: Integer; const C = v; begin end.", "t.pas(1,27): Error 31: Constant expected"},
		{"typisierte konstante im ausdruck", "const A: Integer = 1; B = A + 1; begin end.", "t.pas(1,27): Error 31: Constant expected"},
		{"division durch null", "const C = 1 div 0; begin end.", "t.pas(1,13): Error 62: Division by zero"},
		{"überlauf", "const C = MaxLongInt + 1; begin end.", "t.pas(1,22): Error 76: Constant out of range"},
		{"chr zu groß", "const C = Chr(256); begin end.", "t.pas(1,15): Error 76: Constant out of range"},
		{"succ am ende", "type T = (a, b); const C = Succ(b); begin end.", "t.pas(1,33): Error 76: Constant out of range"},
		{"typkonst zu viele", "const A: array[1..2] of Byte = (1, 2, 3); begin end.", "t.pas(1,32): Error 26: Type mismatch (2 elements expected)"},
		{"typkonst bereich", "const A: array[1..2] of Byte = (1, 256); begin end.", "t.pas(1,36): Error 76: Constant out of range"},
		{"typkonst feld", "type R = record x, y: Integer end; const P: R = (x: 1; z: 2); begin end.", "t.pas(1,56): Error 3: Unknown identifier (z)"},
		{"typkonst datei", "const F: Text = 1; begin end.", "t.pas(1,10): Error 63: Invalid file type (Text)"},
		// Zuweisungen und Ausdrücke
		{"typfehler", "var i: Integer; begin i := 'abc' end.", "t.pas(1,28): Error 26: Type mismatch"},
		{"real an integer", "var i: Integer; begin i := 1.5 end.", "t.pas(1,28): Error 26: Type mismatch"},
		{"byte bereich", "var b: Byte; begin b := 256 end.", "t.pas(1,25): Error 76: Constant out of range"},
		{"word bereich", "var w: Word; begin w := -1 end.", "t.pas(1,25): Error 76: Constant out of range"},
		{"char aus string", "var c: Char; begin c := 'ab' end.", "t.pas(1,25): Error 26: Type mismatch"},
		{"aufzählungen gemischt", "type A = (x, y); B = (u, v); var e: A; begin e := u end.", "t.pas(1,51): Error 26: Type mismatch"},
		{"record ungleich", "type R1 = record a: Byte end; R2 = record a: Byte end; var r: R1; s: R2; begin r := s end.", "t.pas(1,85): Error 26: Type mismatch"},
		{"anonyme arrays", "var a: array[1..3] of Byte; b: array[1..3] of Byte; begin a := b end.", "t.pas(1,64): Error 26: Type mismatch"},
		{"datei zuweisen", "var f, g: Text; begin f := g end.", "t.pas(1,28): Error 26: Type mismatch"},
		{"zeiger verschieden", "var p: ^Integer; q: ^Char; begin p := q end.", "t.pas(1,39): Error 26: Type mismatch"},
		{"an konstante", "const C = 1; begin C := 2 end.", "t.pas(1,20): Error 20: Variable identifier expected"},
		{"operatoren", "var b: Boolean; begin b := 1 + True end.", "t.pas(1,30): Error 41: Operand types do not match operator"},
		{"div real", "var r: Real; begin r := r div 2 end.", "t.pas(1,27): Error 41: Operand types do not match operator"},
		{"mengen ordnung", "var b: Boolean; begin b := [1] < [2] end.", "t.pas(1,32): Error 41: Operand types do not match operator"},
		{"in falscher typ", "var b: Boolean; begin b := 'a' in [1, 2] end.", "t.pas(1,32): Error 41: Operand types do not match operator"},
		{"if ohne boolean", "begin if 1 then end.", "t.pas(1,10): Error 40: Boolean expression expected"},
		{"case real", "var r: Real; begin case r of 1: end end.", "t.pas(1,25): Error 39: Ordinal expression expected"},
		{"case marke typ", "var c: Char; begin case c of 1: end end.", "t.pas(1,30): Error 74: Constant and case types do not match"},
		{"case marke variable", "var c, d: Char; begin case c of d: end end.", "t.pas(1,33): Error 31: Constant expected"},
		{"feld unbekannt", "var r: record a: Byte end; begin r.b := 1 end.", "t.pas(1,36): Error 3: Unknown identifier (b)"},
		{"kein record", "var i: Integer; begin i.a := 1 end.", "t.pas(1,23): Error 75: Record variable expected"},
		{"kein array", "var i: Integer; begin i[1] := 1 end.", "t.pas(1,25): Error 26: Type mismatch (array or string expected)"},
		{"index konstant", "var a: array[1..3] of Byte; begin a[4] := 1 end.", "t.pas(1,37): Error 76: Constant out of range"},
		{"string index", "var s: string[5]; begin s[6] := 'x' end.", "t.pas(1,27): Error 76: Constant out of range"},
		{"deref kein zeiger", "var i: Integer; begin i^ := 1 end.", "t.pas(1,23): Error 65: Pointer variable expected"},
		{"deref untypisiert", "var p: Pointer; begin p^ := 1 end.", "t.pas(1,23): Error 65: Pointer variable expected"},
		{"typumwandlung", "var r: Real; i: Integer; begin i := Integer(r) end.", "t.pas(1,37): Error 61: Invalid typecast"},
		{"at auf konstante", "var p: Pointer; begin p := @5 end.", "t.pas(1,29): Error 83: Invalid @ argument"},
		{"format außerhalb", "var i: Integer; begin Inc(i:3) end.", "t.pas(1,27): Error 135: Invalid format specifier"},
		// Aufrufe
		{"zu viele argumente", "procedure P(a: Integer); begin end; begin P(1, 2) end.", "t.pas(1,48): Error 89: \")\" expected"},
		{"zu wenige argumente", "procedure P(a, b: Integer); begin end; begin P(1) end.", "t.pas(1,49): Error 87: \",\" expected"},
		{"klammer fehlt", "procedure P(a: Integer); begin end; begin P end.", "t.pas(1,43): Error 88: \"(\" expected"},
		{"var-parameter konstante", "procedure P(var a: Integer); begin end; begin P(1) end.", "t.pas(1,49): Error 20: Variable identifier expected"},
		{"var-parameter typ", "procedure P(var a: Integer); begin end; var b: Byte; begin P(b) end.", "t.pas(1,62): Error 26: Type mismatch"},
		{"var-parameter string", "procedure P(var s: string); begin end; var t: string[10]; begin P(t) end.", "t.pas(1,67): Error 26: Type mismatch"},
		{"feld über funktionsnamen", "type R = record a: Byte end; function F: R; begin F.a := 1 end; begin end.", "t.pas(1,51): Error 20: Variable identifier expected"},
		{"standardfunktion als anweisung", "begin UpCase('a') end.", "t.pas(1,7): Error 122: Invalid variable reference"},
		{"typumwandlung als anweisung", "var i: Integer; begin Byte(i) end.", "t.pas(1,23): Error 122: Invalid variable reference"},
		{"variable als anweisung", "var i: Integer; begin i end.", "t.pas(1,23): Error 122: Invalid variable reference"},
		{"prozedur als wert", "procedure P; begin end; var i: Integer; begin i := P end.", "t.pas(1,52): Error 143: Invalid procedure or function reference"},
		{"variable aufrufen", "var i: Integer; begin i(1) end.", "t.pas(1,23): Error 143: Invalid procedure or function reference"},
		{"forward fehlt", "procedure P; forward; begin end.", "t.pas(1,11): Error 59: Undefined forward (P)"},
		{"kopf passt nicht", "procedure P(a: Integer); forward; procedure P(a: Byte); begin end; begin end.", "t.pas(1,45): Error 131: Header does not match previous definition (P)"},
		{"ergebnis ohne typ", "function F; begin end; begin end.", "t.pas(1,10): Error 86: \":\" expected"},
		// Standardroutinen
		{"inc real", "var r: Real; begin Inc(r) end.", "t.pas(1,24): Error 104: Ordinal variable expected"},
		{"new kein zeiger", "var i: Integer; begin New(i) end.", "t.pas(1,27): Error 65: Pointer variable expected"},
		{"write record", "var r: record a: Byte end; begin WriteLn(r) end.", "t.pas(1,42): Error 64: Cannot Read or Write variables of this type"},
		{"read boolean", "var b: Boolean; begin ReadLn(b) end.", "t.pas(1,30): Error 64: Cannot Read or Write variables of this type"},
		{"nachkomma bei integer", "var i: Integer; begin WriteLn(i:5:2) end.", "t.pas(1,35): Error 135: Invalid format specifier"},
		{"writeln typisierte datei", "var f: file of Byte; begin WriteLn(f, 1) end.", "t.pas(1,36): Error 63: Invalid file type"},
		{"append binärdatei", "var f: file; begin Append(f) end.", "t.pas(1,27): Error 63: Invalid file type"},
		{"str ohne variable", "begin Str(1, 'x') end.", "t.pas(1,14): Error 20: Variable identifier expected"},
		{"length zahl", "var i: Integer; begin i := Length(5) end.", "t.pas(1,35): Error 26: Type mismatch (string expected)"},
		// Schleifen, Erweiterungen
		{"break außerhalb", "begin Break end.", "t.pas(1,7): Error 903: BREAK or CONTINUE outside of loop"},
		{"continue in prozedur in schleife", "procedure P; begin Continue end; begin while True do P end.", "t.pas(1,20): Error 903: BREAK or CONTINUE outside of loop"},
		{"for real", "var r: Real; begin for r := 1 to 2 do end.", "t.pas(1,24): Error 104: Ordinal variable expected"},
		{"for globale in prozedur", "var i: Integer; procedure P; begin for i := 1 to 2 do end; begin end.", "t.pas(1,40): Error 97: Invalid FOR control variable"},
		{"for feld", "var r: record i: Integer end; begin with r do for i := 1 to 2 do end.", "t.pas(1,51): Error 97: Invalid FOR control variable"},
		{"for-in zuweisung", "var a: array[1..3] of Integer; x: Integer; begin for x in a do x := 1 end.", "t.pas(1,64): Error 43: Illegal assignment"},
		{"for-in var-parameter", "var s: string; c: Char; begin for c in s do ReadLn(c) end.", "t.pas(1,52): Error 122: Invalid variable reference"},
		{"for-in typ", "var a: array[1..3] of Real; x: Integer; begin for x in a do end.", "t.pas(1,51): Error 26: Type mismatch"},
		{"for-in zahl", "var x: Integer; begin for x in 5 do end.", "t.pas(1,32): Error 26: Type mismatch (array, string or set expected)"},
		{"typkonst zuweisen", "const C: Integer = 1; begin C := 2 end.", "t.pas(1,29): Error 43: Illegal assignment"},
		{"typkonst feld zuweisen", "type R = record a: Byte end; const C: R = (a: 1); begin C.a := 2 end.", "t.pas(1,57): Error 43: Illegal assignment"},
		{"typkonst var-parameter", "const C: Integer = 1; begin Inc(C) end.", "t.pas(1,33): Error 122: Invalid variable reference"},
		{"typkonst adresse", "const C: Integer = 1; var p: Pointer; begin p := @C end.", "t.pas(1,51): Error 83: Invalid @ argument"},
		{"const-parameter", "procedure P(const s: string); begin s := '' end; begin end.", "t.pas(1,37): Error 43: Illegal assignment"},
		{"const-parameter weitergeben", "procedure Q(var s: string); begin end; procedure P(const s: string); begin Q(s) end; begin end.", "t.pas(1,78): Error 122: Invalid variable reference"},
		{"with kein record", "var i: Integer; begin with i do end.", "t.pas(1,28): Error 75: Record variable expected"},
		{"goto", "label 1; begin goto 1; 1: end.", "t.pas(1,16): Error 902: Not supported by WasmPascal (goto is not supported before stage 8)"},
		{"label unbekannt", "begin 1: end.", "t.pas(1,7): Error 3: Unknown identifier (1)"},
		{"label doppelt", "label 1; begin 1: ; 1: end.", "t.pas(1,21): Error 81: Label already defined"},
		{"label außen", "label 1; procedure P; begin 1: end; begin 1: end.", "t.pas(1,29): Error 80: Label not within current block"},
	}
	for _, tt := range tests {
		_, err := checkString(tt.src)
		if got := errorOf(t, err); got != tt.want {
			t.Errorf("%s:\n got: %s\nwant: %s", tt.name, got, tt.want)
		}
	}
}
