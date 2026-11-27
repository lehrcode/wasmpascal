// Package wasmpascal ist die Wurzel von WasmPascal, einem Compiler für
// Turbo Pascal 5.0 nach WebAssembly.
//
// Die öffentliche API liegt in den Paketen compiler und runner, die CLI in
// cmd/pascal. Das Wurzelpaket selbst enthält nur den Test-Harness, der alle
// Programme unter testdata/ übersetzt, ausführt und mit den erwarteten
// Ausgaben vergleicht.
package wasmpascal
