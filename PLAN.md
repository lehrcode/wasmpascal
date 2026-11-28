# Projektplan WasmPascal: Turbo-Pascal-5.0-Compiler nach WASM in Go

Nov 24, 2026 · @Christian Winkler

## Ziel und Rahmenbedingungen

Wir bauen in Go **WasmPascal**, einen Compiler, der Turbo-Pascal-5.0-Quelltext in WebAssembly übersetzt (bis Etappe 15: Turbo Pascal 4.0). Modulpfad: `github.com/lehrcode/wasmpascal`; die CLI heißt `pascal`. Der Compiler läuft nativ und lässt sich selbst nach WASM kompilieren. Einen eigenen Interpreter gibt es nicht: `pascal run` kompiliert und führt das Modul sofort in wazero aus, im Browser übernimmt die Engine des Browsers.

**Sprachumfang:** Turbo Pascal 5.0, plus die unten genannten Erweiterungen aus TP 7 und Delphi. Das umfasst Units, Records (auch variante), Aufzählungen, Teilbereiche, Mengen, Zeiger, ShortStrings, typisierte und untypisierte Dateien, verschachtelte Prozeduren, `var`-Parameter, prozedurale Typen und Exit-Prozeduren (seit Etappe 16). Ziel ist weitgehende Quelltext-Kompatibilität, aber keine Binär- und keine Layout-Kompatibilität.

**Bewusste Vereinfachungen:**

- `goto` und `label` werden ab Etappe 8 unterstützt, über dieselbe Dispatch-Schleife wie der Scheduler (siehe dort). Nicht-lokale Sprünge aus einer Prozedur heraus erlaubt TP ohnehin nicht.
- Das Speicherlayout ist an WASM angepasst, nicht byte-genau wie unter DOS. `SizeOf` liefert die neuen Größen.
- Die Datentypen sind auf WASM abgebildet (siehe Abschnitt Typabbildung). Es gibt keine 16-Bit-Überlaufsemantik.
- Der Compiler bricht beim ersten Fehler ab, wie das Original. Es gibt keine Fehlerkorrektur im Parser.
- Das ganze Programm wird auf einmal kompiliert. Units werden immer aus dem Quelltext übersetzt, ohne `.TPU`-Dateien.
- Es gibt keine Optimierungen. Konstante Ausdrücke werden nur dort ausgewertet, wo die Sprache es verlangt. *Ausnahme ab Etappe 18:* Routinen, die das Programm nicht erreichen kann, kommen nicht ins Modul.
- Laufzeitfehler beenden das Programm mit `Runtime error NNN at datei.pas(zeile) in Prozedur`, also mit Quelltextposition statt Adresse (siehe Abschnitt Laufzeitfehler mit Zeilennummern).

**Weggelassen:** `Graph` (BGI), `Printer`, `Overlay`, `Turbo3`, `Graph3`, `inline(...)`, `external` ohne Modulname und `.OBJ`-Linking (`external 'modul'` gibt es seit Etappe 17), `interrupt`-Prozeduren, `absolute`, `Mem[]`, `Port[]`, `Intr`/`MsDos` und aus `Dos` alles außer Datum, Uhrzeit, `ParamStr`/`ParamCount` und (seit Etappe 15) den Routinen für Attribute, Dateizeit, Suche, Pfade und Umgebung; die in TP 5.0 neuen `GetCBreak`, `SetCBreak`, `GetVerify`, `SetVerify` und `SwapVectors` fehlen ebenfalls. `Sound`/`NoSound` werden zu Nicht-Operationen. Aus Crt entfallen WhereX, WhereY, Window und KeyPressed (GotoXY nach Rückfrage nachgereicht, über ANSI); ReadKey arbeitet nur im Zeilenmodus. Es gibt keinen Schatten-Bildschirm, kein Cursor-Tracking und kein Raw-Mode-Terminal. Die alte Windows-Konsole ohne ANSI-Unterstützung wird nicht unterstützt.

**Direktiven:** Unterstützt werden `{$I datei}`, `{$I-}`/`{$I+}` mit `IOResult` sowie `{$DEFINE}`, `{$UNDEF}`, `{$IFDEF}`, `{$IFNDEF}`, `{$IFOPT}`, `{$ELSE}` und `{$ENDIF}` (`{$UNDEF}` und `{$IFOPT}` seit Etappe 1, weil TP 4.0 sie kennt). Von {$M} wird nur die Stackgröße ausgewertet (siehe Größengrenzen). Alle anderen werden ignoriert, auch `{$A-}` (TP 5.0; Records und Arrays sind immer natürlich ausgerichtet), aber für `{$IFOPT}` mitgeführt. Boolesche Ausdrücke werden immer mit Kurzschlussauswertung berechnet (entspricht `{$B-}`).

**Reservierte Wörter:** die von TP 4.0, zusätzlich immer `object`, `constructor`, `destructor` und `inherited`. Sie sind für eine spätere OOP-Erweiterung im Stil von TP 5.5/6.0 vorgemerkt, haben in diesem Plan aber noch keine Bedeutung; ihre Verwendung als Bezeichner ist ein Compilerfehler. Alle anderen später eingeführten Schlüsselwörter (`asm`, `library`, `exports`) bleiben normale Bezeichner.

**Zeichensatz: durchgehend UTF-8, wie in Go.** Strings sind Byte-Folgen, die normalerweise UTF-8 enthalten. Konsole, Textdateien und Quelltext reichen die Bytes unverändert durch; es gibt keine Codepage-Umwandlung.

- Quelltext muss gültiges UTF-8 sein, sonst Compilerfehler mit Hinweis auf `iconv -f CP437 -t UTF-8`. Ein BOM am Anfang wird übersprungen.
- Bezeichner und Schlüsselwörter bestehen nur aus ASCII; in String-Literalen und Kommentaren ist beliebiges UTF-8 erlaubt.
- `string` bleibt der ShortString von TP: Längenbyte plus höchstens 255 **Bytes**. `Length` zählt Bytes (`Length('Größe')` = 7), `s[0]` bleibt das Längenbyte. Es gibt keine längeren oder dynamischen Strings.
- `Char` ist ein Byte. Ein Literal in Hochkommas, das mehr als ein Byte lang ist (`'ä'`), ist ein String und kein `Char`; wo ein `WideChar` erwartet wird, gilt ein Literal mit genau einem Zeichen bis U+FFFF als `WideChar` (seit Etappe 14).
- `WideChar` (seit Etappe 14) ist ein einzelnes Zeichen bis U+FFFF; als String eingesetzt wird es zu UTF-8. Die Hilfsfunktionen `UTF8Length`, `UTF8Next`, `UTF8Index` und `UTF8Copy` arbeiten zeichenweise, `for … in` mit `WideChar`-Laufvariable ebenso.
- In Literalen sind Steuerzeichen außer Tabulator und Noncharacters verboten (Error 907; Steuerzeichen über `#n`), Bidi-Steuerzeichen im ganzen Quelltext (Error 908).
- `Copy`, `Delete`, `Insert` und Indizierung arbeiten auf Bytes und können ein Mehrbyte-Zeichen zerschneiden.
- `UpCase` und die Groß-/Kleinschreibung von Bezeichnern betreffen nur a–z.
- `#nnn` bis 255 und `Chr(n)` über 127 erzeugen das rohe Byte; `#n` von 256 bis 65535 ist ein `WideChar` (ohne Surrogate und Noncharacters). Alte Programme, die Rahmen mit CP437-Nummern zeichnen (`#196`, `Chr(219)`), geben deshalb ungültiges UTF-8 aus; das ist eine bewusste Abweichung.
- Textdateien (`Text`) werden als UTF-8 gelesen und geschrieben, ohne Umwandlung; typisierte und untypisierte Dateien sind ohnehin Rohbytes.

**Erweiterungen aus späteren Versionen:** Diese Sprachmittel kommen über TP 5.0 hinaus dazu (bis Etappe 15 stand hier „über TP 4.0“; die Routinen der Unit `Dos` aus TP 5.0 gehören seit Etappe 16 zum Sprachumfang). Keines braucht ein neues reserviertes Wort; `Result`, `Break`, `Continue`, `Low` und `High` sind vordefinierte Bezeichner.

| Erweiterung | Herkunft | Regeln |
| --- | --- | --- |
| `Result` | Delphi | In jeder Funktion implizit deklarierte Variable vom Ergebnistyp. Gleichwertig zur Zuweisung an den Funktionsnamen; beides darf gemischt werden. Eine eigene Deklaration namens `Result` in einer Funktion ist ein Fehler. |
| Schreibgeschützte Konstanten | Delphi, `{$J-}` | Typisierte Konstanten sind immer schreibgeschützt: keine Zuweisung, keine Übergabe als `var`-Parameter, kein `@`. Einen Schalter `{$J+}` gibt es nicht. |
| `Break`, `Continue` | TP 7 | Nur innerhalb von `for`, `while`, `repeat` und `for … in`; sonst Compilerfehler. |
| `Low`, `High` | TP 7 | Für Ordinaltypen und deren Variablen, Arrays (Indexgrenzen) und Strings (`Low` = 0, `High` = deklarierte Maximallänge in Bytes). Immer zur Übersetzungszeit bekannt. |
| `const`-Parameter | TP 7 | Im Rumpf schreibgeschützt. Records, Arrays, Strings und Mengen werden per Adresse übergeben, ohne Kopie; Skalare per Wert. |
| Funktionsaufruf als Anweisung | TP 6 (`{$X+}`), Delphi | Immer erlaubt, ohne Schalter: Das Ergebnis eigener Funktionen wird verworfen. Wie bei `{$X+}` gilt das nicht für Standardfunktionen der System-Unit. |
| Strukturierte Funktionsergebnisse | Delphi 1.0 | Funktionen dürfen alle Typen außer Dateien zurückgeben, also auch Records, Arrays und Mengen. Der Ergebnistyp ist ein Typbezeichner. Felder und Elemente des Ergebnisses setzt man über `Result`; `F.x` im Rumpf ist ein rekursiver Aufruf. |
| `WideChar` | Delphi 2.0 | Ein Unicode-Zeichen bis U+FFFF, 2 Byte; Ordinal- und Zeichentyp ohne Arithmetik; als String eingesetzt UTF-8 (Etappe 14). |
| `SmallInt` | Delphi | 16 Bit mit Vorzeichen, 2 Byte; jede Zuweisung kürzt auf 16 Bit (Etappe 14). |
| `Int64` | Delphi 4 | 64 Bit mit Vorzeichen, 8 Byte; 64-Bit-Arithmetik, sobald ein Operand `Int64` ist (Etappe 17). |
| `external 'modul' [name 'x']` | BP 7, Delphi | Import einer Host-Funktion, nur einfache Typen an der Grenze, nur Ziel WASI (Etappe 17). |
| `for … in` | Delphi 2005 | Über Arrays (alle Elemente in Indexreihenfolge), Strings (Bytes als `Char`, mit `WideChar`-Laufvariable die UTF-8-Zeichen – eigene Erweiterung, Etappe 14) und Mengen (enthaltene Elemente aufsteigend). Die Schleifenvariable ist im Rumpf schreibgeschützt. Keine Enumeratoren, da es keine Klassen gibt. |

Bewusst nicht aufgenommen: offene Array-Parameter (`array of T`) und Standardwerte für Parameter.

**Abweichungen dadurch:**

- TP-4.0-Programme, die typisierte Konstanten verändern (oft als „statische lokale Variable“ benutzt), kompilieren nicht mehr und müssen auf globale Variablen umgestellt werden.
- Ein eigener Bezeichner `Result` innerhalb einer Funktion kollidiert mit dem impliziten; außerhalb von Funktionen bleibt er nutzbar. Eigene Deklarationen namens `Break`, `Continue`, `Low` oder `High` überdecken die eingebauten, wie in TP 7.

## Architektur und Repo-Struktur

Eine Pipeline mit einem einzigen Backend: Quelltext → Lexer → Parser → AST → Typprüfung → IR als Kontrollflussgraph → WASM-Codegenerator → eigener Binär-Encoder → `.wasm`. Die Laufzeitbibliothek ist zum größten Teil in Pascal geschrieben und wird wie eine normale Unit mitübersetzt.

**Zwei Ziel-ABIs mit gemeinsamer Laufzeit:**

- `wasi`: WASI Preview 1 (`fd_write`, `fd_read`, `poll_oneoff`, `clock_time_get`, `random_get`, `path_open` usw.). Läuft in wazero, wasmtime und Node.
- `gojs`: dieselbe Import- und Exportschnittstelle wie `GOOS=js GOARCH=wasm` (Importmodul `gojs`, Exporte `run`, `resume`, `getsp`, `mem`). Läuft mit `wasm_exec.js` im Browser. Die Go-Version, deren `wasm_exec.js` wir unterstützen, wird festgelegt und die Datei mitgeliefert (festgelegt: Go 1.27.4, `web/wasm_exec.js`).

Nur die unterste Schicht der Laufzeit (I/O, Zeit, Zufall, Anhalten/Fortsetzen) unterscheidet sich zwischen den beiden Zielen.

**Abhängigkeiten:** Das Paket `compiler` nutzt nur die Go-Standardbibliothek, damit es mit `GOOS=js` und `GOOS=wasip1` baut und sich ohne Ballast einbetten lässt. `runner` und `cmd/pascal` hängen zusätzlich nur von wazero ab. Ein Raw-Mode-Terminal und `golang.org/x/term` werden nicht gebraucht.

**Go-API:** Compiler und Runner sind Bibliotheken, die sich in andere Go-Programme einbetten lassen. `cmd/pascal` ist nur eine dünne Hülle und nutzt ausschließlich diese öffentliche API. Alles andere liegt unter `compiler/internal/`.

```go
package compiler // import "github.com/lehrcode/wasmpascal/compiler"

type Target int

const (
    TargetWASI Target = iota // WASI Preview 1
    TargetGoJS               // GOOS=js-kompatibel
)

type Options struct {
    Target       Target
    FS           fs.FS    // Quelltexte; im Browser z. B. ein fstest.MapFS
    IncludePaths []string // Suchpfade für Units und {$I}
    Defines      []string // wie {$DEFINE}
    StackSize    int      // 0 = {$M} oder 1 MB
    NoLines      bool     // ohne Zeilentabelle und Name Section
}

type Result struct {
    Wasm []byte
}

// Diagnostic ist der Fehlertyp für Compilerfehler (errors.As).
type Diagnostic struct {
    File      string
    Line, Col int
    Code      int    // TP-Fehlernummer
    Msg       string
}

func (d *Diagnostic) Error() string

func Compile(ctx context.Context, mainFile string, opts Options) (*Result, error)
func Check(ctx context.Context, mainFile string, opts Options) error
```

```go
package runner // import "github.com/lehrcode/wasmpascal/runner"

type Options struct {
    Args           []string
    Stdin          io.Reader         // zeilenweise; das Terminal bleibt im Zeilenmodus
    Stdout, Stderr io.Writer
    Dirs           map[string]string // Gastpfad → Hostverzeichnis für Dateizugriffe
}

// RuntimeError ist der Fehlertyp für Pascal-Laufzeitfehler (errors.As).
type RuntimeError struct {
    Code int    // TP-Fehlernummer, z. B. 200
    File string
    Line int
    Proc string
}

func (e *RuntimeError) Error() string

// Run führt ein mit TargetWASI erzeugtes Modul aus und liefert den Exit-Code (Halt).
func Run(ctx context.Context, wasm []byte, opts Options) (exitCode int, err error)
```

**Regeln für die Bibliothek:**

- Kein globaler, veränderlicher Zustand: Mehrere `Compile`- und `Run`-Aufrufe dürfen gleichzeitig laufen.
- Kein `os.Exit`, kein `log.Fatal`, kein Zugriff auf `os.Stdin`/`os.Stdout` außerhalb von `cmd/pascal`. Interne Panics werden abgefangen und als Fehler zurückgegeben.
- `ctx` wird respektiert: Abbruch beendet Kompilierung und laufendes Programm.
- Die Laufzeit-Units kommen per `go:embed` aus dem Root-Paket `rtl` (`rtl.FS`); wer `compiler` importiert, braucht keine weiteren Dateien.
- Jede exportierte Funktion und jeder Typ hat Doku-Kommentare und mindestens ein `Example`-Test.
- Bis Etappe 10 gilt die API als v0 und darf sich ändern; Änderungen werden im Etappen-Bericht genannt.

**Repo-Struktur:**

```
cmd/pascal/         CLI: pascal build | run | check; nur über die öffentliche API
cmd/pascaljs/       Compiler für den Browser (GOOS=js): JS-Funktion pascalCompile
compiler/           Öffentliche API: Compile, Check, Options, Result, Diagnostic
  internal/
    source/         Dateien, Positionen, {$I}-Includes, bedingte Kompilierung
    lexer/          Tokens (case-insensitive, Kommentare { } (* *), #13, ^M, $hex)
    ast/            AST-Knoten
    parser/         Recursive Descent, Abbruch beim ersten Fehler
    types/          Typsystem, Scopes, Symboltabellen, Units
    check/          Typprüfung, Auflösung der Standardprozeduren
    ir/             IR als CFG, Lowering vom getypten AST
    analysis/       Aufrufgraph, Anhaltefähigkeit, adressierte Variablen
    codegen/        IR → WASM-Funktionen (strukturiert oder Zustandsmaschine)
    wasm/           Binär-Encoder (Module, Sections, LEB128, Instruktionen)
    abi/            Ziele wasi und gojs (entfällt mit Etappe 17)
rtl/                Laufzeit in Pascal: system.pas, crt.pas, dos.pas; rtl.go exportiert rtl.FS (go:embed)
runner/             Öffentliche API: Run, Options; wazero-Host, Dateisystem
web/                Browser-Demo: index.html, Loader, xterm.js-Anbindung, Node-Werkzeuge
testdata/           Pascal-Testprogramme mit erwarteter Ausgabe (*.pas + *.out)
```

&#91;embedded content: Compiler-Pipeline · 8 Stufen, 2 Ziele\]

Die Laufzeit-Units aus `rtl/` laufen durch dieselbe Pipeline wie das Anwenderprogramm; erst die letzte Schicht der Laufzeit unterscheidet zwischen `wasi` und `gojs`.

## Typabbildung und Speichermodell

Alle skalaren Typen werden auf WASM-Werttypen abgebildet. Aggregate liegen im linearen Speicher mit natürlicher Ausrichtung.

| Turbo Pascal | In WASM-Werten | Im Speicher |
| --- | --- | --- |
| Byte, ShortInt, Char, Boolean | `i32` | 1 Byte; jede Zuweisung kürzt auf 8 Bit |
| SmallInt, Word, WideChar | `i32` | 2 Byte; jede Zuweisung kürzt auf 16 Bit (seit Etappe 14; `Word` vorher 4 Byte ohne Kürzung) |
| Integer | `i32` | 4 Byte, Bereich wie LongInt (`MaxInt` = 2147483647) |
| LongInt | `i32` | 4 Byte |
| Aufzählungen, Teilbereiche | `i32` | 1 Byte, wenn alle Werte in 0..255 bzw. −128..127 passen, 2 Byte in 0..65535 bzw. −32768..32767, sonst 4 Byte (wie TP) |
| Real, Double, Extended | `f64` | 8 Byte |
| Int64 | `i64` | 8 Byte (seit Etappe 17) |
| Comp | `f64` in Ausdrücken | 8 Byte als `i64`, beim Speichern gerundet (seit Etappe 17; vorher `f64`) |
| Single | `f32` | 4 Byte |
| Zeiger, `nil` | `i32` | 4 Byte |
| `string[n]` / `string` | Adresse (`i32`) | immer 256 Byte: Längenbyte + 255 Bytes UTF-8; beim Zuweisen auf `n` gekürzt |
| `set of T` | Adresse (`i32`) | immer 32 Byte (256 Bit); Operationen mit `v128` |
| Records, Arrays | Adresse (`i32`) | natürliche Ausrichtung; variante Teile überlagern sich wie eine Union |
| Prozedurale Typen | `i32` | 4 Byte: Index in die WASM-Funktionstabelle, 0 = `nil` (seit Etappe 16) |
| Dateivariablen | Adresse (`i32`) | Laufzeit-Record (Handle, Modus, Puffer) |

**Festlegungen zu Wertebereichen (nach Etappe 1):**

- `Integer` hat den Bereich von `LongInt`; `MaxInt` = `High(Integer)` = 2147483647. `Word` bleibt 0..65535; Konstanten außerhalb davon sind Compilerfehler. *Geändert in Etappe 14:* `Word` belegt wie in TP 2 Byte und wird bei jeder Zuweisung auf 16 Bit gekürzt (`65535 + 1` ergibt 0); vorher 4 Byte ohne Kürzung.
- `Byte`, `ShortInt` und `Char` werden bei jeder Zuweisung auf 8 Bit gekürzt (`SmallInt`, `Word` und `WideChar` seit Etappe 14 entsprechend auf 16 Bit) (`ShortInt` mit Vorzeichenerweiterung), egal ob die Variable in einem WASM-Local oder im Speicher liegt. `b := b + 1` ergibt bei `b = 255` also immer 0, wie in TP mit `{$R-}`. Teilbereiche mit 1 Byte Speicher werden ebenso gekürzt.
- Die Speichergröße eines Teilbereichs richtet sich nach dem Wertebereich, nicht nach der Anzahl der Werte: `1000..1100` belegt 2 Byte (bis Etappe 13: 4 Byte), `-1..40000` belegt 4 Byte.

**Speicheraufteilung (linearer Speicher):**

1. Ab Adresse 0 eine freie Seite, die nie belegt wird. WASM kennt keinen Speicherschutz; deshalb prüft jede Dereferenzierung `p^` den Zeiger und bricht bei `nil` mit `Runtime error 216` ab.
2. Typisierte Konstanten und Konstanten des Compilers (Strings, Mengen) im Datensegment, das im Modul steht; dahinter, auf 16 Byte ausgerichtet, die nicht initialisierten globalen Variablen (BSS). Das BSS steht nicht im Modul, weil frischer WASM-Speicher ohnehin 0 ist (seit Etappe 12).
3. Schatten-Stack, wächst nach unten. Er nimmt Records, Arrays, Strings und Mengen als lokale Variablen auf, außerdem alle Locals, deren Adresse gebraucht wird (`var`-Argumente, `@x`), sowie die Frames anhaltefähiger Prozeduren.
4. Heap für `New`/`Dispose`/`GetMem`/`FreeMem`/`Mark`/`Release`: ein einfacher Free-List-Allocator, wächst bei Bedarf mit `memory.grow`. `MemAvail`/`MaxAvail` liefern plausible Werte.

**Größengrenzen:** Die 64-KB-Grenze von TP 4.0 entfällt. Was dort mit „Structure too large“ abgelehnt wurde, ist hier erlaubt.

