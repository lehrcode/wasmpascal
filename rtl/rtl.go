// Package rtl enthält die Laufzeitbibliothek von WasmPascal als Pascal-Quelltext
// (system.pas, später crt.pas und dos.pas). Der Compiler übersetzt sie wie
// normale Units mit.
package rtl

import "embed"

// FS enthält die Pascal-Quelltexte der Laufzeit-Units, z. B. "system.pas".
//
//go:embed *.pas
var FS embed.FS
