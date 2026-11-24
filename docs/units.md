# Units von WasmPascal

Diese Referenz listet alles, was ein Programm aus den mitgelieferten Units
benutzen kann: Typen, Konstanten, Variablen, Prozeduren und Funktionen. Es
gibt drei Units:

| Unit | Einbinden | Inhalt |
| --- | --- | --- |
| [System](#system) | immer da, ohne `uses` | Standardtypen, Ein- und Ausgabe, Dateien, Strings, Mathematik, Speicher, Programmablauf |
| [Crt](#crt) | `uses Crt;` | Farben, Bildschirm löschen, `Delay`, `ReadKey` |
| [Dos](#dos) | `uses Dos;` | Datum und Uhrzeit |

Eigene Units sucht der Compiler im Verzeichnis des Programms und in den
`-I`-Pfaden; eine eigene `crt.pas` dort verdeckt die eingebaute Unit.
Bezeichner, die mit `_` beginnen, gehören zur Laufzeit und sind nicht für
Programme gedacht.

Zum Lesen der Signaturen: `var` heißt, dass eine Variable übergeben werden
muss; Teile in `[ ]` sind optional. Abweichungen von Turbo Pascal 4.0 sind
mit **Abweichung** markiert und in [abweichungen.md](abweichungen.md)
begründet.

## System

### Typen

| Typ | Wertebereich | Größe |
| --- | --- | --- |
| `Boolean` | `False`, `True` | 1 Byte |
| `Char` | ein Byte, `#0`..`#255` | 1 Byte |
| `ShortInt` | −128..127 | 1 Byte |
| `Byte` | 0..255 | 1 Byte |
| `Integer` | −2147483648..2147483647 (**Abweichung:** wie `LongInt`, TP: 16 Bit) | 4 Byte |
| `LongInt` | −2147483648..2147483647 | 4 Byte |
| `Word` | 0..65535 | 4 Byte |
| `Real`, `Double`, `Extended`, `Comp` | Gleitkomma doppelter Genauigkeit (**Abweichung:** alle gleich) | 8 Byte |
| `Single` | Gleitkomma einfacher Genauigkeit | 4 Byte |
| `Pointer` | untypisierter Zeiger, `nil` | 4 Byte |
| `Text` | Textdatei | 528 Byte |

Dazu die Typkonstruktoren der Sprache: `string` und `string[n]` (immer
256 Byte, höchstens 255 Bytes Text), `array`, `record`, `set of`, `file`,
`file of T`, `^T`, Aufzählungen und Teilbereiche.

Texte sind UTF-8: Ein Umlaut belegt zwei Bytes, `'ä'` ist deshalb ein
String und kein `Char`, und `Length('Größe')` ist 7.

### Konstanten

| Name | Wert |
| --- | --- |
| `False`, `True` | Wahrheitswerte |
| `MaxInt` | 2147483647 (**Abweichung:** TP 32767) |
| `MaxLongInt` | 2147483647 |
| `Pi` | 3.14159265358979… (eine Funktion ohne Parameter) |

### Variablen

| Name | Typ | Bedeutung |
| --- | --- | --- |
| `Input` | `Text` | Standardeingabe; `Read`/`ReadLn` ohne Datei lesen von hier |
| `Output` | `Text` | Standardausgabe; `Write`/`WriteLn` ohne Datei schreiben hierhin |
| `RandSeed` | `LongInt` | Startwert des Zufallsgenerators; ein fester Wert ergibt immer dieselbe Folge |
| `FileMode` | `Byte` | vorhanden, aber ohne Wirkung |
| `ExitCode` | `Integer` | vorhanden, aber ohne Wirkung; den Exit-Code setzt `Halt(n)` |

### Ein- und Ausgabe mit Text

| Signatur | Beschreibung |
| --- | --- |
| `Write([var F: Text,] A1, A2, …)` | Schreibt die Werte nacheinander. Erlaubt sind ganze Zahlen, Gleitkommazahlen, `Char`, Strings und `Boolean`. Jeder Wert kann eine Breite haben: `X:Breite` füllt links mit Leerzeichen auf, `R:Breite:Nachkomma` schreibt eine Gleitkommazahl mit festen Nachkommastellen. Ohne Format erscheint eine Gleitkommazahl wissenschaftlich (`1.5` → ` 1.5000000000E+00`), `Boolean` als `TRUE`/`FALSE`. |
| `WriteLn([var F: Text,] A1, …)` | Wie `Write`, danach ein Zeilenende. |
| `Write(var F: file of T, V1, …)` | Schreibt Datensätze in eine typisierte Datei. |
| `Read([var F: Text,] V1, V2, …)` | Liest in Variablen vom Typ ganze Zahl, Gleitkomma, `Char` oder String. Zahlen werden als Wort bis zum nächsten Leerraum gelesen; ein String bis zum Zeilenende (ohne es zu überlesen). `Read(C)` liefert am Zeilenende `#10` (**Abweichung:** TP `#13`), am Dateiende `#26`. |
| `ReadLn([var F: Text,] V1, …)` | Wie `Read`, überliest danach den Rest der Zeile. `ReadLn` ohne Variablen wartet auf Enter. |
| `Read(var F: file of T, V1, …)` | Liest Datensätze aus einer typisierten Datei. |
| `Eof[(var F)]: Boolean` | `True` am Dateiende. Ohne Parameter für `Input`. Auch für typisierte und untypisierte Dateien. |
| `Eoln[(var F: Text)]: Boolean` | `True` am Zeilenende oder Dateiende. |
| `SeekEof[(var F: Text)]: Boolean` | Überspringt Leerzeichen, Tabulatoren und Zeilenenden, dann wie `Eof`. |
| `SeekEoln[(var F: Text)]: Boolean` | Überspringt Leerzeichen und Tabulatoren, dann wie `Eoln`. |

Das Programm wartet bei `Read`/`ReadLn` auf `Input` auf eine ganze, mit
Enter abgeschlossene Zeile. Vor dem Lesen wird `Output` geleert.

### Dateien

Unter dem Ziel `gojs` (Browser) gibt es keine Dateien: Öffnen, Löschen
usw. ergeben E/A-Fehler 5. Unter WASI sind Dateinamen relativ zum
Arbeitsverzeichnis; `\` gilt als `/`, Laufwerksbuchstaben gibt es nicht.

| Signatur | Beschreibung |
| --- | --- |
| `Assign(var F; Name: string)` | Verbindet die Dateivariable mit einem Dateinamen. `Assign(F, '')` steht für Standardein- bzw. -ausgabe. |
| `Reset(var F [; RecSize: Word])` | Öffnet eine vorhandene Datei zum Lesen; typisierte und untypisierte Dateien auch zum Schreiben. `RecSize` nur bei untypisierten Dateien (Standard 128). |
| `Rewrite(var F [; RecSize: Word])` | Legt die Datei neu an bzw. leert sie und öffnet sie zum Schreiben. |
| `Append(var F: Text)` | Öffnet eine Textdatei zum Anhängen. |
| `Close(var F)` | Schreibt den Puffer und schließt die Datei. Nicht geschlossene Dateien verlieren ihren Puffer. |
| `Flush(var F: Text)` | Schreibt den Puffer einer Textdatei. |
| `Erase(var F)` | Löscht die Datei. |
| `Rename(var F; NeuerName: string)` | Benennt die Datei um. |
| `Seek(var F; N: LongInt)` | Setzt die Position auf Datensatz `N` (ab 0); nicht für Textdateien. |
| `FilePos(var F): LongInt` | Aktuelle Datensatznummer. |
| `FileSize(var F): LongInt` | Anzahl der Datensätze. |
| `Truncate(var F)` | Schneidet die Datei an der aktuellen Position ab. |
| `BlockRead(var F: file; var Buf; Count: Word [; var Result: Word])` | Liest `Count` Sätze nach `Buf`. Ohne `Result` ist ein zu kurzes Lesen ein E/A-Fehler, mit `Result` steht dort die Anzahl gelesener Sätze. |
| `BlockWrite(var F: file; var Buf; Count: Word [; var Result: Word])` | Schreibt `Count` Sätze aus `Buf`. |
| `IOResult: Integer` | Liefert den letzten E/A-Fehler und setzt ihn auf 0 zurück. |

**Fehlerbehandlung:** Normal (`{$I+}`) beendet ein E/A-Fehler das Programm
mit einem Laufzeitfehler. Unter `{$I-}` läuft das Programm weiter, aber jede
weitere E/A (auch `Write`) ruht, bis `IOResult` abgefragt wurde. Darum
`IOResult` zuerst in eine Variable lesen:

```pascal
{$I-} Reset(F); {$I+}
Fehler := IOResult;
if Fehler <> 0 then WriteLn('Fehler ', Fehler);
```

### Verzeichnisse

| Signatur | Beschreibung |
| --- | --- |
| `ChDir(Pfad: string)` | Wechselt das aktuelle Verzeichnis des Programms. |
| `MkDir(Pfad: string)` | Legt ein Verzeichnis an. |
| `RmDir(Pfad: string)` | Löscht ein leeres Verzeichnis. |
| `GetDir(Laufwerk: Byte; var S: string)` | Liefert das aktuelle Verzeichnis als `/pfad`; `Laufwerk` wird ignoriert. |

### Strings

| Signatur | Beschreibung |
| --- | --- |
| `Length(S): Integer` | Länge in Bytes. |
| `Copy(S; Index, Count: Integer): string` | Teilstring ab `Index` (ab 1) mit höchstens `Count` Zeichen. `Copy('abc', 2, 5)` = `'bc'`. |
| `Concat(S1, S2, …): string` | Hängt Strings aneinander, wie `+`; gekürzt auf 255 Zeichen. |
| `Pos(Teil, S): Integer` | Position des ersten Vorkommens von `Teil` in `S`, 0 wenn nicht gefunden. |
| `Delete(var S: string; Index, Count: Integer)` | Entfernt `Count` Zeichen ab `Index`. |
| `Insert(Quelle: string; var S: string; Index: Integer)` | Fügt `Quelle` vor `Index` ein; gekürzt auf die deklarierte Länge von `S`. |
| `Str(X[:Breite[:Nachkomma]]; var S: string)` | Zahl in Text, mit denselben Formaten wie `Write`. |
| `Val(S: string; var V; var Code: Integer)` | Text in Zahl (`V` ganzzahlig oder Gleitkomma). `Code` = 0 bei Erfolg, sonst die Position des ersten ungültigen Zeichens. Führende Leerzeichen und `$hex` sind erlaubt. |
| `UpCase(C: Char): Char` | Großbuchstabe zu `'a'`..`'z'`; alle anderen Zeichen bleiben gleich. |

### Arithmetik

| Signatur | Beschreibung |
| --- | --- |
| `Abs(X)` | Betrag; Ergebnis vom Typ des Arguments. |
| `Sqr(X)` | Quadrat; Ergebnis vom Typ des Arguments. |
| `Sqrt(X): Real` | Quadratwurzel; negatives Argument ergibt Laufzeitfehler 207. |
| `Sin(X)`, `Cos(X)`, `ArcTan(X): Real` | Winkelfunktionen im Bogenmaß. |
| `Exp(X): Real` | e hoch `X`. |
| `Ln(X): Real` | Natürlicher Logarithmus; `X` ≤ 0 ergibt Laufzeitfehler 207. |
| `Int(X): Real` | Ganzzahliger Teil (Richtung 0). |
| `Frac(X): Real` | Nachkommateil, `X - Int(X)`. |
| `Trunc(X): LongInt` | Abschneiden Richtung 0: `Trunc(-2.7)` = −2. |
| `Round(X): LongInt` | Runden, halbe Werte von 0 weg: `Round(2.5)` = 3, `Round(-2.5)` = −3. Zu große Werte ergeben Laufzeitfehler 207. |
| `Pi: Real` | Die Kreiszahl. |
| `Random: Real` | Zufallszahl mit 0 ≤ x < 1. |
| `Random(N: Word): Integer` | Zufallszahl 0..N−1 (0, wenn N ≤ 0). |
| `Randomize` | Setzt `RandSeed` aus der Uhr, damit jeder Lauf andere Zufallszahlen liefert. |

### Ordinalwerte

| Signatur | Beschreibung |
| --- | --- |
| `Ord(X): LongInt` | Ordnungszahl eines Ordinalwerts (`Ord('A')` = 65). |
| `Chr(N: Byte): Char` | Zeichen mit dem Code `N`. |
| `Succ(X)`, `Pred(X)` | Nachfolger bzw. Vorgänger, im selben Typ. |
| `Odd(N): Boolean` | `True`, wenn `N` ungerade ist. |
| `Inc(var X [; N: LongInt])` | Erhöht `X` um 1 bzw. `N`. |
| `Dec(var X [; N: LongInt])` | Vermindert `X` um 1 bzw. `N`. |
| `Hi(N): Byte` | Bits 8..15: `Hi($1234)` = `$12`. |
| `Lo(N): Byte` | Bits 0..7: `Lo($1234)` = `$34`. |
| `Swap(N)` | Tauscht die beiden unteren Bytes: `Swap($1234)` = `$3412`. |
| `Low(X)`, `High(X)` | Kleinster bzw. größter Wert eines Ordinaltyps, die Indexgrenzen eines Arrays, bei Strings 0 und die deklarierte Länge. Argument ist ein Typ oder eine Variable (Erweiterung aus TP 7). |

### Speicher und Zeiger

| Signatur | Beschreibung |
| --- | --- |
| `New(var P)` | Legt eine neue dynamische Variable an; `P` zeigt darauf. Ist der Heap erschöpft: Laufzeitfehler 203. |
| `Dispose(var P)` | Gibt die Variable wieder frei; `Dispose` mit `nil` ergibt Laufzeitfehler 204. |
| `GetMem(var P: Pointer; Size: LongInt)` | Reserviert `Size` Bytes. |
| `FreeMem(var P: Pointer; Size: LongInt)` | Gibt `Size` Bytes frei. |
| `Mark(var P: Pointer)` | Merkt sich den aktuellen Stand des Heaps. |
| `Release(var P: Pointer)` | Gibt alles frei, was nach dem zugehörigen `Mark` angelegt wurde. |
| `MemAvail: LongInt` | Freier Heap-Speicher insgesamt in Byte. |
| `MaxAvail: LongInt` | Größter zusammenhängender freier Block in Byte. |
| `SizeOf(X): Integer` | Größe eines Typs oder einer Variablen in Byte (siehe Tabelle der Typen; `string` = 256). |
| `FillChar(var X; Count: LongInt; Wert)` | Füllt `Count` Bytes ab `X` mit `Wert` (Byte oder `Char`). |
| `Move(var Quelle, Ziel; Count: LongInt)` | Kopiert `Count` Bytes, auch bei überlappenden Bereichen. |
| `Addr(X): Pointer` | Adresse von `X`, wie `@X`. |
| `Ofs(X): LongInt` | Adresse von `X` als Zahl. |
| `Seg(X): Word` | Immer 0 (es gibt keine Segmente). |
| `Ptr(Seg, Ofs): Pointer` | Zeiger auf die Adresse `Ofs`; `Seg` wird ignoriert. |

Jeder Zugriff über einen `nil`-Zeiger ergibt Laufzeitfehler 216.

### Programmablauf

| Signatur | Beschreibung |
| --- | --- |
| `Halt[(Code: Word)]` | Beendet das Programm, schreibt vorher `Output` und gibt `Code` (Standard 0) als Exit-Code zurück. |
| `Exit` | Verlässt die aktuelle Prozedur oder Funktion; im Hauptprogramm endet das Programm. |
| `Break` | Verlässt die innerste Schleife (Erweiterung aus TP 7). |
| `Continue` | Springt zum nächsten Durchlauf der innersten Schleife (Erweiterung aus TP 7). |
| `ParamCount: Integer` | Anzahl der Kommandozeilenargumente. |
| `ParamStr(N): string` | Argument `N`; `ParamStr(0)` ist der Programmname (Dateiname ohne `.pas`), außerhalb des Bereichs `''`. |

### Nicht vorhanden

Diese Bezeichner der System-Unit von TP 4.0 setzen das Speichermodell von
DOS voraus. Ihre Verwendung ergibt Error 902 (Not supported by WasmPascal):

`CSeg`, `DSeg`, `SSeg`, `SPtr`, `PrefixSeg`, `FreePtr`, `FreeMin`,
`HeapOrg`, `HeapPtr`, `HeapError`, `Mem`, `MemW`, `MemL`, `Port`, `PortW`,
`Intr`, `MsDos`, `OvrHeapOrg`.

## Crt

Steuert den Bildschirm über ANSI-Sequenzen. Es gibt keinen Bildschirmspeicher
und keine Cursorposition: Die Ausgabe läuft als Text durch, Umbruch und
Rollen übernimmt das Terminal.

### Konstanten

Farben für `TextColor` und `TextBackground`:

| Wert | Name | Wert | Name |
| --- | --- | --- | --- |
| 0 | `Black` | 8 | `DarkGray` |
| 1 | `Blue` | 9 | `LightBlue` |
| 2 | `Green` | 10 | `LightGreen` |
| 3 | `Cyan` | 11 | `LightCyan` |
| 4 | `Red` | 12 | `LightRed` |
| 5 | `Magenta` | 13 | `LightMagenta` |
| 6 | `Brown` | 14 | `Yellow` |
| 7 | `LightGray` | 15 | `White` |

`Blink` (128) lässt die Schrift blinken: `TextColor(Red + Blink)`.

### Variablen

| Name | Typ | Bedeutung |
| --- | --- | --- |
| `TextAttr` | `Byte` | Aktuelle Farben: Bits 0..3 Schrift, 4..6 Hintergrund, 7 Blinken. Startwert `LightGray` auf `Black`. Eine Zuweisung wirkt erst mit dem nächsten Aufruf von `TextColor`, `TextBackground`, `HighVideo` oder `LowVideo`. |
| `CheckBreak`, `CheckEOF`, `DirectVideo`, `CheckSnow` | `Boolean` | vorhanden, aber ohne Wirkung |

### Prozeduren und Funktionen

| Signatur | Beschreibung |
| --- | --- |
| `ClrScr` | Löscht den Bildschirm und setzt den Cursor nach links oben. |
| `ClrEol` | Löscht von der Cursorposition bis zum Zeilenende. |
| `InsLine` | Fügt an der Cursorzeile eine Leerzeile ein. |
| `DelLine` | Löscht die Cursorzeile. |
| `TextColor(Farbe: Byte)` | Setzt die Schriftfarbe (0..15, dazu `Blink`; 16..31 blinken ebenfalls). |
| `TextBackground(Farbe: Byte)` | Setzt die Hintergrundfarbe (0..7). |
| `HighVideo` | Helle Schrift: setzt Bit 3 der Schriftfarbe. |
| `LowVideo` | Dunkle Schrift: löscht Bit 3 der Schriftfarbe. |
| `NormVideo` | Zurück zu den Standardfarben des Terminals. |
| `Delay(MS: Word)` | Wartet `MS` Millisekunden; die Ausgabe wird vorher geschrieben. |
| `ReadKey: Char` | Liefert eine Taste. **Abweichung:** Die Eingabe ist zeilenweise; `ReadKey` wartet, bis eine Zeile mit Enter abgeschlossen ist, liefert deren erstes Zeichen und verwirft den Rest samt Enter. Eine leere Zeile (nur Enter) ergibt `#13`, das Dateiende `#26`. So liest ein folgendes `ReadLn` eine neue Zeile, wie nach einem Tastendruck in TP. Die Eingabe erscheint auf dem Bildschirm; Sondertasten (Pfeile, F-Tasten) gibt es nicht. |
| `Sound(Hz: Word)`, `NoSound` | Ohne Wirkung. |

**Nicht vorhanden:** `GotoXY`, `WhereX`, `WhereY`, `Window`, `KeyPressed`,
`TextMode`, `AssignCrt`. Ihre Verwendung ergibt Error 3 (Unknown identifier).

## Dos

Nur Datum und Uhrzeit. Beide sind **UTC**, weil WASI keine Zeitzone kennt.

| Signatur | Beschreibung |
| --- | --- |
| `GetDate(var Year, Month, Day, DayOfWeek: Word)` | Aktuelles Datum; `DayOfWeek` 0 = Sonntag … 6 = Samstag. |
| `GetTime(var Hour, Minute, Second, Sec100: Word)` | Aktuelle Uhrzeit; `Sec100` sind Hundertstelsekunden. |

Alle anderen Routinen der Unit Dos von TP 4.0 (z. B. `FindFirst`, `Exec`,
`SetDate`) gibt es nicht.

## Laufzeitfehler

Ein Laufzeitfehler beendet das Programm mit einer Meldung wie
`Runtime error 200 at zahlen.pas(42) in Auswerten`; der Exit-Code ist die
Fehlernummer.

| Nummer | Bedeutung |
| --- | --- |
| 2 | Datei nicht gefunden |
| 3 | Pfad nicht gefunden |
| 4 | Zu viele offene Dateien |
| 5 | Zugriff verweigert (unter `gojs` für jeden Dateizugriff) |
| 100 | Lesefehler (z. B. Lesen hinter dem Dateiende) |
| 101 | Schreibfehler |
| 102 | Datei nicht mit `Assign` zugewiesen |
| 103 | Datei nicht geöffnet |
| 104 | Datei nicht zum Lesen geöffnet |
| 105 | Datei nicht zum Schreiben geöffnet |
| 106 | Ungültiges Zahlenformat beim Lesen |
| 200 | Division durch 0 |
| 202 | Stapelüberlauf (zu tiefe Rekursion) |
| 203 | Heap erschöpft |
| 204 | `Dispose`/`FreeMem` mit `nil` |
| 207 | Ungültige Gleitkommaoperation (`Sqrt` negativ, `Ln` ≤ 0, Überlauf bei `Trunc`/`Round`) |
| 216 | Zugriff über `nil` oder außerhalb des Speichers |