- Eine einzelne Struktur (Array, Record) darf höchstens 2 GB groß sein, damit Größen und Offsets als vorzeichenbehaftete `i32` sicher bleiben. Größer → Compilerfehler.
- Globale Variablen sind nur durch den linearen Speicher begrenzt (wasm32: höchstens 4 GB; Browser und Laufzeitumgebungen setzen oft niedrigere Grenzen).
- Der Schatten-Stack hat standardmäßig 1 MB. Einstellbar über die Option `-stack` oder `{$M stack,heapmin,heapmax}` im Hauptprogramm; ausgewertet wird nur der erste Wert. Ein Überlauf bricht mit `Runtime error 202` ab, wie in TP.
- Der Heap wächst bei Bedarf mit `memory.grow`; eine einzelne Anforderung per `New`/`GetMem` darf ebenfalls größer als 64 KB sein. Ist kein Speicher mehr zu bekommen: `Runtime error 203`. `MemAvail` und `MaxAvail` liefern `LongInt`-Werte auch jenseits von 640 KB.

**Adressfunktionen:** Das Segment-Modell von DOS gibt es nicht; die zugehörigen Funktionen werden so abgebildet:

| Funktion | Verhalten |
| --- | --- |
| `Addr(x)`, `@x` | lineare Adresse als Zeiger, wie gewohnt |
| `Ofs(x)` | lineare Adresse als `LongInt` (nicht `Word`) |
| `Seg(x)` | immer 0 |
| `Ptr(seg, ofs)` | Zeiger auf `ofs`; `seg` wird ignoriert |
| `CSeg`, `DSeg`, `SSeg`, `SPtr`, `PrefixSeg`, `FreePtr`, `FreeMin` | Compilerfehler „nicht unterstützt“ |

**Laufzeitfehler mit Zeilennummern:** Jeder Laufzeitfehler nennt Fehlernummer, Datei, Zeile und Prozedur, zum Beispiel `Runtime error 200 at zahlen.pas(42) in Auswerten`.

- **Geprüfte Fehler** (Division durch 0, I/O-Fehler bei `{$I+}`, Heap voll, Stack-Überlauf, `nil`-Dereferenzierung als Runtime error 216): Der Compiler übergibt Datei-Index, Zeile und Prozedur-Index als Konstanten an die Fehlerroutine der Laufzeit.
- **WASM-Traps** (Speicherzugriff außerhalb des gültigen Bereichs, `unreachable`): Der Compiler schreibt die standardisierte Name Section (Funktionsnamen) und eine eigene Custom Section `wasmpascal.lines`, die Byte-Offsets im Code auf Datei und Zeile abbildet. Der Go-Runner und der Browser-Loader übersetzen einen Trap damit in dieselbe Meldung.
- Die Option `-nolines` (API: `Options.NoLines`) lässt Zeilentabelle und Name Section weg; geprüfte Fehler nennen dann nur die Fehlernummer.
- In der Go-API kommt ein Laufzeitfehler als `*runner.RuntimeError` mit `Code`, `File`, `Line` und `Proc` zurück (per `errors.As`).

**Prozeduren:**

- Skalare Parameter und Locals, deren Adresse nie gebraucht wird, sind WASM-Locals.
- `var`-Parameter werden als Adresse (`i32`) übergeben.
- Wertparameter vom Typ Record, Array, String oder Menge kopiert der Aufgerufene in seinen eigenen Frame.
- Verschachtelte Prozeduren bekommen einen Static Link als versteckten ersten Parameter: die Frame-Adresse der umschließenden Prozedur. Locals einer Prozedur, auf die eine innere Prozedur zugreift, liegen deshalb immer im Frame auf dem Schatten-Stack.
- Funktionsergebnisse vom Typ String, Record, Array oder Menge werden über eine vom Aufrufer bereitgestellte Adresse zurückgegeben.

## Scheduler und anhaltefähige Prozeduren

Das Programm kann an festgelegten Punkten anhalten und die Kontrolle an den Host zurückgeben. Das ist für das `gojs`-Ziel zwingend (`run` muss zurückkehren, `resume` setzt fort) und hält im Browser die Seite bedienbar. Unter WASI wird derselbe Mechanismus verwendet, der Host setzt dort einfach sofort fort.

**Anhaltepunkte:** `ReadLn`/`Read` auf Text-Eingabe, `ReadKey`, `Delay`, `Halt` sowie `Write`/`WriteLn`, wenn seit dem letzten Anhalten mehr als 30 ms vergangen sind. Ausgaben werden gepuffert und bei jedem Anhalten geleert.

**Analyse (Paket `analysis`):**

1. Aufrufgraph über das ganze Programm aufbauen, einschließlich der Laufzeit-Units.
2. Eine Prozedur ist anhaltefähig, wenn sie direkt oder transitiv einen Anhaltepunkt erreicht.
3. Alle anderen Prozeduren werden normal und strukturiert übersetzt und kosten nichts extra.

**Codegenerierung für anhaltefähige Prozeduren:**

- Alle Locals liegen im Frame auf dem Schatten-Stack, nicht in WASM-Locals.
- Der Rumpf wird aus dem CFG als Dispatch-Schleife über Basisblöcke erzeugt (`loop` + `br_table` über eine Zustandsvariable).
- Nach jedem Aufruf einer anhaltefähigen Prozedur wird ein globales Flag `unwinding` geprüft. Ist es gesetzt, speichert die Prozedur ihren Wiederaufnahmezustand im Frame und kehrt sofort zurück.
- Beim Fortsetzen (`rewinding`) ruft der Einstiegspunkt die Kette erneut auf. Jede Prozedur lädt ihren Zustand aus dem Frame und springt per `br_table` direkt an den gespeicherten Punkt, bis der Anhaltepunkt selbst erreicht ist und das Ergebnis (zum Beispiel die gelesene Taste) übernimmt.
- Der Frame-Zeiger jeder Ebene muss beim Fortsetzen wiederhergestellt werden; dazu werden die Frames als verkettete Liste mit gespeichertem Zustand geführt.

**Host-Seite:**

- `wasi`: Der Einstiegspunkt `_start` läuft in einer Schleife, bis das Programm endet. Eingabe geht direkt und blockierend über `fd_read`; das Terminal bleibt im normalen Zeilenmodus.
- `gojs`: `run` kehrt beim Anhalten zurück. Für `Delay` wird `runtime.scheduleTimeoutEvent` genutzt. Eingaben kommen zeilenweise über einen per `syscall/js`-Importen registrierten Callback: Der Loader sammelt die Zeile in einem einfachen Zeileneditor (Echo, Backspace, Enter) und setzt das Programm danach über `resume` fort.

**`goto` und `label`:** Eine Prozedur mit `goto` wird ebenfalls als Dispatch-Schleife übersetzt. Jedes Label beginnt einen eigenen Basisblock, `goto` setzt die Zustandsvariable und springt an den Schleifenkopf. Damit funktionieren auch Sprünge aus verschachtelten Schleifen und nicht reduzierbarer Kontrollfluss.

- Ist die Prozedur nicht zugleich anhaltefähig, bleiben ihre Locals in WASM-Locals; nur der Kontrollfluss läuft über die Schleife.
- TP erlaubt `goto` nur innerhalb desselben Blocks, nicht-lokale Sprünge entfallen also.
- Sprünge in eine `for`-Schleife hinein werden wie im Original nicht geprüft; das Verhalten ist dann undefiniert.
- Aufwand: etwa 200–400 Zeilen, weil CFG und Dispatch-Codegenerierung ohnehin vorhanden sind.

## Etappen mit Akzeptanzkriterien

Zwanzig Etappen (0 bis 19). Jede endet mit grünen Tests und einem Commit; die nächste beginnt erst danach. Nach jeder Etappe kurz berichten und auf Freigabe warten.

### Etappe 0: Gerüst ✅

- Go-Modul `github.com/lehrcode/wasmpascal`, Verzeichnisstruktur, `cmd/pascal` mit leeren Unterbefehlen `build`, `run`, `check`. Die CLI nutzt nur die öffentliche API; deren Signaturen aus dem Abschnitt Go-API sind als Stubs angelegt.
- Test-Harness: für jede `testdata/**/*.pas` kompilieren, ausführen, stdout mit `*.out` vergleichen; `*.err` für erwartete Compilerfehler. Flag `-update` schreibt erwartete Ausgaben neu.
- **Fertig, wenn:** `go test ./...` und `go vet ./...` grün sind und `GOOS=wasip1 GOARCH=wasm go build ./compiler/...` sowie `GOOS=js GOARCH=wasm go build ./compiler/...` bauen.
- *Erledigt (Nov 24, 2026).* Entscheidungen: Testfall = `*.pas` mit `*.out` oder `*.err` (Units/Includes ohne beides werden übersprungen), optional `*.in` als Eingabe; stdout und stderr landen gemeinsam in `*.out`, Laufzeitfehler als eigene Zeile, Exit-Code ≠ 0 als `[Exit-Code N]`. `runner.Options.Args` ist wie `os.Args` aufgebaut (`Args[0]` = `ParamStr(0)`). Die CLI übergibt Hostdateien über ein `os.DirFS` auf die Laufwerkswurzel. Handkodierte Test-Module liegen in `internal/wasmtest`.

### Etappe 1: Lexer und Quelltext ✅

- Alle Tokens von TP 4.0 plus die reservierten Wörter object, constructor, destructor, inherited, Bezeichner case-insensitive, Kommentare `{ }` und `(* *)`, Zeichenkonstanten `#65`, `^M`, `$FF`, Strings mit `''`.
- Direktiven-Präprozessor (`{$I}`, `{$DEFINE}`, `{$IFDEF}` ...), Positionsangaben mit Datei, Zeile, Spalte. Prüfung auf gültiges UTF-8, BOM überspringen, Bezeichner nur ASCII.
- **Fertig, wenn:** Tabellengetriebene Lexer-Tests für alle Token-Arten bestehen.
- *Erledigt (Nov 24, 2026).* Pakete `source` (Positionen, Fehler, Loader mit DOS-artiger Dateisuche, Symbole und Bedingungsstapel) und `lexer` (Tokens, Direktiven, Includes). `^C` ist kontextabhängig: Der Parser schaltet mit `Lexer.SetTypeMode` an Typstellen auf Zeiger um. `{$I+/-}` steht als `Token.IOChecks` an jedem Token, `{$M}` über `Lexer.StackSize`. `compiler.Check`/`Compile` zerlegen bereits in Tokens und melden Lexerfehler; Golden-Tests dazu in `testdata/lexer/`. Einzelentscheidungen in `docs/abweichungen.md`.

### Etappe 2: Parser und AST ✅

- Vollständige Grammatik von TP 4.0: `program`, `unit` (interface/implementation/Initialisierung), `uses`, `const` (auch typisiert), `type`, `var`, Prozeduren/Funktionen inkl. `forward`, alle Anweisungen, `with`, `case` mit Bereichen und `else`.
- `goto`/`label` werden vollständig geparst. Bis Etappe 8 meldet die Typprüfung „goto noch nicht unterstützt“.
- Ein AST-Printer für Golden-Tests. Dazu die Syntax der Erweiterungen: const-Parameter und for-in.
- **Fertig, wenn:** Mindestens 30 Parser-Golden-Tests bestehen, darunter typische Programme aus TP-Lehrbüchern (selbst geschrieben).
- *Erledigt (Nov 24, 2026).* Paket `ast` mit S-Ausdruck-Printer (`ast.Sprint`), Paket `parser` (Recursive Descent, Abbruch per Panic/Recover beim ersten Fehler). 37 Golden-Tests in `compiler/internal/parser/testdata/` (`go test ./compiler/internal/parser -update`), dazu Tabellen für Fehler und Sonderfälle sowie Fehler-Golden-Tests in `testdata/parser/`. Namensauflösung (Recordfeld oder `Unit.Name`, Aufruf oder Typumwandlung) bleibt der Typprüfung überlassen. `{$I-}` steht als `IOChecks` an jeder Anweisung.

### Etappe 3: Typprüfung ✅

- Scopes, Symboltabellen, alle Typen aus der Typabbildung, Typverträglichkeit und Zuweisungskompatibilität, Konstantenauswertung, Typumwandlungen `T(x)`.
- Standardprozeduren und -funktionen als eingebaute Symbole: `Write`/`WriteLn`/`Read`/`ReadLn` mit Formatangaben, `Inc`, `Dec`, `Ord`, `Chr`, `Succ`, `Pred`, `Low`/`High` (Erweiterung aus TP 7), `SizeOf`, `Length`, `Copy`, `Concat`, `Pos`, `Delete`, `Insert`, `Str`, `Val`, `New`, `Dispose` usw.
- Erweiterungen: Result, schreibgeschützte Konstanten, const-Parameter, Low/High, Prüfung von Break/Continue. Units noch ohne Mehrdatei-Auflösung; die System-Unit ist zunächst ein Stub.
- **Fertig, wenn:** Fehler-Golden-Tests (`*.err`) für typische Typfehler und positive Tests für alle Konstrukte bestehen.
- *Erledigt (Nov 24, 2026).* Paket `types` (Typen mit Größen und Ausrichtung, Konstantenwerte, Symbole, Scopes) und `check` (Deklarationen, Konstantenauswertung, Ausdrücke, Anweisungen, 84 Standardroutinen, Erweiterungen). `check.Info` hält Typen, Konstanten, Symbolbezüge, Felder, Aufrufe und Scopes für die Codegenerierung. Tests: Symbol-Golden-Tests in `compiler/internal/check/testdata/` (`-update`), Tabellen mit 61 Ausdruckstypen und 107 Fehlerfällen, alle Parser-Lehrbuchprogramme ohne Units/goto, Fehler-Golden-Tests in `testdata/check/`. AST-Positionen von Ausdrücken zeigen jetzt auf den Ausdrucksanfang (`BinaryExpr.OpPos` für den Operator).
- *Nachtrag nach Rückfrage:* Funktionsaufrufe als Anweisung sind immer erlaubt; Funktionen dürfen wie in Delphi 1.0 Records, Arrays und Mengen zurückgeben (siehe Tabelle der Erweiterungen).

### Etappe 4: IR, Codegenerator, Encoder, erstes Programm ✅

- IR als CFG mit Basisblöcken; Lowering vom getypten AST.
- Strukturierte WASM-Erzeugung aus dem CFG für nicht anhaltefähige Prozeduren (Kontrollfluss ohne `goto` ist immer reduzierbar; Prozeduren mit goto folgen in Etappe 8).
- Eigener Binär-Encoder; Validierung der erzeugten Module in Tests über wazero.
- `pascal run` mit wazero, Ziel `wasi`; Ganzzahlen, Reals, Boolean, Char, `if`/`while`/`repeat`/`for`/`case`, Prozeduren, Funktionen, `WriteLn` für Zahlen und Strings (vorerst in Go-Hostfunktionen erlaubt, wird in Etappe 6 ersetzt).
- **Fertig, wenn:** Hello World, Fakultät, Fibonacci, Primzahlsieb und ein Programm mit `case` korrekte Ausgaben liefern (darunter Result, Schleifen mit Break und Continue sowie for-in über ein Array), sowohl über die CLI als auch über einen Example-Test, der compiler.Compile und runner.Run direkt aufruft.
- *Erledigt (Nov 24, 2026).* Pakete `wasm` (Encoder), `ir` (CFG mit Ausdrucksbäumen; Aufrufe nur auf Anweisungsebene; Variablen abstrakt, Ablage entscheidet die Codegenerierung), `codegen` (Locals oder Frame auf dem Schatten-Stack, Prolog/Epilog, strukturierter Kontrollfluss nach Ramsey „Beyond Relooper“ über Dominatorbaum). Übersetzbar: Ganzzahlen, Reals, Boolean, Char, Aufzählungen, Teilbereiche, Arrays (global, lokal, typisierte Konstanten, als Wert-, const- und var-Parameter sowie Funktionsergebnis), var-Parameter, alle Anweisungen außer `with`/`goto`, `Write`/`WriteLn` auf die Standardausgabe über vorläufige Hostfunktionen (Modul `wasmpascal`, Go-Runner). Noch nicht übersetzbare Sprachmittel ergeben Error 902 mit der Etappe. Tests: zehn Programme in `testdata/etappe4/` mit von Hand hergeleiteten Ausgaben, dieselben Programme mit allen Variablen im Frame (`TestForceFrame`), Example-Tests in `compiler` und `runner`, Encoder-Validierung über wazero.

### Etappe 5: Speicher und zusammengesetzte Typen ✅

- Schatten-Stack, Static Link für verschachtelte Prozeduren, `var`-Parameter, Records inkl. Varianten, Arrays (mehrdimensional, beliebige Ordinal-Indizes), ShortStrings, Mengen, Zeiger, Heap-Allocator, `with`.
- Laufzeitfehler: Division durch 0, `nil`-Dereferenzierung (Runtime error 216), Heap erschöpft, jeweils mit Datei, Zeile und Prozedur; dazu Name Section, Zeilentabelle und deren Auswertung im Runner (siehe Laufzeitfehler mit Zeilennummern).
- **Fertig, wenn:** Testprogramme für verkettete Listen, Binärbäume, Quicksort auf Record-Arrays, Mengenoperationen, for-in über Strings und Mengen, const-Parameter mit Records und rekursive innere Prozeduren bestehen, außerdem ein globales Array über 64 KB, ein Heap-Block über 64 KB, ein Test für Addr/Ofs/Seg/Ptr und ein Test, der mit kleinem {$M}-Stack absichtlich Runtime error 202 auslöst. Alle Fehlertests prüfen die vollständige Meldung mit Datei und Zeile, auch für einen provozierten WASM-Trap.
- *Erledigt (Nov 24, 2026).* Nach Rückfrage wird `rtl/system.pas` schon jetzt als implizite System-Unit mitübersetzt (Heap mit Free-List, String- und Mengenroutinen in Pascal; vorgezogen aus Etappe 7). Static Link als versteckter erster Parameter innerer Prozeduren, Zugriff über die Kette der Frames; Records mit Varianten, `with`, ShortStrings (256 Byte, Kürzung auf die deklarierte Länge), Mengen mit `v128`, Zeiger mit `nil`-Prüfung, `New`/`Dispose`/`GetMem`/`FreeMem`/`Mark`/`Release`, `Addr`/`@`/`Ofs`/`Seg`/`Ptr`, `FillChar`/`Move`. Laufzeitfehler 200/202/203/204/216 und WASM-Traps erscheinen als `Runtime error NNN at datei.pas(zeile) in Prozedur`; die Stelle kommt über exportierte Globale statt über Code-Offsets (siehe `docs/abweichungen.md`). Tests: 16 Programme in `testdata/etappe5/` mit von Hand hergeleiteten Ausgaben, alle zusätzlich mit erzwungener Frame-Ablage, sowie `-nolines`.

### Etappe 6: Laufzeitbibliothek in Pascal ✅

- `rtl/system.pas`: String-Routinen, `Str`/`Val`, Formatierung für `Write` wie TP (z. B. `WriteLn(1.5)` ergibt `  1.5000000000E+00 `, `x:8:2`), `Read`/`ReadLn` für Zahlen und Strings, `Sin`/`Cos`/`ArcTan`/`Exp`/`Ln` (Reihen bzw. Polynome), `Random`/`Randomize`, `UpCase`, Dateien (`Assign`, `Reset`, `Rewrite`, `Append`, `Close`, `Eof`, `Eoln`, `Seek`, `FilePos`, `FileSize`, `BlockRead`/`BlockWrite`, `Erase`, `Rename`), `IOResult`, `ParamStr`/`ParamCount`, `Halt`.
- Nur wenige Intrinsics bleiben in Go/WASM: Speicher, `Sqrt`, `Trunc`/`Round`, WASI-Aufrufe.
- **Fertig, wenn:** Alle Hilfsfunktionen aus Etappe 4 durch Pascal-Code ersetzt sind und ein Testprogramm Textdateien schreibt, liest und anhängt.
- *Erledigt (Nov 24, 2026).* Die vorläufigen Go-Hostfunktionen sind entfernt; `Write`/`Read`, Formatierung, `Str`/`Val`, Text-, typisierte und untypisierte Dateien, `IOResult`/`{$I-}`, Verzeichnisse, Mathematik, `Random`, `ParamStr` und `Halt` sind Pascal in `rtl/system.pas`. In Go/WASM bleiben Speicher (`memory.grow`, `memory.copy`), `Sqrt`/`Trunc`/`Round`/`Int` und die WASI-Aufrufe als Intrinsics (`compiler/internal/abi`). Tests: 12 Programme in `testdata/etappe6/` mit von Hand hergeleiteten Ausgaben (Abnahme: `textdatei.pas`), alle auch mit erzwungener Frame-Ablage; jeder Testfall bekommt ein eigenes Verzeichnis.

### Etappe 7: Units ✅

- Mehrdatei-Programme: `uses` sucht `name.pas` im Programmverzeichnis und in `-I`-Pfaden; Abhängigkeitsreihenfolge, zyklische `uses` im Interface als Fehler, Initialisierungsteile in richtiger Reihenfolge.
- `System` implizit, `Crt` und `Dos` aus `rtl/` per `go:embed`.
- **Fertig, wenn:** Ein Programm mit drei eigenen Units und gegenseitigen `uses` im Implementation-Teil läuft.
- *Erledigt (Nov 24, 2026).* Der Compiler sucht und parst alle erreichbaren Units, der Checker prüft erst die Interfaces in Abhängigkeitsreihenfolge (Zyklus: Error 68), dann die Implementation-Teile; Scopes importieren die Interfaces der benutzten Units. Die Initialisierungen sind eigene Funktionen, aufgerufen in Abhängigkeitsreihenfolge (Tiefensuche über die `uses`-Listen). `rtl/crt.pas` (ANSI-Teil; `Delay`/`ReadKey` in Etappe 8) und `rtl/dos.pas` (`GetDate`/`GetTime`) sind eingebettet. Abnahme: `testdata/etappe7/dreiunits/haupt.pas` (Protokoll → Werkzeug → Zaehler → Protokoll); dazu Verdecken/Qualifizieren, Fehlerfälle 3/15/68/69/84, Crt, Dos, `-I`-Pfade.

### Etappe 8: Scheduler und `Crt` ✅

