package compiler

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler/internal/source"
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
