// Package compiler übersetzt Turbo-Pascal-4.0-Quelltext in ein WebAssembly-Modul.
//
// Das Paket nutzt nur die Go-Standardbibliothek und baut auch mit GOOS=js und
// GOOS=wasip1. Quelltexte werden ausschließlich über Options.FS gelesen, damit
// sie im Browser aus dem Speicher kommen können.
//
// Die System-Unit (rtl/system.pas, eingebettet über das Paket rtl) wird
// implizit mitgeprüft und mitübersetzt. Noch nicht übersetzbare Sprachmittel
// melden Error 902 mit der Etappe, in der sie folgen.
//
// # Einbetten in Server
//
// Compile und Check dürfen Quelltexte aus fremder Hand übersetzen. Garantiert
// ist:
//
//   - Das Ergebnis ist immer ein Modul, ein *Diagnostic (Fehler im
//     Quelltext), ctx.Err() (Abbruch) oder ein Fehler mit ErrInternal (Fehler
//     im Compiler, abgefangene Panic). Was Go nicht abfangen kann –
//     Überlauf des Go-Stacks und Speichermangel –, verhindern die Grenzen
//     unten; geprüft mit Extremfällen und Fuzzing (FuzzCompile).
//   - Arbeit und Speicher sind über Options.Limits begrenzt: Quelltextmenge
//     (auch über {$I} vervielfacht), Tokens, Schachtelungstiefe, statische
//     Daten, Stackgröße. Die Voreinstellungen gelten auch, wenn Limits
//     nicht gesetzt ist; jede Grenze lässt sich ändern oder abschalten.
//   - ctx wird zwischen den Phasen, im Lexer alle 4096 Tokens und in der
//     Typprüfung je Routine geprüft.
//   - Der Compiler startet keine Goroutinen, hat keinen globalen
//     veränderlichen Zustand und liest nur über Options.FS.
//
// Nicht garantiert ist eine harte Obergrenze für den Speicher des ganzen
// Prozesses: Die Grenzen halten den Verbrauch einer Übersetzung klein (die
// Tests verlangen unter 64 MB für die bekannten Extremfälle), aber viele
// gleichzeitige Übersetzungen addieren sich. Wer fremde Eingaben in einem
// gemeinsam genutzten Server übersetzt, sollte dafür zusätzlich einen
// eigenen Prozess mit Speichergrenze verwenden.
package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/lehrcode/wasmpascal/compiler/internal/ast"
	"github.com/lehrcode/wasmpascal/compiler/internal/check"
	"github.com/lehrcode/wasmpascal/compiler/internal/codegen"
	"github.com/lehrcode/wasmpascal/compiler/internal/ir"
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/parser"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
	"github.com/lehrcode/wasmpascal/rtl"
)

// Target wählt die Ziel-ABI des erzeugten Moduls.
type Target int

const (
	// TargetWASI erzeugt ein Modul für WASI Preview 1 (wazero, wasmtime, Node).
	TargetWASI Target = iota
	// TargetGoJS erzeugt ein Modul mit der Schnittstelle von GOOS=js GOARCH=wasm
	// (Exporte run, resume, getsp, mem), lauffähig mit wasm_exec.js und
	// web/loader.js im Browser oder in Node.js. Dateizugriffe gehen über
	// WASI-Importe, die web/loader.js bereitstellt.
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
	// Limits begrenzt Arbeit und Speicher der Übersetzung, etwa für
	// Quelltexte aus fremder Hand; der Nullwert sind sichere Voreinstellungen.
	Limits Limits

	// forceFrame legt alle Variablen in den Frame (nur für Tests im Paket).
	forceFrame bool
}