- Analyse der Anhaltefähigkeit, Zustandsmaschinen-Codegenerierung, Unwind/Rewind wie im Scheduler-Abschnitt. Dieselbe Dispatch-Schleife übersetzt auch Prozeduren mit goto/label.
- `rtl/crt.pas` über ANSI: `ClrScr`, `ClrEol`, `TextColor`/`TextBackground` (CGA-Farbtabelle → ANSI, 8–15 als helle Farben, +16 = Blinken), `TextAttr`, `HighVideo`/`LowVideo`/`NormVideo`, `InsLine`/`DelLine`, `Delay` und `ReadKey` im Zeilenmodus: `ReadKey` wartet, bis eine Zeile mit Enter abgeschlossen ist, liefert deren erstes Zeichen und verwirft den Rest samt Enter (eine leere Zeile ergibt `#13`; nach Rückfrage geändert, damit ein folgendes `ReadLn` eine neue Zeile liest); Sondertasten gibt es nicht. `GotoXY` setzt den Cursor mit `ESC[y;xH` (nach Rückfrage ergänzt). Nicht enthalten sind `WhereX`, `WhereY`, `Window` und `KeyPressed`; ihre Verwendung ergibt den normalen Fehler für unbekannte Bezeichner.
- Kein Schatten-Bildschirm und kein Cursor-Tracking: Die Ausgabe geht direkt als Text und ANSI-Sequenzen ans Terminal, Umbruch und Scrollen übernimmt das Terminal. Alle Bytes gehen unverändert durch (UTF-8, siehe Zeichensatz).
- Nativer Runner: Das Terminal bleibt im normalen Zeilenmodus, Echo und Zeilenbearbeitung übernimmt das Terminal. Die CLI fängt Strg+C mit `os/signal` ab und gibt `ESC[0m` aus, damit keine Farbe hängen bleibt. Unterstützt werden Terminals mit ANSI-Unterstützung (Linux, macOS, Windows Terminal); die alte Windows-Konsole ohne VT-Modus wird nicht unterstützt.
- **Fertig, wenn:** Ein kleines Zahlenraten `zahlen.pas` (selbst geschrieben: farbige Ausgabe, `ClrScr`, Eingaben mit `ReadLn`, eine kurze Animation mit `Delay`, „Weiter mit Enter“ über `ReadKey`) im Terminal korrekt läuft. Außerdem besteht ein Testprogramm mit goto (Sprung aus verschachtelten Schleifen und zurück nach oben).
- *Erledigt (Nov 24, 2026).* `analysis` markiert anhaltefähige Funktionen über den Aufrufgraph der IR (Anhaltepunkt: Intrinsic `_Suspend`). Anhaltefähige Funktionen und Funktionen mit `goto` werden als Dispatch-Schleife (`loop` + `br_table`) übersetzt, gesplittet vor jedem Anhaltepunkt und jedem Aufruf einer anhaltefähigen Funktion; Abwickeln sichert das Segment im Frame, Fortsetzen springt dorthin zurück. Der WASI-Treiber `_start` wickelt bei jedem Anhalten wirklich ab und setzt fort. `rtl/crt.pas` hat jetzt `Delay` und `ReadKey`; die CLI fängt Strg+C ab. Tests: `testdata/etappe8/` mit `zahlen.pas` (Abnahme, reproduzierbar mit `-fest`), Anhalten tief in einer Rekursion mit inneren Prozeduren, Zeitscheiben und `goto` samt irreduziblem Sprung; alle Programme auch mit erzwungener Frame-Ablage.

### Etappe 9: Ziel `gojs` ✅

- Importe aus Modul `gojs` mit Go-Stack-Layout (`sp`), Exporte `run`, `resume`, `getsp`, `mem`; Ausgabe über `runtime.wasmWrite`, Zeit, Timer, Zufall, Exit; Eingabe-Callback (mit einfachem Zeileneditor im Loader: Echo, Backspace, Enter) über `syscall/js`-Importe.
- `wasm_exec.js` der festgelegten Go-Version wird mitgeliefert. Ein kleiner Zusatz zum Loader übersetzt WASM-Traps über die Zeilentabelle in die Pascal-Fehlermeldung.
- **Fertig, wenn:** Das Zahlenraten unverändert mit `pascal build -target gojs` im Browser mit `wasm_exec.js` läuft.
- *Erledigt (Nov 24, 2026).* `rtl/system.pas` hat eine unterste Schicht (`SysWrite`, `SysRead`, `SysOpen`, …, `_Clock`, `SysArgs`, `_SuspendAction`) je Ziel per `{$IFDEF GOJS}`; alles darüber ist gemeinsam. Unter gojs: Ausgabe über `runtime.wasmWrite`, Uhren über `nanotime1`/`walltime`, `Delay` und Zeitscheiben über `scheduleTimeoutEvent`, `Halt` über `wasmExit` (jetzt unter beiden Zielen ein Anhaltepunkt), Eingabe zeilenweise über eine mit `_makeFuncWrapper` erzeugte Callback-Funktion und `go.wasmPascalReadLine`, gelesen in der neuen Routine `_GoResumeAction` vor dem Fortsetzen; `memory.grow` meldet `resetMemoryDataView`. Der Codegenerator erzeugt die Exporte `run`, `resume`, `getsp`, `mem`. `web/`: `wasm_exec.js` (Go 1.27.4), `loader.js` (Browser und Node, Traps → Pascal-Meldung über `__rt_*` und `wasmpascal.lines`, mehrere Programme gleichzeitig, Abbruch über `AbortSignal`), `run.html` (xterm.js 5.5.0 mit Zeileneditor), `node-run.js`. Tests: `TestProgramsGoJS` führt alle Testprogramme ohne Dateizugriff (74) mit Node aus und vergleicht mit denselben `.out`-Dateien; Abnahme im Browser mit headless Chromium (`zahlen.pas`, Laufzeitfehler). Nach Rückfrage bettet das Modul nur noch Dateinamen ohne Verzeichnis ein (keine Hostpfade im `.wasm`).

### Etappe 10: Compiler als WASM und Browser-Demo ✅

- `compiler` mit `GOOS=js GOARCH=wasm` bauen, eine JS-Funktion `pascalCompile(sources) → wasm | error` exportieren.
- `web/`: Editor-Textfeld, xterm.js-Terminal, Kompilieren und Ausführen im Browser.
- **Fertig, wenn:** Die Demo-Seite lokal über einen statischen Server das Zahlenraten kompiliert und startet.
- *Erledigt (Nov 24, 2026).* `cmd/pascaljs` (nur `GOOS=js GOARCH=wasm`) legt `pascalCompile(sources, options)` an: Quelltexte als Objekt Dateiname → String oder `Uint8Array` (oder ein einzelner String), Optionen `main`, `defines`, `stack`; Ergebnis `{wasm}` (Ziel gojs) oder `{error, file, line, column, code}`. `web/index.html`: Editor, Beispiele (`web/beispiele/`), xterm.js-Terminal, Strg+Enter, Sprung zur Fehlerstelle, Stopp; das Terminal mit Zeileneditor liegt jetzt in `web/terminal.js` (auch für `run.html`). `web/node-compile.js` übersetzt mit demselben WASM-Compiler in Node. Tests: `TestWebCompiler` baut den Browser-Compiler, übersetzt alle Testprogramme damit in Node und verlangt byte-gleiche Module und dieselben Compilermeldungen wie der native Compiler; `TestWebExamples` übersetzt die Beispiele. Abnahme mit headless Chromium: Zahlenraten auf der Seite übersetzt (etwa 0,2 s) und durchgespielt, Compilerfehler mit markierter Fehlerstelle.

### Etappe 11: Dateien unter `gojs` ✅

**Idee:** Unter `gojs` benutzt die RTL für echte Dateien (Handle ≥ 3)
dieselben WASI-Aufrufe wie unter WASI. Konsole, Eingabe, Uhr, Argumente und
Anhaltepunkte bleiben auf dem gojs-Weg. Das Modul importiert dann neben
`gojs` auch eine kleine Teilmenge von `wasi_snapshot_preview1`, die
`web/loader.js` bereitstellt. Alle Dateiaufrufe sind synchron; die Eingabe
bleibt asynchron über `go.wasmPascalReadLine`. Der Compiler muss dafür nicht
geändert werden: `abi.Intrinsics` enthält WASI- und gojs-Intrinsics für
beide Ziele, und Importe entstehen erst, wenn ein Intrinsic aufgerufen wird
(`codegen.go`, `hostIndex`).

Zum Umfang gehören ausdrücklich auch `ChDir`, `MkDir`, `RmDir` und `GetDir`:
`MkDir`/`RmDir` laufen über `path_create_directory`/`path_remove_directory`
zum Host, `ChDir` prüft das Ziel über `path_open` mit `O_DIRECTORY`, und
`GetDir` liefert das in der RTL geführte aktuelle Verzeichnis.

**Abgrenzung zu Go (`GOOS=js`, `syscall/fs_js.go`):** Go ruft die
Callback-Variante der Node-API über `globalThis.fs` auf und blockiert dabei
die Goroutine; `Chdir`/`Getwd` gehen an `process.chdir`/`process.cwd`, und
es gibt keine Sandbox. Wir übernehmen von Go die Node-API als Vorbild für den
Host-Vertrag, weichen aber bewusst ab:

- **Synchron statt Callbacks:** Bei uns müsste sonst jede Dateioperation ein
  Anhaltepunkt sein, und jede Routine mit `Read(f)`/`Write(f)` würde zur
  Dispatch-Schleife. Folge: Hosts im Browser müssen synchron sein
  (Dateisystem im Speicher, OPFS nur in einem Worker).
- **Aktuelles Verzeichnis in der RTL** (`CurDir`) statt `process.chdir`: Es
  gehört wie in TP zum Programm, mehrere Programme dürfen gleichzeitig
  laufen, und im Browser gibt es kein `process`. Unter WASI ist es schon so.
- **Wurzel als Sandbox** statt freiem Zugriff auf das Host-Dateisystem.

#### RTL (`rtl/system.pas`)

- Die unterste Schicht neu aufteilen:
  - **Gemeinsam** (aus `{$IFNDEF GOJS}` herausziehen): `SysOpen`, `SysClose`
    (Handles ≤ 2 nicht schließen), `SysSeek`, `SysSize`, `SysSetSize`,
    `SysUnlink`, `SysRename`, `SysMkDir`, `SysRmDir`.
  - **Je Ziel** bleiben nur: `SysWrite`, `SysRead`, `InputPending`,
    `MemGrown`, `_Clock`, `SysArgs`, `_SuspendAction` (gojs zusätzlich
    `_GoResumeAction` und die Hilfsroutinen).
- gojs-`SysWrite`: Handle 1 und 2 über `_GoWasmWrite` wie bisher, alle
  anderen über `_FdWrite`. Handle 0 bleibt `EBADF`.
- gojs-`SysRead`: Handle 0 aus dem Zeilenpuffer wie bisher, alle anderen über
  `_FdRead`. `PeekChar` hält nur bei Handle 0 an; daran ändert sich nichts.
- **Pfade normalisieren (beide Ziele):** `_Path` löst `.` und `..` auf und
  fasst doppelte `/` zusammen, bevor ein Pfad an `Sys*` geht; `ChDir` speichert
  nur normalisierte Pfade in `CurDir`. Behebt einen Fehler, der heute schon
  unter WASI auftritt: Nach `MkDir('a'); ChDir('a'); ChDir('..')` liefert
  `GetDir` `/a/..` statt `/`. Ein `..` oberhalb der Wurzel ergibt E/A-Fehler 3
  bei `ChDir` bzw. 5 bei den übrigen Routinen (`// TP-ANNAHME:`, unter DOS
  bleibt man in der Wurzel; wir melden den Fehler, statt still zu kürzen).
  Der Loader prüft trotzdem selbst auf Ausbruch aus der Wurzel.
- **`FileMode` auswerten (beide Ziele):** `Reset` öffnet typisierte und
  untypisierte Dateien wie in TP je nach `FileMode and 3`: 0 = nur lesen,
  1 = nur schreiben, 2 = lesen und schreiben (Standard), 3 wie 2
  (`// TP-ANNAHME:`; die Sharing-Bits ab `$10` werden ignoriert). Die RTL
  übergibt die passenden WASI-Rechte (`fd_read` 0x2, `fd_write` 0x40) und
  merkt sich die Zugriffsart in der Dateivariablen. Lesen aus einer nur zum
  Schreiben bzw. Schreiben in eine nur zum Lesen geöffnete Datei ergibt
  E/A-Fehler 5, geprüft in der RTL vor dem Host-Aufruf, damit beide Ziele
  gleich reagieren (`// TP-ANNAHME:` DOS meldet „Zugriff verweigert“).
  `Rewrite` und `Append` sowie Textdateien beachten `FileMode` nicht (wie TP).
  Eine schreibgeschützte Datei lässt sich damit mit `FileMode := 0` lesen;
  mit dem Standardwert 2 scheitert `Reset` wie in TP mit Fehler 5.
- Die Kopfkommentare der System-Unit und von `compiler/internal/abi`
  anpassen („Nur die unterste Schicht unterscheidet sich“ gilt weiter, aber
  gojs nutzt jetzt auch WASI-Importe für Dateien).
- Danach importiert ein gojs-Modul genau diese WASI-Funktionen:
  `fd_read`, `fd_write`, `fd_close`, `fd_seek`, `fd_filestat_get`,
  `fd_filestat_set_size`, `path_open`, `path_unlink_file`, `path_rename`,
  `path_create_directory`, `path_remove_directory`. Nicht dazu gehören
  `args_*`, `clock_time_get`, `proc_exit`, `poll_oneoff`.

#### Loader (`web/loader.js`)

- `go.importObject.wasi_snapshot_preview1` **immer** belegen, sonst schlägt
  `WebAssembly.instantiate` fehl. Ohne Host liefert jede Funktion
  `ENOTCAPABLE` (76) – das ist das heutige Verhalten (E/A-Fehler 5).
- Neue Option `files` für `WasmPascal.run`: ein **synchroner Host mit einer
  Teilmenge der Node-API `fs`**. So kann `node-run.js` direkt Nodes `fs`
  benutzen, und ein Playground kann sein virtuelles Dateisystem um dieselben
  Methoden ergänzen. Vertrag (Pfade sind relativ zur Wurzel, mit `/`):

  | Methode | Verwendung |
  | --- | --- |
  | `openSync(path, flags)` → fd | `flags` als String: `"r"`, `"r+"`, `"w"`, `"w+"` |
  | `readSync(fd, buf, offset, length, position)` → Anzahl | `position` immer angegeben (Loader führt die Position) |
  | `writeSync(fd, buf, offset, length, position)` → Anzahl | dito |
  | `closeSync(fd)` | |
  | `fstatSync(fd)`, `statSync(path)` → `{ size, isDirectory() }` | Größe, `ChDir`, Dateityp |
  | `ftruncateSync(fd, length)` | `Truncate` |
  | `unlinkSync(path)`, `renameSync(from, to)` | `Erase`, `Rename` |
  | `mkdirSync(path)`, `rmdirSync(path)` | `MkDir`, `RmDir`; darf `EPERM` werfen |

  Fehler wirft der Host als Exception mit Node-Code in `err.code`. Der
  Loader übersetzt in WASI-Fehlernummern: ENOENT→44, EACCES→2, EPERM→63,
  EBADF→8, EEXIST→20, EISDIR→31, ENOTDIR→54, ENOTEMPTY→55, EMFILE→33,
  ENOSPC→51, alles andere → 29 (EIO). Die RTL (`IoError`) macht daraus die
  TP-Nummern 2, 3, 4, 5 bzw. den Standardwert.
- Umsetzung im Loader:
  - Handle 3 ist die vorgeöffnete Wurzel (`RootFd` in der RTL); weitere
    Handles verwaltet der Loader selbst (eigene Tabelle Handle →
    `{hostFd, pos, append, write, dir}`), damit die Nummern nicht vom Host
    abhängen.
  - `path_open`: Pfad aus dem Speicher (UTF-8) lesen und normalisieren.
    Pfade, die die Wurzel verlassen (`..`), ergeben `ENOTCAPABLE`, wie unter
    wazero. Abbildung: `O_DIRECTORY` (2) → `statSync` + Verzeichnis-Handle,
    sonst `ENOTDIR`; Schreibrecht = Bit `fd_write` (0x40) in
    `rights_base`; `O_CREAT|O_TRUNC` (9) → `"w"` bzw. `"w+"`; Schreiben ohne
    `O_CREAT` → `"r+"`; nur lesen → `"r"`. `FDFLAGS_APPEND` (1) setzt die
    Position vor jedem Schreiben ans Ende. Kein Rückfall auf `"r"`, wenn
    `"r+"` scheitert: Schreibgeschützte Dateien öffnet das Programm über
    `FileMode := 0` (siehe RTL). Nur schreiben ohne `O_CREAT`
    (`FileMode = 1`) lässt sich mit den String-Flags nicht ausdrücken und
    wird als `"r+"` geöffnet; die RTL verhindert das Lesen
    (`// TP-ANNAHME:` eine Datei ohne Leserecht scheitert dann unter gojs
    mit Fehler 5, unter WASI nicht).
  - Die Handle-Tabelle gehört zu einem Programmlauf (`WasmPascal.run`),
    nicht zum Loader; mehrere Programme dürfen gleichzeitig laufen.
  - `fd_read`/`fd_write`: iovec-Liste (je 8 Byte: Adresse, Länge), Anzahl
    an die Ergebnisadresse.
  - `fd_seek`: `whence` 0/1/2 (SET/CUR/END, END über `fstatSync`), neue
    Position als u64 an die Ergebnisadresse.
  - `fd_filestat_get`: 64 Byte, Dateityp (u8) bei Offset 16 (3 =
    Verzeichnis, 4 = Datei), Größe (u64) bei Offset 32, Rest 0.
  - **Fallstricke:** i64-Parameter (`fd_seek` offset, `rights_*`,
    `fd_filestat_set_size`) kommen in JS als `BigInt` (JavaScript-Typ, kein
    Pascal-Typ; in Pascal bleibt alles `LongInt`). Den Speicher bei
    jedem Aufruf neu über `instance.exports.mem.buffer` ansprechen, weil
    `memory.grow` den Puffer austauscht. Binärdaten nie über Strings
    leiten.
- Kopfkommentar von `loader.js` und `web/README.md` um die Option `files`
  ergänzen; der Vertrag oben gilt als feste Schnittstelle.

#### Node-Werkzeuge und Tests

- `web/node-run.js`: neue Option `--dir verz`. Dann bekommt der Loader als
  `files` einen Host aus Nodes `fs`, der Pfade unter `verz` auflöst (und
  nichts außerhalb). Ohne `--dir` kein Dateizugriff wie bisher.
- `TestProgramsGoJS` (`harness_gojs_test.go`): Den Ausschluss über
  `usesFiles` entfernen. Jeder Testfall bekommt wie unter WASI ein eigenes
  Verzeichnis und läuft mit `--dir`. Die Programme mit Dateizugriff (u. a.
  `testdata/etappe6/textdatei.pas`) müssen dieselben `.out` liefern wie
  unter WASI.
- `TestGoJSWithoutFiles` bleibt: ohne `--dir` scheitern `Rewrite` und
  `MkDir` weiter mit 5.
- Neue Testprogramme (unter beiden Zielen):
  - Pfad außerhalb der Wurzel (`'..\geheim.txt'`) → `IOResult` 5.
  - `Append` auf eine fehlende Datei → 2.
  - `file of Byte` mit allen Werten 0..255 schreiben und wieder lesen
    (Binärdaten bleiben unverändert).
  - `Seek`/`FilePos`/`FileSize`/`Truncate` auf einer typisierten Datei.
  - `Erase` und `Rename` einer geschlossenen Datei. (Nicht bei geöffneter
    Datei: In TP undefiniert, und das Ergebnis hängt vom Betriebssystem des
    Hosts ab – Windows verweigert das Löschen offener Dateien.)
  - Verzeichnisse: `MkDir`, `ChDir` hinein und mit `'..'` zurück, `GetDir`
    jeweils normalisiert (`/a`, dann `/`); `ChDir` auf ein fehlendes
    Verzeichnis → 3; `ChDir('..')` in der Wurzel → 3; `RmDir` auf ein nicht
    leeres Verzeichnis → 5, auf ein fehlendes → 2; `MkDir` auf ein
    vorhandenes → 5; Datei in einem Unterverzeichnis über einen relativen Pfad
    nach `ChDir` anlegen und lesen.
  - `FileMode`: schreibgeschützte typisierte Datei (Schreibschutz setzt der
    Go-Test vorher auf dem Host) mit `FileMode := 0` lesen, mit dem
    Standardwert 2 → 5; mit `FileMode := 0` geöffnet schreiben → 5, mit
    `FileMode := 1` lesen → 5. Unter Windows läuft der Teil mit Schreibschutz
    nicht (`t.Skip`).
- Browser-Demo (`index.html`, `run.html`) bleibt ohne Dateien; keine
  Änderung nötig.

#### Doku

- `docs/abweichungen.md`: Zeilen für Etappe 11 (Dateien unter gojs über
  WASI-Importe, Host-Vertrag, Verhalten ohne Host, Abgrenzung zu Go,
  `FileMode`, normalisierte Pfade, `..` oberhalb der Wurzel); die Zeile
  „Dateien unter gojs“ aus Etappe 9 und den Teil „`FileMode` wird nicht
  ausgewertet“ der Zeile „Datei-Records“ aus Etappe 6 als überholt
  kennzeichnen.
- `docs/units.md`: Dateiroutinen und Verzeichnisse unter gojs, `FileMode`
  (Variable und `Reset`); `CLAUDE.md`: Abschnitt „Ziele“.

**Fertig, wenn:** Alle Testprogramme mit Dateizugriff, auch
`etappe6/verzeichnis.pas` und die neuen Verzeichnis- und `FileMode`-Tests,
laufen unter gojs mit Node und `--dir` mit denselben Ausgaben wie unter WASI;
ohne Host verhalten sich gojs-Module wie bisher; `TestWebCompiler` bleibt
grün (byte-gleiche Module).
- *Erledigt (Nov 25, 2026).* Die Dateiroutinen der untersten Schicht (`SysOpen` … `SysRmDir`, dazu `WasiWrite`/`WasiRead`) sind gemeinsam; gojs-`SysWrite`/`SysRead` leiten Handles ≥ 3 an WASI weiter. `_Path` ist durch `NormPath`/`HostPath` ersetzt (`.`/`..` aufgelöst, `..` oberhalb der Wurzel: `ChDir` 3, sonst 5). `Reset` beachtet `FileMode`; die Zugriffsart steht im Feld `Access` von `TFileRec` (vorher `Reserved`), `CanRead`/`CanWrite` prüfen sie vor jedem Lesen bzw. Schreiben. `web/loader.js` stellt die elf WASI-Funktionen bereit (Handle-Tabelle je Lauf, Option `files`), `node-run.js --dir` den Host über Nodes `fs`. Tests: `testdata/etappe11/` (`verzeichnisse`, `dateien`, `filemode`, auch mit erzwungener Frame-Ablage), `TestReadOnlyFile` (schreibgeschützte Datei unter beiden Zielen); `TestProgramsGoJS` läuft jetzt ohne Ausschluss über alle 84 Testprogramme.

### Etappe 12: Härtung für fremde Quelltexte ✅

**Ziel:** `compiler.Compile` darf mit beliebigem Quelltext im Prozess eines
Servers laufen: Er endet immer mit Ergebnis oder `*Diagnostic`, in
begrenzter Zeit und mit begrenztem Speicher, und bringt den Prozess nie zum
Absturz.

#### Gefundene Schwachstellen

Gemessen am 25. 11. 2026 mit `pascal build` bzw. `check` und einem CPU-Profil:

| Stelle | Problem | Messung |
| --- | --- | --- |
| `compiler/internal/lexer/lexer.go` (`posAt`) | Die Spalte jedes Tokens wird ab Zeilenanfang neu gezählt (`utf8.RuneCount`): quadratischer Aufwand in der **Zeilenlänge**, ganz ohne Schachtelung. Die Tiefengrenze hilft dagegen nicht. | 400 KB flacher Code in einer Zeile: 19 s (auf viele Zeilen verteilt: 0,1 s); mit 16 MB in einer Zeile wären es Stunden |
| `compiler/internal/ir/lower.go` (`alloc`, `declare`) | Globale Variablen liegen im Datensegment und werden **Byte für Byte** angehängt; jede darf bis `MaxStructSize` (2 GB) groß sein, die Summe ist unbegrenzt. Das Modul enthält alle Nullbytes. | `var A, B, C: array[1..2000000000] of Byte` → `fatal error: out of memory` (nicht abfangbar, Prozess beendet); ein 10-MB-Array ergibt ein 10-MB-Modul |
| `compiler/internal/parser/parser.go` | Recursive Descent ohne Tiefengrenze. Linkslastige Ketten (`a+b+c+…`), `else if`-Ketten, verschachtelte Klammern, Typen und Prozeduren erzeugen beliebig tiefe ASTs, über die auch `check`, `ir` und `codegen` rekursiv laufen. Bei genügend Tiefe läuft der Go-Stack über (`fatal error`). | 20 000 Klammern 0,46 s (10 000: 0,14 s); 20 000 `else if` 34 s |
| `compiler/internal/codegen/structure.go` (`push`) | Kopiert den Kontext je Ebene: quadratischer Aufwand in der Schachtelungstiefe. | Teil der 34 s bei `else if` |
| `compiler/internal/lexer/lexer.go` (`include`) | *Erledigt (vorgezogen, siehe unten).* | |
| alle Phasen | `ctx` wird nur zwischen den Phasen und je Unit geprüft; eine laufende Phase lässt sich nicht abbrechen. | |

#### Grenzen

Alle Grenzen kommen in `compiler.Options.Limits` (Typ `compiler.Limits`, mit
Doku-Kommentar und `Example`). Ein Feld mit 0 bedeutet die Voreinstellung,
−1 schaltet die Grenze ab. Die Voreinstellungen lassen alle bisherigen
Testprogramme zu; die CLI bekommt keine neuen Flags (bei Bedarf später).

| Feld | Voreinstellung | Fehler |
| --- | --- | --- |
| `MaxDepth` – Schachtelung von Ausdrücken, Anweisungen, Typen, Prozeduren | 500 | 904 „Nesting too deep“ |
| `MaxSourceBytes` – alle gelesenen Quelltextbytes inkl. Units und Includes, jede Einbindung erneut (ohne eingebettete RTL) | 16 MB | 905 „Source too large“ (*vorhanden*) |
| `MaxIncludes` – Anzahl `{$I}` insgesamt | 1000 | 905 (*vorhanden*) |
| `MaxTokens` – Tokens aller Dateien einer Übersetzung (statt `MaxNodes`: ein Zähler an einer Stelle, begrenzt die AST-Größe ebenso) | 2 000 000 | 906 „Program too large“ |
| `MaxStaticData` – Summe aller globalen Variablen und typisierten Konstanten | 256 MB | 49 „Data segment too large“ (`// TP-ANNAHME:` Nummer wie TP 7) |
| `MaxStackSize` – `{$M}` bzw. `Options.StackSize` | 256 MB | 905er-Reihe oder passende TP-Nummer |

Fehlernummern ab 904 in `compiler/internal/source/error.go` und in
`docs/abweichungen.md` eintragen.

#### Änderungen in dieser Reihenfolge

Geordnet nach Nutzen und Aufwand: Die Punkte 1 bis 4 sind klein und beheben
alle gemessenen Abstürze und das quadratische Verhalten; die übrigen bringen
Einstellbarkeit und Komfort für den Einsatz im Server.

1. **Spalte im Lexer schrittweise mitführen** (Nutzen sehr hoch, Aufwand
   sehr gering): Je Datei die zuletzt berechnete Stelle (Offset, Spalte)
   merken und nur ab dort weiterzählen; bei einem Zeilenwechsel neu
   beginnen. Positionen und Meldungen bleiben byte-gleich.
2. **`MaxStaticData` in der Typprüfung** (hoch, gering): Größen der globalen
   Variablen und typisierten Konstanten je Übersetzung aufsummieren und an
   der Deklaration, die die Grenze überschreitet, Error 49 melden – bevor
   `ir` Speicher belegt. `alloc` legt den Platz auf einmal an (kein Anhängen
   in einer Schleife).
3. **Tiefengrenze im Parser** (hoch, gering bis mittel): ein Zähler mit
   `enter()`/`leave()` in den rekursiven Produktionen (Ausdruck, Term,
   Faktor, Anweisung, Typ, Prozedurblock). In den Schleifen für zweistellige
   Operatoren zählt jeder weitere Operator eine Ebene, weil der Baum dort
   tiefer wird. Damit sind auch die späteren Phasen begrenzt. `structure.go`
   dabei auf einen Stapel mit `append`/Kürzen umstellen (kein Kopieren je
   Ebene).
4. **`ErrInternal` und `MaxTokens`** (mittel, sehr gering): Das `recover`
   in `compile` bleibt; interne Fehler bekommen ein Kennzeichen
   (`errors.Is(err, compiler.ErrInternal)`), damit ein Server sie von
   Quelltextfehlern unterscheiden und protokollieren kann. Die Token-Grenze
   zählt wie die Include-Grenzen im `source.Loader` über alle Dateien.
5. **`compiler.Limits`** (mittel, gering bis mittel): die Grenzen oben als
   API; Weitergabe an Loader, Lexer, Parser und `check`; die festen
   Konstanten aus `source` (`MaxIncludes`, `MaxSourceBytes`) ablösen.
6. **Datensegment ohne Nullbytes (BSS)** (mittel, mittel bis hoch):
   Nicht initialisierte globale Variablen in einen eigenen Bereich hinter
   dem Datensegment legen: `ir.Module` bekommt `BSSSize`, `alloc` für
   Variablen vergibt nur Offsets, `codegen` setzt den Schatten-Stack hinter
   Daten + BSS. Ins Modul kommen nur die initialisierten Bytes.
   Speicherlayout in `PLAN.md` (Typabbildung und Speichermodell) und
   `CLAUDE.md` nachziehen.
7. **Abbruch über `ctx`, minimal** (gering, sobald 1 bis 5 greifen; volle
   Durchreichung wäre aufwendig): `ctx` über die Config-Strukturen
   durchreichen (kein globaler Zustand) und prüfen im Lexer alle 4096
   Tokens (deckt Lexer und Parser ab) und in `check` je Routine. Weitere
   Phasen nur, wenn ihre Laufzeit trotz der Grenzen messbar ins Gewicht
   fällt. Ergebnis ist dann `ctx.Err()`, kein `Diagnostic`.
8. **Fuzzing, zeitlich begrenzt** (hoch fürs Aufdecken, Befunde nach oben
   offen): `FuzzCompile` im Paket `compiler` mit den Testprogrammen aus
   `testdata/` als Startkorpus, jeder Aufruf mit Timeout-`ctx` und den
   Voreinstellungen. Ein interner Fehler (`ErrInternal`) oder ein Hängen
   gilt als Befund. In der Etappe werden nur Abstürze und Hänger behoben;
   alle übrigen Befunde kommen als Liste in den Bericht. In
   `testdata/README.md` beschreiben, wie man es laufen lässt
   (`go test ./compiler -fuzz=FuzzCompile -fuzztime=10m`).
9. **Doku:** Im Paketkommentar von `compiler` einen Abschnitt „Einbetten in
   Server“: was garantiert ist (Grenzen, Abbruch, keine Goroutinen, kein
   Dateizugriff außer `Options.FS`) und was nicht (keine harte
   Speicherobergrenze des Prozesses – für fremde Eingaben in gemeinsam
   genutzten Servern zusätzlich einen eigenen Prozess empfehlen).

*Schon erledigt – Quelltextmenge begrenzen (vorgezogen, Nov 25, 2026):*
`source.Loader` zählt je Übersetzung jede Einbindung und ihre Bytes erneut
(`MaxIncludes` = 1000, `MaxSourceBytes` = 16 MB, eingebettete Laufzeit
ausgenommen) → Error 905 „Source too large“. Zusätzlich erkennt der Lexer
Include-Zyklen sofort (Error 9 „circular include of …“). Pfade in `{$I}`
bleiben wie in TP erlaubt (eine Beschränkung auf Dateinamen hilft gegen die
Vervielfachung nicht und bräche TP-Quelltexte).

#### Tests

`compiler/limits_test.go`, jeweils mit `ctx`-Timeout von 2 s und Prüfung des
Speicherverbrauchs (`runtime.MemStats` vorher/nachher, z. B. < 64 MB):

- 400 KB flacher Code in einer Zeile wird so schnell übersetzt wie auf viele
  Zeilen verteilt (Punkt 1); Spaltenangaben in Meldungen unverändert.
- `var a, b, c: array[1..2000000000] of Byte;` → 49, ohne großen
  Speicherverbrauch (Punkt 2).
- 100 000 verschachtelte Klammern, `a+a+…+a` mit 100 000 Operanden,
  100 000 `else if`, 10 000 verschachtelte `begin`, tief verschachtelte
  Typen (`array … of array …`, Records) und Prozeduren → 904 (Punkt 3).
- Ein Quelltext mit mehr als `MaxTokens` Tokens → 906; ein künstlich
  ausgelöster interner Fehler → `ErrInternal` (Punkt 4).
- Jede Grenze lässt sich über `Limits` ändern und mit −1 abschalten
  (Punkt 5).
- Ein Programm mit 1 MB globaler Variablen: Modul bleibt klein (Punkt 6).
- Abbruch: großer, gültiger Quelltext mit sofort abgelaufenem `ctx` →
  `context.Canceled` bzw. `DeadlineExceeded` (Punkt 7).
- Alle bisherigen Tests unverändert grün, `TestWebCompiler` byte-gleich.

**Fertig, wenn:** Alle Tests oben liefern innerhalb von 1 s ein
`*Diagnostic` bzw. `ctx.Err()`, ohne nennenswerten Speicher zu belegen;
`FuzzCompile` läuft 10 Minuten ohne Absturz oder Hänger; alle bisherigen
Tests und `TestWebCompiler` sind unverändert grün.
- *Erledigt (Nov 25, 2026).* Punkte 1 bis 9 umgesetzt: Spalten im Lexer schrittweise (400 KB in einer Zeile: 19 s → 0,08 s); `MaxStaticData` in `check` (Error 49, ohne Speicher zu belegen); Tiefe im Parser (`ParseDepth`, Error 904); `ErrInternal`; `MaxTokens` im `source.Loader` (Error 906); `compiler.Limits` (0 = Voreinstellung, −1 = aus; `{$M}` über `MaxStackSize`: Error 17); BSS (`ir.Module.BSSBase`/`BSSSize`, Adressen am Ende von `Lower` nachgetragen); `ctx` im Lexer alle 4096 Tokens und in `check` je Routine; `FuzzCompile` 10 Minuten (8,75 Mio. Aufrufe) ohne Befund. **Abweichung vom Plan:** `structure.go` kopiert den Kontext weiter je Ebene – bei höchstens 500 Ebenen gemessen unter 0,03 s, der Umbau hätte jede Funktion des Strukturierers berührt. Tests: `compiler/limits_test.go` (lange Zeile, statische Daten, zwölf Schachtelungsfälle, knapp unter der Grenze, Tokens, `ErrInternal`, alle Grenzen einstellbar, BSS mit Laufzeitprobe, Abbruch über `ctx`), `TestCheckCanceled`, `ExampleLimits`.

### Etappe 13: Compiler-Server ✅

**Ziel:** `pascal serve` übersetzt Quelltexte aus fremder Hand im
Dauerbetrieb über JSON-Zeilen auf stdin/stdout. Der Compiler läuft nativ im
Prozess (`compiler.Compile`); Schutz geben die Grenzen aus Etappe 12.

**Entscheidungen (nach Rückfrage):** Dauerbetrieb mit JSON-Zeilen; der
Compiler läuft **nativ im Prozess**, nicht als `wasip1`-Modul in wazero.
Abgewogen wurde (Prototyp vom 25. 11. 2026): Als WASM-Sandbox gäbe es eine
harte Speichergrenze je Anfrage, Abbruch an jeder Stelle und eingegrenzte
Abstürze, aber etwa 0,4 s je Anfrage, ein eingechecktes Modul von 4,7 MB
und einen `go generate`-Schritt nach jeder Compiler-Änderung. Nativ dauert
eine Anfrage Millisekunden, dafür gilt:

- **Speicher:** Die Grenzen (`Limits`) halten den Verbrauch einer
  Übersetzung klein; eine harte Grenze gibt es nicht. `-memory` setzt
  `debug.SetMemoryLimit` (weiches Ziel der Speicherbereinigung für den
  ganzen Prozess).
- **Zeit:** `-timeout` gibt jeder Anfrage einen `ctx`; er wirkt dort, wo
  der Compiler ihn prüft (zwischen den Phasen, im Lexer alle 4096 Tokens,
  in der Typprüfung je Routine). Mit den Grenzen sind die übrigen Phasen
  kurz.
- **Abstürze:** Panics kommen als `ErrInternal` zurück, der Server läuft
  weiter. Ein `fatal error` von Go (Stacküberlauf, Speichermangel) beendet
  dagegen den Prozess; die Grenzen aus Etappe 12 sollen genau das
  verhindern (Extremfälle und Fuzzing ohne Befund). Für den Betrieb mit
  fremden Quelltexten wird ein Container mit Speichergrenze und
  automatischem Neustart empfohlen (Doku).

#### Aufbau

- **`internal/compilereq`** (nur Standardbibliothek und `compiler`): Typen
  `Request`/`Response`, `Decode` (JSON, unbekannte Felder sind ein Fehler,
  damit Tippfehler in `limits` auffallen) und `Handle(ctx, Request)
  Response`. Quelltexte kommen ausschließlich aus `files` (`fstest.MapFS`);
  Dateinamen werden mit `fs.ValidPath` geprüft (kein `/` am Anfang, kein
  `..`), `IncludePaths` sind leer. `cmd/pascaljs` kann das Paket später
  übernehmen (nicht Teil dieser Etappe).
- **`pascal serve`** in `cmd/pascal`:
  - liest stdin zeilenweise, je Zeile eine Anfrage, und schreibt je Anfrage
    genau eine Antwortzeile nach stdout, in derselben Reihenfolge;
    Anfragen werden nacheinander bearbeitet;
  - Optionen: `-timeout` (je Anfrage, Standard 10 s), `-memory` (MB, setzt
    `debug.SetMemoryLimit`, Standard 0 = nicht setzen), `-maxrequest` (MB,
    Standard 64; eine längere Zeile wird verworfen und als kaputte Anfrage
    beantwortet);
  - endet mit Exit-Code 0 bei Ende von stdin; ungleich 0 nur, wenn der Start
    scheitert (ungültige Optionen) oder stdout nicht beschreibbar ist.
    Fehler einzelner Anfragen beenden den Server nie;
  - liest keine Hostdateien und nutzt keine Umgebungsvariablen.

#### Protokoll

Anfrage, eine Zeile JSON (wie `pascalCompile`):

```json
{"id": 7, "files": {"name.pas": "…"}, "main": "name.pas", "target": "gojs",
 "defines": [], "stack": 0, "limits": {"maxDepth": 500, "maxStaticData": -1}}
```

- `files` Pflicht, `main` Pflicht bei mehr als einer Datei; `target`
  `"wasi"` (Standard, wie `pascal build`) oder `"gojs"`; `limits` mit den
  Feldern von `compiler.Limits` in camelCase (0 = Voreinstellung, −1 = aus).
- `id` ist optional und beliebiges JSON; die Antwort enthält es unverändert.

Antwort, eine Zeile JSON:

- Erfolg: `{"id": 7, "wasm": "<base64>"}`
- Quelltextfehler: `{"id": 7, "kind": "source", "error": "name.pas(3,9): Error 85: \";\" expected", "file": "name.pas", "line": 3, "column": 9, "code": 85}`
- kaputte Anfrage (kein JSON, unbekanntes Feld, Zeile zu lang, ungültiger
  Name, …): `{"id": 7, "kind": "request", "error": "…"}`
- interner Fehler (`compiler.ErrInternal`): `{"id": 7, "kind": "internal", "error": "…"}`
- Zeitüberschreitung: `{"id": 7, "kind": "timeout", "error": "…"}`

Die Exit-Code-Regel aus der Anforderung (0 auch bei Quelltextfehlern,
ungleich 0 nur bei internem Fehler oder kaputter Anfrage) gilt im
Dauerbetrieb je Anfrage und steht deshalb in `kind`; der Prozess selbst
endet nur am Ende der Eingabe.

#### Tests

- `internal/compilereq`: gültige Anfrage (Modul byte-gleich zu
  `compiler.Compile`, als base64 in JSON), Quelltextfehler mit allen
  Feldern, kaputte Anfragen (kein JSON, unbekanntes Feld, fehlende `files`,
  fehlendes `main` bei mehreren Dateien, ungültige Namen wie `../x.pas` oder
  `/x.pas`, unbekanntes `target`), `limits` kommt im Compiler an, `id` kommt
  zurück, Zeitüberschreitung → `timeout`, Panic → `internal` (über einen
  Test-Haken im Paket, nicht über die Anfrage).
- `pascal serve` (über `run()` wie `TestCLI`): mehrere Zeilen in einer
  Eingabe, darunter Erfolg, Quelltextfehler, kaputte Zeile, zu lange Zeile,
  Zeitüberschreitung (`-timeout` sehr kurz, großer Quelltext), danach jeweils
  eine gültige Anfrage → Erfolg; Antworten in Reihenfolge, Exit-Code 0 am
  Ende.
- Keine Hostdateien: `{$I /etc/passwd}` und `{$I ../geheim.inc}` ergeben
  Quelltextfehler, auch wenn die Dateien existieren.

#### Doku

- `README.md`, Abschnitt „In eigene Projekte einbinden“: neuer
  Unterabschnitt „Compiler-Server“ mit einem kurzen Beispiel – eine Anfrage
  per Shell (`echo '{…}' | pascal serve`) und ein Go-Ausschnitt, der
  `pascal serve` als Kindprozess startet und über `json.Encoder`/
  `json.Decoder` Anfragen schickt. Beide Beispiele werden beim Schreiben
  ausgeführt. Dazu die Garantien und Grenzen aus den Entscheidungen oben
  (Container empfohlen).
- Hilfetext von `pascal` (`usage`) und `pascal serve -h`.
- `CLAUDE.md`: `pascal serve` und `internal/compilereq` kurz erwähnen.

**Fertig, wenn:** `pascal serve` beantwortet eine Folge von Anfragen
einschließlich Quelltextfehler, kaputter Anfrage und Zeitüberschreitung
jeweils mit genau einer Antwortzeile und läuft danach weiter; erzeugte
Module sind byte-gleich zu `compiler.Compile`; kein Zugriff auf
Hostdateien; alle bisherigen Tests sind grün.
- *Erledigt (Nov 25, 2026).* `internal/compilereq` (`Request`, `Response`, `Decode`, `Handle`, `RequestError`; unbekannte Felder und ungültige Namen sind kaputte Anfragen, Fehler ohne Diagnostic aus `compiler.Compile` ebenso) und `pascal serve` (`cmd/pascal/serve.go`: `-timeout`, `-memory`, `-maxrequest`, Leerzeilen werden übersprungen, Strg+C ohne ANSI-Ausgabe). Tests: `internal/compilereq` (byte-gleiche Module für wasi und gojs, Quelltextfehler, neun kaputte Anfragen, Grenzen, keine Hostdateien, Zeitlimit, interner Fehler und Panic, `Example`), `cmd/pascal/serve_test.go` (Folge aus sieben Anfragen samt kaputter und zu langer Zeile, Zeitlimit, Optionen). README mit Shell- und Go-Beispiel, beide ausgeführt.

### Etappe 14: `WideChar`, 16-Bit-Ganzzahlen und UTF-8-Hilfsfunktionen ✅

**Ziel:** Einzelne Unicode-Zeichen lassen sich als Wert behandeln, und
UTF-8-Strings lassen sich zeichenweise statt byteweise verarbeiten.
`string` bleibt UTF-8 mit 255 Bytes, `Char` bleibt ein Byte. Dazu kommen
16-Bit-Ganzzahlen wie in TP: `SmallInt` neu, `Word` künftig mit 2 Byte.

**Entscheidungen (nach Rückfrage):**

- Zeichen außerhalb der Basic Multilingual Plane (U+10000 und höher, z. B.
  Emojis) passen wie in Delphi nicht in ein `WideChar`.
- `SmallInt` (16 Bit mit Vorzeichen) kommt neu hinzu. `Word` belegt
  künftig wie in TP 2 Byte, und jede Zuweisung kürzt auf 16 Bit
  (`65535 + 1` ergibt 0). Das löst die Festlegung aus Etappe 1 („Word
  4 Byte, kein 16-Bit-Überlauf“) ab. Teilbereiche und Aufzählungen belegen
  wie in TP 1, 2 oder 4 Byte. `Integer` bleibt 32 Bit.
- Im Quelltext werden Steuerzeichen (U+0000–U+001F außer Tabulator) in
  String- und Zeichenliteralen verboten, wie JSON sie in Strings nur als
  Escape erlaubt; sie bleiben über `#n` erreichbar. Bidi-Steuerzeichen
  werden im ganzen Quelltext verboten (Schutz gegen „Trojan Source“).
- Noncharacters (U+FDD0–U+FDEF und die jeweils letzten zwei Codepoints
  jeder Ebene, U+FFFE/U+FFFF bis U+10FFFE/U+10FFFF) und einzelne Surrogate
  (U+D800–U+DFFF) sind als Zeichenkonstanten verboten: weder direkt im
  Literal noch über `#n`.

#### 16-Bit-Ganzzahlen und Speichergrößen

| Typ | Wertebereich | Im Speicher | Zuweisung |
| --- | --- | --- | --- |
| `SmallInt` (neu, aus Delphi) | −32768..32767 | 2 Byte, Ausrichtung 2 | kürzt auf 16 Bit mit Vorzeichen |
| `Word` (geändert) | 0..65535 | 2 Byte, Ausrichtung 2 (bisher 4) | kürzt auf 16 Bit (bisher keine Kürzung) |
| Teilbereiche | wie bisher | 1 Byte, wenn alle Werte in 0..255 bzw. −128..127 passen; **2 Byte**, wenn in 0..65535 bzw. −32768..32767; sonst 4 | auf die Speichergröße gekürzt |
| Aufzählungen | wie bisher | 1 Byte bis 256 Werte, **2 Byte** bis 65536, sonst 4 | |

- Gerechnet wird weiter in 32 Bit (`i32`); gekürzt wird nur beim Speichern,
  genau wie heute bei den 1-Byte-Typen: in WASM-Locals mit `and $FFFF` bzw.
  Vorzeichenerweiterung (`i32.extend16_s`), im Speicher über 16-Bit-Zugriffe.
- Konstanten außerhalb des Bereichs bleiben Compilerfehler (Error 76),
  Typumwandlungen (`SmallInt(x)`, `Word(x)`) kürzen.
- `SmallInt` ist ein Ganzzahltyp wie `ShortInt`; für das Ergebnis von
  Ausdrücken gilt dieselbe Regel wie heute (`Integer`, mit `LongInt` dann
  `LongInt`).
