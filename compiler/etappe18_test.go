package compiler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

// etappe18: Nur erreichbare Routinen landen im Modul (PLAN.md, Etappe 18).

// modSections liest Import- und Funktionsnamen-Section eines WASM-Moduls.
func modSections(t *testing.T, wasm []byte) (imports []string, funcNames []string) {
	t.Helper()
	if len(wasm) < 8 || string(wasm[:4]) != "\x00asm" {
		t.Fatal("kein WASM-Modul")
	}
	i := 8
	leb := func() uint32 {
		var r uint32
		var s uint
		for {
			x := wasm[i]
			i++
			r |= uint32(x&0x7f) << s
			if x&0x80 == 0 {
				return r
			}
			s += 7
		}
	}
	name := func() string {
		n := leb()
		s := string(wasm[i : i+int(n)])
		i += int(n)
		return s
	}
	for i < len(wasm) {
		id := wasm[i]
		i++
		end := i + int(leb())
		switch id {
		case 2: // Import
			for n := leb(); n > 0; n-- {
				mod, nm := name(), name()
				kind := wasm[i]
				i++
				if kind == 0 {
					leb() // type index
					imports = append(imports, mod+"."+nm)
					continue
				}
				// table/memory/global überspringen
				switch kind {
				case 1:
					i++ // elem type
					fallthrough
				case 2:
					flags := wasm[i]
					i++
					leb()
					if flags&1 != 0 {
						leb()
					}
				case 3:
					i += 2
				}
			}
		case 0: // custom
			if name() == "name" {
				for i < end {
					sub := wasm[i]
					i++
					send := i + int(leb())
					if sub == 1 { // function names
						for n := leb(); n > 0; n-- {
							leb() // func index
							funcNames = append(funcNames, name())
						}
					}
					i = send
				}
			}
		}
		i = end
	}
	return imports, funcNames
}

func compilePascal(t *testing.T, src string) []byte {
	t.Helper()
	res, err := Compile(context.Background(), "p.pas", Options{
		FS: fstest.MapFS{"p.pas": {Data: []byte(src)}},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return res.Wasm
}

const halloSrc = "program P; begin WriteLn('Hallo!') end."

// Kleine Programme ergeben deutlich kleinere Module (Obergrenzen vom Stand
// der Umsetzung: 4.545 Byte bzw. 10.100 Byte auf windows/amd64 gemessen).
func TestPruneModuleSize(t *testing.T) {
	if n := len(compilePascal(t, halloSrc)); n >= 8<<10 {
		t.Errorf("Hallo-Welt: %d Byte, want < 8 KiB", n)
	}
	gross := "program P; uses Crt, Dos; var S: SearchRec; begin TextColor(Yellow); WriteLn('Hallo!') end."
	if n := len(compilePascal(t, gross)); n >= 16<<10 {
		t.Errorf("mit Crt, Dos: %d Byte, want < 16 KiB", n)
	}
}

// Ein Programm ohne Dateizugriff importiert keine path_*- und fd_read-Aufrufe.
func TestPruneImports(t *testing.T) {
	imports, _ := modSections(t, compilePascal(t, halloSrc))
	for _, im := range imports {
		if strings.HasPrefix(im, "wasi_snapshot_preview1.path_") ||
			im == "wasi_snapshot_preview1.fd_read" || im == "wasi_snapshot_preview1.fd_seek" ||
			im == "wasi_snapshot_preview1.fd_readdir" || im == "wasi_snapshot_preview1.fd_close" {
			t.Errorf("Hallo-Welt importiert %s trotz fehlendem Dateizugriff", im)
		}
	}
}

// Ungenutzte Routinen fehlen in der Name Section, über @ oder eine typisierte
// Konstante adressierte sind darin (sie sind Wurzeln der Erreichbarkeit).
func TestPruneNameSection(t *testing.T) {
	_, names := modSections(t, compilePascal(t, halloSrc))
	has := func(n string) bool {
		for _, f := range names {
			if f == n {
				return true
			}
		}
		return false
	}
	if !has("main") {
		t.Error("Name Section enthält main nicht")
	}
	for _, dead := range []string{"System.UTF8Copy", "System._MemAvail", "System._StrInsert", "System._BlockRead", "System._ChDir"} {
		if has(dead) {
			t.Errorf("Name Section enthält ungenutzte Routine %s", dead)
		}
	}
	// Routine nur über typisierte Konstante und @ erreichbar
	src := `program P;
type TBinOp = function(A, B: Integer): Integer;
function Plus(A, B: Integer): Integer;
begin
  Plus := A + B
end;
const KOps: array[1..1] of TBinOp = (Plus);
var F: TBinOp;
begin
  F := KOps[1];
  WriteLn(F(3, 4));
  F := TBinOp(@Plus);
  WriteLn(F(5, 6))
end.`
	_, names = modSections(t, compilePascal(t, src))
	if !has("Plus") {
		t.Errorf("Name Section enthält adressierte Routine Plus nicht: %v", names)
	}
}

// Die Typprüfung bleibt vollständig: Ein Fehler in einer ungenutzten Routine
// wird weiterhin gemeldet.
func TestPruneDeadCodeStillChecked(t *testing.T) {
	src := `program P;
procedure Tot(x: Integer);
begin
  x := 'kein Integer'
end;
begin
  WriteLn('ok')
end.`
	_, err := Compile(context.Background(), "p.pas", Options{
		FS: fstest.MapFS{"p.pas": {Data: []byte(src)}},
	})
	var d *Diagnostic
	if !errors.As(err, &d) {
		t.Fatalf("Compile ohne Dateizugriff: err = %v, want *Diagnostic", err)
	}
	if d.Code != 26 {
		t.Errorf("Code = %d, want 26 (Type mismatch in toter Routine)", d.Code)
	}
}