// Limits begrenzt, wie viel Arbeit und Speicher eine Übersetzung kostet. In
// jedem Feld bedeutet 0 die Voreinstellung und eine negative Zahl, dass die
// Grenze abgeschaltet ist. Die Voreinstellungen lassen auch große
// Lehrprogramme zu. Wird eine Grenze überschritten, liefert Compile ein
// *Diagnostic mit der angegebenen Nummer.
type Limits struct {
	// MaxDepth begrenzt die Schachtelung von Ausdrücken, Anweisungen, Typen,
	// typisierten Konstanten und Prozeduren (500; Error 904). Ketten wie
	// a+b+c zählen je Operator eine Ebene.
	MaxDepth int
	// MaxSourceBytes begrenzt den gelesenen Quelltext aller Dateien; jede
	// {$I}-Einbindung zählt erneut, die eingebettete Laufzeit nicht
	// (16 MB; Error 905).
	MaxSourceBytes int
	// MaxIncludes begrenzt die Anzahl der {$I}-Einbindungen (1000; Error 905).
	MaxIncludes int
	// MaxTokens begrenzt die Tokens aller Dateien und damit die Größe des
	// Syntaxbaums (2 000 000; Error 906).
	MaxTokens int
	// MaxStaticData begrenzt die Summe aller globalen Variablen und
	// typisierten Konstanten in Byte (256 MB; Error 49).
	MaxStaticData int64
	// MaxStackSize begrenzt die Größe des Schatten-Stacks aus {$M}
	// (Error 17) bzw. Options.StackSize (Fehler ohne Diagnostic) (256 MB).
	MaxStackSize int
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

// ErrInternal kennzeichnet einen internen Fehler des Compilers (eine
// abgefangene Panic) im Unterschied zu Fehlern im Quelltext (*Diagnostic);
// errors.Is(err, ErrInternal) erkennt ihn. Ein Server sollte ihn protokollieren.
var ErrInternal = errors.New("compiler: interner Fehler")

func compile(ctx context.Context, mainFile string, opts Options, emit bool) (res *Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("%w: %v", ErrInternal, r)
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
	lim := opts.Limits
	if max := lim.MaxStackSize; max >= 0 {
		if max == 0 {
			max = source.DefaultMaxStackSize
		}
		if opts.StackSize > max {
			return nil, fmt.Errorf("compiler: Stackgröße %d über der Grenze %d", opts.StackSize, max)
		}
	}
	// Das Ziel ist als bedingtes Symbol WASI bzw. GOJS definiert.
	opts.Defines = append(append([]string{}, opts.Defines...), targetSymbol(opts.Target))
	loader := source.NewLoader(opts.FS, opts.IncludePaths)
	loader.MaxIncludes, loader.MaxSourceBytes = lim.MaxIncludes, lim.MaxSourceBytes
	loader.MaxTokens, loader.MaxStackSize = lim.MaxTokens, lim.MaxStackSize
	loader.Ctx = ctx
	lx, err := lexer.New(loader, mainFile, source.NewDefines(opts.Defines...))
	if err != nil {
		return nil, diagnostic(err)
	}
	file, err := parser.ParseDepth(lx, lim.MaxDepth)
	if err != nil {
		return nil, diagnostic(err)
	}
	system, err := parseSystem(opts.Defines)
	if err != nil {
		return nil, err
	}
	units, embedded, err := loadUnits(ctx, file, loader, path.Dir(mainFile), opts)
	if err != nil {
		return nil, diagnostic(err)
	}
	info, err := check.Check(file, check.Config{System: system, Units: units, Embedded: embedded,
		MaxStaticData: lim.MaxStaticData, Ctx: ctx})
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
	stack := int64(opts.StackSize)
	if stack == 0 {
		stack = int64(lx.StackSize())
	}
	if stack == 0 {
		stack = 1 << 20
	}
	m, err := ir.Lower(prog, system, info, ir.Options{StackSize: stack, NoLines: opts.NoLines})
	if err != nil {
		return nil, diagnostic(err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Result{Wasm: codegen.Generate(m, codegen.Options{ForceFrame: opts.forceFrame, NoLines: opts.NoLines,
		GoJS: opts.Target == TargetGoJS})}, nil
}

// loadUnits sucht und parst alle Units, die von file aus per uses erreichbar
// sind: zuerst im Verzeichnis des Programms, dann in den IncludePaths, zuletzt
// unter den eingebetteten Laufzeit-Units (Crt, Dos). Dateinamen sind der
// Unit-Name mit .pas; Groß-/Kleinschreibung ist egal.
func loadUnits(ctx context.Context, file ast.File, loader *source.Loader, dir string, opts Options) (map[string]*ast.Unit, map[*ast.Unit]bool, error) {
	units := map[string]*ast.Unit{}
	embedded := map[*ast.Unit]bool{}
	rtlLoader := source.NewLoader(rtl.FS, nil)
	var queue []*ast.Ident
	addUses := func(f ast.File) {
		switch f := f.(type) {
		case *ast.Program:
			queue = append(queue, f.Uses...)
		case *ast.Unit:
			queue = append(queue, f.IntfUses...)
			queue = append(queue, f.ImplUses...)
		}
	}
	addUses(file)
	if u, ok := file.(*ast.Unit); ok {
		units[u.Name.Name] = u
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, done := units[id.Name]; done || id.Name == "system" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		name := id.Text + ".pas"
		l, found, isRTL := loader, "", false
		if f, err := loader.Resolve(name, dir); err == nil {
			found = f
		} else if f, err := rtlLoader.Resolve(name, "."); err == nil {
			l, found, isRTL = rtlLoader, f, true
		} else {
			return nil, nil, source.Errorf(id.Pos(), source.ErrFileNotFound, "%s", name)
		}
		lx, err := lexer.New(l, found, source.NewDefines(opts.Defines...))
		if err != nil {
			return nil, nil, err
		}
		f, err := parser.ParseDepth(lx, opts.Limits.MaxDepth)
		if err != nil {
			return nil, nil, err
		}
		u, ok := f.(*ast.Unit)
		if !ok {
			return nil, nil, source.Errorf(f.Pos(), source.ErrUnitExpected, "")
		}
		if u.Name.Name != id.Name {
			return nil, nil, source.Errorf(u.Name.Pos(), source.ErrUnitNameMismatch, "%s", u.Name.Text)
		}
		units[id.Name] = u
		if isRTL {
			embedded[u] = true
		}
		addUses(u)
	}
	if u, ok := file.(*ast.Unit); ok {
		delete(units, u.Name.Name)
	}
	return units, embedded, nil
}

func targetSymbol(t Target) string {
	if t == TargetGoJS {
		return "GOJS"
	}
	return "WASI"
}

// parseSystem liest die System-Unit aus rtl.FS. Fehler darin sind Fehler des
// Compilers selbst und werden als interne Fehler gemeldet.
func parseSystem(defines []string) (*ast.Unit, error) {
	lx, err := lexer.New(source.NewLoader(rtl.FS, nil), "system.pas", source.NewDefines(defines...))
	if err != nil {
		return nil, fmt.Errorf("compiler: rtl/system.pas: %w", err)
	}
	f, err := parser.Parse(lx)
	if err != nil {
		return nil, fmt.Errorf("compiler: rtl/system.pas: %w", err)
	}
	return f.(*ast.Unit), nil
}

// diagnostic wandelt interne Compilerfehler in *Diagnostic um.
func diagnostic(err error) error {
	var se *source.Error
	if errors.As(err, &se) {
		return &Diagnostic{File: se.Pos.File, Line: se.Pos.Line, Col: se.Pos.Col, Code: se.Code, Msg: se.Msg}
	}
	return err
}
