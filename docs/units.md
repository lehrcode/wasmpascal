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
| `WideChar` | ein Unicode-Zeichen, `#0`..`#$FFFF` (siehe unten; aus Delphi) | 2 Byte |
| `ShortInt` | −128..127 | 1 Byte |
| `Byte` | 0..255 | 1 Byte |
| `SmallInt` | −32768..32767 (aus Delphi) | 2 Byte |
| `Integer` | −2147483648..2147483647 (**Abweichung:** wie `LongInt`, TP: 16 Bit) | 4 Byte |
| `LongInt` | −2147483648..2147483647 | 4 Byte |
| `Word` | 0..65535 | 2 Byte |
| `Real`, `Double`, `Extended`, `Comp` | Gleitkomma doppelter Genauigkeit (**Abweichung:** alle gleich) | 8 Byte |
| `Single` | Gleitkomma einfacher Genauigkeit | 4 Byte |
| `Pointer` | untypisierter Zeiger, `nil` | 4 Byte |
| `Text` | Textdatei | 528 Byte |

Dazu die Typkonstruktoren der Sprache: `string` und `string[n]` (immer
256 Byte, höchstens 255 Bytes Text), `array`, `record`, `set of`, `file`,
`file of T`, `^T`, Aufzählungen und Teilbereiche. Aufzählungen und
Teilbereiche belegen wie in TP 1, 2 oder 4 Byte, je nach Wertebereich.

Jede Zuweisung an eine Variable mit 1 oder 2 Byte kürzt den Wert auf diese
Breite, wie in TP: `Byte` 255 + 1 ergibt 0, `Word` 65535 + 1 ergibt 0,
`SmallInt` 32767 + 1 ergibt −32768. Gerechnet wird in 32 Bit.

Texte sind UTF-8: Ein Umlaut belegt zwei Bytes, `'ä'` ist deshalb ein
String und kein `Char`, und `Length('Größe')` ist 7.

#### WideChar

Ein `WideChar` ist ein einzelnes Unicode-Zeichen bis U+FFFF (Zeichen wie
Emojis darüber passen nicht hinein). Es ist ein Ordinaltyp wie `Char`:
`Ord`, `Succ`, `Pred`, `Inc`, `Dec`, `Low`, `High`, Vergleiche, `case`,
Array-Index und `for`-Schleifen gehen, Arithmetik nicht (`w + 1` ist ein
Fehler, `Ord(w) + 1` geht).

```pascal
var W: WideChar; S: string;
begin
  W := 'ä';                  { ein Literal mit einem Zeichen }
  W := #$20AC;               { '€' als Codepoint }
  if W = '€' then WriteLn('Euro: ', W, ' = ', Ord(W));   { Euro: € = 8364 }
  case W of
    'a'..'z': WriteLn('klein');
    'ä', 'ö', 'ü': WriteLn('Umlaut')
  end;
  S := 'Preis: ' + W;        { als String wird es UTF-8 }
  WriteLn(Length(S))         { 10: Length zählt Bytes }
end.
```

- **Konstanten:** Ein Literal mit genau einem Zeichen (`'ä'`, `'€'`) ist dort
  ein `WideChar`, wo eines erwartet wird; sonst bleibt es ein String.
  `#n` von 256 bis 65535 ist ein `WideChar`, `#n` bis 255 bleibt ein Byte
  (`#$E9` ist also **nicht** `'é'`). Zeichen ab U+10000, Surrogate und
  Noncharacters (`#$FFFE`, `#$FFFF`, `#$FDD0`..`#$FDEF`) ergeben als
  Konstante Error 76.
- **Umwandlungen:** Ein `Char` wird automatisch zum `WideChar` (über seinen
  Code, Bytes 128..255 werden also U+0080..U+00FF); umgekehrt nur mit
  `Char(w)`. Wo ein String erwartet wird (Zuweisung, `+`, Vergleich mit
  einem String, `Length`, `Pos`, `Assign`, …), wird ein `WideChar` als UTF-8
  eingesetzt; `Length(w)` liefert also 1 bis 3. `WideChar(n)` und `Word(w)`
  wandeln wie bei anderen Ordinaltypen.
- **Ausgabe und Eingabe:** `Write(w)` schreibt UTF-8, die Breite in `w:n`
  zählt Zeichen. `Read(F, w)` liest ein UTF-8-Zeichen; ungültige Bytes und
  Zeichen ab U+10000 ergeben `#$FFFD`, das Zeilenende `#10`, das Dateiende
  `#26`. `file of WideChar` speichert 2 Byte je Zeichen.
