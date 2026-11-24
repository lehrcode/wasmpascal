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
