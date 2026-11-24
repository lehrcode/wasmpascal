//go:build js && wasm

// Command pascaljs ist der Compiler für den Browser. Mit GOOS=js GOARCH=wasm
// gebaut, läuft er mit wasm_exec.js und legt die globale JavaScript-Funktion
// pascalCompile an:
//
//	const r = pascalCompile(sources, options);
//
// sources ist ein Objekt Dateiname → Quelltext (Pfade mit "/", relativ, z. B.
// {"zahlen.pas": "...", "lib/werkzeug.pas": "..."}) oder ein einzelner String,
// der dann "programm.pas" heißt. Ein Quelltext ist ein String oder ein
// Uint8Array mit den Bytes der Datei (dann meldet der Compiler Dateien, die
// nicht UTF-8 sind, mit Error 900). options ist optional:
//
//	main     Hauptprogramm; ohne Angabe die einzige Datei bzw. "programm.pas"
//	defines  Array von Symbolen wie bei -D
//	stack    Größe des Schatten-Stacks in Byte (0 = {$M} oder 1 MB)
//
// Ergebnis ist entweder {wasm: Uint8Array} (Ziel gojs, ausführbar mit
// web/loader.js) oder {error, file, line, column, code} mit der Meldung im
// Format "datei.pas(zeile,spalte): Error NN: Text". Units werden im
// Verzeichnis des Hauptprogramms gesucht, Crt und Dos sind eingebaut.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"syscall/js"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
)

func main() {
	js.Global().Set("pascalCompile", js.FuncOf(compileJS))
	select {} // auf Aufrufe aus JavaScript warten
}

// compileJS ist pascalCompile(sources, options).
func compileJS(_ js.Value, args []js.Value) (result any) {
	defer func() {
		if r := recover(); r != nil {
			result = errorResult(fmt.Sprintf("pascalCompile: %v", r), nil)
		}
	}()
	if len(args) == 0 {
		return errorResult("pascalCompile: Quelltexte fehlen", nil)
	}
	files, err := sourcesFrom(args[0])
	if err != nil {
		return errorResult(err.Error(), nil)
	}
	opts := compiler.Options{Target: compiler.TargetGoJS, FS: files}
	main := ""
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		o := args[1]
		if v := o.Get("main"); v.Type() == js.TypeString {
			main = v.String()
		}
		if v := o.Get("defines"); v.Type() == js.TypeObject {
			for i := 0; i < v.Length(); i++ {
				opts.Defines = append(opts.Defines, v.Index(i).String())
			}
		}
		if v := o.Get("stack"); v.Type() == js.TypeNumber {
			opts.StackSize = v.Int()
		}
	}
	if main == "" {
		if main, err = defaultMain(files); err != nil {
			return errorResult(err.Error(), nil)
		}
	}
	res, err := compiler.Compile(context.Background(), main, opts)
	if err != nil {
		var d *compiler.Diagnostic
		if errors.As(err, &d) {
			return errorResult(d.Error(), d)
		}
		return errorResult(err.Error(), nil)
	}
	wasm := js.Global().Get("Uint8Array").New(len(res.Wasm))
	js.CopyBytesToJS(wasm, res.Wasm)
	return map[string]any{"wasm": wasm}
}

// sourcesFrom liest die Quelltexte aus einem Objekt oder einem String.
func sourcesFrom(v js.Value) (fstest.MapFS, error) {
	files := fstest.MapFS{}
	switch v.Type() {
	case js.TypeString:
		files["programm.pas"] = &fstest.MapFile{Data: []byte(v.String())}
	case js.TypeObject:
		keys := js.Global().Get("Object").Call("keys", v)
		for i := 0; i < keys.Length(); i++ {
			name := keys.Index(i).String()
			if !fs.ValidPath(name) || name == "." {
				return nil, fmt.Errorf("pascalCompile: ungültiger Dateiname %q", name)
			}
			data, err := bytesFrom(v.Get(name))
			if err != nil {
				return nil, fmt.Errorf("pascalCompile: %q: %v", name, err)
			}
			files[name] = &fstest.MapFile{Data: data}
		}
	default:
		return nil, errors.New("pascalCompile: Quelltexte als Objekt oder String erwartet")
	}
	if len(files) == 0 {
		return nil, errors.New("pascalCompile: keine Quelltexte")
	}
	return files, nil
}

// bytesFrom liefert den Inhalt eines Strings oder Uint8Array.
func bytesFrom(v js.Value) ([]byte, error) {
	if v.Type() == js.TypeString {
		return []byte(v.String()), nil
	}
	if v.Type() == js.TypeObject && v.InstanceOf(js.Global().Get("Uint8Array")) {
		b := make([]byte, v.Length())
		js.CopyBytesToGo(b, v)
		return b, nil
	}
	return nil, errors.New("String oder Uint8Array erwartet")
}

// defaultMain wählt das Hauptprogramm, wenn options.main fehlt.
func defaultMain(files fstest.MapFS) (string, error) {
	if _, ok := files["programm.pas"]; ok {
		return "programm.pas", nil
	}
	if len(files) == 1 {
		for name := range files {
			return name, nil
		}
	}
	return "", errors.New("pascalCompile: mehrere Dateien, options.main fehlt")
}

// errorResult liefert {error, file, line, column, code}.
func errorResult(msg string, d *compiler.Diagnostic) map[string]any {
	r := map[string]any{"error": msg}
	if d != nil {
		r["file"] = d.File
		r["line"] = d.Line
		r["column"] = d.Col
		r["code"] = d.Code
	}
	return r
}