- **Vorsicht bei `s[i]`:** `s[i]` ist ein **Byte**. `w := s[1]` mit
  `s = 'ä'` ergibt `'Ã'` (das erste UTF-8-Byte). Die Zeichen eines Strings
  liefern `UTF8Next` und `for w in s do` (siehe Strings).

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
| `FileMode` | `Byte` | Zugriffsart für `Reset` typisierter und untypisierter Dateien: 0 nur lesen, 1 nur schreiben, 2 lesen und schreiben (Standard) |
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

Dateinamen sind relativ zum aktuellen Verzeichnis des Programms (siehe
`ChDir`) innerhalb einer Wurzel: in der CLI das Arbeitsverzeichnis, unter
dem Ziel `gojs` das Verzeichnis, das der Host bereitstellt (z. B.
`node web/node-run.js --dir verz`). `\` gilt als `/`, ein führendes `/`
meint die Wurzel, `.` und `..` werden aufgelöst; Laufwerksbuchstaben gibt
es nicht. Pfade, die die Wurzel verlassen, ergeben E/A-Fehler 5. Stellt
der Host unter `gojs` kein Dateisystem bereit (so in der Browser-Demo),
ergibt jeder Dateizugriff E/A-Fehler 5.

**Groß-/Kleinschreibung** in Datei- und Verzeichnisnamen bestimmt das
Dateisystem des Rechners, auf dem das Programm läuft (**Abweichung:** TP
kannte keinen Unterschied). Unter Windows und macOS (Standardeinstellung)
findet `Assign(F, 'DATEN.TXT')` auch `daten.txt`, unter Linux nicht (E/A-Fehler
2, bei `ChDir` 3). Wer Programme auf allen Systemen gleich laufen lassen will,
schreibt Dateinamen genau so, wie sie auf der Platte heißen. Units und
`{$I}`-Dateien findet der Compiler dagegen immer unabhängig von der
Schreibweise.

| Signatur | Beschreibung |
| --- | --- |
| `Assign(var F; Name: string)` | Verbindet die Dateivariable mit einem Dateinamen. `Assign(F, '')` steht für Standardein- bzw. -ausgabe. |
| `Reset(var F [; RecSize: Word])` | Öffnet eine vorhandene Datei: Textdateien zum Lesen, typisierte und untypisierte Dateien nach `FileMode` (Standard 2: lesen und schreiben; eine schreibgeschützte Datei braucht `FileMode := 0`). Lesen bzw. Schreiben gegen die Zugriffsart ergibt E/A-Fehler 5. `RecSize` nur bei untypisierten Dateien (Standard 128). |
| `Rewrite(var F [; RecSize: Word])` | Legt die Datei neu an bzw. leert sie und öffnet sie zum Schreiben; typisierte und untypisierte Dateien unabhängig von `FileMode` auch zum Lesen. |
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
| `ChDir(Pfad: string)` | Wechselt das aktuelle Verzeichnis des Programms. Fehlt das Verzeichnis oder läge es oberhalb der Wurzel: E/A-Fehler 3. |
| `MkDir(Pfad: string)` | Legt ein Verzeichnis an. |
| `RmDir(Pfad: string)` | Löscht ein leeres Verzeichnis. |
| `GetDir(Laufwerk: Byte; var S: string)` | Liefert das aktuelle Verzeichnis als `/pfad` (normalisiert, die Wurzel als `/`); `Laufwerk` wird ignoriert. |

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

Die Routinen oben zählen **Bytes**. Für **Zeichen** (Unicode-Codepoints)
gibt es diese vier; ein ungültiges Byte bzw. eine abgebrochene UTF-8-Folge
zählt dabei als ein Zeichen `#$FFFD`:

| Signatur | Beschreibung |
| --- | --- |
| `UTF8Length(const S: string): Integer` | Anzahl der Zeichen: `UTF8Length('Größe')` = 5. |
| `UTF8Next(const S: string; var Index: Integer): WideChar` | Zeichen ab Byte `Index`; setzt `Index` dahinter. Außerhalb von 1..`Length(S)` ergibt es `#0`, `Index` bleibt. |
| `UTF8Index(const S: string; CharIndex: Integer): Integer` | Byte-Index des `CharIndex`-ten Zeichens; hinter dem letzten `Length(S) + 1`, unter 1 ergibt es 1. Damit lassen sich `Delete` und `Insert` zeichenweise verwenden. |
| `UTF8Copy(const S: string; Index, Count: Integer): string` | Wie `Copy`, aber in Zeichen: `UTF8Copy('Grüße', 3, 2)` = `'üß'`. Zerschneidet nie ein Zeichen. |

