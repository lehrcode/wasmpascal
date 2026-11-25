// Package compilereq setzt das JSON-Protokoll des Compiler-Servers um
// (pascal serve, PLAN.md Etappe 13): eine Anfrage mit Quelltexten lesen und
// prüfen, mit compiler.Compile übersetzen und die Antwort bauen.
//
// Quelltexte kommen ausschließlich aus der Anfrage (Feld files, als
// fstest.MapFS); Hostdateien sind nicht erreichbar. Das Paket hat keinen
// globalen Zustand und darf aus mehreren Goroutinen benutzt werden.
package compilereq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
)

// Request ist eine Anfrage, wie sie pascal serve je Zeile liest.
type Request struct {
	// ID ist beliebiges JSON; die Antwort enthält es unverändert.
	ID json.RawMessage `json:"id,omitempty"`
	// Files bildet Dateinamen (relativ, mit "/") auf Quelltexte ab.
	Files map[string]string `json:"files"`
	// Main ist das Hauptprogramm; ohne Angabe die einzige Datei.
	Main string `json:"main,omitempty"`
	// Target ist "wasi" (Standard) oder "gojs".
	Target string `json:"target,omitempty"`
	// Defines sind Symbole wie bei -D.
	Defines []string `json:"defines,omitempty"`
	// Stack ist die Größe des Schatten-Stacks in Byte (0 = {$M} oder 1 MB).
	Stack int `json:"stack,omitempty"`
	// Limits sind die Grenzen der Übersetzung (0 = Voreinstellung, -1 = aus).
	Limits Limits `json:"limits,omitempty"`
}

// Limits entspricht compiler.Limits, mit Feldnamen in camelCase.
type Limits struct {
	MaxDepth       int   `json:"maxDepth,omitempty"`
	MaxSourceBytes int   `json:"maxSourceBytes,omitempty"`
	MaxIncludes    int   `json:"maxIncludes,omitempty"`
	MaxTokens      int   `json:"maxTokens,omitempty"`
	MaxStaticData  int64 `json:"maxStaticData,omitempty"`
	MaxStackSize   int   `json:"maxStackSize,omitempty"`
}

// Arten von Fehlern in Response.Kind; leer bei Erfolg.
const (
	KindSource   = "source"   // Fehler im Quelltext, mit Datei und Stelle
	KindRequest  = "request"  // kaputte Anfrage
	KindInternal = "internal" // Fehler im Compiler (compiler.ErrInternal)
	KindTimeout  = "timeout"  // Zeitüberschreitung oder Abbruch über ctx
)

// Response ist die Antwort auf eine Anfrage: entweder Wasm oder Kind und
// Error (bei KindSource zusätzlich File, Line, Column und Code).
type Response struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Wasm   []byte          `json:"wasm,omitempty"` // in JSON base64-kodiert
	Kind   string          `json:"kind,omitempty"`
	Error  string          `json:"error,omitempty"`
	File   string          `json:"file,omitempty"`
	Line   int             `json:"line,omitempty"`
	Column int             `json:"column,omitempty"`
	Code   int             `json:"code,omitempty"`
}

// Decode liest eine Anfrage aus einer JSON-Zeile und prüft sie (unbekannte
// Felder, Dateinamen, Hauptprogramm, Ziel). Bei einem Fehler enthält die
// gelieferte Anfrage zumindest die ID, soweit sie sich lesen ließ.
func Decode(line []byte) (Request, error) {
	var req Request
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var idOnly struct {
			ID json.RawMessage `json:"id"`
		}
		_ = json.Unmarshal(line, &idOnly)
		return Request{ID: idOnly.ID}, fmt.Errorf("ungültiges JSON: %v", err)
	}
	if dec.More() {
		return Request{ID: req.ID}, errors.New("ungültiges JSON: mehr als ein Wert in der Zeile")
	}
	if err := validate(&req); err != nil {
		return Request{ID: req.ID}, err
	}
	return req, nil
}

func validate(req *Request) error {
	if len(req.Files) == 0 {
		return errors.New("files fehlt oder ist leer")
	}
	for name := range req.Files {
		if !fs.ValidPath(name) || name == "." {
			return fmt.Errorf("ungültiger Dateiname %q (relativ, mit /, ohne ..)", name)
		}
	}
	if req.Main == "" {
		if len(req.Files) > 1 {
			return errors.New("main fehlt (mehrere Dateien in files)")
		}
		for name := range req.Files {
			req.Main = name
		}
	}
	if _, ok := req.Files[req.Main]; !ok {
		names := make([]string, 0, len(req.Files))
		for name := range req.Files {
			names = append(names, name)
		}
		sort.Strings(names)
		return fmt.Errorf("main %q steht nicht in files %q", req.Main, names)
	}
	switch req.Target {
	case "", "wasi", "gojs":
	default:
		return fmt.Errorf("unbekanntes target %q (wasi oder gojs)", req.Target)
	}
	return nil
}

// RequestError ist die Antwort auf eine kaputte Anfrage.
func RequestError(id json.RawMessage, err error) Response {
	return Response{ID: id, Kind: KindRequest, Error: err.Error()}
}

// Handle übersetzt eine mit Decode gelesene Anfrage. ctx begrenzt die Zeit
// (Antwort KindTimeout); Panics kommen als KindInternal zurück.
func Handle(ctx context.Context, req Request) Response {
	return handle(ctx, req, compiler.Compile)
}

type compileFunc func(context.Context, string, compiler.Options) (*compiler.Result, error)

func handle(ctx context.Context, req Request, compile compileFunc) (resp Response) {
	defer func() {
		if r := recover(); r != nil {
			resp = Response{ID: req.ID, Kind: KindInternal, Error: fmt.Sprintf("%v: %v", compiler.ErrInternal, r)}
		}
	}()
	if err := validate(&req); err != nil {
		return RequestError(req.ID, err)
	}
	files := fstest.MapFS{}
	for name, text := range req.Files {
		files[name] = &fstest.MapFile{Data: []byte(text)}
	}
	opts := compiler.Options{
		FS:        files,
		Defines:   req.Defines,
		StackSize: req.Stack,
		Limits:    compiler.Limits(req.Limits),
	}
	if req.Target == "gojs" {
		opts.Target = compiler.TargetGoJS
	}
	res, err := compile(ctx, req.Main, opts)
	if err == nil {
		return Response{ID: req.ID, Wasm: res.Wasm}
	}
	var d *compiler.Diagnostic
	switch {
	case errors.As(err, &d):
		return Response{ID: req.ID, Kind: KindSource, Error: d.Error(),
			File: d.File, Line: d.Line, Column: d.Col, Code: d.Code}
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return Response{ID: req.ID, Kind: KindTimeout, Error: "Zeitüberschreitung: " + err.Error()}
	case errors.Is(err, compiler.ErrInternal):
		return Response{ID: req.ID, Kind: KindInternal, Error: err.Error()}
	default:
		return RequestError(req.ID, err)
	}
}
