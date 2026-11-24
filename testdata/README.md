# testdata/

Pascal-Testprogramme für den Test-Harness (`harness_test.go` im Wurzelverzeichnis).

- **Testfall** ist jedes `*.pas` (auch in Unterverzeichnissen), zu dem es eine
  `*.out` oder `*.err` mit gleichem Namen gibt. Dateien ohne beides (Units,
  `{$I}`-Includes) werden nicht als eigenes Programm übersetzt.
- `name.out`: erwartete Ausgabe. stdout und stderr landen gemeinsam darin.
  Ein Laufzeitfehler erscheint als eigene Zeile
  (`Runtime error 200 at name.pas(12) in Proc`), ein Exit-Code ungleich 0 als
  letzte Zeile `[Exit-Code N]`.
- `name.err`: erwartete Compilermeldung, z. B.
  `fehler/semikolon.pas(3,9): Error 85: ";" expected`. Pfade sind relativ zu `testdata/`.
- `name.in` (optional): Eingabe auf stdin.

Neuer Testfall: `name.pas` schreiben, eine leere `name.out` (oder `name.err`)
anlegen und `go test . -run TestPrograms -update` ausführen. Danach den Inhalt
der erzeugten Datei prüfen. `-update` ersetzt eine `.err` durch eine `.out`
(und umgekehrt), wenn sich das Ergebnis geändert hat.