```pascal
var S: string; I: Integer; W: WideChar;
begin
  S := 'Grüße';
  I := 1;
  while I <= Length(S) do
  begin
    W := UTF8Next(S, I);      { G, r, ü, ß, e }
    Write(Ord(W), ' ')
  end;
  WriteLn;
  for W in S do               { dasselbe kürzer: mit WideChar zeichenweise }
    Write(W, '.');            { G.r.ü.ß.e. }
  WriteLn
end.
```

Mit einer `Char`-Laufvariable läuft `for c in s` weiter über die Bytes.

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

Steuert den Bildschirm über ANSI-Sequenzen. Es gibt keinen Bildschirmspeicher,
und das Programm kennt die Cursorposition nicht: Die Ausgabe läuft als Text
durch, Umbruch und Rollen übernimmt das Terminal. Setzen lässt sich der
Cursor mit `GotoXY`.

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
| `GotoXY(X, Y: Byte)` | Setzt den Cursor auf Spalte `X`, Zeile `Y`; 1,1 ist links oben im sichtbaren Bereich des Terminals. Ist `X` oder `Y` kleiner als 1, bleibt der Cursor stehen. **Abweichung:** Ohne `Window` gilt immer der ganze Bildschirm, und eine Obergrenze (TP: 80×25) wird nicht geprüft, weil das Terminal größer sein kann. |
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

**Nicht vorhanden:** `WhereX`, `WhereY`, `Window`, `KeyPressed`,
`TextMode`, `AssignCrt`. Ihre Verwendung ergibt Error 3 (Unknown identifier).

## Dos

Datum und Uhrzeit, Dateiattribute und Dateizeit, Verzeichnissuche, Pfade und
Umgebungsvariablen. Fehler melden die Routinen in `DosError`, nicht über
`IOResult`.

### Konstanten, Typen, Variablen

| Name | Bedeutung |
| --- | --- |
| `ReadOnly = $01`, `Hidden = $02`, `SysFile = $04`, `VolumeID = $08`, `Directory = $10`, `Archive = $20`, `AnyFile = $3F` | Dateiattribute. **Abweichung:** Nur `Directory` und `Hidden` haben eine Wirkung (siehe unten). |
| `PathStr`, `DirStr`, `NameStr`, `ExtStr` | **Abweichung:** alle `string` mit 255 Bytes (TP: 79, 67, 8, 4), damit lange Namen ganz bleiben. |
| `DateTime = record Year, Month, Day, Hour, Min, Sec: Word end` | Zerlegte Zeit für `PackTime`/`UnpackTime`. |
| `SearchRec = record Fill; Attr: Byte; Time, Size: LongInt; Name: string end` | Ergebnis von `FindFirst`/`FindNext`; `Fill` ist intern. **Abweichung:** `Name` mit 255 Bytes (TP: 12). |
| `DosError: Integer` | Fehler der letzten Routine: 0 ok, 2 Datei fehlt, 3 Pfad fehlt, 6 Datei nicht geöffnet, 18 keine weiteren Dateien. |

### Datum und Uhrzeit

| Signatur | Beschreibung |
| --- | --- |
| `GetDate(var Year, Month, Day, DayOfWeek: Word)` | Aktuelles Datum; `DayOfWeek` 0 = Sonntag … 6 = Samstag. |
| `GetTime(var Hour, Minute, Second, Sec100: Word)` | Aktuelle Uhrzeit; `Sec100` sind Hundertstelsekunden. |
| `PackTime(var T: DateTime; var P: LongInt)` | Zeit in das gepackte DOS-Format (Sekunden nur gerade). |
| `UnpackTime(P: LongInt; var T: DateTime)` | gepackte DOS-Zeit zerlegen. |

**Ortszeit:** Datum, Uhrzeit und Dateizeiten gelten in Ortszeit. Den Versatz
zu UTC liest die Unit beim Programmstart aus der Umgebungsvariable
`WASMPASCAL_UTC_OFFSET` (Minuten östlich von UTC, z. B. `60` für MEZ);
`pascal run` und der Browser setzen sie selbst. Fehlt sie, gilt UTC.
**Abweichung:** Der Versatz bleibt für den ganzen Lauf gleich – ein Wechsel
der Sommerzeit während des Laufs und Dateizeiten aus der anderen
Jahreshälfte sind um eine Stunde verschoben.

### Dateien und Verzeichnisse

