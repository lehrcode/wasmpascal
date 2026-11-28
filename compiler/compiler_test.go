package compiler

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/runner"
)

func TestDiagnosticError(t *testing.T) {
	tests := []struct {
		d    Diagnostic
		want string
	}{
		{Diagnostic{File: "a.pas", Line: 3, Col: 7, Code: 85, Msg: "\";\" expected"}, `a.pas(3,7): Error 85: ";" expected`},
		{Diagnostic{File: "a.pas", Line: 3, Code: 85, Msg: "x"}, "a.pas(3): Error 85: x"},
		{Diagnostic{File: "a.pas", Code: 15, Msg: "File not found"}, "a.pas: Error 15: File not found"},
	}
	for _, tt := range tests {
		if got := tt.d.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestCompilePreconditions(t *testing.T) {
	ctx := context.Background()
	src := fstest.MapFS{"p.pas": {Data: []byte("program P; begin end.")}}

	if _, err := Compile(ctx, "p.pas", Options{}); err == nil {
		t.Error("Compile ohne FS: kein Fehler")
	}
	if _, err := Compile(ctx, "p.pas", Options{FS: src, Target: Target(99)}); err == nil {
		t.Error("Compile mit unbekanntem Ziel: kein Fehler")
	}

	var d *Diagnostic
	if _, err := Compile(ctx, "fehlt.pas", Options{FS: src}); !errors.As(err, &d) || d.Code != source.ErrFileNotFound {
		t.Errorf("Compile mit fehlender Datei: %v", err)
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Compile(cctx, "p.pas", Options{FS: src}); !errors.Is(err, context.Canceled) {
		t.Errorf("Compile mit abgebrochenem ctx: %v", err)
	}
}

func TestCompileReportsErrors(t *testing.T) {
	src := fstest.MapFS{
		"p.pas":     {Data: []byte("program P;\n{$I teil}\nbegin end.")},
		"teil.pas":  {Data: []byte("{$IFDEF EXTRA}\nconst x = 'offen\n{$ENDIF}")},
		"lib/x.pas": {Data: []byte("")},
	}
	if _, err := Compile(context.Background(), "p.pas", Options{FS: src}); err != nil {
		t.Errorf("ohne EXTRA: %v", err)
	}
	_, err := Compile(context.Background(), "p.pas", Options{FS: src, Defines: []string{"extra"}})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Error() != "teil.pas(2,11): Error 8: String constant exceeds line" {
		t.Errorf("mit EXTRA: %v", err)
	}
}

func TestCompileReportsSyntaxErrors(t *testing.T) {
	src := fstest.MapFS{"p.pas": {Data: []byte("program P;\nbegin\n  if x then\nend")}}
	err := Check(context.Background(), "p.pas", Options{FS: src})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Error() != "p.pas(4,4): Error 10: Unexpected end of file" {
		t.Errorf("Check = %v", err)
	}
}

func TestUnitsInIncludePaths(t *testing.T) {
	src := fstest.MapFS{
		"prog/p.pas":     {Data: []byte("program P;\nuses Hilfe;\nbegin\n  WriteLn(Doppelt(21))\nend.\n")},
		"lib/HILFE.PAS":  {Data: []byte("unit Hilfe;\ninterface\nfunction Doppelt(x: Integer): Integer;\nimplementation\nfunction Doppelt;\nbegin\n  Doppelt := 2 * x\nend;\nend.\n")},
		"lib2/hilfe.pas": {Data: []byte("unit Hilfe;\ninterface\nimplementation\nend.\n")},
	}
	ctx := context.Background()
	var d *Diagnostic
	if _, err := Compile(ctx, "prog/p.pas", Options{FS: src}); !errors.As(err, &d) || d.Error() != "prog/p.pas(2,6): Error 15: File not found (Hilfe.pas)" {
		t.Errorf("ohne -I: %v", err)
	}
	// Der erste Suchpfad gewinnt; Groß-/Kleinschreibung des Dateinamens ist egal.
	if _, err := Compile(ctx, "prog/p.pas", Options{FS: src, IncludePaths: []string{"lib", "lib2"}}); err != nil {
		t.Errorf("mit -I lib: %v", err)
	}
	if _, err := Compile(ctx, "prog/p.pas", Options{FS: src, IncludePaths: []string{"lib2", "lib"}}); !errors.As(err, &d) || d.Code != source.ErrUnknownIdent {
		t.Errorf("mit -I lib2: %v", err)
	}
	// Check einer Unit, die Crt benutzt
	if err := Check(ctx, "lib/HILFE.PAS", Options{FS: src}); err != nil {
		t.Errorf("Check der Unit: %v", err)
	}
	unit := fstest.MapFS{"u.pas": {Data: []byte("unit U;\ninterface\nuses Crt;\nprocedure Rot;\nimplementation\nuses Dos;\nprocedure Rot;\nbegin\n  TextColor(Red)\nend;\nend.\n")}}
	if err := Check(ctx, "u.pas", Options{FS: unit}); err != nil {
		t.Errorf("Check mit Crt/Dos: %v", err)
	}
}

// TestDelayWaits prüft, dass Delay über den Scheduler wirklich wartet.
func TestDelayWaits(t *testing.T) {
	src := fstest.MapFS{"p.pas": {Data: []byte("program P;\nuses Crt;\nbegin\n  Write('a');\n  Delay(200);\n  WriteLn('b')\nend.\n")}}
	res, err := Compile(context.Background(), "p.pas", Options{FS: src})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	start := time.Now()
	if _, err := runner.Run(context.Background(), res.Wasm, runner.Options{Stdout: &out}); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 200*time.Millisecond || d > 5*time.Second {
		t.Errorf("Delay(200) dauerte %v", d)
	}
	if out.String() != "ab\n" {
		t.Errorf("Ausgabe %q", out.String())
	}
}

// TestIncludeLimitAcrossUnits prüft, dass die Grenze für {$I} über alle
// Dateien einer Übersetzung zählt, auch über Units hinweg.
func TestIncludeLimitAcrossUnits(t *testing.T) {
	incs := strings.Repeat("{$I x.inc}\n", 600)
	src := fstest.MapFS{
		"p.pas": {Data: []byte("program P;\nuses U;\n" + incs + "begin\nend.\n")},
		"u.pas": {Data: []byte("unit U;\ninterface\n" + incs + "implementation\nend.\n")},
		"x.inc": {Data: []byte("{ leer }")},
	}
	_, err := Compile(context.Background(), "p.pas", Options{FS: src})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != 905 {
		t.Fatalf("err = %v, want Error 905", err)
	}
	// p.pas bindet 600-mal ein, die 1001. Einbindung ist die 401. in u.pas
	if want := "u.pas(403,1): Error 905: Source too large (more than 1000 {$I} directives)"; d.Error() != want {
		t.Errorf("\n got: %s\nwant: %s", d.Error(), want)
	}
}

// external (Etappe 17): unter gojs nur in der System-Unit; derselbe Import
// aus mehreren Deklarationen steht einmal im Modul.
func TestExternal(t *testing.T) {
	const src = `program P;
function A(X: LongInt): LongInt; external 'host' name 'f';
function B(Y: LongInt): LongInt; external 'host' name 'f';
begin
  WriteLn(A(1) + B(2))
end.
`
	fsys := fstest.MapFS{"p.pas": {Data: []byte(src)}}
	res, err := Compile(context.Background(), "p.pas", Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(res.Wasm, []byte("\x04host\x01f")); n != 1 {
		t.Errorf("Import host.f %d-mal im Modul", n)
	}
	_, err = Compile(context.Background(), "p.pas", Options{FS: fsys, Target: TargetGoJS})
	wantDiag(t, err, 902)
}
