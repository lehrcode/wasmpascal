package runner_test

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/lehrcode/wasmpascal/internal/wasmtest"
	"github.com/lehrcode/wasmpascal/runner"
)

func ExampleRun() {
	// Später kommt wasm aus compiler.Compile; hier ein handkodiertes Modul.
	wasm := wasmtest.Print(1, "Hallo, Welt!\n", 0)

	code, err := runner.Run(context.Background(), wasm, runner.Options{
		Args:   []string{"hallo"},
		Stdout: os.Stdout,
	})
	var rte *runner.RuntimeError
	if errors.As(err, &rte) {
		fmt.Println(rte)
		return
	}
	fmt.Println("Exit-Code:", code)
	// Output:
	// Hallo, Welt!
	// Exit-Code: 0
}

func ExampleRuntimeError() {
	err := &runner.RuntimeError{Code: 200, File: "zahlen.pas", Line: 42, Proc: "Auswerten"}
	fmt.Println(err)
	// Output: Runtime error 200 at zahlen.pas(42) in Auswerten
}