| Signatur | Beschreibung |
| --- | --- |
| `GetFAttr(var F; var Attr: Word)` | Attribute der Datei, die mit `Assign` an `F` hängt (offen oder geschlossen). Fehlt sie: `Attr` = 0, `DosError` = 2 bzw. 3. |
| `GetFTime(var F; var Time: LongInt)` | Änderungszeit der **geöffneten** Datei `F` als gepackte DOS-Zeit; nicht geöffnet: `DosError` = 6. |
| `FindFirst(Path: PathStr; Attr: Word; var S: SearchRec)` | Erster Eintrag, der auf das Muster im letzten Teil von `Path` passt (`*` und `?`, Groß-/Kleinschreibung egal; `*` und `*.*` passen auf alle Namen, auch ohne Punkt). Normale Dateien immer, Verzeichnisse nur mit `Directory` in `Attr`, versteckte nur mit `Hidden`. Kein Treffer: `DosError` = 18, Verzeichnis fehlt: 3. |
| `FindNext(var S: SearchRec)` | Nächster Eintrag; am Ende `DosError` = 18. |

```pascal
var S: SearchRec;
begin
  FindFirst('*.pas', 0, S);
  while DosError = 0 do
  begin
    WriteLn(S.Name, ' ', S.Size);
    FindNext(S)
  end
end.
```

- **Attribute (Abweichung):** Nur zwei haben eine Wirkung: `Directory` für
  Verzeichnisse und `Hidden` für Namen, die mit einem Punkt beginnen
  (Unix-Konvention). `ReadOnly`, `SysFile`, `Archive` und `VolumeID` werden
  nie gemeldet und beim Suchen ignoriert; eine normale Datei hat `Attr` = 0.
- **Suche (Abweichung):** `.` und `..` werden nie geliefert; die Reihenfolge
  bestimmt der Rechner (nicht sortiert). Den Suchzustand gibt `FindNext` am
  Ende frei; bricht ein Programm eine Suche vorher ab, bleibt er bis
  Programmende belegt (TP 4.0 kennt kein `FindClose`). `?` steht für ein
  Byte, ein Umlaut braucht also `??`.

### Pfade und Umgebung (aus TP 5.0)

| Signatur | Beschreibung |
| --- | --- |
| `FExpand(Path: PathStr): PathStr` | Vollständiger Pfad ab der Wurzel wie bei `GetDir`: `FExpand('..\b\c.pas')` in `/a` ergibt `/b/c.pas`. **Abweichung:** `/` als Trenner, kein Laufwerk, keine Großschreibung; `..` über die Wurzel hinaus bleibt in der Wurzel. |
| `FSplit(Path: PathStr; var Dir: DirStr; var Name: NameStr; var Ext: ExtStr)` | Zerlegt `Path`: `Dir` mit abschließendem Trenner (`\` oder `/`), `Ext` mit Punkt (ab dem letzten Punkt im Namen). |
| `FSearch(Path: PathStr; DirList: string): PathStr` | Sucht eine **Datei** zuerst im aktuellen Verzeichnis, dann in den Verzeichnissen aus `DirList` (getrennt mit `;`); liefert den gefundenen Pfad oder `''`. |
| `GetEnv(EnvVar: string): string` | Wert der Umgebungsvariable; zuerst der exakte Name, dann ohne Rücksicht auf Groß-/Kleinschreibung. Fehlt sie: `''`. |
| `EnvCount: Integer` | Anzahl der Umgebungsvariablen. |
| `EnvStr(Index: Integer): string` | `'NAME=WERT'` der `Index`-ten Variable (ab 1), außerhalb `''`. |
| `DosVersion: Word` | Immer 5: `Lo(DosVersion)` = 5, `Hi(DosVersion)` = 0. |

Woher die Umgebung kommt: `pascal run` gibt die Umgebung des Rechners weiter,
`runner.Options.Env` und die Option `env` des Loaders genau die angegebenen
Variablen. Im Browser ohne `env` enthält sie nur `WASMPASCAL_UTC_OFFSET`.
Unter `gojs` sind die Variablen nach Namen sortiert, und Argumente plus
Umgebung dürfen zusammen höchstens etwa 8 KB lang sein (Grenze von
`wasm_exec.js`).

Alle anderen Routinen der Unit Dos (z. B. `Exec`, `SetDate`, `SetFAttr`,
`DiskFree`) gibt es nicht.

## Laufzeitfehler

Ein Laufzeitfehler beendet das Programm mit einer Meldung wie
`Runtime error 200 at zahlen.pas(42) in Auswerten`; der Exit-Code ist die
Fehlernummer.

| Nummer | Bedeutung |
| --- | --- |
| 2 | Datei nicht gefunden |
| 3 | Pfad nicht gefunden |
| 4 | Zu viele offene Dateien |
| 5 | Zugriff verweigert (schreibgeschützt, gegen `FileMode`, Pfad außerhalb der Wurzel, Verzeichnis nicht leer oder schon vorhanden; unter `gojs` ohne Datei-Host jeder Dateizugriff) |
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
