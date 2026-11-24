package runner_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing/fstest"

	"github.com/lehrcode/wasmpascal/compiler"
	"github.com/lehrcode/wasmpascal/runner"
)

// Übersetzt die Fakultät mit compiler.Compile und führt sie mit runner.Run aus.
func ExampleRun() {
	src := fstest.MapFS{"fak.pas": {Data: []byte(`program Fak;
var
  i: Integer;

function Fakultaet(n: Integer): LongInt;
begin
  if n <= 1 then Fakultaet := 1 else Fakultaet := n * Fakultaet(n - 1)
end;

begin
  for i := 1 to 5 do
    WriteLn(i, '! = ', Fakultaet(i));
  Halt(2)
end.
`)}}
	res, err := compiler.Compile(context.Background(), "fak.pas", compiler.Options{FS: src})
	if err != nil {
		fmt.Println(err)
		return
	}
	code, err := runner.Run(context.Background(), res.Wasm, runner.Options{
		Args:   []string{"fak"},
		Stdout: os.Stdout,
	})
	var rte *runner.RuntimeError
	if errors.As(err, &rte) {
		fmt.Println(rte)
		return
	}
	fmt.Println("Exit-Code:", code)
	// Output:
	// 1! = 1
	// 2! = 2
	// 3! = 6
	// 4! = 24
	// 5! = 120
	// Exit-Code: 2
}

func ExampleRuntimeError() {
	err := &runner.RuntimeError{Code: 200, File: "zahlen.pas", Line: 42, Proc: "Auswerten"}
	fmt.Println(err)
	// Output: Runtime error 200 at zahlen.pas(42) in Auswerten
}
