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

// Die Umgebung des Programms; die Unit Dos liest sie mit GetEnv und nimmt
// die Ortszeit aus WASMPASCAL_UTC_OFFSET.
func ExampleRun_env() {
	src := fstest.MapFS{"gruss.pas": {Data: []byte(`program Gruss;
uses Dos;
begin
  WriteLn('Hallo, ', GetEnv('NAME'), ' (', EnvCount, ' Variablen)')
end.
`)}}
	res, err := compiler.Compile(context.Background(), "gruss.pas", compiler.Options{FS: src})
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err := runner.Run(context.Background(), res.Wasm, runner.Options{
		Stdout: os.Stdout,
		Env:    []string{"NAME=Ada", "WASMPASCAL_UTC_OFFSET=60"},
	}); err != nil {
		fmt.Println(err)
	}
	// Output: Hallo, Ada (2 Variablen)
}

// Ein Programm ruft Go-Funktionen als external-Routinen auf: Werte direkt,
// Strings über die Adresse und Memory.
func ExampleRun_imports() {
	src := fstest.MapFS{"gruss.pas": {Data: []byte(`program Gruss;
function Quadrat(X: Int64): Int64; external 'host' name 'square';
procedure Log(const S: string); external 'host' name 'log';
begin
  WriteLn(Quadrat(3000000000));
  Log('Hallo vom Programm')
end.
`)}}
	res, err := compiler.Compile(context.Background(), "gruss.pas", compiler.Options{FS: src})
	if err != nil {
		fmt.Println(err)
		return
	}
	_, err = runner.Run(context.Background(), res.Wasm, runner.Options{
		Stdout: os.Stdout,
		Imports: map[string]map[string]any{"host": {
			"square": func(x int64) int64 { return x * x },
			"log": func(m runner.Memory, addr uint32) {
				s, _ := m.ReadShortString(addr)
				fmt.Println("log:", s)
			},
		}},
	})
	if err != nil {
		fmt.Println(err)
	}
	// Output:
	// 9000000000000000000
	// log: Hallo vom Programm
}