- Folgen: `SizeOf(Word)` = 2, Records und typisierte Dateien mit
  `Word`-Feldern haben das Layout von TP (binäre Dateien aus TP passen).
  `testdata/etappe4/typen.out` ändert sich (`Word 65535+1 = 0`), weitere
  Ausgaben möglicherweise bei `SizeOf` von Records; jeden Diff mit
  `-update` prüfen und im Bericht nennen. `GetDate`/`GetTime`,
  `Delay`/`Sound` haben `Word`-Parameter und laufen unverändert.

#### Der Typ `WideChar`

| Eigenschaft | Festlegung |
| --- | --- |
| Herkunft | Delphi 2.0 |
| Wertebereich | `#0`..`#$FFFF` (UTF-16-Codeeinheit). Surrogate (U+D800..U+DFFF) und Noncharacters (U+FDD0..U+FDEF, U+FFFE, U+FFFF) lassen sich nicht als Konstante schreiben (Error 76), entstehen aber über `WideChar(n)` und als `High(WideChar)`; beim Dekodieren entstehen Surrogate nie |
| Darstellung | WASM-Wert `i32`, im Speicher 2 Byte, Ausrichtung 2 |
| Art | Ordinaltyp und Zeichentyp, kein Ganzzahltyp: keine Arithmetik (`w + 1` ist Error 41), aber `Ord`, `Succ`, `Pred`, `Inc`, `Dec`, `Low`, `High`, Vergleiche, `case`-Marken, Array-Index, `for`-Laufvariable, Teilbereiche (Größe nach der Tabelle oben) |
| Mengen | `set of WideChar` ist wie jede Menge mit mehr als 256 Elementen ein Fehler (Error 23); Teilbereiche innerhalb `#0`..`#255` sind erlaubt |
| Typumwandlung | `WideChar(n)` aus jedem Ordinalwert (auf 16 Bit gekürzt), `Char(w)` auf 8 Bit gekürzt, `Word(w)`, `Integer(w)` usw. wie bei anderen Ordinaltypen |
| Ausgabe als UTF-8 | 1–3 Bytes; ein Surrogat-Wert (U+D800..U+DFFF) wird als U+FFFD ausgegeben, damit nie ungültiges UTF-8 entsteht |

**Konstanten:**

- Ein String-Literal, das genau **ein** Unicode-Zeichen bis U+FFFF enthält
  (`'ä'`, `'€'`), ist zuweisungsverträglich mit `WideChar` und darf überall
  stehen, wo ein `WideChar` erwartet wird (Zuweisung, Wertparameter,
  Vergleich mit einem `WideChar`, `case`-Marke über einem `WideChar`,
  typisierte Konstante, Teilbereichsgrenze nach `WideChar(…)`). Ohne
  solchen Kontext bleibt es wie bisher ein String (bzw. `Char`, wenn es
  ein Byte lang ist), damit `Write('ä')` und `s := 'ä'` unverändert
  funktionieren.
- **Vorrang beim Vergleich:** Steht ein solches Literal einem `WideChar`
  gegenüber (`w = 'ä'`, `w < '€'`), wird es `WideChar`, und verglichen wird
  ordinal. Die Regel „`WideChar` → String beim Vergleich mit einem String“
  gilt nur für String-Ausdrücke, die kein solches Literal sind. (Für
  Zeichen bis U+FFFF käme dasselbe heraus, weil UTF-8 die Reihenfolge der
  Codepoints erhält; der ordinale Vergleich ist aber billiger und
  eindeutig.)
- Ein Literal mit einem Zeichen ab U+10000 ergibt in `WideChar`-Kontext
  Error 76 („Constant out of range“).
- `#n` mit n in 256..65535 (auch `#$20AC`) ist eine `WideChar`-Konstante,
  wie in Delphi; ausgenommen Surrogate (`#$D800`..`#$DFFF`) und
  Noncharacters (`#$FDD0`..`#$FDEF`, `#$FFFE`, `#$FFFF`), die Error 76
  „Constant out of range (surrogate)“ bzw. „(noncharacter)“ ergeben. `#n` bis 255 bleibt `Char` (ein **Byte**: `#$E9` ist also
  nicht `'é'`, das als UTF-8 zwei Bytes hat; als Abweichung dokumentieren);
  über 65535 bleibt es Error 76. `#n`-Folgen mit gemischten Größen
  (`'x'#$20AC`) sind Strings: Jedes `WideChar` darin wird als UTF-8
  eingesetzt.
- Der Lexer setzt `#n` über 255 als UTF-8 in den Wert des String-Tokens
  ein und vermerkt im Token, wenn es aus genau einem solchen `#n` besteht
  (Feld mit dem Codepoint). Die Typprüfung macht daraus eine
  `WideChar`-Konstante, sonst einen String.

**Verträglichkeit:**

| Von → nach | Regel |
| --- | --- |
| `Char` → `WideChar` | implizit, über den Ordinalwert (Byte 128..255 wird zu U+0080..U+00FF; sinnvoll nur für ASCII, als Abweichung in `docs/abweichungen.md`) |
| `WideChar` → `Char` | nur explizit mit `Char(w)` |
| `WideChar` → `string` | implizit überall, wo ein String-Wert erwartet wird: Zuweisung, Wert- und `const`-Parameter, `+`, `Concat`, Vergleich mit einem String (siehe Vorrang oben) und die Standardroutinen mit String-Argument (`Length`, `Pos`, `Copy`, `Insert`, `Assign`, `ChDir`, `MkDir`, `RmDir`, `Val`, …). Das Zeichen wird als UTF-8 eingesetzt; `Length(w)` liefert also 1–3 (Bytes). |
| `string` → `WideChar` | nie implizit (Error 26); dafür `UTF8Next` |
| `WideChar` ↔ `WideChar`/`Char` im Vergleich | ordinal, `Char` wird erweitert |
| `var`-Parameter | identischer Typ wie bisher |

**Ein- und Ausgabe:**

- `Write(w)`/`Write(w:n)` schreibt das Zeichen als UTF-8; die Breite zählt
  Zeichen, nicht Bytes (also `n − 1` Leerzeichen davor).
- `Read(F, w)` auf einer Textdatei (auch `Input`) liest eine
  UTF-8-Sequenz von 1–4 Bytes und dekodiert sie. Eine Sequenz wird immer
  ganz verbraucht; ungültige Bytes (je Byte), Überlangkodierungen und
  Zeichen ab U+10000 (alle 4 Bytes zusammen) ergeben einmal `#$FFFD`. Am
  Zeilenende `#10`, am Dateiende `#26`, wie `_ReadChar`.
- `file of WideChar` speichert 2 Bytes little-endian, wie jede typisierte
  Datei die Rohbytes.
- `Str`/`Val` gibt es für `WideChar` nicht; `UpCase` bleibt auf `Char`
  beschränkt.

**`for … in` über Strings:** Ist die Laufvariable ein `WideChar`, läuft die
Schleife über die **Zeichen** des Strings (dekodiert wie `UTF8Next`, also
ungültige Bytes und Zeichen ab U+10000 als `#$FFFD`). Ist sie ein `Char`,
bleibt es bei den Bytes. Das ist eine kleine eigene Erweiterung von
`for … in` (Delphi kennt den Fall nicht, weil Strings dort UTF-16 sind).

**Hinweis für die Doku:** `s[i]` ist ein **Byte**. `w := s[1]` mit
`s = 'ä'` ergibt `'Ã'` (U+00C3, das erste UTF-8-Byte über die
`Char`→`WideChar`-Regel). Zeichenweise Zugriffe gehen über `UTF8Next`
bzw. `for … in` mit `WideChar`; das gehört mit Beispiel in
`docs/units.md`.

#### Prüfungen des Quelltexts

| Was | Wo | Fehler |
| --- | --- | --- |
| Steuerzeichen U+0000–U+001F außer Tabulator (U+0009) **direkt** in einem String- oder Zeichenliteral `'…'` | Lexer, beim Lesen des Literals | 907 „Invalid character in string literal (control character U+001B); use #27“ (Stelle des Zeichens) |
| Noncharacter **direkt** in einem Literal (U+FDD0–U+FDEF, U+xFFFE, U+xFFFF aller Ebenen) | Lexer, beim Lesen des Literals | 907 „Invalid character in string literal (noncharacter U+FFFE)“ |
| Surrogat oder Noncharacter über `#n` (`#$D800`..`#$DFFF`, `#$FDD0`..`#$FDEF`, `#$FFFE`, `#$FFFF`) | Lexer, bei `#n` | 76 „Constant out of range (surrogate)“ bzw. „(noncharacter)“ |
| Surrogat direkt im Quelltext | `source.Loader` (UTF-8-Prüfung, schon vorhanden) | 900 wie bisher: kodierte Surrogate sind kein gültiges UTF-8 |
| Bidi-Steuerzeichen U+202A–U+202E und U+2066–U+2069 **irgendwo** im Quelltext, auch in Kommentaren und Literalen | `source.Loader`, gleich nach der UTF-8-Prüfung | 908 „Bidirectional control character (U+202E)“, in Literalen mit Hinweis auf `#$202E` |

- Vorbild sind JSON (RFC 8259: Steuerzeichen in Strings nur als Escape)
  und Rust bzw. GCC (`-Wbidi-chars`), die Bidi-Steuerzeichen wegen
  CVE-2021-42574 („Trojan Source“) ablehnen: Mit ihnen kann ein Editor
  Quelltext anders anzeigen, als er übersetzt wird.
- Über `#n` bleibt alles erreichbar: `#27` für ANSI-Sequenzen, `#7`,
  `'Zeile'#13#10`, nach dieser Etappe auch `#$202E`. Zeilenumbrüche im
  Literal sind wie bisher Error 8.
- Weiter erlaubt: Tabulator im Literal, Steuerzeichen und Noncharacters in
  Kommentaren (außer Bidi), Null-Breite-Zeichen in Strings und Kommentaren,
  Typumwandlungen wie `WideChar($FFFE)` und `High(WideChar)` (Werte, keine
  Zeichenkonstanten) und alles im laufenden Programm (`Chr(7)`, eingelesene
  Dateien; `Read` und `UTF8Next` dekodieren Noncharacters als gültige
  Zeichen, Surrogat-Werte gibt `Write` als U+FFFD aus).
- **TP-Kompatibilität:** TP nahm jedes Byte im Literal an; alte
  DOS-Programme enthalten oft ein rohes ESC für ANSI-Sequenzen. Sie brechen
  jetzt mit 907 ab und müssen auf `#27` umgestellt werden (Abweichung).

#### UTF-8-Hilfsfunktionen in `rtl/system.pas`

Alle in Pascal, öffentlich in der System-Unit. „Zeichen“ heißt
Unicode-Codepoint; ein ungültiges Byte zählt als ein Zeichen und dekodiert
zu `#$FFFD`. Indizes sind wie bei `Copy` 1-basiert.

| Routine | Wirkung |
| --- | --- |
| `function UTF8Length(const S: string): Integer` | Anzahl der Zeichen |
| `function UTF8Next(const S: string; var Index: Integer): WideChar` | dekodiert das Zeichen ab Byte `Index` und setzt `Index` dahinter; `Index` außerhalb von 1..`Length(S)` → `#0`, `Index` bleibt |
| `function UTF8Index(const S: string; CharIndex: Integer): Integer` | Byte-Index des `CharIndex`-ten Zeichens; hinter dem letzten Zeichen `Length(S) + 1`, unter 1 → 1 |
| `function UTF8Copy(const S: string; Index, Count: Integer): string` | wie `Copy`, aber in Zeichen gezählt (zerschneidet nie ein Zeichen) |

Nicht aufgenommen: Löschen und Einfügen in Zeichen (geht mit `UTF8Index`
plus `Delete`/`Insert`).

#### Umsetzung (Hinweise)

- `compiler/internal/types/types.go`: neue Arten `KWideChar` **direkt
  hinter `KChar`** und `KSmallInt` **zwischen `KByte` und `KInteger`**
  (dann stimmen `IsInteger` = `KShortInt`..`KLongInt` und `IsReal` ab
  `KReal` weiter). `WideChar` und `SmallInt` mit `size: 2`, `Word` mit
  `size: 2`; `Enum.Size` und `Subrange.Size` um die 2-Byte-Stufe
  erweitern; Zeichentyp-Prüfungen (`IsOrdinal`, Ausrichtung) ergänzen.
- **16-Bit-Speicherzugriffe:** `ir` bekommt `MemI16` und `MemU16`
  (bisher nur `MemI8`, `MemU8`, `MemI32` und Gleitkomma); `codegen` nutzt
  `i32.load16_s`/`i32.load16_u`/`i32.store16` (Opcodes hat der Encoder
  schon) und kürzt Locals mit `and $FFFF` bzw. `i32.extend16_s` (dafür in
  `ir` eine Operation `Ext16` analog zu `Ext8`); `encode`
  schreibt 2-Byte-Werte für typisierte Konstanten.
- `compiler/internal/check/universe.go`: `WideChar` und `SmallInt` als
  vordefinierte Typen; `UTF8…` kommen über `rtl/system.pas`.
- `compiler/internal/check/expr.go`: `isStringish` wird an acht Stellen
  benutzt (u. a. `stringExpr` für alle Standardroutinen mit
  String-Argument, `compatible`, `isAssignable`, `binary`, `read`,
  `write`) – jede Stelle ausdrücklich auf die Regeln oben prüfen.
  `compatible` prüft Ordinaltypen vor Strings; `Char` gegen `WideChar`
  muss dort als verträglich (ordinal) gelten. Literale mit einem Zeichen
  bis U+FFFF im `WideChar`-Kontext.
- `compiler/internal/lexer`: `#n` bis 65535 zulassen, UTF-8 in den Wert
  einsetzen, Codepoint im Token vermerken (siehe Konstanten).
- `compiler/internal/check/stmt.go` und `ir/lower.go`: `for … in` mit
  `WideChar`-Laufvariable über Strings (über `UTF8Next`).
- `compiler/internal/ir/lower_builtin.go` und `lower_expr.go`: `Write` und
  `Read` für `WideChar`, Umwandlung `WideChar` → String an allen
  Stellen von `isStringish`.
- `rtl/system.pas`: `_WriteWideChar`, `_ReadWideChar`, `_WideCharToStr`
  (UTF-8-Kodierung, 1–3 Bytes, Surrogate → U+FFFD) und die öffentlichen
  Routinen aus der Tabelle; keine neuen Intrinsics.
- Neue Fehlernummern nur für die Quelltext-Prüfungen: 907 und 908 in
  `compiler/internal/source/error.go`; sonst reichen 26, 41, 76 und 23.
- Lexer: im String-Literal jedes Zeichen unter U+0020 außer Tabulator und
  jedes Noncharacter mit 907 ablehnen; bei `#n` Surrogate und
  Noncharacters mit 76. `source.Loader`: nach `utf8.Valid` die Bidi-Codepoints
  suchen (die Position wie bei Error 900 bestimmen).

#### Tests

- `testdata/etappe14/ganzzahl16.pas`: `SmallInt` und `Word` mit Überlauf
  bei Zuweisung (`32767 + 1` → −32768, `65535 + 1` → 0), auch als
  Record-Feld, Array-Element, `var`-Parameter und in typisierten
  Konstanten; `SizeOf` von `SmallInt`, `Word`, Teilbereichen (`0..1000`
  → 2, `-1..40000` → 4) und einer Aufzählung mit 300 Werten (→ 2);
  `file of Word` mit Dateigröße = 2 × Anzahl; Typumwandlungen.
- `testdata/etappe14/widechar.pas`: Zuweisungen aus Literalen (`'ä'`,
  `'€'`, `#$20AC`), `Ord`, `Succ`/`Pred`, `Inc`/`Dec`, `Low`/`High`,
  Vergleiche mit `Char`, `WideChar` und Literalen (ordinal), `case` mit
  `WideChar`-Marken, Array mit `WideChar`-Index, Verkettung mit Strings,
  `Length(w)`, `Write` mit Breite, Surrogat-Wert in `Write` (→ U+FFFD),
  Typumwandlungen in beide Richtungen, `SizeOf(WideChar)` = 2, `#$E9`
  gegen `'é'`, `s[1]`-Falle.
- `testdata/etappe14/utf8.pas`: alle Hilfsfunktionen, auch mit
  ungültigen Bytes (`#$FF`), abgeschnittenen Sequenzen, leeren Strings,
  Indizes außerhalb; `for … in` mit `WideChar` und mit `Char` über
  denselben String.
- `testdata/etappe14/lesen.pas` mit `.in`: `Read` einzelner `WideChar`
  aus einer Eingabe mit Umlauten, `€`, einem Emoji (→ einmal `#$FFFD`),
  einem ungültigen Byte und Zeilenende (`#10`).
- `testdata/etappe14/datei.pas`: `file of WideChar` schreiben und lesen,
  Dateigröße = 2 × Anzahl.
- Fehlertests (`*.err`): `set of WideChar` (23), Emoji-Literal an
  `WideChar` (76), `#70000` (76), String-Variable an `WideChar` (26),
  `w + 1` (41), `WideChar` an `var c: Char` (26), `SmallInt`-Konstante
  40000 (76).
- `compiler/internal/check/testdata`: Symbol-Golden-Test mit `WideChar`
  und `SmallInt` in Typ-, Konstanten- und Variablendeklarationen.
- Lexer-Test für `#256`, `#$FFFD` (größter erlaubter Wert unterhalb der
  Noncharacters), `#$FFFF` und `#$FFFE` (76, noncharacter), `#$FDD0` (76),
  `#$D800` und `#$DFFF` (76, surrogate), `#$D7FF` und `#$E000` (erlaubt),
  `#65536` (76) – der bisherige Fall „# zu groß“ mit `#256` ändert sich.
- Quelltext-Prüfungen (Lexer- bzw. Source-Tests und `*.err`): rohes ESC und
  NUL im Literal (907 an der Stelle des Zeichens, Meldung mit `#27` bzw.
  `#0`), Tabulator im Literal erlaubt, `#27` erlaubt, Steuerzeichen im
  Kommentar erlaubt; U+202E im Kommentar, im Literal und in einer
  `{$I}`-Datei (908 mit Datei und Stelle), U+2066 ebenso; U+FFFE, U+FDD0
  und U+10FFFF im Literal (907, noncharacter), im Kommentar erlaubt;
  U+200B im Literal erlaubt; `WideChar($FFFE)` und `High(WideChar)`
  übersetzen.
- Prüfen, dass kein bestehendes Testprogramm und kein Beispiel in
  `web/beispiele/` ein rohes Steuerzeichen im Literal enthält.
- Bestehende Golden-Files mit `-update` neu schreiben und jeden Diff prüfen
  (mindestens `typen.out`).
- Alle neuen Programme laufen wie üblich auch unter `gojs`, mit erzwungener
  Frame-Ablage und mit dem Browser-Compiler (byte-gleich).

#### Doku

- `PLAN.md`: Typabbildung (`Word` 2 Byte, Zeilen `SmallInt` und
  `WideChar`), „Festlegungen zu Wertebereichen“ (Word mit 16-Bit-Kürzung),
  Regel für Teilbereiche und Aufzählungen, Abschnitt Zeichensatz (Hinweis
  auf `WideChar` und die Hilfsfunktionen), Tabelle der Erweiterungen um
  `WideChar` (Delphi 2.0), `SmallInt` und `for … in` mit `WideChar`.
- `CLAUDE.md`: Typtabelle (`Word`, `SmallInt`, `WideChar`, Teilbereiche
  und Aufzählungen), Erweiterungsliste.
- `docs/units.md`: Typtabelle (`SmallInt`, `Word` mit 2 Byte, `WideChar`),
  `WideChar` (Konstanten, Ein-/Ausgabe), die vier Routinen mit Beispielen,
  ausdrücklich `UTF8Next` statt `s[i]` für Zeichen.
- `docs/abweichungen.md`: Zeilen für Etappe 14 (Umwandlung `Char` →
  `WideChar` über den Ordinalwert, `#128`..`#255` als Byte, `#$FFFD` für
  ungültige Eingaben, Surrogate → U+FFFD, `for … in` mit `WideChar`,
  `Length(w)` in Bytes, Steuerzeichen in Literalen nur über `#n`,
  Bidi-Steuerzeichen verboten, Noncharacters und Surrogate als
  Zeichenkonstanten verboten, Fehler 907 und 908); die Zeile aus Etappe 1
  zu `Word` als überholt kennzeichnen.
- `README.md`, Abschnitt Umfang: `WideChar`, `SmallInt`, 16-Bit-`Word` und
  UTF-8-Hilfsfunktionen.

**Fertig, wenn:** ein Programm Umlaute und `€` als `WideChar` speichern,
vergleichen, in `case` verwenden, ausgeben und einlesen kann; die
Hilfsfunktionen zählen, kopieren und durchlaufen UTF-8-Strings zeichenweise
auch mit ungültigen Bytes korrekt; `SmallInt` und `Word` belegen 2 Byte und
laufen bei Zuweisung wie in TP über; rohe Steuerzeichen in Literalen und
Bidi-Steuerzeichen im Quelltext werden mit 907 bzw. 908 abgelehnt,
Noncharacters und Surrogate als Zeichenkonstanten mit 907 bzw. 76; alle
Fehlertests melden die erwarteten
Nummern; alle bisherigen Tests (mit geprüften Diffs) sind unter WASI,
`gojs` und mit dem Browser-Compiler grün.
- *Erledigt (Nov 26, 2026).* `SmallInt` neu, `Word`, Teilbereiche und Aufzählungen mit 2 Byte (IR `MemI16`/`MemU16`, `Ext16`); `WideChar` als Basistyp hinter `Char`, Verträglichkeit über `charPair`/`isWide` statt über `host` (sonst würden `'a'..'z'`-Teilbereiche zu `WideChar`), Literale mit einem Zeichen über `wideLiteral` im Kontext (Zuweisung, Parameter, Vergleich, `case`, typisierte Konstante, Teilbereich, Array-Index); `#n` bis 65535 mit `CodePoint` im Token; Prüfungen 907 (Lexer) und 908 (`source.Loader`); Laufzeit `_WideCharToStr`, `_WriteWideChar`, `_ReadWideChar`, `UTF8Length`/`Next`/`Index`/`Copy`, `for … in` über `UTF8Next`. **Entscheidung bei der Umsetzung:** ungültige UTF-8-Folgen nach der Unicode-Empfehlung (maximal subpart, wie der Encoding-Standard der Browser) je Abschnitt als ein `#$FFFD`, nicht je Byte wie Go – nur so verhalten sich `Read` (sieht nur ein Byte voraus) und `UTF8Next` gleich; ein Byte, das nie gültig ist (`C0`, `FF`, …), bleibt ein eigenes Zeichen. Geänderte Golden-Files: `typen.out` (`Word 65535+1 = 0`, `SizeOf(w)` = 2), `deklarationen.sym` (Größen von `Word` und Teilbereich `1000..1100`), `lexer/zeichen_zu_gross.pas` (`#300` ist jetzt erlaubt, Test nutzt `#70000`). Tests: `testdata/etappe14/` (vier Programme, zehn Fehlertests), Lexer- und Source-Tests, `breit.sym`, Frame-Ablage. Nebenbei: `TestServeTimeout` mit mehr Abstand (unter Last lief auch die kleine Anfrage ins 20-ms-Limit).

