package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	prog := filepath.Join(dir, "p.pas")
	if err := os.WriteFile(prog, []byte("program P; begin end.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "fehlt.pas")

	tests := []struct {
		args     []string
		wantCode int
		wantErr  string
	}{
		{nil, 2, "Aufruf:"},
		{[]string{"tanzen"}, 2, "unbekannter Befehl"},
		{[]string{"build"}, 2, "genau eine Quelldatei"},
		{[]string{"build", "-target", "dos", prog}, 2, "unbekanntes Ziel"},
		{[]string{"check", missing}, 1, missing + ": Error 15: File not found"},
		{[]string{"build", missing}, 1, "Error 15"},
		{[]string{"run", missing, "a", "b"}, 1, "Error 15"},
		{[]string{"check", prog}, 0, ""},
		{[]string{"build", "-target", "gojs", "-o", filepath.Join(dir, "p.wasm"), prog}, 0, ""},
		{[]string{"run", "-target", "gojs", prog}, 2, "-target"},
		{[]string{"run", "-I", dir, "-D", "X", "-stack", "65536", "-nolines", prog}, 0, ""},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), tt.args, strings.NewReader(""), &stdout, &stderr)
		if code != tt.wantCode || !strings.Contains(stderr.String(), tt.wantErr) {
			t.Errorf("pascal %v = %d, stderr %q; want %d, %q", tt.args, code, stderr.String(), tt.wantCode, tt.wantErr)
		}
	}
}

func TestHostFS(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "p.pas"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(dir, "src", "lib"))

	root, main, incs, err := hostFS("../p.pas", []string{".", ".."})
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(os.DirFS(root), main)
	if err != nil || string(data) != "x" {
		t.Fatalf("ReadFile(%q) = %q, %v", main, data, err)
	}
	if got := hostPath(root, main); got != filepath.Join("..", "p.pas") && got != filepath.Join(dir, "src", "p.pas") {
		t.Errorf("hostPath = %q", got)
	}
	if len(incs) != 2 || !fs.ValidPath(incs[0]) || !strings.HasSuffix(incs[0], "src/lib") || !strings.HasSuffix(incs[1], "src") {
		t.Errorf("IncludePaths = %q", incs)
	}
}

func TestCLIRunAndBuild(t *testing.T) {
	dir := t.TempDir()
	prog := filepath.Join(dir, "gruss.pas")
	src := "program Gruss;\nbegin\n  WriteLn('Hallo');\n  Halt(4)\nend.\n"
	if err := os.WriteFile(prog, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", prog}, strings.NewReader(""), &stdout, &stderr)
	if code != 4 || stdout.String() != "Hallo\n" || stderr.Len() != 0 {
		t.Errorf("run = %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	code = run(context.Background(), []string{"build", prog}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("build = %d: %s", code, stderr.String())
	}
	wasm, err := os.ReadFile(filepath.Join(dir, "gruss.wasm"))
	if err != nil || len(wasm) < 8 || string(wasm[:4]) != "\x00asm" {
		t.Errorf("gruss.wasm: %v, %d Bytes", err, len(wasm))
	}
}
