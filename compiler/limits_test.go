package compiler

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lehrcode/wasmpascal/runner"
)

// compileSrc übersetzt main.pas mit src und einem Zeitlimit von 2 s.
func compileSrc(t *testing.T, src string, opts Options) (*Result, error, time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	opts.FS = fstest.MapFS{"main.pas": {Data: []byte(src)}}
	start := time.Now()
	res, err := Compile(ctx, "main.pas", opts)
	return res, err, time.Since(start)
}

// wantDiag verlangt ein *Diagnostic mit der Nummer code.
func wantDiag(t *testing.T, err error, code int) *Diagnostic {
	t.Helper()
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != code {
		t.Fatalf("err = %v, want Error %d", err, code)
	}
	return d
}

// Eine lange Zeile kostet nicht mehr quadratisch viel Zeit (Spalten werden
// schrittweise gezählt), und die Spalte in der Meldung stimmt weiter.
func TestLongLine(t *testing.T) {
	prefix := "program P; var x: Integer; begin " + strings.Repeat("x := 1; ", 50000) + "x := Größe; "
	src := prefix + "x := ; end."
	_, err, d := compileSrc(t, src, Options{})
	if d > time.Second {
		t.Errorf("Übersetzung dauerte %v", d)
	}
	diag := wantDiag(t, err, 901) // Umlaut im Bezeichner
	wantCol := len([]rune(prefix[:strings.Index(prefix, "Größe")])) + 3
	if diag.Line != 1 || diag.Col != wantCol {
		t.Errorf("Stelle (%d,%d), erwartet (1,%d)", diag.Line, diag.Col, wantCol)
	}
}

// heapGrowth liefert, wie viel Heap f zusätzlich belegt hat (grob).
func heapGrowth(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	if after.TotalAlloc < before.TotalAlloc {
		return 0
	}
	return after.TotalAlloc - before.TotalAlloc
}

// Riesige globale Variablen ergeben Error 49, bevor Speicher belegt wird.
func TestStaticDataLimit(t *testing.T) {
	var err error
	grown := heapGrowth(func() {
		_, err, _ = compileSrc(t, "program P;\nvar A, B, C: array[1..2000000000] of Byte;\nbegin\nend.\n", Options{})
	})
	if d := wantDiag(t, err, 49); d.Line != 2 || d.Col != 5 {
		t.Errorf("Stelle (%d,%d), erwartet (2,5)", d.Line, d.Col)
	}
	if grown > 64<<20 {
		t.Errorf("%d MB Speicher belegt", grown>>20)
	}
	// Die Summe zählt: drei Arrays zu je 100 MB überschreiten 256 MB beim dritten.
	_, err, _ = compileSrc(t, "program P;\nvar A, B: array[1..100000000] of Byte;\n  C: array[1..100000000] of Byte;\nbegin\nend.\n", Options{})
	if d := wantDiag(t, err, 49); d.Line != 3 || d.Col != 3 {
		t.Errorf("Stelle (%d,%d), erwartet (3,3)", d.Line, d.Col)
	}
	// Lokale Variablen liegen auf dem Schatten-Stack und zählen nicht.
	_, err, _ = compileSrc(t, "program P;\nprocedure Q;\nvar A: array[1..300000000] of Byte;\nbegin\nend;\nbegin\nend.\n", Options{})
	if err != nil {
		t.Errorf("lokales Array: %v", err)
	}
}

