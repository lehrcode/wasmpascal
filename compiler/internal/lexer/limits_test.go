package lexer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

// lexCount zerlegt main.pas aus files vollständig und zählt die Tokens.
func lexCount(t *testing.T, files fstest.MapFS) (int, error) {
	t.Helper()
	l, err := New(source.NewLoader(files, nil), "main.pas", source.NewDefines())
	if err != nil {
		return 0, err
	}
	n := 0
	for {
		tok, err := l.Next()
		if err != nil {
			return n, err
		}
		if tok.Kind == EOF {
			return n, nil
		}
		n++
	}
}

func wantCode(t *testing.T, err error, code int) {
	t.Helper()
	var se *source.Error
	if !errors.As(err, &se) || se.Code != code {
		t.Fatalf("err = %v, want Error %d", err, code)
	}
}

// Dieselbe Datei mehrmals hintereinander einzubinden ist kein Zyklus.
func TestIncludeTwice(t *testing.T) {
	files := fstest.MapFS{
		"main.pas": {Data: []byte("{$I x.inc}{$I x.inc}")},
		"x.inc":    {Data: []byte("a b")},
	}
	n, err := lexCount(t, files)
	if err != nil || n != 4 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
}

// Eine Kette, in der jede Datei die nächste zweimal einbindet, vervielfacht
// den Quelltext (2^14 Einbindungen); die Gesamtzahl der {$I} begrenzt das.
func TestIncludeDoublingChain(t *testing.T) {
	files := fstest.MapFS{"main.pas": {Data: []byte("{$I f1.inc}")}}
	for i := 1; i <= 14; i++ {
		next := fmt.Sprintf("{$I f%d.inc}{$I f%d.inc}", i+1, i+1)
		if i == 14 {
			next = "x"
		}
		files[fmt.Sprintf("f%d.inc", i)] = &fstest.MapFile{Data: []byte(next)}
	}
	start := time.Now()
	_, err := lexCount(t, files)
	wantCode(t, err, source.ErrSourceTooLarge)
	if !strings.Contains(err.Error(), "Error 905: Source too large (more than 1000 {$I} directives)") {
		t.Errorf("err = %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("Abbruch erst nach %v", d)
	}
}

// Genau MaxIncludes Einbindungen sind erlaubt, eine mehr nicht.
func TestIncludeCount(t *testing.T) {
	for _, tt := range []struct {
		n    int
		fail bool
	}{{source.MaxIncludes, false}, {source.MaxIncludes + 1, true}} {
		files := fstest.MapFS{
			"main.pas": {Data: []byte(strings.Repeat("{$I x.inc}\n", tt.n))},
			"x.inc":    {Data: []byte("a")},
		}
		n, err := lexCount(t, files)
		if !tt.fail {
			if err != nil || n != tt.n {
				t.Errorf("%d Einbindungen: n = %d, err = %v", tt.n, n, err)
			}
			continue
		}
		wantCode(t, err, source.ErrSourceTooLarge)
		if want := fmt.Sprintf("main.pas(%d,1):", tt.n); !strings.HasPrefix(err.Error(), want) {
			t.Errorf("err = %v, want Stelle %s", err, want)
		}
	}
}

// Die Grenze für die Quelltextmenge zählt jede Einbindung erneut.
func TestIncludeSourceBytes(t *testing.T) {
	big := strings.Repeat("{ Kommentar }\n", 1<<20/14+1) // gut 1 MB
	files := fstest.MapFS{
		"main.pas":  {Data: []byte(strings.Repeat("{$I gross.inc}\n", 16))},
		"gross.inc": {Data: []byte(big)},
	}
	_, err := lexCount(t, files)
	wantCode(t, err, source.ErrSourceTooLarge)
	if !strings.Contains(err.Error(), "(more than 16 MB of source text)") {
		t.Errorf("err = %v", err)
	}
}
