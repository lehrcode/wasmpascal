package compiler_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
	"github.com/lehrcode/wasmpascal/runner"
)

func ExampleCompile() {
	src := fstest.MapFS{
		"hallo.pas": {Data: []byte("program Hallo;\nbegin\n  WriteLn('Hallo, Welt!')\nend.\n")},
	}
	res, err := compiler.Compile(context.Background(), "hallo.pas", compiler.Options{
		Target: compiler.TargetWASI,
		FS:     src,
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err := runner.Run(context.Background(), res.Wasm, runner.Options{Stdout: os.Stdout}); err != nil {
		fmt.Println(err)
	}
	// Output: Hallo, Welt!
}

func ExampleCheck() {
	src := fstest.MapFS{}
	err := compiler.Check(context.Background(), "fehlt.pas", compiler.Options{FS: src})
	var d *compiler.Diagnostic
	if errors.As(err, &d) {
		fmt.Println(d.Code, d.Msg)
	}
	// Output: 15 File not found
}

func ExampleDiagnostic() {
	d := &compiler.Diagnostic{File: "zahlen.pas", Line: 12, Col: 5, Code: 3, Msg: "Unknown identifier"}
	fmt.Println(d)
	// Output: zahlen.pas(12,5): Error 3: Unknown identifier
}

// Engere Grenzen für Quelltexte aus fremder Hand, etwa in einem Server.
func ExampleLimits() {
	src := fstest.MapFS{"tief.pas": {Data: []byte("program Tief; var x: Integer; begin x := ((((1)))) end.")}}
	_, err := compiler.Compile(context.Background(), "tief.pas", compiler.Options{
		FS:     src,
		Limits: compiler.Limits{MaxDepth: 4, MaxSourceBytes: 1 << 20, MaxStaticData: 1 << 20},
	})
	fmt.Println(err)
	// Output: tief.pas(1,43): Error 904: Nesting too deep
}