// Tief verschachtelte Quelltexte ergeben schnell Error 904, ohne großen
// Speicherverbrauch.
func TestNestingLimit(t *testing.T) {
	rep := strings.Repeat
	tests := []struct {
		name, src string
	}{
		{"klammern", "program P; var x: Integer; begin x := " + rep("(", 100000) + "1" + rep(")", 100000) + " end."},
		{"plus-kette", "program P; var x: Integer; begin x := 0" + rep("+x", 100000) + " end."},
		{"mal-kette", "program P; var x: Integer; begin x := 1" + rep("*x", 100000) + " end."},
		{"vorzeichen", "program P; var x: Integer; begin x := " + rep("-", 100000) + "1 end."},
		{"else-if", "program P; var x: Integer; begin " + rep("if x = 0 then x := 1 else ", 100000) + "x := 2 end."},
		{"begin", "program P; begin " + rep("begin ", 10000) + rep("end; ", 10000) + "end."},
		{"while", "program P; var x: Integer; begin " + rep("while x > 0 do ", 10000) + "x := 0 end."},
		{"array-typ", "program P; var a: " + rep("array[1..2] of ", 10000) + "Byte; begin end."},
		{"record-typ", "program P; var r: " + rep("record a: ", 10000) + "Byte" + rep(" end", 10000) + "; begin end."},
		{"prozeduren", "program P; " + rep("procedure Q; ", 5000) + rep("begin end; ", 5000) + "begin end."},
		{"designator", "program P; var x: Integer; begin x := x" + rep("^", 100000) + " end."},
		{"konstante", "program P; const C: array[1..1] of Integer = " + rep("(", 10000) + "1" + rep(")", 10000) + "; begin end."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			var d time.Duration
			grown := heapGrowth(func() { _, err, d = compileSrc(t, tt.src, Options{}) })
			wantDiag(t, err, 904)
			if d > time.Second {
				t.Errorf("dauerte %v", d)
			}
			if grown > 64<<20 {
				t.Errorf("%d MB Speicher belegt", grown>>20)
			}
		})
	}
}

// Bis zur Grenze (500) geht es; der Parser zählt Ebenen, nicht Klammern.
func TestNestingWithinLimit(t *testing.T) {
	rep := strings.Repeat
	src := "program P; var x: Integer; begin x := " + rep("(", 200) + "1" + rep(")", 200) + rep("+x", 200) + "; WriteLn(x) end."
	if _, err, _ := compileSrc(t, src, Options{}); err != nil {
		t.Fatal(err)
	}
}

// Programme knapp unter der Tiefengrenze laufen schnell durch alle Phasen.
func TestNestingNearLimit(t *testing.T) {
	rep := strings.Repeat
	tests := []struct{ name, src string }{
		{"else-if", "program P; var x: Integer; begin " + rep("if x = 0 then x := 1 else ", 240) + "x := 2 end."},
		{"while", "program P; var x: Integer; begin " + rep("while x > 0 do ", 450) + "x := 0 end."},
		{"begin", "program P; var x: Integer; begin " + rep("begin x := 1; ", 450) + rep("end; ", 450) + "end."},
		{"klammern", "program P; var x: Integer; begin x := " + rep("(", 160) + "1" + rep(")", 160) + " end."},
		{"plus-kette", "program P; var x: Integer; begin x := 0" + rep("+x", 450) + " end."},
		{"prozeduren", "program P; " + rep("procedure Q; var x: Integer; ", 240) + rep("begin x := 1 end; ", 240) + "begin end."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err, d := compileSrc(t, tt.src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if d > time.Second {
				t.Errorf("dauerte %v", d)
			}
		})
	}
}

// Mehr als DefaultMaxTokens Tokens ergeben Error 906.
func TestTokenLimit(t *testing.T) {
	src := "program P; var x: Integer; begin\n" + strings.Repeat("x := 1;\n", 500_000) + "end.\n"
	_, err, d := compileSrc(t, src, Options{})
	diag := wantDiag(t, err, 906)
	if !strings.Contains(diag.Msg, "more than 2000000 tokens") {
		t.Errorf("Meldung %q", diag.Msg)
	}
	if d > time.Second {
		t.Errorf("dauerte %v", d)
	}
}

// panicFS löst beim Öffnen eine Panic aus, als stünde ein Fehler im Compiler.
type panicFS struct{}

func (panicFS) Open(string) (fs.File, error) { panic("kaputt") }

// Eine Panic im Compiler kommt als ErrInternal zurück, nicht als Diagnostic.
func TestErrInternal(t *testing.T) {
	_, err := Compile(context.Background(), "main.pas", Options{FS: panicFS{}})
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want ErrInternal", err)
	}
	var d *Diagnostic
	if errors.As(err, &d) {
		t.Errorf("auch Diagnostic: %v", d)
	}
	if !strings.Contains(err.Error(), "kaputt") {
		t.Errorf("err = %v", err)
	}
}

