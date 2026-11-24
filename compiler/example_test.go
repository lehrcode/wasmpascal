package compiler_test

import (
	"context"
	"errors"
	"fmt"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
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
	fmt.Println(len(res.Wasm) > 0)
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
