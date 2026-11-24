# Abweichungen von Turbo Pascal 4.0

Bewusste Abweichungen und getroffene Annahmen. Die grundsätzlichen Abweichungen
(Typabbildung, Speichermodell, UTF-8, Erweiterungen) stehen in `PLAN.md`; hier
kommen die Einzelentscheidungen dazu, die während der Umsetzung fallen.

| Etappe | Thema | Entscheidung |
| --- | --- | --- |
| 0 | Laufzeitfehlermeldung | Fehlernummer ohne führende Nullen: `Runtime error 2 at …` (`// TP-ANNAHME`). |
| 1 | Reservierte Wörter | Die 44 Wörter von TP 4.0 (einschließlich `inline`) plus `object`, `constructor`, `destructor`, `inherited`. `absolute`, `external`, `forward` und `interrupt` sind wie ab TP 5 Standarddirektiven, also normale Bezeichner (`// TP-ANNAHME`). |
| 1 | Bezeichner | Signifikant sind die ersten 63 Zeichen, wie in TP. Nur ASCII; sonst Error 901. |
| 1 | Positionen | Spalten zählen Zeichen (UTF-8-Codepoints), nicht Bytes; ein Tab zählt als ein Zeichen. |
| 1 | Dateiende | Ein Strg+Z (`#26`) beendet die Quelldatei wie unter DOS. |
| 1 | Steuerzeichen `^X` | `^` gefolgt von einem Zeichen aus `@`..`_` oder `a`..`z` ist ein Steuerzeichen (`Ord xor 64`), wenn danach kein Bezeichnerzeichen folgt und das `^` nicht hinter einem Operanden (Bezeichner, `)`, `]`, `^`) oder im Typkontext steht. Den Typkontext setzt der Parser (`type P = ^T`). `^a` = `^A` (`// TP-ANNAHME`). |
| 1 | Bedingte Übersetzung | Zusätzlich zu den im Plan genannten Direktiven werden `{$UNDEF}` und `{$IFOPT x+}` unterstützt, weil TP 4.0 sie kennt und ein ignoriertes `{$IFOPT}` das `{$ENDIF}` aus dem Gleichgewicht brächte. Die Schalter werden dafür mitgeführt, wirken aber (außer `I`) nicht. |
| 1 | Vordefinierte Symbole | `VER40` und `WASMPASCAL`. `MSDOS` und `CPU86` fehlen bewusst, weil DOS-spezifische Teile nicht existieren. |
| 1 | `{$I datei}` | Ohne Endung wird `.pas` angenommen. Gesucht wird im Verzeichnis der einbindenden Datei, dann in den `-I`-Pfaden; `\` gilt als Trenner, Groß-/Kleinschreibung ist egal. Höchstens 16 Ebenen, sonst Error 9. |
| 1 | Fehlernummern | Unzulässiges ASCII-Zeichen: Error 5; `#n` über 255: Error 76; fehlplatziertes `{$ELSE}`/`{$ENDIF}` und fehlerhafte Direktiven: Error 17; fehlendes `{$ENDIF}`: Error 10 am Dateiende (jeweils `// TP-ANNAHME`). Eigene Nummern ab 900: 900 ungültiges UTF-8, 901 Nicht-ASCII-Zeichen außerhalb von Strings und Kommentaren. |
| 1 | CLI | Die CLI liest Dateien über ein `os.DirFS` der Laufwerkswurzel und zeigt Pfade in Meldungen relativ zum Arbeitsverzeichnis. |
| 1 | `Integer` | Bereich wie `LongInt` (`MaxInt` = 2147483647); TP-4.0-Programme, die mit 16-Bit-Überlauf rechnen, verhalten sich anders. `Word` bleibt 0..65535. |
| 1 | 1-Byte-Typen | `Byte`, `ShortInt`, `Char` (und 1-Byte-Teilbereiche) werden bei jeder Zuweisung auf 8 Bit gekürzt, wie TP mit `{$R-}`. |
| 1 | Teilbereiche | Speichergröße nach Wertebereich: 1 Byte nur, wenn alle Werte in 0..255 bzw. −128..127 passen. |
| 1 | `nil` | TP prüft Dereferenzierungen nicht. WasmPascal prüft jedes `p^` und meldet `Runtime error 216` (Nummer wie „General protection fault“ in BP 7), weil WASM keinen Speicherschutz hat. |
| 2 | Vorzeichen | `+`/`-` als Vorzeichen binden wie `not` am stärksten, damit `a * -b` wie in TP erlaubt ist. `-a*b` wird als `(-a)*b` gelesen; das Ergebnis ist dasselbe. |
| 2 | Vergleiche | Ein Ausdruck enthält höchstens einen Vergleichsoperator (`a = b = c` ist ein Syntaxfehler), wie in der Pascal-Grammatik. |
| 2 | Programmende | Nach dem abschließenden `end.` wird nichts mehr gelesen, wie in TP. Ein danach offenes `{$IFDEF}` fällt deshalb nicht auf. |
| 2 | Programmkopf | `program Name;` ist optional; Parameter wie `(Input, Output)` werden gelesen und ignoriert. |
| 2 | Nicht unterstützt | `absolute`, `external`, `interrupt` und `inline` ergeben Error 902 „Not supported by WasmPascal“. |
| 2 | Fehlernummern | Syntaxfehler benutzen die Nummern der TP-7-Liste (z. B. 85 `";" expected`, 57 `THEN expected`, 113 `Error in statement` für `; else`); `// TP-ANNAHME`, dass TP 4.0 dieselben hat. |
| 2 | Typisierte Konstanten | `(a, b)` ist eine Array-Konstante, `(x: 1; y: 2)` eine Record-Konstante. Ein einzelner geklammerter Wert `(5)` bleibt ein Ausdruck; die Typprüfung darf ihn bei Array-Typen als einelementige Liste deuten. |
| 2 | `^` im Typkontext | Der Parser liest nach `=` im type-Teil, nach `:` in Deklarationen und nach `array … of`/`file of` im Typmodus (`^C` = Zeiger). Nach `set of` nicht, damit `set of ^A..^Z` möglich bleibt. |
| 3 | Konstantenausdrücke | Erlaubt wie ab TP 5 (TP 4.0 kannte nur einfache Konstanten), mit `Abs`, `Chr`, `Hi`, `High`, `Length`, `Lo`, `Low`, `Odd`, `Ord`, `Pred`, `Round`, `SizeOf`, `Succ`, `Swap`, `Trunc` sowie `UpCase` und `Concat`. Typisierte Konstanten sind darin nicht erlaubt (Error 31), wie in TP. |
| 3 | Ganzzahlkonstanten | `$80000000`..`$FFFFFFFF` ergeben negative Werte; Dezimalkonstanten über `MaxLongInt` sind Real-Konstanten (`// TP-ANNAHME`). Überlauf beim Falten ist Error 76. |
| 3 | Ganzzahlarithmetik | Ergebnistyp `LongInt`, wenn ein Operand `LongInt` ist, sonst `Integer`; beide sind 32 Bit. |
| 3 | Typumwandlungen | Ordinal ↔ Ordinal (Konstanten werden auf die Zielbreite gekürzt, `Word(x)` auf 16 Bit), Zeiger ↔ Zeiger, Zeiger ↔ 4-Byte-Ganzzahl; sonst Variablenumwandlung bei gleicher Größe. `Real(i)` ist wie in TP ein Fehler (61). |
| 3 | `var`-Parameter | Verlangen identische Typen, auch bei Strings (`string[10]` an `var s: string` ist Error 26), wie TP mit der Voreinstellung `{$V+}`. |
| 3 | Funktionsaufruf als Anweisung | Erweiterung (siehe PLAN.md): Aufrufe eigener Funktionen dürfen immer als Anweisung stehen, das Ergebnis wird verworfen. Standardfunktionen der System-Unit, Typumwandlungen und Variablen ergeben wie bei `{$X+}` in TP 7 Error 122. |
| 3 | Funktionsergebnisse | Erweiterung wie Delphi 1.0 (siehe PLAN.md): alle Typen außer Dateien (und Records/Arrays mit Dateien), also auch Records, Arrays und Mengen (Error 34 sonst). Der Ergebnistyp muss wie in TP ein Typbezeichner sein. Nur `F := …` setzt das Ergebnis; `F.x`, `F[i]` usw. im Rumpf sind rekursive Aufrufe, Felder und Elemente setzt man über `Result.x`/`Result[i]`. |
| 3 | Zeigerverträglichkeit | Typisierte Zeiger sind nur bei gleichem Basistyp verträglich; `Pointer`, `@x` und `nil` mit allen. |
| 3 | Record-Konstanten | Felder stehen in Deklarationsreihenfolge; Felder dürfen fehlen (`// TP-ANNAHME`). |
| 3 | `for`-Laufvariable | Muss eine Variable des aktuellen Blocks (oder ein Wertparameter) sein, sonst Error 97. |
| 3 | `case` | Marken werden auf Verträglichkeit geprüft, aber nicht auf Doppelungen oder Wertebereich (`// TP-ANNAHME`). |
| 3 | Argumentzahl | Wie TP: zu viele Argumente ergeben `")" expected` (89), zu wenige `"," expected` (87). |
| 3 | `Seg(x)` | Ist die Konstante 0. |
| 3 | Vordefinierte Variablen | `Input`, `Output`, `RandSeed`, `FileMode`, `ExitCode`. Die Größen der Datei-Records (48 bzw. 304 Byte für `Text`) sind vorläufig bis Etappe 6. |
| 3 | `uses` | Bis Etappe 7 ergibt jede Unit außer `System` Error 15 (`File not found (Crt.pas)`). |
| 3 | Fehlernummern | Typfehler nach der TP-7-Liste (`// TP-ANNAHME`); eigene Nummer 903 für `Break`/`Continue` außerhalb einer Schleife. |
| 4 | Zeilenende | `WriteLn` schreibt `LF` (`\n`), nicht `CR LF` wie unter DOS. |
| 4 | `Write` für Real | Ohne Formatangabe Exponentialschreibweise mit Breite 17 (` 1.5000000000E+00`), mit `:w` entsprechend weniger Stellen (mindestens eine), mit `:w:d` Festkomma. Gerundet wird wie TP: halbe Werte von null weg, bezogen auf den exakten Binärwert. Exponenten über 99 bekommen drei Stellen. |
| 4 | `Write` für Boolean | `TRUE`/`FALSE` in Großbuchstaben, wie TP. |
| 4 | `Single` | Wird in Ausdrücken als `f64` gerechnet und beim Speichern auf Single-Genauigkeit gerundet. |
| 4 | Auswertungsreihenfolge | Funktionsaufrufe in einem Ausdruck werden vor dem übrigen Ausdruck ausgewertet (Voraussetzung für den Scheduler). Ändert eine Funktion eine Variable, die im selben Ausdruck gelesen wird, kann das Ergebnis von TP abweichen; TP legt die Reihenfolge ebenfalls nicht fest. |
| 4 | `Round` | Rundet halbe Werte von null weg (`Round(-2.5) = -3`), wie TP. |
| 4 | Laufzeitprüfungen | Keine Bereichs- und Überlaufprüfungen (wie TP mit `{$R-}`); Division durch 0 und Überlauf bei `Trunc`/`Round` führen bis Etappe 5 zu einem WASM-Trap ohne Pascal-Meldung. |
