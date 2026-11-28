// Package runner führt mit compiler.TargetWASI erzeugte Module in wazero aus.
package runner

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
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
	// Env ist die Umgebung des Programms als "NAME=WERT" (Unit Dos: GetEnv,
	// EnvCount, EnvStr); nil bedeutet leere Umgebung. Einträge ohne Namen
	// (leer vor dem ersten "=", z. B. die versteckten cmd.exe-Variablen
	// "=C:") werden übersprungen, weil WASI solche Namen nicht darstellen
	// kann. Die Ortszeit der Unit Dos kommt aus WASMPASCAL_UTC_OFFSET
	// (Minuten östlich von UTC); fehlt die Variable, gilt UTC.
	Env []string

	// walltime ersetzt die Uhr (nur für Tests im Paket).
	walltime sys.Walltime
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
// Exit-Code (Halt). Ein Laufzeitfehler kommt als *RuntimeError zurück; der
// Exit-Code ist dann die Fehlernummer, wie in TP. Nach einem Laufzeitfehler
// laufen wie in TP die Exit-Prozeduren des Programms (ExitProc); sie dürfen
// die Meldung unterdrücken (dann ist err nil) und den Exit-Code ändern. Ein
// Abbruch von ctx beendet das Programm; Run liefert dann ctx.Err().
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
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader)
	if opts.walltime != nil {
		cfg = cfg.WithWalltime(opts.walltime, sys.ClockResolution(1))
	} else {
		cfg = cfg.WithSysWalltime()
	}
	for _, kv := range opts.Env {
		k, v, _ := strings.Cut(kv, "=")
		if k == "" {
			// Leerer Name, z. B. die versteckten cmd.exe-Variablen "=C:"
			// (devblogs.microsoft.com/oldnewthing/20100506-00/?p=14133/):
			// WASI stellt die als "NAME=WERT" dar und kann Namen mit "="
			// nicht von Werten unterscheiden; wazero lehnt sie ab.
			continue
		}
		cfg = cfg.WithEnv(k, v)
	}
	if opts.Stdin != nil {
		cfg = cfg.WithStdin(opts.Stdin)
	}
	if opts.Stdout != nil {
		cfg = cfg.WithStdout(opts.Stdout)
	}
	if opts.Stderr != nil {
		cfg = cfg.WithStderr(opts.Stderr)
	}

	// _start selbst aufrufen: Nach einem Trap bleibt das Modul so lesbar
	// (Laufzeitfehler-Globalen).
	mod, err := rt.InstantiateWithConfig(ctx, wasm, cfg.WithStartFunctions())
	if err != nil {
		return 0, fmt.Errorf("runner: %w", err)
	}
	defer mod.Close(context.WithoutCancel(ctx))
	start := mod.ExportedFunction("_start")
	if start == nil {
		return 0, errors.New("runner: Modul exportiert kein _start")
	}
	_, err = start.Call(ctx)
	if err == nil {
		return 0, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, ctxErr
	}
	var exitErr *sys.ExitError
	if errors.As(err, &exitErr) {
		return int(exitErr.ExitCode()), nil
	}
	rte := runtimeError(mod, wasm, err)
	if rte == nil {
		return 0, fmt.Errorf("runner: %w", err)
	}
	return errorExit(ctx, mod, wasm, rte)
}

// maxErrorExits begrenzt die Laufzeitfehler in Exit-Prozeduren. Jede läuft
// höchstens einmal (ExitProc wird vor dem Aufruf nil); die Grenze schützt nur
// vor einem Fehler in der Laufzeit selbst.
const maxErrorExits = 1000

// errorExit beendet das Programm nach einem Laufzeitfehler wie TP: Der Export
// _exit(code, line) führt die Exit-Prozeduren aus und beendet das Programm
// über proc_exit mit ExitCode * 2 + 1, wenn die Meldung erscheinen soll
// (ErrorAddr <> nil), sonst ExitCode * 2. Ein weiterer Laufzeitfehler in
// einer Exit-Prozedur ersetzt den ersten; die Kette läuft weiter. Module
// ohne _exit enden sofort mit dem Fehler.
func errorExit(ctx context.Context, mod api.Module, wasm []byte, rte *RuntimeError) (int, error) {
	exit := mod.ExportedFunction("_exit")
	if exit == nil {
		return rte.Code, rte
	}
	for i := 0; i < maxErrorExits; i++ {
		line := uint64(0)
		if g := mod.ExportedGlobal("__rt_line"); g != nil {
			line = g.Get()
		}
		_, err := exit.Call(ctx, uint64(uint32(rte.Code)), line)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, ctxErr
		}
		var exitErr *sys.ExitError
		if errors.As(err, &exitErr) {
			packed := int32(exitErr.ExitCode())
			code := int(packed >> 1)
			if packed&1 == 0 {
				return code, nil
			}
			rte.Code = code
			return code, rte
		}
		if err == nil {
			break
		}
		next := runtimeError(mod, wasm, err)
		if next == nil {
			return 0, fmt.Errorf("runner: %w", err)
		}
		rte = next
	}
	return rte.Code, rte
}
