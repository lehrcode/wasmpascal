// Package runner führt mit compiler.TargetWASI erzeugte Module in wazero aus.
package runner

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// Options steuert die Ausführung.
type Options struct {
	// Args ist die Kommandozeile wie os.Args: Args[0] ist der Programmname
	// (ParamStr(0)), die übrigen Einträge sind ParamStr(1) bis ParamStr(ParamCount).
	// Ist Args leer, heißt das Programm "program".
	Args []string
	// Stdin liefert die Eingabe, zeilenweise; das Terminal bleibt im Zeilenmodus.
	// nil bedeutet leere Eingabe.
	Stdin io.Reader
	// Stdout und Stderr nehmen die Ausgaben auf; nil verwirft sie.
	Stdout, Stderr io.Writer
	// Dirs bildet Gastpfade auf Hostverzeichnisse ab, z. B. {"/": "."}.
	// Ohne Eintrag hat das Programm keinen Dateizugriff.
	Dirs map[string]string
}

// RuntimeError ist der Fehlertyp für Pascal-Laufzeitfehler; er lässt sich mit
// errors.As aus dem Fehler von Run herausholen.
type RuntimeError struct {
	// Code ist die TP-Fehlernummer, z. B. 200.
	Code int
	// File, Line und Proc nennen die Fehlerstelle; leer bzw. 0, wenn ohne
	// Zeilentabelle übersetzt wurde.
	File string
	Line int
	Proc string
}

// Error formatiert die Meldung als "Runtime error NNN at datei.pas(zeile) in Prozedur".
// Fehlende Angaben werden weggelassen.
func (e *RuntimeError) Error() string {
	// TP-ANNAHME: Die Fehlernummer erscheint ohne führende Nullen ("Runtime error 2").
	s := fmt.Sprintf("Runtime error %d", e.Code)
	if e.File != "" {
		s += " at " + e.File
		if e.Line > 0 {
			s += fmt.Sprintf("(%d)", e.Line)
		}
	}
	if e.Proc != "" {
		s += " in " + e.Proc
	}
	return s
}

// Run führt ein mit compiler.TargetWASI erzeugtes Modul aus und liefert den
// Exit-Code (Halt). Ein Abbruch von ctx beendet das Programm; Run liefert dann
// ctx.Err().
func Run(ctx context.Context, wasm []byte, opts Options) (exitCode int, err error) {
	defer func() {
		if r := recover(); r != nil {
			exitCode, err = 0, fmt.Errorf("runner: interner Fehler: %v", r)
		}
	}()

	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer rt.Close(context.WithoutCancel(ctx))

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		return 0, fmt.Errorf("runner: WASI: %w", err)
	}
	if err := instantiateDevHost(ctx, rt, opts.Stdout); err != nil {
		return 0, fmt.Errorf("runner: %w", err)
	}

	args := opts.Args
	if len(args) == 0 {
		args = []string{"program"}
	}
	fsCfg := wazero.NewFSConfig()
	for guest, host := range opts.Dirs {
		fsCfg = fsCfg.WithDirMount(host, guest)
	}
	cfg := wazero.NewModuleConfig().
		WithArgs(args...).
		WithFSConfig(fsCfg).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader)
	if opts.Stdin != nil {
		cfg = cfg.WithStdin(opts.Stdin)
	}
	if opts.Stdout != nil {
		cfg = cfg.WithStdout(opts.Stdout)
	}
	if opts.Stderr != nil {
		cfg = cfg.WithStderr(opts.Stderr)
	}

	mod, err := rt.InstantiateWithConfig(ctx, wasm, cfg)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		var exitErr *sys.ExitError
		if errors.As(err, &exitErr) {
			return int(exitErr.ExitCode()), nil
		}
		return 0, fmt.Errorf("runner: %w", err)
	}
	_ = mod.Close(ctx)
	return 0, nil
}
