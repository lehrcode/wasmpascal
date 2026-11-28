# Changelog

Alle nennenswerten Änderungen an WasmPascal stehen in dieser Datei.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/de/1.1.0/),
und dieses Projekt hält sich an [Semantic Versioning](https://semver.org/lang/de/).
Solange die Hauptversion 0 ist, können sich Sprache, Speicherlayout und API
auch in kleineren Versionen inkompatibel ändern; solche Änderungen sind unten
ausdrücklich genannt. Einzelheiten zu jeder Änderung stehen in
[PLAN.md](PLAN.md) (Etappen) und [docs/abweichungen.md](docs/abweichungen.md).

## [Unreleased]

Etappe 18: Ungenutzten Code entfernen.

### Geändert

- **Inkompatibel:** Module enthalten nur noch die Routinen, die das Programm
  erreicht (Wurzeln: Hauptprogramm, die Laufzeitroutinen des Codegenerators
  und jede Routine der Funktionstabelle). „Hallo Welt“ schrumpft dadurch von
  33,4 KB auf 4,5 KB und importiert keine `path_*`-WASI-Aufrufe mehr; mit
  `uses Crt, Dos` 42,5 KB auf 10,1 KB. Ungenutzte `external`-Deklarationen
  erscheinen nicht als Importe. Die Typprüfung bleibt vollständig: Fehler in
  ungenutzten Routinen werden weiterhin gemeldet. Globale Variablen und
  typisierte Konstanten bleiben im Modul.

Etappe 17: `Int64`, `external` und eine Laufzeit ohne fest eingebaute
WASI-Aufrufe.

### Hinzugefügt

- Typ `Int64` (aus Delphi 4): 64-Bit-Arithmetik, sobald ein Operand `Int64`
  ist; `Write`, `Read`, `Str`, `Val`, `file of Int64`, `for`-Schleifen.
  Hexkonstanten mit 9 bis 16 Ziffern sind `Int64`.
- `external 'modul' [name 'importname']` für Host-Funktionen (Ziel WASI),
  in Go über `runner.Options.Imports` und `runner.Memory`; fehlende
  Importe meldet `runner.Run` vor dem Start. Neue Fehler 909 (Typ an der
  Grenze nicht erlaubt) und 910 (widersprüchlicher Import).

### Geändert

- **Inkompatibel:** Dezimalkonstanten über `MaxLongInt` sind `Int64` statt
  Real (`3000000000` ist eine Ganzzahl; als `LongInt` zugewiesen Error 76).
- **Inkompatibel:** `Comp` wird wie in TP als 64-Bit-Ganzzahl gespeichert
  und beim Speichern gerundet (`C := 1.5` ergibt 2, `C := 2.5` ergibt 2);
  Records und Dateien mit `Comp` haben das Layout von TP.
- Die Laufzeit deklariert ihre WASI- und `gojs`-Aufrufe selbst mit
  `external`; die fest eingebauten Intrinsics dafür sind entfernt.
- Interne Bezeichner der Laufzeit (`rtl/*.pas`) und der Go-Tests sind
  englisch; neue Arbeitsregel: Bezeichner englisch, Kommentare und Doku
  deutsch. Gemeinsame Hilfsroutinen stehen einmal in der System-Unit
  (`_LastSep`, `_KindFile` usw.), doppelter Code in `system.pas` und
  `dos.pas` ist zusammengefasst. Das Verhalten ändert sich nicht.

### Behoben

- `pascal run` (und `runner.Run` allgemein) brach unter `cmd.exe` mit
  `runner: environ invalid: empty key` ab, bevor das Programm lief: Die von
  cmd.exe vererbten versteckten Variablen (`=C:` usw.) ergaben beim Aufteilen
  am ersten `=` einen leeren Namen, den wazero ablehnt. `runner.Options.Env`
  überspringt namense-lose Einträge jetzt; WASI kann solche Namen nicht
  darstellen (siehe [docs/abweichungen.md](docs/abweichungen.md)).

## [0.3.0] - 2026-11-27

Etappen 15 und 16: Sprachumfang von Turbo Pascal 5.0 mit prozeduralen Typen
und Exit-Prozeduren, Unit `Dos` erweitert, Ortszeit.

### Hinzugefügt

- Prozedurale Typen wie in TP 5.0: `type TFunc = function(X: Real): Real;`,
  Prozedur- und Funktionsvariablen, Funktionsparameter, Arrays, Records und
  typisierte Konstanten mit Routinen, `@Routine`, `@P = nil`, `TProc(Ptr)`.
  Zuweisbar sind Routinen der obersten Ebene; Aufrufe über `nil` ergeben
  Runtime error 216. Indirekt aufgerufene Routinen dürfen anhalten
  (`ReadLn`, `Delay`).
- `ExitProc`, `ErrorAddr` und verkettete Exit-Prozeduren wie in TP: am
  Programmende, nach `Halt` und nach jedem Laufzeitfehler, auch nach
  WASM-Traps. Eine Exit-Prozedur kann die Meldung „Runtime error …“
  unterdrücken (`ErrorAddr := nil`) und `ExitCode` ändern. Dafür exportiert
  jedes Modul `_exit`, das `runner` und `web/loader.js` nach einem Trap
  rufen.
- `RunError[(Code)]` (aus TP 5.0).
- `{$A+}`/`{$A-}` wird angenommen (ohne Wirkung auf die Ausrichtung).
- Unit `Dos`: `GetFAttr`, `GetFTime`, `FindFirst`, `FindNext`, `PackTime`,
  `UnpackTime` (aus TP 4.0) sowie `FExpand`, `FSplit`, `FSearch`, `GetEnv`,
  `EnvCount`, `EnvStr` und `DosVersion` (aus TP 5.0); Typen `SearchRec`,
  `DateTime`, `PathStr`, `DirStr`, `NameStr`, `ExtStr`, die
  Attributkonstanten und `DosError`. Nur die Attribute `Directory` und
  `Hidden` (Name mit Punkt am Anfang) haben eine Wirkung.
- Umgebungsvariablen für Programme: `runner.Options.Env`, Option `env` in
  `web/loader.js`, `node-run.js --env`/`--setenv`; `pascal run` gibt die
  Umgebung des Rechners weiter.
- Datei-Host des Loaders: `readdirSync` und `mtimeMs` (für `FindFirst` und
  Dateizeiten unter `gojs`).

### Geändert

- **Inkompatibel:** Das vordefinierte Symbol heißt `VER50` statt `VER40`.
- Die Dokumentation nennt Turbo Pascal 5.0 als Sprachumfang; der Hinweis
  „ohne OOP“ entfällt (Objekte kamen erst mit TP 5.5).
- Das Programmende wirkt wie `Halt(0)` und läuft über die Exit-Prozeduren;
  unter WASI endet ein Programm dadurch immer über `proc_exit`.
- Der Aufruf einer Variable ohne prozeduralen Typ (`I(1)`) ergibt Error 142
  statt 143; `@` auf eine verschachtelte Routine ergibt Error 143 statt 83.
- `GetDate`, `GetTime` und Dateizeiten liefern Ortszeit statt UTC, wenn
  `WASMPASCAL_UTC_OFFSET` gesetzt ist (`pascal run` und der Browser setzen
  sie). Der Versatz gilt für den ganzen Lauf.

### Behoben

- `GetTime` lieferte bei manchen Bruchteilen einer Sekunde (z. B. 0,25 s)
  einen Hundertstel zu wenig.

## [0.2.0] - 2026-11-26

Etappe 14: Unicode-Zeichen als Wert, 16-Bit-Ganzzahlen wie in TP und
Prüfungen der Zeichen im Quelltext.

### Hinzugefügt

- Typ `WideChar` (aus Delphi 2.0): ein Unicode-Zeichen bis U+FFFF, 2 Byte,
  Ordinaltyp ohne Arithmetik. Ein Literal mit genau einem Zeichen (`'ä'`,
  `'€'`) gilt dort als `WideChar`, wo eines erwartet wird; gegenüber einem
  `WideChar` wird ordinal verglichen. Als String eingesetzt (Zuweisung, `+`,
  Vergleich mit Strings, `Length`, `Pos`, …) wird ein `WideChar` zu UTF-8.
- `#n` von 256 bis 65535 ist eine `WideChar`-Konstante (`#$20AC` = `'€'`);
  in einer Folge wie `'x'#$20AC` wird das Zeichen als UTF-8 eingesetzt.
- `Read`, `Write` (Breite in Zeichen) und `file of WideChar` für `WideChar`.
- `for … in` mit einer `WideChar`-Laufvariable läuft über die Zeichen eines
  Strings statt über die Bytes (eigene Erweiterung).
- UTF-8-Routinen in der System-Unit: `UTF8Length`, `UTF8Next`, `UTF8Index`
  und `UTF8Copy`. Ungültige Bytefolgen werden nach der Unicode-Empfehlung zu
  `#$FFFD`.
- Typ `SmallInt` (16 Bit mit Vorzeichen, 2 Byte).

### Geändert

- **Inkompatibel:** `Word` belegt wie in TP 2 Byte statt 4, und jede
  Zuweisung kürzt auf 16 Bit: `65535 + 1` ergibt jetzt 0 statt 65536.
  `SizeOf(Word)` ist 2; Records und typisierte Dateien mit `Word`-Feldern
  haben dadurch das Layout von TP.
- **Inkompatibel:** Teilbereiche und Aufzählungen belegen wie in TP 1, 2 oder
  4 Byte (bisher 1 oder 4): `1000..1100` hat jetzt 2 Byte, eine Aufzählung
  mit 300 Werten ebenfalls.
- **Inkompatibel:** Steuerzeichen (U+0000–U+001F außer Tabulator) und
  Noncharacters direkt in einem String-Literal ergeben Error 907; über `#n`
  und `^X` bleiben Steuerzeichen erreichbar (`#27'[34m'`, `'Zeile'#10`).
  Alte Quelltexte mit einem rohen ESC im String müssen auf `#27` umgestellt
  werden.
- `#n` über 255 ist kein Fehler mehr; `#n` über 65535, Surrogate
  (`#$D800`..`#$DFFF`) und Noncharacters (`#$FFFE`, `#$FFFF`,
  `#$FDD0`..`#$FDEF`) ergeben Error 76.

### Sicherheit

- Bidi-Steuerzeichen (U+202A–U+202E, U+2066–U+2069) ergeben im ganzen
  Quelltext Error 908, auch in Kommentaren und eingebundenen Dateien. Mit
  ihnen kann ein Editor Quelltext anders anzeigen, als er übersetzt wird
  („Trojan Source“, CVE-2021-42574).

## [0.1.0] - 2026-11-25

Erste Version: Etappen 0 bis 13.

### Hinzugefügt

- Compiler für Turbo Pascal 4.0 ohne OOP nach WebAssembly, geschrieben in Go:
  Units, Records mit Varianten, Aufzählungen, Teilbereiche, Mengen, Zeiger,
  ShortStrings, typisierte und untypisierte Dateien, verschachtelte
  Prozeduren, `goto`. Erweiterungen aus TP 7 und Delphi: `Result`,
  `Break`/`Continue`, `Low`/`High`, `const`-Parameter, `for … in`,
  Funktionsaufrufe als Anweisung, Record- und Array-Ergebnisse.
- Laufzeit in Pascal: Units `System`, `Crt` (Farben über ANSI, `ClrScr`,
  `GotoXY`, `Delay`, `ReadKey` zeilenweise) und `Dos` (Datum, Uhrzeit);
  Laufzeitfehler als `Runtime error NNN at datei.pas(zeile) in Prozedur`.
- Ziele WASI (Preview 1) und `gojs` (Schnittstelle von `GOOS=js`, mit
  `web/wasm_exec.js` und `web/loader.js`); ein Scheduler hält Programme bei
  Eingaben, `Delay` und langen Ausgaben an, damit der Browser bedienbar
  bleibt. Dateien und Verzeichnisse unter beiden Zielen, unter `gojs` über
  einen Datei-Host des Loaders.
- Kommandozeile `pascal` mit `build`, `run`, `check` und `serve`
  (Compiler-Server mit JSON-Zeilen).
- Go-API: Paket `compiler` (nur Standardbibliothek, baut auch für `GOOS=js`
  und `GOOS=wasip1`) mit einstellbaren Grenzen (`compiler.Limits`) für
  Quelltexte aus fremder Hand, Paket `runner` (wazero).
- Compiler als WebAssembly im Browser (`pascalCompile`), Demo-Seite mit
  Editor und Terminal, Node.js-Werkzeuge `node-compile.js` und `node-run.js`.
- Dokumentation: README, Referenz der Units (`docs/units.md`), Abweichungen
  von TP (`docs/abweichungen.md`), MIT-Lizenz.

[Unreleased]: https://github.com/lehrcode/wasmpascal/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/lehrcode/wasmpascal/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/lehrcode/wasmpascal/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/lehrcode/wasmpascal/releases/tag/v0.1.0
