// Command pascal übersetzt Turbo-Pascal-Programme nach WebAssembly und führt sie aus.
//
//	pascal build [Optionen] datei.pas        übersetzt nach datei.wasm
//	pascal run   [Optionen] datei.pas [arg…] übersetzt und führt in wazero aus
//	pascal check [Optionen] datei.pas        prüft nur
//	pascal serve [Optionen]                  Compiler-Server (JSON-Zeilen auf stdin/stdout)
//
// Die CLI nutzt die öffentliche API der Pakete compiler und runner, für serve
// zusätzlich internal/compilereq.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lehrcode/wasmpascal/compiler"
	"github.com/lehrcode/wasmpascal/runner"
)

func main() {
	// Strg+C: Farben zurücksetzen (ESC[0m), damit keine Farbe im Terminal
	// hängen bleibt, und beenden. Das Terminal bleibt im Zeilenmodus; ein
	// Programm, das gerade auf eine Eingabezeile wartet, wird so sofort beendet.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	isServe := len(os.Args) > 1 && os.Args[1] == "serve"
	go func() {
		<-sig
		if !isServe { // serve schreibt nur JSON-Zeilen nach stdout
			fmt.Fprint(os.Stdout, resetColors)
		}
		os.Exit(130)
	}()
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// resetColors setzt die ANSI-Attribute zurück und beginnt eine neue Zeile.
const resetColors = "\x1b[0m\n"

const usage = `Aufruf:
  pascal build [Optionen] datei.pas          übersetzt nach WebAssembly
  pascal run   [Optionen] datei.pas [arg...] übersetzt und führt aus (Ziel wasi)
  pascal check [Optionen] datei.pas          prüft nur, erzeugt nichts
  pascal serve [Optionen]                    Compiler-Server: JSON-Zeilen auf stdin/stdout

Optionen mit "pascal <befehl> -h".
`

// run ist die eigentliche CLI; sie liefert den Exit-Code.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "serve":
		return serve(ctx, args, stdin, stdout, stderr)
	case "build", "run", "check":
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "pascal: unbekannter Befehl %q\n\n%s", cmd, usage)
		return 2
	}

	fl := flag.NewFlagSet("pascal "+cmd, flag.ContinueOnError)
	fl.SetOutput(stderr)
	var (
		includes, defines stringList
		target            = "wasi"
		out               string
		stack             int
		noLines           bool
	)
	fl.Var(&includes, "I", "Suchpfad für Units und {$I} (mehrfach möglich)")
	fl.Var(&defines, "D", "Symbol wie {$DEFINE} setzen (mehrfach möglich)")
	if cmd == "build" {
		fl.StringVar(&target, "target", target, "Ziel-ABI: wasi oder gojs")
		fl.StringVar(&out, "o", "", "Ausgabedatei (Standard: datei.wasm)")
	}
	if cmd != "check" {
		fl.IntVar(&stack, "stack", 0, "Größe des Schatten-Stacks in Byte (0 = {$M} oder 1 MB)")
		fl.BoolVar(&noLines, "nolines", false, "ohne Zeilentabelle und Name Section")
	}
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fl.NArg() == 0 || (cmd != "run" && fl.NArg() > 1) {
		fmt.Fprintf(stderr, "pascal %s: genau eine Quelldatei erwartet\n", cmd)
		return 2
	}
	mainFile := fl.Arg(0)

	opts := compiler.Options{Defines: defines, StackSize: stack, NoLines: noLines}
	switch target {
	case "wasi":
		opts.Target = compiler.TargetWASI
	case "gojs":
		opts.Target = compiler.TargetGoJS
	default:
		fmt.Fprintf(stderr, "pascal: unbekanntes Ziel %q (wasi oder gojs)\n", target)
		return 2
	}
	root, fsMain, fsIncludes, err := hostFS(mainFile, includes)
	if err != nil {
		fmt.Fprintln(stderr, "pascal:", err)
		return 1
	}
	opts.FS, opts.IncludePaths = os.DirFS(root), fsIncludes

	if cmd == "check" {
		if err := compiler.Check(ctx, fsMain, opts); err != nil {
			printCompileError(stderr, err, root)
			return 1
		}
		return 0
	}

	res, err := compiler.Compile(ctx, fsMain, opts)
	if err != nil {
		printCompileError(stderr, err, root)
		return 1
	}

	if cmd == "build" {
		if out == "" {
			out = strings.TrimSuffix(mainFile, filepath.Ext(mainFile)) + ".wasm"
		}
		if err := os.WriteFile(out, res.Wasm, 0o644); err != nil {
			fmt.Fprintln(stderr, "pascal:", err)
			return 1
		}
		return 0
	}

	// run
	progArgs := append([]string{strings.TrimSuffix(filepath.Base(mainFile), filepath.Ext(mainFile))}, fl.Args()[1:]...)
	code, err := runner.Run(ctx, res.Wasm, runner.Options{
		Args:   progArgs,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Dirs:   map[string]string{"/": "."},
		Env:    programEnv(os.Environ(), time.Now()),
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// programEnv ist die Umgebung für "pascal run": die des Hosts, wie bei einem
// nativen Programm, dazu WASMPASCAL_UTC_OFFSET (Minuten östlich von UTC) für
// die Ortszeit der Unit Dos, wenn die Variable nicht schon gesetzt ist.
func programEnv(host []string, now time.Time) []string {
	for _, kv := range host {
		if strings.HasPrefix(kv, "WASMPASCAL_UTC_OFFSET=") {
			return host
		}
	}
	_, offset := now.Zone()
	return append(append([]string{}, host...), "WASMPASCAL_UTC_OFFSET="+strconv.Itoa(offset/60))
}

// hostFS bildet Hostpfade auf ein fs.FS ab. Weil Units und Includes auch über
// "../" erreichbar sein können, ist die Wurzel das Laufwerk bzw. "/" und alle
// Pfade werden absolut, aber ohne führenden Trenner übergeben.
func hostFS(mainFile string, includes []string) (root, fsMain string, fsIncludes []string, err error) {
	abs, err := filepath.Abs(mainFile)
	if err != nil {
		return "", "", nil, err
	}
	root = filepath.VolumeName(abs) + string(filepath.Separator)
	rel := func(p string) (string, error) {
		a, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		r, err := filepath.Rel(root, a)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(r), nil
	}
	if fsMain, err = rel(mainFile); err != nil {
		return "", "", nil, err
	}
	for _, inc := range includes {
		r, err := rel(inc)
		if err != nil {
			return "", "", nil, err
		}
		fsIncludes = append(fsIncludes, r)
	}
	return root, fsMain, fsIncludes, nil
}

// printCompileError gibt einen Compilerfehler aus. Dateinamen in Diagnosen
// werden aus der fs.FS-Form wieder zu Pfaden relativ zum Arbeitsverzeichnis.
func printCompileError(w io.Writer, err error, root string) {
	var d *compiler.Diagnostic
	if errors.As(err, &d) {
		d.File = hostPath(root, d.File)
	}
	fmt.Fprintln(w, err)
}

func hostPath(root, fsPath string) string {
	abs := filepath.Join(root, filepath.FromSlash(fsPath))
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return abs
}

// stringList ist ein wiederholbarer Kommandozeilenparameter.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }
