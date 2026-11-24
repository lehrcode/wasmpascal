package compiler

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"
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
	if _, err := Compile(ctx, "fehlt.pas", Options{FS: src}); !errors.As(err, &d) || d.Code != errFileNotFound {
		t.Errorf("Compile mit fehlender Datei: %v", err)
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Compile(cctx, "p.pas", Options{FS: src}); !errors.Is(err, context.Canceled) {
		t.Errorf("Compile mit abgebrochenem ctx: %v", err)
	}
}
