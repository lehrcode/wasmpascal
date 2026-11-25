package compiler

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

// FuzzCompile übersetzt beliebige Quelltexte mit den voreingestellten
// Grenzen. Befund ist ein interner Fehler (ErrInternal), ein Hängen (mehr als
// 5 s) oder ein Absturz; Quelltextfehler (*Diagnostic) sind erwartet.
// Startkorpus sind die Testprogramme unter testdata/. Laufen lassen mit
//
//	go test ./compiler -run '^$' -fuzz=FuzzCompile -fuzztime=10m
func FuzzCompile(f *testing.F) {
	_ = filepath.WalkDir(filepath.Join("..", "testdata"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".pas" {
			if data, err := os.ReadFile(p); err == nil {
				f.Add(data)
			}
		}
		return nil
	})
	f.Fuzz(func(t *testing.T, src []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := Compile(ctx, "main.pas", Options{FS: fstest.MapFS{"main.pas": {Data: src}}})
		if errors.Is(err, ErrInternal) {
			t.Fatalf("interner Fehler: %v", err)
		}
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			t.Fatalf("hängt: %v", err)
		}
	})
}
