# CLAUDE.md – WasmPascal

Turbo-Pascal-4.0-Compiler (ohne OOP, plus einige TP-7-/Delphi-Erweiterungen) nach WebAssembly, geschrieben in Go.
Modul `github.com/lehrcode/wasmpascal`, CLI `pascal` (`build` | `run` | `check` | `serve`).

**Verbindlich ist [PLAN.md](PLAN.md).** Diese Datei fasst nur die Arbeitsregeln und die Typabbildung zusammen;
bei Widerspruch gilt PLAN.md. Abweichungen von TP 4.0 stehen in `docs/abweichungen.md`.
`docs/units.md` ist die Referenz der Units System, Crt und Dos für Programmierer; bei Änderungen an
Standardroutinen oder `rtl/*.pas` mitpflegen.

## Arbeitsregeln

- **Eine Etappe nach der anderen.** Am Ende jeder Etappe: `go test ./...` grün, `go vet ./...` sauber, `gofmt` angewendet,
  Commit, kurzer Bericht (was geht, was fehlt, offene Fragen) – dann auf Freigabe warten.
- **Tests zuerst.** Jede neue Sprachfunktion bekommt ein Testprogramm in `testdata/` (`*.pas` + `*.out` bzw. `*.err`).
  Golden-Files nur mit `-update` neu schreiben und die Diffs prüfen.
- **Abhängigkeiten:** Paket `compiler` nur Go-Standardbibliothek; wazero nur in `runner` und `cmd/pascal`. Sonst nichts ohne Rückfrage.
- **WASM-Tauglichkeit des Compilers:** kein `os/exec`, kein cgo, Dateizugriff nur über `fs.FS`.
  `GOOS=js GOARCH=wasm` und `GOOS=wasip1 GOARCH=wasm` müssen `./compiler/...` bauen (wird im Testlauf geprüft).
- **Bibliotheksregeln:** kein globaler veränderlicher Zustand, kein `os.Exit`/`log.Fatal`, kein `os.Stdin`/`os.Stdout`
  außerhalb von `cmd/pascal`; Panics abfangen und als Fehler liefern; `ctx` respektieren.
  Jede exportierte API hat Doku-Kommentar und mindestens ein `Example`-Test. `cmd/pascal` nutzt nur die öffentliche API (für `serve` zusätzlich `internal/compilereq`).
- **Fehlermeldungen:** `datei.pas(zeile,spalte): Error NN: Text`, Abbruch beim ersten Fehler.
- **Codegenerator auf den Scheduler auslegen:** IR immer als CFG; jede Variable muss sich in den Frame auslagern lassen.
- **Keine eigenmächtigen Spracherweiterungen.** Im Zweifel TP 4.0 nachbilden, Abweichungen in `docs/abweichungen.md`.
- **Unklares TP-Verhalten:** plausible Annahme treffen, im Code mit `// TP-ANNAHME:` markieren, im Bericht nennen.
- **Laufzeit in Pascal bevorzugen** (`rtl/*.pas`); Intrinsics in Go/WASM nur, wenn es nicht anders geht.
- **PLAN.md und `docs/abweichungen.md` pflegen:** erledigte Etappen abhaken, Entscheidungen festhalten.

## Typabbildung und Speichermodell (Kurzfassung)

| Turbo Pascal | WASM-Wert | Im Speicher |
| --- | --- | --- |
| Byte, ShortInt, Char, Boolean | `i32` | 1 Byte; jede Zuweisung kürzt auf 8 Bit |
| Integer, LongInt | `i32` | 4 Byte, beide −2³¹..2³¹−1 (`MaxInt` = 2147483647) |
| Word | `i32` | 4 Byte, 0..65535, kein 16-Bit-Überlauf |
| Aufzählungen, Teilbereiche | `i32` | 1 Byte, wenn alle Werte in 0..255 bzw. −128..127 passen, sonst 4 Byte |
| Real, Double, Extended, Comp | `f64` | 8 Byte |
| Single | `f32` | 4 Byte |
| Zeiger, `nil` | `i32` | 4 Byte |
| `string[n]`/`string` | Adresse | immer 256 Byte (Längenbyte + 255 Bytes UTF-8), beim Zuweisen auf `n` gekürzt |
| `set of T` | Adresse | immer 32 Byte, Operationen mit `v128` |
| Records, Arrays | Adresse | natürliche Ausrichtung, Varianten als Union |
| Dateivariablen | Adresse | Laufzeit-Record |

- **Linearer Speicher:** Seite ab Adresse 0 bleibt frei; jedes `p^` prüft auf `nil` (Runtime error 216) → Datensegment (typisierte Konstanten, im Modul) → BSS (globale Variablen, nicht im Modul) →
  Schatten-Stack (wächst nach unten, Standard 1 MB, `-stack`/`{$M}`, Überlauf = Runtime error 202) →
  Heap (Free-List, `memory.grow`, erschöpft = Runtime error 203).