// Jede Grenze lässt sich über Limits ändern und mit -1 abschalten.
func TestLimitsConfigurable(t *testing.T) {
	rep := strings.Repeat
	deep := "program P; var x: Integer; begin x := " + rep("(", 1000) + "1" + rep(")", 1000) + " end."
	small := "program P; var x: Integer; begin x := ((1)) end."
	big := "program P; var A: array[1..300000000] of Byte; begin end."
	tests := []struct {
		name string
		src  string
		lim  Limits
		code int // 0 = muss übersetzen
	}{
		{"tiefe kleiner", small, Limits{MaxDepth: 3}, 904},
		{"tiefe aus", deep, Limits{MaxDepth: -1}, 0},
		{"tokens kleiner", small, Limits{MaxTokens: 10}, 906},
		{"tokens aus", small, Limits{MaxTokens: -1}, 0},
		{"bytes kleiner", small, Limits{MaxSourceBytes: 20}, 905},
		{"daten kleiner", "program P; var A: array[1..2000] of Byte; begin end.", Limits{MaxStaticData: 1000}, 49},
		{"daten aus", big, Limits{MaxStaticData: -1}, 0},
		{"stack kleiner", "{$M 70000} program P; begin end.", Limits{MaxStackSize: 65536}, 17},
		{"stack aus", "{$M 300000000} program P; begin end.", Limits{MaxStackSize: -1}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err, _ := compileSrc(t, tt.src, Options{Limits: tt.lim})
			if tt.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			wantDiag(t, err, tt.code)
		})
	}
	// Includes
	src := fstest.MapFS{
		"main.pas": {Data: []byte("program P; {$I a.inc} {$I a.inc} begin end.")},
		"a.inc":    {Data: []byte("")},
	}
	_, err := Compile(context.Background(), "main.pas", Options{FS: src, Limits: Limits{MaxIncludes: 1}})
	wantDiag(t, err, 905)
	if _, err := Compile(context.Background(), "main.pas", Options{FS: src, Limits: Limits{MaxIncludes: -1}}); err != nil {
		t.Error(err)
	}
	// Options.StackSize über der Grenze ist ein Fehler ohne Diagnostic
	if _, err, _ := compileSrc(t, "program P; begin end.", Options{StackSize: 1 << 29}); err == nil || errors.As(err, new(*Diagnostic)) {
		t.Errorf("StackSize: err = %v", err)
	}
}

// Nicht initialisierte globale Variablen liegen im BSS: Das Modul bleibt
// klein, und das Programm kann den Speicher trotzdem benutzen.
func TestBSS(t *testing.T) {
	src := `program Gross;
var
  A: array[1..1000000] of Byte;
  B: array[1..1000] of LongInt;
  I: LongInt;
const
  C: array[1..3] of Integer = (7, 8, 9);
begin
  for I := 1 to 1000000 do
    A[I] := I mod 251;
  for I := 1 to 1000 do
    B[I] := I * 3;
  WriteLn(A[1], ' ', A[1000000], ' ', B[1000], ' ', C[3], ' ', Ofs(B) - Ofs(A))
end.
`
	res, err, _ := compileSrc(t, src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(res.Wasm); n > 200_000 {
		t.Errorf("Modul hat %d Byte", n)
	}
	var out bytes.Buffer
	if _, err := runner.Run(context.Background(), res.Wasm, runner.Options{Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if want := "1 16 3000 9 1000000\n"; out.String() != want {
		t.Errorf("Ausgabe %q, erwartet %q", out.String(), want)
	}
	// Ohne Grenze für statische Daten geht auch ein Array von 300 MB schnell.
	_, err, d := compileSrc(t, "program P; var A: array[1..300000000] of Byte; begin A[1] := 1 end.", Options{Limits: Limits{MaxStaticData: -1}})
	if err != nil {
		t.Fatal(err)
	}
	if d > time.Second/2 {
		t.Errorf("dauerte %v", d)
	}
}

// Ein ctx, der während der Übersetzung abläuft, bricht sie mitten im Lexer ab.
func TestCompileDeadline(t *testing.T) {
	src := "program P; var x: Integer; begin\n" + strings.Repeat("x := 1;\n", 1_000_000) + "end.\n"
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Compile(ctx, "main.pas", Options{
		FS:     fstest.MapFS{"main.pas": {Data: []byte(src)}},
		Limits: Limits{MaxTokens: -1},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("Abbruch erst nach %v", d)
	}
}