### Etappe 15: Unit `Dos` erweitern und Ortszeit ✅

**Ziel:** Die Unit `Dos` bekommt die Routinen für Dateiattribute, Dateizeit,
Verzeichnissuche, Pfade und Umgebungsvariablen, die Lehrprogramme typisch
benutzen. `GetDate`, `GetTime` und Dateizeiten liefern Ortszeit statt UTC.

**Befund vor der Etappe (27. 11. 2026):** `GetDate`/`GetTime` stimmen unter
WASI und `gojs` auf die Sekunde mit der Systemuhr überein, aber in **UTC**
(Test unter Europe/Berlin: Programm 13:58, lokal 14:58 CET). WASI Preview 1
kennt keine Zeitzone, und die Laufzeit bekommt auch sonst keine.

**Entscheidungen** (1 und 4 nach Rückfrage; 2, 3 und 5 als Vorschlag
angenommen, ohne Widerspruch):

1. *Entschieden:* Ortszeit über einen festen UTC-Versatz beim Programmstart
   (Variante A), siehe „Ortszeit“.
2. `SearchRec.Name` und `PathStr`/`DirStr`/`NameStr`/`ExtStr` als `string`
   mit 255 Bytes statt TP-Längen (12, 79, 67, 8, 4), damit lange Namen nicht
   abgeschnitten werden.
3. Zusätzlich zu den gewünschten Routinen `DosError`, `DateTime`,
   `PackTime`, `UnpackTime` und die Attributkonstanten (ohne sie lassen sich
   `GetFAttr`, `GetFTime` und `FindFirst` nicht sinnvoll benutzen).
4. *Entschieden:* Nur `Directory` und `Hidden` haben eine Wirkung.
   `ReadOnly`, `SysFile`, `Archive` und `VolumeID` werden nie gemeldet und
   beim Suchen ignoriert (kein Probe-Öffnen; WASI kennt keine Rechtebits).
5. `pascal run` gibt die Umgebung des Hosts an das Programm weiter (wie ein
   natives Programm); `runner` und Loader geben ohne Angabe keine weiter.

#### Neue Routinen und Typen in `rtl/dos.pas`