- Einzelne Struktur höchstens 2 GB; die 64-KB-Grenze von TP entfällt.
- **Prozeduren:** Skalare ohne Adressbedarf sind WASM-Locals; `var`-Parameter als Adresse; Wertparameter vom Typ
  Record/Array/String/Menge kopiert der Aufgerufene; verschachtelte Prozeduren bekommen einen Static Link als
  ersten Parameter; String-, Record-, Array- und Mengenergebnisse über eine Adresse des Aufrufers.
- **Erweiterungen** (Tabelle in PLAN.md): `Result`, schreibgeschützte typisierte Konstanten, `Break`/`Continue`,
  `Low`/`High`, `const`-Parameter, `for … in`, Funktionsaufrufe als Anweisung (immer), strukturierte
  Funktionsergebnisse wie Delphi 1.0.
- **Adressfunktionen:** `@x`/`Addr` = lineare Adresse, `Ofs` als LongInt, `Seg` = 0, `Ptr(seg,ofs)` ignoriert `seg`,
  `CSeg`/`DSeg`/`SSeg`/`SPtr`/`PrefixSeg`/`FreePtr`/`FreeMin` sind Compilerfehler.
- **Laufzeitfehler:** `Runtime error NNN at datei.pas(zeile) in Prozedur` (nur Dateiname, ohne Verzeichnis); Traps über Name Section und Custom Section
  `wasmpascal.lines`; `-nolines` lässt beides weg.
- **Zeichensatz:** durchgehend UTF-8, `Char` = Byte, `Length` zählt Bytes, Bezeichner nur ASCII.

## Test-Harness

`go test ./...` führt jedes `testdata/**/*.pas` mit einer zugehörigen `*.out` oder `*.err` aus (Details in
`testdata/README.md`). `go test . -update` schreibt die erwarteten Ausgaben neu.

Parser-Golden-Tests: `compiler/internal/parser/testdata/*.pas` → `*.ast`, neu schreiben mit
`go test ./compiler/internal/parser -update`.
Symbol-Golden-Tests der Typprüfung: `compiler/internal/check/testdata/*.pas` → `*.sym`, neu schreiben mit
`go test ./compiler/internal/check -update`.

## Pipeline (Stand Etappe 10)

`source` → `lexer` → `parser` (AST) → `check` (Typprüfung, `check.Info`) → `ir.Lower` (CFG) → `analysis` (anhaltefähige Funktionen) →
`codegen.Generate` (Ablage, Frames, Static Links; strukturierter Kontrollfluss oder Dispatch-Schleife für
anhaltefähige Funktionen und `goto`, siehe `codegen/dispatch.go`) → `wasm` (Encoder). Die System-Unit
`rtl/system.pas` (eingebettet über `rtl.FS`) wird immer mitgeprüft und mitübersetzt; sie enthält die gesamte
Laufzeit in Pascal; `crt.pas` und `dos.pas` liegen daneben und werden per `uses` gefunden (nach Programmverzeichnis und `-I`). Routinen mit `_` am Anfang ruft der Compiler direkt auf; WASI erreicht die Laufzeit über
Intrinsics aus `compiler/internal/abi`. Laufzeitroutinen (`Func.RTL`) pflegen die Fehlerstelle nicht.
Laufzeitfehler: exportierte Globale `__rt_*` plus Custom Section `wasmpascal.lines`, ausgewertet in
`runner/rterror.go` bzw. `web/loader.js`.

Ziele: Nur die unterste Schicht von `rtl/system.pas` unterscheidet sich, per `{$IFDEF GOJS}`; ohne Symbol gilt WASI.
Je Ziel getrennt sind Konsole, Eingabe, Uhr, Argumente und Anhaltepunkte (`SysWrite`, `SysRead`, `_Clock`, `SysArgs`,
`_SuspendAction`); Dateien und Verzeichnisse laufen unter beiden Zielen über WASI-Aufrufe, unter gojs stellt
`web/loader.js` sie bereit (Option `files`, synchroner Host nach der Node-API `fs`). `gojs` läuft mit
`web/wasm_exec.js` (Go 1.27.4) und `web/loader.js`; `TestProgramsGoJS` führt alle Testprogramme mit Node aus
(`web/node-run.js --dir`, übersprungen ohne `node`).

Browser: `cmd/pascaljs` (nur `GOOS=js GOARCH=wasm`) legt `pascalCompile` an; `web/index.html` ist die Demo
(Bau und Start in `web/README.md`). `TestWebCompiler` verlangt byte-gleiche Module wie der native Compiler.

Server: `pascal serve` (`cmd/pascal/serve.go`) liest JSON-Anfragen zeilenweise und antwortet je Zeile; Prüfen,
Übersetzen und Antworten liegen in `internal/compilereq`. Der Compiler läuft nativ im Prozess, geschützt durch
`compiler.Limits` und ein Zeitlimit je Anfrage (PLAN.md, Etappe 13).
