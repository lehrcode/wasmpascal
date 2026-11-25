package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/lehrcode/wasmpascal/internal/compilereq"
)

// serve ist "pascal serve": Compiler-Server im Dauerbetrieb. Je Zeile auf
// stdin eine JSON-Anfrage, je Anfrage genau eine JSON-Zeile auf stdout, in
// derselben Reihenfolge (Protokoll in internal/compilereq und PLAN.md,
// Etappe 13). Der Compiler läuft im Prozess; Schutz geben die Grenzen aus
// compiler.Limits, ein Zeitlimit je Anfrage und optional debug.SetMemoryLimit.
func serve(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("pascal serve", flag.ContinueOnError)
	fl.SetOutput(stderr)
	timeout := fl.Duration("timeout", 10*time.Second, "Zeitlimit je Anfrage")
	memory := fl.Int("memory", 0, "weiches Speicherziel des Prozesses in MB (debug.SetMemoryLimit, 0 = nicht setzen)")
	maxRequest := fl.Int("maxrequest", 64, "größte Anfrage (eine Zeile) in MB")
	fl.Usage = func() {
		fmt.Fprint(stderr, `Aufruf: pascal serve [Optionen]

Liest je Zeile eine JSON-Anfrage von stdin und schreibt je Anfrage eine
JSON-Zeile nach stdout, z. B.
  {"id": 1, "files": {"p.pas": "program P; begin end."}, "target": "gojs"}
  {"id": 1, "wasm": "AGFzbQ…"}
Fehler stehen in der Antwort ("kind": source, request, internal, timeout);
der Server läuft danach weiter und endet am Ende der Eingabe.

`)
		fl.PrintDefaults()
	}
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fl.NArg() > 0 || *timeout <= 0 || *memory < 0 || *maxRequest <= 0 {
		fmt.Fprintln(stderr, "pascal serve: keine Argumente; -timeout und -maxrequest müssen positiv sein")
		return 2
	}
	if *memory > 0 {
		debug.SetMemoryLimit(int64(*memory) << 20)
	}

	in := bufio.NewReaderSize(stdin, 64<<10)
	out := bufio.NewWriter(stdout)
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	limit := *maxRequest << 20
	for {
		line, tooLong, readErr := readLine(in, limit)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			fmt.Fprintln(stderr, "pascal serve:", readErr)
			return 1
		}
		var resp *compilereq.Response
		switch {
		case tooLong:
			r := compilereq.RequestError(nil, fmt.Errorf("Anfrage länger als %d MB", *maxRequest))
			resp = &r
		case len(bytes.TrimSpace(line)) > 0:
			r := handleLine(ctx, line, *timeout)
			resp = &r
		}
		if resp != nil {
			err := enc.Encode(resp)
			if err == nil {
				err = out.Flush()
			}
			if err != nil {
				fmt.Fprintln(stderr, "pascal serve:", err)
				return 1
			}
		}
		if readErr != nil { // io.EOF
			return 0
		}
		if err := ctx.Err(); err != nil {
			return 0
		}
	}
}

// handleLine bearbeitet eine Anfrage mit eigenem Zeitlimit.
func handleLine(ctx context.Context, line []byte, timeout time.Duration) compilereq.Response {
	req, err := compilereq.Decode(line)
	if err != nil {
		return compilereq.RequestError(req.ID, err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return compilereq.Handle(ctx, req)
}

// readLine liest eine Zeile ohne Zeilenende. Ist sie länger als max Byte,
// wird der Rest bis zum Zeilenende überlesen und tooLong gemeldet. Am Ende der
// Eingabe kommt die letzte (auch unvollständige) Zeile mit io.EOF.
func readLine(r *bufio.Reader, max int) (line []byte, tooLong bool, err error) {
	for {
		chunk, err := r.ReadSlice('\n')
		if !tooLong {
			if len(line)+len(chunk) > max+1 { // +1 für das Zeilenende
				tooLong, line = true, nil
			} else {
				line = append(line, chunk...)
			}
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return bytes.TrimRight(line, "\r\n"), tooLong, err
	}
}
