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