| Routine | Festlegung |
| --- | --- |
| `procedure GetFAttr(var F; var Attr: Word)` | `F` zugewiesen (offen oder geschlossen). Nur zwei Bits haben eine Wirkung (Entscheidung 4): `Directory` aus dem Dateityp, `Hidden` für Namen mit `.` am Anfang (Unix-Konvention). `ReadOnly`, `SysFile`, `Archive` und `VolumeID` sind nie gesetzt; eine normale Datei hat also `Attr` = 0. Fehlt die Datei: `DosError` = 2 bzw. 3. |
| `procedure GetFTime(var F; var Time: LongInt)` | `F` geöffnet; Änderungszeit (`mtim`) als gepackte DOS-Zeit in Ortszeit. Nicht geöffnet: `DosError` = 6 (`// TP-ANNAHME`). |
| `procedure FindFirst(Path: PathStr; Attr: Word; var S: SearchRec)` | Muster mit `*` und `?` im letzten Teil von `Path`, Groß-/Kleinschreibung beim Vergleich egal (wie DOS; der Dateizugriff selbst bleibt beim Host). `*` und `*.*` passen auf alle Namen (auch ohne Punkt). Normale Dateien immer; Verzeichnisse nur mit `Directory`, versteckte nur mit `Hidden` in `Attr`; die übrigen Bits in `Attr` haben keine Wirkung. `.` und `..` werden nie geliefert. Kein Treffer: `DosError` = 18, Pfad fehlt: 3. |
| `procedure FindNext(var S: SearchRec)` | nächster Treffer, am Ende `DosError` = 18. Reihenfolge wie vom Host (nicht sortiert). |
| `function FExpand(Path: PathStr): PathStr` | vollständiger Pfad ab der Wurzel wie `GetDir`: `/a/b/datei.txt`, mit `/` als Trenner, `.` und `..` aufgelöst, ohne Laufwerk und ohne Großschreibung. |
| `procedure FSplit(Path: PathStr; var Dir: DirStr; var Name: NameStr; var Ext: ExtStr)` | reine Zeichenkettenoperation: `Dir` mit abschließendem Trenner (`\` und `/` gelten beide), `Ext` mit Punkt (letzter Punkt im letzten Teil). |
| `function FSearch(Path: PathStr; DirList: string): PathStr` | zuerst das aktuelle Verzeichnis, dann die Verzeichnisse aus `DirList` (getrennt durch `;`); liefert den ersten Pfad zu einer vorhandenen **Datei**, sonst `''`. |
| `function GetEnv(EnvVar: string): string` | Wert der Variable; zuerst exakter Name, dann ohne Rücksicht auf Groß-/Kleinschreibung (`// TP-ANNAHME`: DOS speicherte Namen groß). Fehlt sie: `''`. |
| `function EnvCount: Integer` | Anzahl der Variablen. |
| `function EnvStr(Index: Integer): string` | `'NAME=WERT'` der `Index`-ten Variable (ab 1), außerhalb `''`. |
| `function DosVersion: Word` | immer 5, also `Lo` = 5 (Hauptversion), `Hi` = 0. |

Dazu (Entscheidung 3):

- `var DosError: Integer` – Fehler der letzten Dos-Routine (0, 2, 3, 6, 18).
- Konstanten `ReadOnly = $01`, `Hidden = $02`, `SysFile = $04`,
  `VolumeID = $08`, `Directory = $10`, `Archive = $20`, `AnyFile = $3F`.
- `type DateTime = record Year, Month, Day, Hour, Min, Sec: Word end`,
  `procedure PackTime(var T: DateTime; var P: LongInt)`,
  `procedure UnpackTime(P: LongInt; var T: DateTime)` – gepackt wie DOS
  (Bits: Jahr−1980, Monat, Tag, Stunde, Minute, Sekunde/2).
- `type SearchRec = record Fill: array[1..21] of Byte; Attr: Byte; Time:
  LongInt; Size: LongInt; Name: string end` (Entscheidung 2: `Name` mit 255
  Bytes; `Fill` enthält einen Zeiger auf den Suchzustand im Heap).
  Größe über 2 GB wird auf `MaxLongInt` gekürzt.
- `PathStr`, `DirStr`, `NameStr`, `ExtStr` = `string` (Entscheidung 2).
- Herkunft: Beim Umsetzen gegen die TP-4.0-Referenz prüfen, welche
  Routinen es erst ab TP 5.0 gibt (vermutlich `FExpand`, `FSplit`,
  `GetEnv`, `DosVersion`); die gehören dann in die Tabelle der
  Erweiterungen.

**Suchzustand:** `FindFirst` legt den Zustand (Verzeichnis-Handle, Cookie
von `fd_readdir`, Muster, Attribute, Lesepuffer) im Heap ab, `FindNext` liest
ihn, beim Ende (`DosError` = 18) wird er freigegeben. Bricht ein Programm
eine Suche vorher ab, bleibt er bis Programmende liegen (TP 4.0 hat kein
`FindClose`; dokumentieren).

#### Ortszeit (Entscheidung 1)

**Wie Go es macht** (`$GOROOT/src/time`): Unter `GOOS=js` nimmt Go einmal
beim Start `new Date().getTimezoneOffset()` und rechnet danach mit diesem
festen Versatz (Zone „UTC+1“, `zoneinfo_js.go`). Unter `GOOS=wasip1` ist
`time.Local` immer UTC („zoneinfo is managed by the runtime“,
`zoneinfo_wasip1.go`). `-tags timetzdata` bettet die IANA-Datenbank
(`zoneinfo.zip`, gut 400 KB) ins Programm ein, damit `time.LoadLocation`
ohne Systemdateien funktioniert; den Namen der Zone muss das Programm selbst
bekommen, z. B. aus `Intl.DateTimeFormat().resolvedOptions().timeZone` im
Browser. Für WasmPascal hieße das: gut 400 KB zusätzlich in **jedem**
übersetzten Programm (heute meist 30–60 KB) – zu viel. Übernehmen lässt
sich die Idee, die Regeln der einen Zone vom Host zu bekommen (Variante B).

**Variante A – fester Versatz (gewählt):**

- Die Laufzeit liest beim Start die Umgebungsvariable
  `WASMPASCAL_UTC_OFFSET` (Minuten östlich von UTC, z. B. `60` für CET,
  `120` für CEST). Fehlt sie, gilt wie bisher UTC.
- `GetDate`, `GetTime`, `GetFTime` und `SearchRec.Time` rechnen mit diesem
  Versatz. Er gilt für die ganze Laufzeit: Ein Wechsel der Sommerzeit
  während des Laufs und Dateizeiten aus der anderen Jahreshälfte sind um
  eine Stunde verschoben (Abweichung, dokumentieren).
- Wer setzt sie: `pascal run` aus `time.Now().Zone()`; `web/loader.js`
  aus `new Date().getTimezoneOffset()`, wenn `env` sie nicht schon enthält;
  `runner` als Bibliothek nur, wenn der Aufrufer sie in `Env` angibt.
**Verworfen – Variante B (Zeitzonenregel):** Der Host hätte die Regel der
lokalen Zone als POSIX-TZ-String (`CET-1CEST,M3.5.0,M10.5.0/3`) in `TZ`
übergeben, die Laufzeit sie in Pascal ausgewertet; dann stimmten auch
Sommerzeitwechsel und Dateizeiten aus der anderen Jahreshälfte. Nach
Rückfrage zugunsten der einfacheren Variante A zurückgestellt; bei Bedarf
später nachrüstbar, ohne Variante A zu brechen (`TZ` hätte Vorrang).

#### Unterste Schicht, Runner, Loader

- Neue WASI-Importe in `compiler/internal/abi`: `environ_sizes_get`,
  `environ_get`, `fd_readdir`, `path_filestat_get`. In `rtl/system.pas`
  gemeinsame `Sys…`-Routinen dafür; Umgebung unter `gojs` aus dem Speicher
  hinter `argv` (dort legt `wasm_exec.js` sie ab, Ende mit einem
  Null-Zeiger). Für die Dos-Unit bekommt die System-Unit Routinen mit `_`
  im Interface (Pfad normalisieren wie `NormPath`, Umgebung lesen, Datei
  stat/readdir), damit `dos.pas` nichts doppelt implementiert.
- `runner.Options` bekommt `Env []string` (`"NAME=WERT"`; nil = leere
  Umgebung) mit Doku-Kommentar und Beispiel. `pascal run` übergibt
  `os.Environ()` plus `WASMPASCAL_UTC_OFFSET` (Entscheidung 5).
- `web/loader.js`: Option `env` (Objekt Name → Wert) an `go.env`; die
  WASI-Funktionen `fd_readdir` und `path_filestat_get` für den Datei-Host.
  Der Host-Vertrag wächst um `readdirSync(path)` (Namen, ohne `.`/`..`) und
  `mtimeMs` in `statSync`/`fstatSync` (Node liefert beides schon);
  `node-run.js` reicht seine Umgebung mit `--env` weiter (sonst keine).
- Tests mit fester Uhr: `runner` bekommt eine nicht exportierte
  Testmöglichkeit für `WithWalltime`, damit Datum und Uhrzeit in Tests
  feststehen.

#### Tests

- `testdata/etappe15/` (laufen wie üblich auch unter `gojs`):
  `fsplit.pas` (Pfade mit `\` und `/`, ohne Endung, mehrere Punkte,
  Punkt im Verzeichnis), `fexpand.pas` (nach `ChDir`, mit `..`),
  `fsearch.pas` (Treffer im aktuellen Verzeichnis, in der Liste, Verzeichnis
  statt Datei, kein Treffer), `findfirst.pas` (Muster `*.txt`, `?`,
  `*.*`, Verzeichnisse nur mit `Directory`, versteckte Datei, kein Treffer
  18, fehlender Pfad 3; Ausgabe ohne Abhängigkeit von der Reihenfolge, z. B.
  gezählt oder sortiert im Programm), `fattr.pas` (`GetFAttr` für Datei,
  Verzeichnis, fehlende Datei), `ftime.pas` (`GetFTime` + `UnpackTime` +
  `PackTime` hin und zurück; Plausibilität statt fester Zeit),
  `umgebung.pas` (mit `.env`-Datei neben `.args`/`.in`: `GetEnv`,
  `EnvCount`, `EnvStr`, Groß-/Kleinschreibung), `dosversion.pas`.
- Harness: optionale `name.env` (Zeilen `NAME=WERT`) für beide Ziele.
- `runner`-Tests mit fester Uhr und `WASMPASCAL_UTC_OFFSET`: Datum,
  Uhrzeit, Wochentag und Tageswechsel (23:30 UTC mit +60 → nächster Tag,
  00:30 UTC mit −300 → Vortag), Schaltjahr (29. 2.), Jahreswechsel, ein
  Versatz mit halben Stunden (+330), ohne Variable UTC; `GetFTime` einer
  Datei mit bekannter Änderungszeit.
- `pascal run` übergibt den Versatz des Systems (Test mit gesetztem `TZ`
  des Go-Prozesses, z. B. `TZ=Asia/Kolkata` → 330).
- `GetFAttr` und `FindFirst` melden für eine schreibgeschützte Datei kein
  `ReadOnly` (Entscheidung 4).

#### Doku

- `docs/units.md`: Abschnitt Dos neu (alle Routinen, Typen, Konstanten,
  `DosError`, Ortszeit).
- `docs/abweichungen.md`: Ortszeit über den festen Versatz beim Start
  (Sommerzeitwechsel während des Laufs und Dateizeiten aus der anderen
  Jahreshälfte um eine Stunde verschoben),
  Attribute (nur `Directory` und `Hidden` per Punkt), lange Namen in
  `SearchRec`/`PathStr`, Pfade mit `/` in `FExpand`, keine `.`/`..` in
  `FindFirst`, Reihenfolge vom Host, Suchzustand ohne `FindClose`;
  Zeile „Dos: nur Datum und Uhrzeit, UTC“ (Etappe 7) als überholt.
- `PLAN.md`: „Weggelassen“ (Dos) anpassen, ggf. Erweiterungstabelle;
  `README.md` (Umfang), `web/README.md` (Host-Vertrag, `env`),
  `CHANGELOG.md` (Unreleased).

**Fertig, wenn:** alle genannten Routinen unter WASI und `gojs` dieselben
Ergebnisse liefern; `GetDate`/`GetTime` mit gesetztem Versatz Ortszeit
liefern (geprüft mit fester Uhr) und `pascal run` den Versatz des Systems
übergibt; alle bisherigen Tests sind grün.
- *Erledigt (Nov 27, 2026).* `rtl/dos.pas` mit allen Routinen, Typen, Konstanten und `DosError`; Ortszeit über `WASMPASCAL_UTC_OFFSET`. System-Unit: WASI-Importe `environ_sizes_get`, `environ_get`, `fd_readdir`, `path_filestat_get`; im Interface `_EnvCount`, `_EnvStr`, `_FullPath`, `_FileName`, `_FileStat`, `_FileStatF`, `_DirOpen`, `_DirNext`, `_DirClose`; Umgebung unter `gojs` hinter `argv`. `runner.Options.Env` (mit `ExampleRun_env`) und eine Uhr für Tests im Paket; `pascal run` gibt die Umgebung des Rechners plus Versatz weiter; Loader: Option `env`, `fd_readdir`, `path_filestat_get`, `mtim`, Host-Vertrag um `readdirSync` und `mtimeMs` erweitert; `node-run.js` mit `--env`, `--setenv`, `--emptyenv`; Harness mit `name.env`. Herkunft geklärt: `DosVersion`, `EnvCount`, `EnvStr`, `FExpand`, `FSearch`, `FSplit`, `GetEnv` sind laut README zu TP 5.0 dort neu (Erweiterungstabelle). Nebenbei behoben: `GetTime` lieferte bei Bruchteilen wie 0,25 s wegen der `f64`-Genauigkeit der Nanosekunden manchmal einen Hundertstel zu wenig (gefunden durch die Tests mit fester Uhr). Tests: `testdata/etappe15/` (sieben Programme, WASI und `gojs`, auch mit Frame-Ablage), `runner/zeit_test.go` (zehn Fälle mit fester Uhr, Dateizeit), `TestProgramEnv`.

### Etappe 16: Turbo Pascal 5.0 – prozedurale Typen, `ExitProc` ✅

**Ziel:** WasmPascal deckt den Sprachumfang von Turbo Pascal 5.0 ab (ohne
Overlays und Debugger, die ohne DOS bedeutungslos sind) und darf sich in
der Doku so nennen.

**Entscheidungen (nach Rückfrage):**

- Prozedurale Typen wie in TP 5.0. Damit wird `ExitProc` möglich, das es
  schon in TP 4.0 gibt, WasmPascal aber bisher fehlt.
- `VER50` ersetzt `VER40` als Bedingungssymbol.
- `RunError[(Code)]` (neu in TP 5.0, einzige neue Routine der System-Unit)
  kommt dazu.
- `{$A+}`/`{$A-}` (neu in TP 5.0) wird angenommen und für `{$IFOPT}`
  mitgeführt (Voreinstellung `A+`), wirkt aber nicht: Records und Arrays
  sind immer natürlich ausgerichtet. Steht in `docs/abweichungen.md`.
- Exit-Prozeduren laufen bei **jedem** Laufzeitfehler, auch nach einem
  WASM-Trap; der Host stößt sie nach dem Trap an (Abschnitt unten).
- Weiterhin weggelassen: Overlays (`Overlay`, `{$O}`), 8087-Emulation
  (`{$E}`), Debugger-Direktiven (`{$D}`, `{$L}`), `Exec`, `SwapVectors`,
  Interrupt-Vektoren, `DiskFree`/`DiskSize`, `SetFAttr` sowie die neuen
  Dos-Routinen `GetCBreak`, `SetCBreak`, `GetVerify` und `SetVerify`.
- Fehlernummern für prozedurale Typen wie in TP 5.0 (neue Nummern 133–145
  laut README zu TP 5.0, z. B. 142 und 143), keine eigenen.

#### Prozedurale Typen

| Thema | Festlegung |
| --- | --- |
| Deklaration | `type TProc = procedure(X: Integer); TFunc = function(X: Real): Real;` Parameterlisten wie bei normalen Routinen (Wert-, `var`- und `const`-Parameter). Als Typ von Variablen, Parametern, Record-Feldern, Array-Elementen und typisierten Konstanten. |
| Zuweisung | `P := Name` mit einer Prozedur oder Funktion, die auf oberster Ebene eines Programms oder einer Unit deklariert ist. Verschachtelte Routinen und Standardroutinen der System-Unit (`WriteLn`, `Length`, …) ergeben Error 143 („Invalid procedure or function reference“), wie in TP. `{$F+}` ist nicht nötig. |
| Verträglichkeit | Gleiche Anzahl Parameter, paarweise identische Typen und Arten (Wert/`var`/`const`), bei Funktionen identischer Ergebnistyp. Parameternamen zählen nicht. Zwei prozedurale Typen mit gleicher Signatur sind zuweisungsverträglich. |
| Aufruf | `P(1)`; eine parameterlose Prozedurvariable als Anweisung `P;`. Eine parameterlose Funktionsvariable `F` in einem Ausdruck ruft auf, außer das Ziel ist selbst prozedural (`G := F` kopiert), wie in TP. |
| `nil` | `P := nil` ist erlaubt; `@P = nil`/`@P <> nil` prüft (`// TP-ANNAHME`). Ein Aufruf über `nil` ergibt Runtime error 216. |
| `@` | `@Name` ist ein `Pointer` auf die Routine. `@P` mit einer Prozedurvariable liefert ihren Wert als `Pointer` (für Vergleiche und `ExitProc`). Der Rückweg `Pointer` → prozeduraler Typ geht nur über eine Typumwandlung `TProc(Ptr)`. |
| Darstellung | `i32`: Index in die WASM-Funktionstabelle; 0 = `nil` (Slot 0 bleibt leer). Aufruf über `call_indirect` mit der WASM-Signatur des prozeduralen Typs; ein falscher Index oder eine falsche Signatur fängt WASM selbst ab (Trap → 216). Im Speicher 4 Byte. |
| Speicher | Kein Static Link nötig, weil nur Routinen der obersten Ebene zugewiesen werden dürfen. |
| Typisierte Konstanten | `const P: TProc = Name;` (auch in Records und Arrays). Der Tabellenindex steht erst im Codegenerator fest, `ir` schreibt die Bytes aber schon beim Kodieren der Konstanten. Deshalb hinterlässt `encode` einen Platzhalter (0) und trägt Stelle und Routine in eine Liste des `ir.Module` ein; der Codegenerator setzt dort den Index ein, bevor er das Datensegment schreibt. Die Routine zählt damit als adressiert. |

**Scheduler:** Ein indirekter Aufruf kann jede Routine treffen, deren
Adresse genommen wird. `analysis.MarkSuspendable` behandelt deshalb einen
indirekten Aufruf als anhaltefähig, wenn eine adressierte Routine mit
derselben WASM-Signatur anhaltefähig ist. Beim Fortsetzen ruft der
Aufrufer über den im Frame gesicherten Tabellenindex erneut auf.
Adressierte Routinen und indirekte Aufrufe stehen in einer eigenen Liste
im `ir.Module`, damit der Aufrufgraph sie kennt.

#### `ExitProc`, `ErrorAddr`, `ExitCode`

- `ExitProc: Pointer` und `ErrorAddr: Pointer` sind neue Variablen der
  System-Unit (TP 4.0).
- Die Kette der Exit-Prozeduren läuft wie in TP in einer Schleife:
  `P := ExitProc; ExitProc := nil; P aufrufen`, bis `ExitProc` nil bleibt.
  Exit-Prozeduren dürfen `ExitProc` neu setzen (Verkettung), `ExitCode` und
  `ErrorAddr` lesen und ändern und Anhaltepunkte benutzen (`WriteLn`,
  `ReadLn`).
- `ErrorAddr` ist nil, wenn kein Laufzeitfehler vorliegt, sonst ein Wert
  ungleich nil (die Zeilennummer, `// TP-ANNAHME`). Setzt eine
  Exit-Prozedur `ErrorAddr := nil`, unterbleibt die Meldung „Runtime
  error …“, wie in TP.
- **Normales Ende und `Halt`:** Die Laufzeit ruft die Kette selbst (am Ende
  des Hauptprogramms und in `_Halt`), danach endet das Programm mit
  `ExitCode`.
- **Laufzeitfehler (geprüft oder Trap):** Ein Laufzeitfehler bleibt ein
  Trap wie heute; `_RunError` und `RunError` laufen nicht selbst über die
  Kette. So muss nicht jede Funktion mit einer Fehlerstelle anhaltefähig
  werden, und auch Traps ohne Prüfung (Speicherzugriff außerhalb, 202, 207
  bei `Trunc`/`Round`) sind abgedeckt. Nach dem Trap ruft der Host
  (`runner`, `web/loader.js`) eine exportierte Routine (Arbeitsname
  `_exit`) mit der Fehlernummer auf. Sie setzt den Schatten-Stack zurück,
  setzt `ExitCode` und `ErrorAddr` und lässt die Kette laufen; danach liest
  der Host `ExitCode` und `ErrorAddr` und meldet den Fehler nur, wenn
  `ErrorAddr` nicht nil ist. Die Routine ist anhaltefähig wie das
  Hauptprogramm: unter WASI mit eigener Treiberschleife wie `_start`, unter
  `gojs` treibt sie der Loader mit `resume` wie `run`. Geprüft: Eine
  wazero-Instanz bleibt nach einem Trap aufrufbar, eine Browser-Instanz
  ebenso.
- Ein Laufzeitfehler in einer Exit-Prozedur ist wieder ein Trap; der Host
  ruft `_exit` erneut und die Kette läuft mit dem Rest weiter. Weil
  `ExitProc` vor jedem Aufruf nil wird, endet das sicher.
- **Grenze:** Fremde WASI-Laufzeiten ohne unseren Runner (`wasmtime`,
  `node:wasi`) kennen `_exit` nicht; dort laufen bei Laufzeitfehlern keine
  Exit-Prozeduren (normales Ende und `Halt` gehen). Steht in
  `docs/abweichungen.md`.

#### `RunError`

`procedure RunError[(Code: Word)]` beendet das Programm mit Runtime error
`Code` (ohne Parameter 0) an der Stelle des Aufrufs, wie in TP 5.0. Als
Laufzeitroutine pflegt sie die Fehlerstelle nicht, die Meldung nennt also
die Zeile des Aufrufers. Exit-Prozeduren laufen wie bei jedem anderen
Laufzeitfehler.

#### Bedingungssymbol

`VER50` statt `VER40`; `WASMPASCAL` bleibt. Programme, die `VER40`
abfragen, verhalten sich wie unter TP 5.0.

#### Umsetzung (Hinweise)

- `compiler/internal/types`: neuer Typ `Proc` (Signatur, Funktion oder
  Prozedur); `IsOrdinal` nein, Größe 4.
- `compiler/internal/check`: Deklaration, Verträglichkeit, Zuweisung mit
  der Kontextregel (Aufruf oder Kopie), Error 143, `@`, Typumwandlung aus
  `Pointer`, `nil`.
- `compiler/internal/lexer`: Schalter `A` (Voreinstellung `+`) in den
  Voreinstellungen, sonst ohne Wirkung.
- `compiler/internal/ir`: `CallIndirect` (Signatur, Index-Wert) und
  `FuncRef` (Tabellenindex einer Routine); Liste der adressierten Routinen
  und der Platzhalter in typisierten Konstanten im Modul.
- `compiler/internal/analysis`: indirekte Aufrufe im Aufrufgraph, Regel
  oben; `_exit` ist eine weitere Wurzel wie das Hauptprogramm.
- `compiler/internal/codegen` und `wasm`: Table-, Element- und
  Typabschnitt für die Funktionstabelle, `call_indirect`; im
  Dispatch-Code den Index im Frame sichern; Platzhalter im Datensegment
  füllen; Export `_exit` samt Treiber unter WASI.
- `rtl/system.pas`: `ExitProc`, `ErrorAddr`, `RunError`, die Schleife der
  Exit-Prozeduren am Programmende und in `_Halt`, die Routine hinter
  `_exit`.
- `runner/rterror.go`, `web/loader.js`: nach einem Trap `_exit` aufrufen,
  danach `ExitCode`/`ErrorAddr` auswerten; Ausgaben der Exit-Prozeduren
  erscheinen vor der Meldung „Runtime error …“.

#### Doku

- Der ausdrückliche Hinweis „ohne OOP“ entfällt überall in der Doku
  (`README.md` zweimal, `CLAUDE.md`, Kopf von `PLAN.md`); dass es keine
  Objekte gibt, folgt aus „Turbo Pascal 5.0“ (OOP kam erst mit 5.5).
  Bereits veröffentlichte Einträge in `CHANGELOG.md` (0.1.0) bleiben
  unverändert.
- Überall „Turbo Pascal 4.0“ → „Turbo Pascal 5.0“, wo der Sprachumfang
  gemeint ist. Die Stellen:
  - `README.md`: Einleitung, Abschnitt Sprache, Doku-Tabelle (Zeile zu
    `docs/abweichungen.md`);
  - `CLAUDE.md`: Kopfzeile, Verweis auf die Abweichungen, Arbeitsregel
    „Im Zweifel TP 4.0 nachbilden“;
  - `PLAN.md`: Titel, Ziel, Sprachumfang, Einleitung der
    Erweiterungstabelle („über TP 5.0 hinaus“; die Zeilen der
    Dos-Routinen aus TP 5.0 wandern dort heraus in den Sprachumfang),
    Arbeitsregel „Im Zweifel … TP 4.0“;
  - `docs/abweichungen.md`: Titel; Etappe-1-Zeile zu `VER40` als überholt
    kennzeichnen;
  - `docs/units.md`: Einleitung (Abweichungen von …);
  - `web/index.html`: Untertitel;
  - Paketdoku in `doc.go`, `compiler/compiler.go` und
    `compiler/internal/parser`; Kommentar zu `VER40` in
    `compiler/internal/source/cond.go`.
  
  Historische Aussagen bleiben („TP 4.0 kannte nur …“, die reservierten
  Wörter von TP 4.0, die sich in 5.0 nicht geändert haben, Herkunftsangaben
  wie „aus TP 4.0“).
- `PLAN.md`: Typabbildung Zeile „Prozedurale Typen“ (`i32`, Tabellenindex,
  4 Byte); Direktiven (`{$A}`); Weggelassen-Liste der Unit `Dos` anpassen
  (die vier Routinen zu Break und Verify).
- `docs/units.md`: `ExitProc`, `ErrorAddr`, `RunError`, prozedurale Typen
  in den Grundlagen.
- `docs/abweichungen.md`: Zeilen für Etappe 16 (Exit-Prozeduren bei
  Laufzeitfehlern nur mit unserem Runner und Loader, `ErrorAddr`-Wert,
  `{$A}` ohne Wirkung).

**Fertig, wenn:** Programme mit Prozedurvariablen, Funktionsparametern,
prozeduralen typisierten Konstanten und verketteten Exit-Prozeduren laufen
unter WASI, `gojs` und mit dem Browser-Compiler gleich; Exit-Prozeduren
laufen nach `Halt`, `RunError`, einem geprüften Laufzeitfehler und einem
Trap (z. B. 216, 202), auch mit `WriteLn` darin und mit einem
Laufzeitfehler in der Exit-Prozedur selbst; die Doku spricht von Turbo
Pascal 5.0; alle bisherigen Tests sind grün.

- *Erledigt (Nov 27, 2026).* Prozedurale Typen in AST (`ast.ProcType`), Parser, `types.ProcType`, Typprüfung (`autoCall`, `operand`, `procRef`, `check.CallIndirect`, `Info.ProcRefs`), IR (`CallIndirect`, `FuncSig`, `Module.Table`/`Sigs`), Analyse und Codegenerator (Table- und Element-Abschnitt, `call_indirect`, Index im Frame); `RunError`, `ExitProc`, `ErrorAddr`, `_Halt`/`_ErrorExit`/`Terminate` in `rtl/system.pas`, Export `_exit` mit Auswertung in `runner` und `web/loader.js`; `VER50`, Schalter `A`. Abweichungen von den Hinweisen oben: (1) Der Tabellenindex steht schon beim Lowering fest (Reihenfolge der ersten Verwendung), deshalb braucht `encode` keinen Platzhalter; (2) `_ErrorExit` kehrt nie zurück, sondern endet wie `Halt` über `SuspendHalt`; nach einem Laufzeitfehler kodiert der Exit-Code `ExitCode * 2 + 1` (Meldung) bzw. `ExitCode * 2` (`ErrorAddr = nil`), so wirken auch `Halt` in einer Exit-Prozedur und Anhaltepunkte ohne eigenes Protokoll; (3) das Programmende ist `_Halt(0)` (TP-ANNAHME: ein im Hauptprogramm gesetzter `ExitCode` zählt nicht); (4) `RunError` setzt Bit 16 der Fehlernummer, damit `RunError(0)` kein echter Trap ist; (5) Aufrufe als Anweisung auch über Index und Zeiger (`A[1];`, `P^;`), Error 91 sonst erst in der Typprüfung; (6) Error 142 für den Aufruf einer Variable, die keine Prozedurvariable ist (bisher 143), `@` und `Addr` auf verschachtelte Routinen ergeben 143 (bisher 83); (7) `Addr(Routine)` wie `@Routine`. Tests: `testdata/etappe16/` (14 Programme, 8 Fehlerfälle, WASI, `gojs`, Frame-Ablage), Golden-Tests `38_prozedurtypen` und `prozedurtypen.sym`, `TestMarkSuspendableIndirect`.

### Etappe 17: `Int64`, `external` und Laufzeit ohne Intrinsics ✅

**Ziel:** Programme können Funktionen des Hosts als `external` aufrufen,
und die Laufzeit erreicht WASI und `gojs` auf demselben Weg statt über die
34 fest eingebauten Intrinsics aus `compiler/internal/abi`. Neue
WASI-Aufrufe brauchen danach nur noch Pascal-Code. Für die
64-Bit-Parameter von WASI kommt der Typ `Int64` hinzu; `Comp` wird wie in
TP eine echte 64-Bit-Ganzzahl im Speicher.

**Entscheidungen (nach Rückfrage):**

- Neuer Typ `Int64` (aus Delphi 4). `Comp` wird im Speicher auf `i64`
  umgestellt; die Laufzeit benutzt `Int64`, nicht `Comp`.
- `external` wie in Borland Pascal 7/Delphi. Eigene externe Routinen in
  Programmen und Units gibt es nur für das Ziel WASI; unter `gojs` sind sie
  ein Fehler. Die Laufzeit selbst deklariert ihre WASI- **und** ihre
  `gojs`-Aufrufe mit `external` (Modul `gojs` bleibt der System-Unit
  vorbehalten).
- Nur einfache Typen an der Grenze (wie später bei `exports`, Etappe 19).
- `library` und `exports` folgen in Etappe 19, das Entfernen ungenutzten
  Codes in Etappe 18.

#### Der Typ `Int64`

| Thema | Festlegung |
| --- | --- |
| Wertebereich, Darstellung | −2⁶³..2⁶³−1; WASM-Wert `i64`, im Speicher 8 Byte, Ausrichtung 8 |
| Art | Ganzzahltyp und Ordinaltyp: Arithmetik (`+ - * div mod`, Vorzeichen), Bitoperationen (`and or xor not shl shr`), Vergleiche, `Inc`, `Dec`, `Succ`, `Pred`, `Odd`, `Abs`, `Sqr`, `Low`, `High`, `Ord` (Ergebnis `Int64`), `for`-Laufvariable. **Nicht** als Index- oder Basistyp von Arrays, Mengen und Teilbereichen (Error 29 „Ordinal type expected“) und nicht als `case`-Selektor (Error 39 „Ordinal expression expected“), wie bei Real-Typen. |
| Ergebnistyp | Ist ein Operand `Int64`, wird in 64 Bit gerechnet und das Ergebnis ist `Int64`; sonst bleibt alles wie heute (32 Bit). `shl`/`shr` auf `Int64` schieben modulo 64. |
| Zuweisung | Ganzzahl → `Int64` implizit (mit Vorzeichen erweitert); `Int64` → kleinere Ganzzahltypen implizit, abgeschnitten wie heute bei `LongInt` → `Byte` (keine Überlaufprüfung, wie `{$R-}`); `Int64` → Real-Typen implizit (ab 2⁵³ gerundet). Real → `Int64` nicht direkt: `Trunc` und `Round` liefern weiter `LongInt` (TP, Überlauf 207); für größere Werte geht der Weg über `Comp` (`C := X; I := C;`, gerundet). |
| Konstanten | Dezimalkonstanten über `MaxLongInt` sind `Int64`-Konstanten (bisher Real-Konstanten), bis 9223372036854775807; darüber weiter Real. Hexkonstanten mit bis zu 8 Ziffern bleiben `LongInt` (`$FFFFFFFF` = −1), mit 9 bis 16 Ziffern sind sie `Int64`. Konstantenausdrücke falten in 64 Bit, wenn ein Operand `Int64` ist; Überlauf ist Error 76. |
| Typumwandlungen | `Int64(x)` aus Ordinalwerten und Zeigern; `LongInt(i64)` usw. schneiden ab. |
| Laufzeitfehler | Division durch 0 → 200; `Low(Int64) div -1` → 200 (WASM-Trap, wie bei `LongInt`). |
| Ein-/Ausgabe | `Write`/`WriteLn` mit Breite, `Read`/`ReadLn`, `Str`, `Val` für `Int64` (in Pascal in `rtl/system.pas`); `file of Int64` speichert 8 Byte little-endian. |

**`Comp`:** bleibt wie in TP ein **Real-Typ** (Ausdrücke rechnen in
`f64`, `Write` gibt Exponentialschreibweise aus, `Comp / Comp` ist Real),
wird aber als `i64` gespeichert: Laden mit `f64.convert_i64_s`, Speichern
rundet zur nächsten Ganzzahl (bei Gleichstand zur geraden, wie die
8087-Voreinstellung) und ergibt außerhalb des Bereichs sowie für NaN den
Wert `$8000000000000000` („indefinite“ des 8087, `// TP-ANNAHME`).
`SizeOf(Comp)` bleibt 8; Records und typisierte Dateien mit `Comp`-Feldern
haben damit das Format von TP. `Comp` ↔ `Int64`: implizit in beide
Richtungen über den Ganzzahlwert (ohne Umweg über `f64`, wenn kein Rechnen
dazwischen liegt). Werte über 2⁵³ verlieren in Ausdrücken Stellen; in TP
rechnete der 8087 mit 64 Bit Mantisse (Abweichung).

#### `external`

```pascal
function HostRandom(Max: LongInt): LongInt; external 'host' name 'random';
procedure HostLog(P: Pointer; Len: LongInt); external 'host';  { Importname = Pascal-Name }
```

| Thema | Festlegung |
| --- | --- |
| Syntax | `external 'modul' [name 'importname'];` hinter dem Kopf einer Routine auf oberster Ebene eines Programms oder einer Unit (im `interface` einer Unit erlaubt, dann ohne Rumpf im `implementation`-Teil). Ohne `name` ist der Importname der Pascal-Name in der deklarierten Schreibweise. `external` ohne Modulname (TP-Form für `.OBJ`) bleibt Error 902. |
| Typen | Parameter und Ergebnis nur einfach: Ordinaltypen (außer `Int64`) und `Boolean` → `i32`; `Int64` und `Comp` → `i64`; `Single` → `f32`; `Real`, `Double`, `Extended` → `f64`; `Pointer`, typisierte Zeiger und `var`-Parameter → `i32` (Adresse). Strings, Records, Arrays, Mengen, Dateien und prozedurale Typen als Wertparameter oder Ergebnis sind ein Fehler (neue Nummer 909 „Type not allowed in external or exported routine“); als `var`- oder `const`-Parameter werden sie als Adresse übergeben. |
| Verhalten | Synchron, nie ein Anhaltepunkt; kein Static Link; Laufzeitfehler im Host sind Traps und laufen wie andere Traps über `_exit`. Adressierbar mit `@` und Prozedurvariablen: Die importierte Funktion steht direkt in der Funktionstabelle (WASM erlaubt Importe dort, die Signatur stimmt mit der des prozeduralen Typs überein); keine Hüllfunktion. Die Analyse der Anhaltefähigkeit behandelt Importe in der Tabelle als nie anhaltend. |
| Ein Import, mehrere Deklarationen | Derselbe Import (Modul + Name) darf mehrfach deklariert sein, wenn die WASM-Signatur gleich ist; sonst Error 910 „Conflicting external declaration“. Im Modul erscheint er einmal. |
| Ziele | Unter `gojs` ist `external` außerhalb der System-Unit Error 902. Das Modul `gojs` ist außerhalb der System-Unit immer Error 910. `wasi_snapshot_preview1` dürfen Programme benutzen (umgehen damit die Laufzeit; Doku). |
| Host (Go) | `runner.Options.Imports map[string]map[string]any`: Modul → Name → Go-Funktion mit passenden Parametern (`int32`/`uint32`, `int64`/`uint64`, `float32`, `float64`, optional `context.Context` und `runner.Memory` als erste Parameter). wazero bindet solche Funktionen direkt ein; der Speicherzugriff ginge dort nur über `api.Module`, deshalb gibt es eine eigene schmale Schnittstelle `runner.Memory` (Lesen und Schreiben von Bytes und Zahlen an einer Adresse), die der Runner an die Go-Funktion weitergibt. Keine wazero-Typen in der öffentlichen API. `wasi_snapshot_preview1` und `gojs` lassen sich dort nicht belegen. Fehlt ein Import, liefert `runner.Run` vor dem Start einen klaren Fehler (Modul, Name, Signatur). |
| CLI | `pascal run` stellt keine eigenen Importe bereit; ein Programm mit eigenem `external` scheitert dort mit dieser Meldung. `pascal build` übersetzt es normal. |

#### Laufzeit ohne Intrinsics

- `rtl/system.pas` deklariert alle Aufrufe aus `abi.WASI` und `abi.GoJS`
  selbst mit `external` (`wasi_snapshot_preview1` bzw. `gojs`), mit den
  heutigen Pascal-Namen (`_FdWrite`, `_PathOpen`, `_GoWasmWrite`, …), die
  `gojs`-Aufrufe unter `{$IFDEF GOJS}`. 64-Bit-Parameter (`fd_seek`,
  `fd_filestat_set_size`, `path_open`, `clock_time_get`, `fd_readdir`)
  werden `Int64`; die Aufrufer übergeben `Int64`-Werte statt erweiterter
  `LongInt`.
- `compiler/internal/abi` und die Intrinsic-Behandlung in `check` und `ir`
  (`BWasiBase`, `wasiHost`, `ExtS64`/`ExtU64` für Argumente) entfallen.
  Echte Intrinsics bleiben nur, wo es keinen Import gibt (`_MemGrow`,
  `_MemSize`, `_HeapBase`, Anhaltepunkte usw.).
- Die Reihenfolge der Importe darf sich ändern (Reihenfolge der ersten
  Verwendung); die Module bleiben deterministisch und byte-gleich zum
  Browser-Compiler. Ein Diff der erzeugten Module vor/nach der Umstellung
  (nach Normalisieren der Importreihenfolge) dient als Prüfung, dass sich
  sonst nichts ändert.

#### Umsetzung (Hinweise)

- IR und Codegenerator: `i64` als vollwertiger Werttyp (bisher nur für
  WASI-Argumente): Locals, Frame-Slots, `MemI64`, Arithmetik, Vergleiche,
  Umwandlungen `i32` ↔ `i64` ↔ `f64`, Konstanten; der Scheduler sichert
  `i64`-Locals im Frame wie die übrigen.
- `types`: `KInt64` als Ganzzahlart **innerhalb** von `KShortInt`..`KLongInt`
  einordnen oder `IsInteger` anpassen; `Comp` bleibt Real-Art, bekommt aber
  eine eigene Speicherdarstellung (`MemI64` + Umwandlung).
- `check`: `host()` macht heute jede Ganzzahl zu `LongInt`; für `Int64` muss
  es `Int64` bleiben, sonst rechnet `I64 + 1` in 32 Bit. Dasselbe gilt für
  `intResult`.
- `Comp` speichern: `f64.nearest` (Gleichstand zur geraden), dann eine
  eigene Prüfung auf NaN und den Bereich, weil weder `i64.trunc_f64_s`
  (Trap) noch die sättigende Variante „indefinite“ liefern; außerhalb wird
  `$8000000000000000` gespeichert.
- `check`: Regeln aus den Tabellen, Error 909/910, `external` im Parser
  (heute Error 902 bei `external`).
- Konstantenfaltung: 64-Bit-Überlaufprüfung für `Int64`, 32 Bit wie bisher
  für die übrigen.
- `runner`: `Options.Imports`, Prüfung fehlender Importe vor dem Start.

#### Tests

- `testdata/etappe17/int64.pas`: Arithmetik und Bitoperationen an den
  Grenzen (`High(Int64)`, `Low(Int64)`, Überlauf bei `+` ohne Prüfung),
  gemischte Ausdrücke mit `LongInt`, Zuweisung in beide Richtungen,
  Konstanten (dezimal, `$` mit 9–16 Ziffern), `Inc`/`Dec`, `for`-Schleife,
  `Write` mit Breite, `Str`/`Val`, `Read` (`.in`), `file of Int64`,
  Division durch 0 (200).
- `testdata/etappe17/comp.pas`: Rundung beim Speichern (0,5 → 0, 1,5 → 2,
  −2,5 → −2), Ausgabe wie bisher, `SizeOf`, `Comp` in Record und Datei
  (8 Byte Ganzzahl), Wert außerhalb (`$8000000000000000`).
- `testdata/etappe17/extern.pas`: Programm mit eigenen Importen; der
  Test-Harness stellt einen festen Satz Test-Importe (Modul `test`) über
  `runner.Options.Imports` bereit, Go-seitig im Harness (eine Textdatei
  kann keine Go-Funktionen beschreiben); Parameter aller erlaubten Typen,
  `var`-Parameter, Ergebnis `Int64`/`Real`, Aufruf über Prozedurvariable.
  Programme mit eigenen Importen laufen nur unter WASI: Eine Datei
  `name.wasi` (leer) neben dem Programm lässt `TestProgramsGoJS` es
  überspringen; `testdata/README.md` beschreibt das.
- Fehlertests: String als Wertparameter (909), Record als Ergebnis (909),
  widersprüchliche Deklaration (910), Modul `gojs` im Programm (910),
  `external` unter `gojs` (902), `external` ohne Modul (902), `Int64` als
  Array-Index und `case`-Selektor.
- `runner`: fehlender Import → Fehler vor dem Start; `Imports` mit
  reserviertem Modul → Fehler.
- Alle bisherigen Tests (WASI, `gojs`, Frame-Ablage, `TestWebCompiler`)
  bleiben grün; `pascal run` funktioniert nach der Umstellung unverändert.

#### Doku

- `PLAN.md`: Typabbildung (`Int64`; `Comp` als `i64` im Speicher),
  Tabelle der Erweiterungen (`Int64` aus Delphi 4, `external` aus BP 7),
  Weggelassen-Liste (`external` nur noch ohne Modulname).
- `CLAUDE.md`: Typtabelle, Pipeline (keine WASI-Intrinsics mehr, Laufzeit
  deklariert Importe selbst).
- `docs/units.md`: `Int64`; Abschnitt „Externe Routinen“ mit Beispiel in
  Pascal und Go.
- `docs/abweichungen.md`: Zeilen für Etappe 17 (`Comp`-Rundung und
  „indefinite“, Genauigkeit von `Comp` in Ausdrücken, Dezimalkonstanten
  über `MaxLongInt` jetzt `Int64`, `Trunc`/`Round` bleiben `LongInt`,
  `external`-Regeln); Zeile Etappe 6 „WASI-Aufrufe“ als überholt
  kennzeichnen.
- `README.md` (Umfang, Go-Beispiel mit `Imports`), `CHANGELOG.md`
  (Unreleased; **inkompatibel:** Dezimalkonstanten über `MaxLongInt`,
  Layout von `Comp`, `Comp` rundet beim Speichern: `C := 1.5` ergibt 2).

**Fertig, wenn:** `Int64` und `Comp` verhalten sich wie festgelegt; ein
Programm ruft eigene Host-Funktionen über `runner.Options.Imports` auf; die
Laufzeit enthält keine Intrinsics mehr für WASI und `gojs`, und alle
bisherigen Tests sind unter WASI, `gojs` und mit dem Browser-Compiler grün.

- *Erledigt (Nov 28, 2026).* `types.Int64` (`KInt64`, `IsInteger` reicht bis dahin), `host`/`intResult`/`compatible`/`isAssignable` für `Int64`, Konstanten und 64-Bit-Faltung (`foldInt` mit `math/big`); IR mit `i64` als vollwertigem Werttyp (`Int64Const`, `MemI64`, L-Operationen, `widen`/`wrap`), `MemComp` mit Rundung im Codegenerator (`compBits`: `f64.nearest` plus Bereichsprüfung); RTL: ein `_IntStr`/`_WriteInt` für alle Ganzzahlen (Parameter `Int64`), `_ValInt64` als Kern von `_ValInt`, `_ReadInt64`, gemeinsames `ReadNumber`. `external` in Parser, Typprüfung (909, 910, 902) und IR (`ir.HostCall`, Import je Modul + Name einmal, `Func.Host` für Tabelleneinträge, Hüllfunktion bei Single/Comp); `runner.Options.Imports` mit eigener Schnittstelle `runner.Memory` (Adapter per Reflection über `WithGoModuleFunction`), Prüfung aller Importe vor dem Start. `rtl/system.pas` deklariert alle 20 WASI- und 14 `gojs`-Aufrufe selbst; `compiler/internal/abi` ist entfernt. Abweichungen von den Hinweisen: Der Modulvergleich vor/nach der Umstellung entfiel, weil die RTL in derselben Etappe auf `Int64` umgestellt wurde und die Module sich deshalb ohnehin ändern; abgesichert ist die Umstellung durch alle bisherigen Tests (WASI, `gojs`, Frame-Ablage, `TestWebCompiler`). `Low(Int64) div -1` ergibt wie bei `LongInt` 207, nicht 200 (offene Frage). Tests: `testdata/etappe17/` (`int64`, `comp`, `extern` mit `extern.wasi`, 8 Fehlerfälle), `TestExternal`, `TestRunImports`, `ExampleRun_imports`, neue Fälle in `TestExprTypes`.

### Etappe 18: Ungenutzten Code entfernen ✅

**Ziel:** Ein Modul enthält nur die Routinen, die das Programm erreichen
kann. Heute landet die ganze Laufzeit in jedem Modul: „Hallo Welt“ hatte
vor Etappe 17 32,6 KB (27,5 KB mit `-nolines`), mit `uses Dos, Crt`
41,8 KB; die System-Unit allein hat rund 200 Routinen. Die Größen ändern
sich mit Etappe 17 und werden zu Beginn dieser Etappe neu gemessen. Das
Verhalten der Programme ändert sich nicht.

**Entscheidungen:**

- Entfernt werden nur ganze Routinen (samt den Stringkonstanten ihrer
  Rümpfe) und unbenutzte `external`-Deklarationen: Seit Etappe 17 sind auch
  die WASI- und `gojs`-Aufrufe der Laufzeit solche Deklarationen; ein
  Import erscheint nur, wenn eine erreichbare Routine ihn aufruft oder er
  in der Funktionstabelle steht. Globale
  Variablen bleiben: Sie liegen im BSS und kosten im Modul nichts.
  Typisierte Konstanten bleiben ebenfalls.
- Die Typprüfung bleibt vollständig: Fehler in ungenutzten Routinen meldet
  der Compiler weiterhin, wie TP.
- Immer an, ohne Schalter. Die Reihenfolge der verbleibenden Funktionen
  bleibt die heutige (Reihenfolge der Deklaration), damit die Module
  deterministisch und zwischen nativem und Browser-Compiler byte-gleich
  bleiben.
- Andere Optimierungen (etwa Variablen anhaltefähiger Funktionen, die über
  keinen Anhaltepunkt leben, in WASM-Locals) gehören nicht dazu.

#### Erreichbarkeit

Wurzeln sind:

- das Hauptprogramm (es ruft die Initialisierungen der Units auf),
- die Routinen, die der Codegenerator selbst aufruft: `SuspendAction`,
  `ResumeAction` und `ErrorExit` (für `_start`, `run`, `resume` und
  `_exit`),
- jede Routine der Funktionstabelle, also jede, deren Adresse genommen
  wird (`@P`, `Addr`, Zuweisung an eine Prozedurvariable, typisierte
  Konstanten); indirekte Aufrufe brauchen dann keine eigene Behandlung.
- ab Etappe 19 jede exportierte Routine einer Bibliothek. Die Wurzeln
  deshalb als erweiterbare Liste anlegen.

Erreichbar ist, was von einer Wurzel aus über `ir.Call` aufgerufen wird.
Verschachtelte Routinen werden nur über ihre umschließende Routine oder
Geschwister erreicht und fallen mit ihr weg.

#### Umsetzung in zwei Schritten

1. **Funktionen streichen:** Nach `ir.Lower` bestimmt `analysis` die
   erreichbaren Funktionen; `Module.Funcs` behält nur diese, in der
   bisherigen Reihenfolge. Name Section, `wasmpascal.lines`, Importe
   (in Reihenfolge der ersten Verwendung) und Funktionsindizes richten sich
   danach. Die Stringkonstanten gestrichener Rümpfe bleiben in diesem
   Schritt noch im Datensegment.
2. **Nur erreichbare Rümpfe übersetzen:** `ir.Lower` legt weiter für jede
   Routine eine `Func` an (Signaturen braucht jeder Aufruf), übersetzt
   Rümpfe aber über eine Arbeitsliste erst, wenn ein Aufruf oder ein
   Tabelleneintrag sie erreicht. Dann entfallen auch die Konstanten toter
   Rümpfe, und das Übersetzen wird schneller (Browser-Compiler,
   `pascal serve`). Die Reihenfolge in `Module.Funcs` bleibt die der
   Deklaration, nicht die der Arbeitsliste.

#### Tests

- Größe: „Hallo Welt“ und ein Programm mit `uses Dos, Crt` werden
  deutlich kleiner; ein Test hält eine Obergrenze fest (Wert beim
  Umsetzen messen und im Bericht nennen).
- Eine ungenutzte Routine fehlt in der Name Section, eine über `@` oder
  eine typisierte Konstante adressierte ist da; eine ungenutzte Routine mit
  Typfehler ergibt weiterhin den Fehler.
- Ein Programm ohne Dateizugriff importiert keine `path_*`-Aufrufe.
- Alle bisherigen Tests (WASI, `gojs`, Frame-Ablage, `TestWebCompiler`)
  bleiben grün.

#### Doku

- `PLAN.md`: Bewusste Vereinfachungen (Satz zu Optimierungen), Etappe
  abhaken.
- `CLAUDE.md`: Pipeline um den Schritt Erreichbarkeit ergänzen.
- `README.md`: Größe kleiner Module erwähnen, falls sie dort vorkommt;
  `CHANGELOG.md` (Unreleased).

**Fertig, wenn:** kleine Programme deutlich kleinere Module ergeben, der
Größentest und alle bisherigen Tests grün sind und die Module weiterhin
deterministisch und byte-gleich zum Browser-Compiler sind.

- *Erledigt (Nov 28, 2026).* `analysis.Reachable`/`Prune` (Wurzeln: Hauptprogramm,
  `SuspendAction`/`ResumeAction`/`ErrorExit`, Funktionstabelle; Kanten:
  `ir.Call`), aufgerufen in `compiler.Compile` zwischen `ir.Lower` und
  `codegen.Generate`. `ir.Lower` übersetzt Rümpfe lazy über eine Arbeitsliste
  (`pending`/`enqueue`/`drain`; `emit` und `tableIndex` tragen Aufrufe und
  Tabelleneinträge nach), sodass auch die Stringkonstanten toter Rümpfe
  entfallen. Gemessen (windows/amd64): „Hallo Welt“ 33.363 → 4.545 Byte
  (166 → 24 Funktionen, 20 → 4 WASI-Importe, keine `path_*` mehr), mit
  `uses Crt, Dos` 42.539 → 10.100 Byte (210 → 50 Funktionen). Tests:
  `TestReachablePrune` (analysis), `TestPruneModuleSize`/`TestPruneImports`/
  `TestPruneNameSection`/`TestPruneDeadCodeStillChecked` (compiler),
  `testdata/etappe18/` (toter Typfehler, adressierte Routine). Globale
  Variablen und typisierte Konstanten bleiben wie geplant; die Typprüfung
  läuft unverändert vor dem Lowering.

### Etappe 19: Bibliotheken mit `library` und `exports`

**Ziel:** Aus Pascal-Quelltext entsteht ein WASM-Modul, dessen Routinen ein
Host (Go, `node:wasi`, wasmtime …) aufrufen kann. Baut auf `Int64` und den
Typregeln für `external` (Etappe 17) und auf der Erreichbarkeit (Etappe 18)
auf: Exportierte Routinen sind zusätzliche Wurzeln.

**Entscheidungen (nach Rückfrage):**

- Exporte gibt es nur in Bibliotheken (`library`), nicht in Programmen:
  Ein Programm endet nach dem Hauptprogramm über `proc_exit`, danach ließe
  sich nichts mehr aufrufen.
- Nur das Ziel WASI. `library` mit `-target gojs` ist Error 902.
- Nur einfache Typen an der Grenze, dieselben wie bei `external`
  (Etappe 17, Error 909); keine strukturierten Funktionsergebnisse.

#### Sprache

```pascal
library Mathe;

type
  PLongInt = ^LongInt;

function Quadrat(X: LongInt): LongInt;
begin
  Quadrat := X * X
end;

function Summe(P: Pointer; N: LongInt): Int64;
var
  I: LongInt;
begin
  Result := 0;
  for I := 0 to N - 1 do
    Result := Result + PLongInt(LongInt(P) + 4 * I)^
end;

exports
  Quadrat,
  Quadrat name 'square',
  Summe;

begin
  { Initialisierung, läuft einmal in _initialize }
end.
```

| Thema | Festlegung |
| --- | --- |
| Kopf | `library Name;` statt `program Name;`. `library` und `exports` bleiben normale Bezeichner; der Parser erkennt sie nur am Anfang der Datei bzw. als Abschnitt im Hauptblock, wie Standarddirektiven. |
| `exports` | Ein oder mehrere Abschnitte im Deklarationsteil der Bibliothek (nach den Routinen, die sie nennen), wie in BP 7/Delphi. Einträge `Routine [name 'exportname']`, durch Komma getrennt; dieselbe Routine darf unter mehreren Namen exportiert werden. Ohne `name` ist der Exportname der Pascal-Name in der deklarierten Schreibweise. Routinen aus Units dürfen exportiert werden (`Dos.GetEnv` geht nicht: Strings, Error 909). |
| Erlaubt | Routinen auf oberster Ebene einer Bibliothek oder Unit, auch `external` (dann wird der Import weitergereicht). Verschachtelte Routinen → Error 143. |
| Namen | Exportnamen sind eindeutig und dürfen nicht mit den internen Exporten kollidieren (`memory`, `_initialize`, `_exit`, `__rt_*`, alles mit `_` am Anfang); sonst neue Nummer 911 „Duplicate or reserved export name“. |
| Typen | wie bei `external` (Etappe 17); `var`/`const`-Parameter mit strukturiertem Typ als Adresse im dokumentierten Layout (z. B. String: Längenbyte + Bytes). Speicher für den Host stellt die Bibliothek bei Bedarf selbst bereit (eigene exportierte Routinen mit `GetMem`/`FreeMem`). |
| Hauptblock | `begin … end.` der Bibliothek läuft in `_initialize`, nach den Initialisierungsteilen der Units, genau einmal. |

#### Modul (WASI Reactor)

- Exporte: `memory`, `_initialize` statt `_start`, `_exit` und die
  `__rt_*`-Globalen wie bisher, dazu je Exporteintrag eine Funktion.
- `_initialize` führt Laufzeit- und Unit-Initialisierung und den Hauptblock
  aus und kehrt zurück (kein `proc_exit`). Der Host ruft es einmal vor allen
  anderen Exporten (WASI-Reactor-Konvention).
- **Anhalten:** Erreicht eine exportierte Routine einen Anhaltepunkt
  (`ReadLn`, `Delay`, lange Ausgabe), bekommt sie eine Hülle, die wie heute
  `_start` die Fortsetzung in einer Schleife ausführt; für den Host bleibt
  der Aufruf synchron. Dasselbe gilt für `_initialize`.
- **Programmende:** In einer Bibliothek endet der Hauptblock nicht mit
  `_Halt(0)` wie bei Programmen (Etappe 16); `_initialize` kehrt nach dem
  Hauptblock zurück.
- **Ausgabe:** `Output` ist gepuffert und wird bei Programmen erst am Ende
  geschrieben (`_Done`). Die Hülle jedes Exports und `_initialize` leeren
  `Output` vor der Rückkehr zum Host, sonst sähe der Host die Ausgabe nicht.
- **Aufrufe zurück in Pascal:** Ruft eine Host-Funktion (`external`)
  während eines Exports wieder einen Export auf, darf die Hülle den
  Schatten-Stack nicht auf den Anfang zurücksetzen (wie `_start` es beim
  Fortsetzen tut), sonst überschreibt sie die Frames des äußeren Aufrufs.
  Die Hülle merkt sich den Schatten-Stack und den Zustand des Schedulers
  beim Eintritt und setzt beim Fortsetzen darauf zurück; beim Verlassen
  stellt sie beides wieder her. Ein innerer Export, der anhält, während der
  äußere noch läuft, wird so von der eigenen Hülle fortgesetzt. Test dazu.
- **Exit-Prozeduren:** Sie laufen nur nach `Halt` und nach Laufzeitfehlern,
  nicht bei `Library.Close` (eine Reactor-Instanz wird nicht „entladen“).
  In BP 7 liefen sie beim Entladen der DLL; Abweichung, in
  `docs/abweichungen.md`.
- **`Halt` und Laufzeitfehler** in einer exportierten Routine oder in
  `_initialize`: Die Exit-Prozeduren laufen wie bei Programmen, die Meldung
  „Runtime error …“ geht an stderr, dann endet die Instanz über
  `proc_exit`. Danach ist sie unbrauchbar; der Host bekommt einen Fehler
  (in Go: `*runner.RuntimeError` bzw. ein Fehler mit Exit-Code).
  `// TP-ANNAHME`: In BP 7 beendete `Halt` in einer DLL den ganzen Prozess.

#### Go-API

```go
lib, err := runner.Load(ctx, res.Wasm, runner.Options{Stdout: os.Stdout})
if err != nil { … }            // auch Fehler aus _initialize
defer lib.Close(ctx)

r, err := lib.Call(ctx, "square", 7)   // []uint64 wie die WASM-Werte
fmt.Println(int32(r[0]))               // 49
```

- `runner.Load(ctx, wasm, opts) (*Library, error)`: instanziiert, ruft
  `_initialize`; `Options` wie bei `Run` (Stdin/Stdout/Stderr, Dirs, Env,
  Imports aus Etappe 17). Ein Modul ohne `_initialize` (also ein Programm)
  ist ein Fehler, ebenso `runner.Run` mit einer Bibliothek.
- `(*Library).Call(ctx, name string, args ...uint64) ([]uint64, error)`:
  Werte wie in wazero (`api.EncodeI32`, `EncodeF64` …) — beim Umsetzen
  prüfen, ob kleine Hilfsfunktionen in `runner` (`runner.I32(x)`,
  `runner.F64(x)`) ohne wazero-Typen in der API besser sind.
- `(*Library).Memory() []byte` (Sicht auf den linearen Speicher, gültig bis
  zum nächsten Aufruf), `Exports() []string`, `Close(ctx)`.
- Nebenläufigkeit: Eine `Library` ist nicht für gleichzeitige Aufrufe
  gedacht (eine Instanz, ein Speicher); dokumentieren, ein Mutex schützt
  vor Datenrennen.

#### CLI

- `pascal build mathe.pas` erzeugt bei einer Bibliothek `mathe.wasm` wie
  gewohnt; `-target gojs` → Error 902.
- `pascal run` mit einer Bibliothek: Fehler „mathe.pas is a library; use
  pascal build“. Kein eigener Befehl zum Aufrufen einzelner Exporte in
  dieser Etappe.

#### Tests

- `testdata/etappe19/`: Bibliothek mit Exporten aller erlaubten Typen
  (`LongInt`, `Int64`, `Real`, `Single`, `Boolean`, `Char`, `Pointer`,
  `var`-Parameter), Umbenennung mit `name`, doppelter Export derselben
  Routine, Export aus einer Unit, Initialisierung mit Ausgabe, Export, der
  `Delay` benutzt, Export mit Laufzeitfehler (Exit-Prozedur läuft, danach
  Fehler beim Host). Neuer Harness-Zweig: Bibliotheken werden über
  `runner.Load` geladen und nach einer Aufrufliste (`*.calls`) aufgerufen,
  Ergebnisse und Ausgaben gegen `*.out` verglichen. Format von `*.calls`:
  je Zeile ein Aufruf `exportname arg …`, Argumente als Pascal-Literale
  passend zur Signatur des Exports (Ganzzahlen dezimal oder `$hex`, Real
  mit Punkt, `True`/`False`, `'c'` für `Char`); leere Zeilen und Zeilen mit
  `#` am Anfang zählen nicht. Der Harness schreibt nach den Ausgaben des
  Aufrufs eine Zeile `= ergebnis` (nach Ergebnistyp formatiert, bei
  Prozeduren `=`) bzw. bei einem Fehler die Meldung, und bricht danach ab.
  Ein Export mit `var`-Parameter bekommt im Test eine Adresse aus einem
  eigenen Export der Bibliothek (`GetMem`).
- Fehlertests: `exports` in einem Programm, verschachtelte Routine (143),
  String-Parameter (909), Record-Ergebnis (909), doppelter und reservierter
  Exportname (911), `library` mit `-target gojs` (902).
- Ist `node` installiert: dieselbe Bibliothek mit `node:wasi` als Reactor
  laden (`wasi.initialize(instance)`), einen Export aufrufen.
- Erreichbarkeit (Etappe 18): eine nicht exportierte, ungenutzte Routine
  fehlt im Modul, eine exportierte ist da.
- `runner`: `Load` mit Programm → Fehler, `Run` mit Bibliothek → Fehler,
  `Call` mit unbekanntem Namen und falscher Argumentzahl → Fehler.

#### Doku

- `PLAN.md`: Tabelle der Erweiterungen (`library`/`exports` aus BP 7/
  Delphi), Etappe abhaken.
- `README.md`: Abschnitt „Bibliotheken“ mit dem Pascal-Beispiel oben und
  dem Go-Aufruf; Hinweis auf `node:wasi` als Reactor.
- `docs/units.md` oder neues `docs/bibliotheken.md`: Aufrufkonvention,
  Typen, Layout von Strings und Records, Initialisierung, Fehlerverhalten.
- `docs/abweichungen.md`: Zeilen für Etappe 19 (nur WASI, Verhalten von
  `Halt` und Laufzeitfehlern, Exportnamen).
- `CLAUDE.md`, `CHANGELOG.md`.

**Fertig, wenn:** eine Bibliothek lässt sich übersetzen, über
`runner.Load` laden und ihre Exporte aus Go aufrufen (auch solche, die
anhalten); Laufzeitfehler kommen als Fehler beim Host an; die Bibliothek
läuft auch mit `node:wasi`; alle bisherigen Tests sind grün.

## Arbeitsregeln für Claude Code

- **Eine Etappe nach der anderen.** Am Ende jeder Etappe: Tests grün, `go vet` sauber, `gofmt` angewendet, Commit mit aussagekräftiger Nachricht, kurzer Bericht (was geht, was fehlt, offene Fragen). Dann auf Freigabe warten.
- **Tests zuerst denken.** Jede neue Sprachfunktion bekommt mindestens ein Pascal-Testprogramm in `testdata/` mit erwarteter Ausgabe. Golden-Files nur mit `-update` neu schreiben und die Diffs prüfen.
- **Abhängigkeiten:** nur Go-Standardbibliothek im Paket `compiler`; wazero nur in `runner` und `cmd`. Nichts weiteres ohne Rückfrage.
- **WASM-Tauglichkeit des Compilers:** kein `os/exec`, keine cgo-Abhängigkeiten, Dateizugriff im Compiler nur über ein `fs.FS`-artiges Interface, damit der Browser Quelltexte aus dem Speicher liefern kann. Beide `GOOS=js`- und `GOOS=wasip1`-Builds in jedem Test-Lauf prüfen.
- **Fehlermeldungen:** Format `datei.pas(zeile,spalte): Error NN: Text`, angelehnt an TP. Abbruch beim ersten Fehler.
- **Codegenerator von Anfang an auf den Scheduler auslegen:** IR immer als CFG; jede Variable muss sich wahlweise in den Frame auslagern lassen. Auch wenn die Zustandsmaschinen erst in Etappe 8 kommen.
- **Keine eigenmächtigen Erweiterungen** des Sprachumfangs. Im Zweifel das Verhalten von TP 5.0 nachbilden und Abweichungen in `docs/abweichungen.md` festhalten.
- **Keine Raterei bei TP-Verhalten:** Wenn das genaue Verhalten unklar ist (Ausgabeformat, Randfälle), eine plausible Annahme treffen, im Code mit `// TP-ANNAHME:` markieren und im Bericht nennen.
- **Laufzeit in Pascal bevorzugen.** Neue Laufzeitfunktionen zuerst in `rtl/*.pas` schreiben; nur wenn das nicht geht, als Intrinsic in Go/WASM.
- **Sprache im Code** (seit Nov 28, 2026): Bezeichner englisch, im Go-Code wie in `rtl/*.pas`; Kommentare und Doku bleiben deutsch. Ausgenommen sind die Pascal-Testprogramme in `testdata/` und die Beispiele unter `web/beispiele/`; öffentliche Namen der Units sind ohnehin die englischen TP-Namen.
- **RTL ohne Dopplungen:** Was mehrere Units brauchen, steht einmal in `rtl/system.pas` im Interface (Name mit `_`, für Programme nicht gedacht). Kein `{$I}` für gemeinsamen Code: Eingebundener Code wird je Unit erneut übersetzt; werden mehrere Kopien benutzt, stehen sie alle im Modul.
- **`PLAN.md` und `docs/abweichungen.md` pflegen:** erledigte Etappen abhaken, Entscheidungen kurz festhalten.
