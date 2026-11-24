// Package compiler übersetzt Turbo-Pascal-4.0-Quelltext in ein WebAssembly-Modul.
//
// Das Paket nutzt nur die Go-Standardbibliothek und baut auch mit GOOS=js und
// GOOS=wasip1. Quelltexte werden ausschließlich über Options.FS gelesen, damit
// sie im Browser aus dem Speicher kommen können.
//
// Stand Etappe 4: Compile erzeugt WASI-Module für Programme mit Ganzzahlen,
// Reals, Boolean, Char, Arrays, Prozeduren und Funktionen. Noch nicht
// übersetzbare Sprachmittel melden Error 902 mit der Etappe, in der sie folgen.
package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/codegen"
	"github.com/lehrcode/wasmpascal/compiler/internal/ir"
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/parser"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

// Target wählt die Ziel-ABI des erzeugten Moduls.
type Target int

const (
	// TargetWASI erzeugt ein Modul für WASI Preview 1 (wazero, wasmtime, Node).
	TargetWASI Target = iota
	// TargetGoJS erzeugt ein Modul mit der Schnittstelle von GOOS=js GOARCH=wasm,
	// lauffähig mit wasm_exec.js im Browser.
	TargetGoJS
)

// Options steuert die Übersetzung.
type Options struct {
	// Target ist die Ziel-ABI.
	Target Target
	// FS liefert die Quelltexte; im Browser z. B. ein fstest.MapFS.
	// Alle Pfade sind fs.FS-Pfade (Schrägstriche, relativ zur Wurzel von FS).
	FS fs.FS
	// IncludePaths sind zusätzliche Suchpfade für Units und {$I}.
	IncludePaths []string
	// Defines sind Symbole, die wie {$DEFINE} vorab gesetzt werden.
	Defines []string
	// StackSize ist die Größe des Schatten-Stacks in Byte; 0 bedeutet {$M} oder 1 MB.
	StackSize int
	// NoLines lässt Zeilentabelle und Name Section weg.
	NoLines bool

	// forceFrame legt alle Variablen in den Frame (nur für Tests im Paket).
	forceFrame bool
}

// Result ist das Ergebnis einer erfolgreichen Übersetzung.
type Result struct {
	// Wasm ist das binäre WebAssembly-Modul.
	Wasm []byte
}

// Diagnostic ist der Fehlertyp für Compilerfehler; er lässt sich mit errors.As
// aus dem Fehler von Compile und Check herausholen.
type Diagnostic struct {
	// File ist der Pfad der Quelldatei, wie er in Options.FS heißt.
	File string
	// Line und Col sind 1-basiert; 0 bedeutet „unbekannt“.
	Line, Col int
	// Code ist die TP-Fehlernummer.
	Code int
	// Msg ist der Fehlertext.
	Msg string
}

// Error formatiert die Meldung wie Turbo Pascal: "datei.pas(zeile,spalte): Error NN: Text".
// Ohne Positionsangabe entfällt der Klammerteil.
func (d *Diagnostic) Error() string {
	switch {
	case d.Line > 0 && d.Col > 0:
		return fmt.Sprintf("%s(%d,%d): Error %d: %s", d.File, d.Line, d.Col, d.Code, d.Msg)
	case d.Line > 0:
		return fmt.Sprintf("%s(%d): Error %d: %s", d.File, d.Line, d.Code, d.Msg)
	default:
		return fmt.Sprintf("%s: Error %d: %s", d.File, d.Code, d.Msg)
	}
}

// Compile übersetzt das Programm mainFile (ein Pfad in opts.FS) samt aller
// benutzten Units in ein WebAssembly-Modul.
//
// Compilerfehler werden als *Diagnostic geliefert. Ein Abbruch von ctx beendet
// die Übersetzung mit ctx.Err().
func Compile(ctx context.Context, mainFile string, opts Options) (*Result, error) {
	return compile(ctx, mainFile, opts, true)
}

// Check prüft das Programm mainFile wie Compile, erzeugt aber kein Modul.
func Check(ctx context.Context, mainFile string, opts Options) error {
	_, err := compile(ctx, mainFile, opts, false)
	return err
}

func compile(ctx context.Context, mainFile string, opts Options, emit bool) (res *Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("compiler: interner Fehler: %v", r)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.FS == nil {
		return nil, errors.New("compiler: Options.FS ist nil")
	}
	if opts.Target != TargetWASI && opts.Target != TargetGoJS {
		return nil, fmt.Errorf("compiler: unbekanntes Ziel %d", opts.Target)
	}
	if opts.StackSize < 0 {
		return nil, fmt.Errorf("compiler: negative Stackgröße %d", opts.StackSize)
	}
	loader := source.NewLoader(opts.FS, opts.IncludePaths)
	lx, err := lexer.New(loader, mainFile, source.NewDefines(opts.Defines...))
	if err != nil {
		return nil, diagnostic(err)
	}
	// Stand Etappe 3: Der Quelltext wird geparst und typgeprüft.
	file, err := parser.Parse(lx)
	if err != nil {
		return nil, diagnostic(err)
	}
	info, err := check.Check(file, check.Config{})
	if err != nil {
		return nil, diagnostic(err)
	}
	if !emit {
		return nil, nil
	}
	prog, ok := file.(*ast.Program)
	if !ok {
		return nil, &Diagnostic{File: mainFile, Code: source.ErrSyntax, Msg: "Syntax error (program expected, found a unit)"}
	}
	if opts.Target == TargetGoJS {
		return nil, errors.New("compiler: Ziel gojs folgt in Etappe 9")
	}
	stack := int64(opts.StackSize)
	if stack == 0 {
		stack = int64(lx.StackSize())
	}
	if stack == 0 {
		stack = 1 << 20
	}
	m, err := ir.Lower(prog, info, ir.Options{StackSize: stack})
	if err != nil {
		return nil, diagnostic(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Result{Wasm: codegen.Generate(m, codegen.Options{ForceFrame: opts.forceFrame})}, nil
}

// diagnostic wandelt interne Compilerfehler in *Diagnostic um.
func diagnostic(err error) error {
	var se *source.Error
	if errors.As(err, &se) {
		return &Diagnostic{File: se.Pos.File, Line: se.Pos.Line, Col: se.Pos.Col, Code: se.Code, Msg: se.Msg}
	}
	return err
}
