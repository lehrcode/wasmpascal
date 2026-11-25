# testdata/

Pascal-Testprogramme für den Test-Harness (`harness_test.go` im Wurzelverzeichnis).

- **Testfall** ist jedes `*.pas` (auch in Unterverzeichnissen), zu dem es eine
  `*.out` oder `*.err` mit gleichem Namen gibt. Dateien ohne beides (Units,
  `{$I}`-Includes) werden nicht als eigenes Programm übersetzt.
- `name.out`: erwartete Ausgabe. stdout und stderr landen gemeinsam darin.
  Ein Laufzeitfehler erscheint als eigene Zeile
  (`Runtime error 200 at name.pas(12) in Proc`), sonst ein Exit-Code ungleich 0
  als letzte Zeile `[Exit-Code N]`.
- `name.err`: erwartete Compilermeldung, z. B.
  `fehler/semikolon.pas(3,9): Error 85: ";" expected`. Pfade sind relativ zu `testdata/`.
- `name.in` (optional): Eingabe auf stdin.
- `name.args` (optional): weitere Kommandozeilenargumente (`ParamStr(1)` …).
- Jeder Testfall bekommt ein eigenes leeres Verzeichnis als Wurzel `/` für
  Dateizugriffe.

Zusätzlich führt `TestProgramsGoJS` jeden Testfall mit dem Ziel `gojs` in
Node.js aus (`web/node-run.js --dir`) und erwartet dieselbe Ausgabe; auch dort
bekommt jeder Testfall ein eigenes Verzeichnis als Wurzel. Ohne `node` wird
der Test übersprungen.

Neuer Testfall: `name.pas` schreiben, eine leere `name.out` (oder `name.err`)
anlegen und `go test . -run TestPrograms -update` ausführen. Danach den Inhalt
der erzeugten Datei prüfen. `-update` ersetzt eine `.err` durch eine `.out`
(und umgekehrt), wenn sich das Ergebnis geändert hat.
