# Projektplan WasmPascal: Turbo-Pascal-4.0-Compiler nach WASM in Go

Nov 24, 2026 · @Christian Winkler

## Ziel und Rahmenbedingungen

Wir bauen in Go **WasmPascal**, einen Compiler, der Turbo-Pascal-4.0-Quelltext in WebAssembly übersetzt. Modulpfad: `github.com/lehrcode/wasmpascal`; die CLI heißt `pascal`. Der Compiler läuft nativ und lässt sich selbst nach WASM kompilieren. Einen eigenen Interpreter gibt es nicht: `pascal run` kompiliert und führt das Modul sofort in wazero aus, im Browser übernimmt die Engine des Browsers.

**Sprachumfang:** Turbo Pascal 4.0 ohne OOP, plus die unten genannten Erweiterungen aus TP 7 und Delphi. Das umfasst Units, Records (auch variante), Aufzählungen, Teilbereiche, Mengen, Zeiger, ShortStrings, typisierte und untypisierte Dateien, verschachtelte Prozeduren und `var`-Parameter. Ziel ist weitgehende Quelltext-Kompatibilität, aber keine Binär- und keine Layout-Kompatibilität.

**Bewusste Vereinfachungen:**

- `goto` und `label` werden ab Etappe 8 unterstützt, über dieselbe Dispatch-Schleife wie der Scheduler (siehe dort). Nicht-lokale Sprünge aus einer Prozedur heraus erlaubt TP ohnehin nicht.
- Das Speicherlayout ist an WASM angepasst, nicht byte-genau wie unter DOS. `SizeOf` liefert die neuen Größen.
- Die Datentypen sind auf WASM abgebildet (siehe Abschnitt Typabbildung). Es gibt keine 16-Bit-Überlaufsemantik.
- Der Compiler bricht beim ersten Fehler ab, wie das Original. Es gibt keine Fehlerkorrektur im Parser.
- Das ganze Programm wird auf einmal kompiliert. Units werden immer aus dem Quelltext übersetzt, ohne `.TPU`-Dateien.
- Es gibt keine Optimierungen. Konstante Ausdrücke werden nur dort ausgewertet, wo die Sprache es verlangt.
- Laufzeitfehler beenden das Programm mit `Runtime error NNN at datei.pas(zeile) in Prozedur`, also mit Quelltextposition statt Adresse (siehe Abschnitt Laufzeitfehler mit Zeilennummern).

**Weggelassen:** `Graph` (BGI), `Printer`, `Overlay`, `Turbo3`, `Graph3`, `inline(...)`, `external` und `.OBJ`-Linking, `interrupt`-Prozeduren, `absolute`, `Mem[]`, `Port[]`, `Intr`/`MsDos` und aus `Dos` alles außer Datum, Uhrzeit, `ParamStr`/`ParamCount`. `Sound`/`NoSound` werden zu Nicht-Operationen. Aus Crt entfallen GotoXY, WhereX, WhereY, Window und KeyPressed; ReadKey arbeitet nur im Zeilenmodus. Es gibt keinen Schatten-Bildschirm, kein Cursor-Tracking und kein Raw-Mode-Terminal. Die alte Windows-Konsole ohne ANSI-Unterstützung wird nicht unterstützt.

**Direktiven:** Unterstützt werden `{$I datei}`, `{$I-}`/`{$I+}` mit `IOResult` sowie `{$DEFINE}`, `{$UNDEF}`, `{$IFDEF}`, `{$IFNDEF}`, `{$IFOPT}`, `{$ELSE}` und `{$ENDIF}` (`{$UNDEF}` und `{$IFOPT}` seit Etappe 1, weil TP 4.0 sie kennt). Von {$M} wird nur die Stackgröße ausgewertet (siehe Größengrenzen). Alle anderen werden ignoriert. Boolesche Ausdrücke werden immer mit Kurzschlussauswertung berechnet (entspricht `{$B-}`).

**Reservierte Wörter:** die von TP 4.0, zusätzlich immer `object`, `constructor`, `destructor` und `inherited`. Sie sind für eine spätere OOP-Erweiterung im Stil von TP 5.5/6.0 vorgemerkt, haben in diesem Plan aber noch keine Bedeutung; ihre Verwendung als Bezeichner ist ein Compilerfehler. Alle anderen später eingeführten Schlüsselwörter (`asm`, `library`, `exports`) bleiben normale Bezeichner.

**Zeichensatz: durchgehend UTF-8, wie in Go.** Strings sind Byte-Folgen, die normalerweise UTF-8 enthalten. Konsole, Textdateien und Quelltext reichen die Bytes unverändert durch; es gibt keine Codepage-Umwandlung.

- Quelltext muss gültiges UTF-8 sein, sonst Compilerfehler mit Hinweis auf `iconv -f CP437 -t UTF-8`. Ein BOM am Anfang wird übersprungen.
- Bezeichner und Schlüsselwörter bestehen nur aus ASCII; in String-Literalen und Kommentaren ist beliebiges UTF-8 erlaubt.
- `string` bleibt der ShortString von TP: Längenbyte plus höchstens 255 **Bytes**. `Length` zählt Bytes (`Length('Größe')` = 7), `s[0]` bleibt das Längenbyte. Es gibt keine längeren oder dynamischen Strings.
- `Char` ist ein Byte. Ein Literal in Hochkommas, das mehr als ein Byte lang ist (`'ä'`), ist ein String und kein `Char`.
- `Copy`, `Delete`, `Insert` und Indizierung arbeiten auf Bytes und können ein Mehrbyte-Zeichen zerschneiden.
- `UpCase` und die Groß-/Kleinschreibung von Bezeichnern betreffen nur a–z.
- `#nnn` und `Chr(n)` über 127 erzeugen das rohe Byte. Alte Programme, die Rahmen mit CP437-Nummern zeichnen (`#196`, `Chr(219)`), geben deshalb ungültiges UTF-8 aus; das ist eine bewusste Abweichung.
- Textdateien (`Text`) werden als UTF-8 gelesen und geschrieben, ohne Umwandlung; typisierte und untypisierte Dateien sind ohnehin Rohbytes.

**Erweiterungen aus späteren Versionen:** Diese Sprachmittel kommen über TP 4.0 hinaus dazu. Keines braucht ein neues reserviertes Wort; `Result`, `Break`, `Continue`, `Low` und `High` sind vordefinierte Bezeichner.

| Erweiterung | Herkunft | Regeln |
| --- | --- | --- |
| `Result` | Delphi | In jeder Funktion implizit deklarierte Variable vom Ergebnistyp. Gleichwertig zur Zuweisung an den Funktionsnamen; beides darf gemischt werden. Eine eigene Deklaration namens `Result` in einer Funktion ist ein Fehler. |
| Schreibgeschützte Konstanten | Delphi, `{$J-}` | Typisierte Konstanten sind immer schreibgeschützt: keine Zuweisung, keine Übergabe als `var`-Parameter, kein `@`. Einen Schalter `{$J+}` gibt es nicht. |
| `Break`, `Continue` | TP 7 | Nur innerhalb von `for`, `while`, `repeat` und `for … in`; sonst Compilerfehler. |
| `Low`, `High` | TP 7 | Für Ordinaltypen und deren Variablen, Arrays (Indexgrenzen) und Strings (`Low` = 0, `High` = deklarierte Maximallänge in Bytes). Immer zur Übersetzungszeit bekannt. |
| `const`-Parameter | TP 7 | Im Rumpf schreibgeschützt. Records, Arrays, Strings und Mengen werden per Adresse übergeben, ohne Kopie; Skalare per Wert. |
| Funktionsaufruf als Anweisung | TP 6 (`{$X+}`), Delphi | Immer erlaubt, ohne Schalter: Das Ergebnis eigener Funktionen wird verworfen. Wie bei `{$X+}` gilt das nicht für Standardfunktionen der System-Unit. |
| Strukturierte Funktionsergebnisse | Delphi 1.0 | Funktionen dürfen alle Typen außer Dateien zurückgeben, also auch Records, Arrays und Mengen. Der Ergebnistyp ist ein Typbezeichner. Felder und Elemente des Ergebnisses setzt man über `Result`; `F.x` im Rumpf ist ein rekursiver Aufruf. |
| `for … in` | Delphi 2005 | Über Arrays (alle Elemente in Indexreihenfolge), Strings (Bytes als `Char`) und Mengen (enthaltene Elemente aufsteigend). Die Schleifenvariable ist im Rumpf schreibgeschützt. Keine Enumeratoren, da es keine Klassen gibt. |

Bewusst nicht aufgenommen: offene Array-Parameter (`array of T`) und Standardwerte für Parameter.

**Abweichungen dadurch:**

- TP-4.0-Programme, die typisierte Konstanten verändern (oft als „statische lokale Variable“ benutzt), kompilieren nicht mehr und müssen auf globale Variablen umgestellt werden.
- Ein eigener Bezeichner `Result` innerhalb einer Funktion kollidiert mit dem impliziten; außerhalb von Funktionen bleibt er nutzbar. Eigene Deklarationen namens `Break`, `Continue`, `Low` oder `High` überdecken die eingebauten, wie in TP 7.

## Architektur und Repo-Struktur

Eine Pipeline mit einem einzigen Backend: Quelltext → Lexer → Parser → AST → Typprüfung → IR als Kontrollflussgraph → WASM-Codegenerator → eigener Binär-Encoder → `.wasm`. Die Laufzeitbibliothek ist zum größten Teil in Pascal geschrieben und wird wie eine normale Unit mitübersetzt.

**Zwei Ziel-ABIs mit gemeinsamer Laufzeit:**

- `wasi`: WASI Preview 1 (`fd_write`, `fd_read`, `poll_oneoff`, `clock_time_get`, `random_get`, `path_open` usw.). Läuft in wazero, wasmtime und Node.
- `gojs`: dieselbe Import- und Exportschnittstelle wie `GOOS=js GOARCH=wasm` (Importmodul `gojs`, Exporte `run`, `resume`, `getsp`, `mem`). Läuft mit `wasm_exec.js` im Browser. Die Go-Version, deren `wasm_exec.js` wir unterstützen, wird festgelegt und die Datei mitgeliefert.

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
    abi/            Ziele wasi und gojs
rtl/                Laufzeit in Pascal: system.pas, crt.pas, dos.pas; rtl.go exportiert rtl.FS (go:embed)
runner/             Öffentliche API: Run, Options; wazero-Host, Dateisystem
web/                Browser-Demo: index.html, Loader, xterm.js-Anbindung
testdata/           Pascal-Testprogramme mit erwarteter Ausgabe (*.pas + *.out)
```

&#91;embedded content: Compiler-Pipeline · 8 Stufen, 2 Ziele\]

Die Laufzeit-Units aus `rtl/` laufen durch dieselbe Pipeline wie das Anwenderprogramm; erst die letzte Schicht der Laufzeit unterscheidet zwischen `wasi` und `gojs`.

## Typabbildung und Speichermodell

Alle skalaren Typen werden auf WASM-Werttypen abgebildet. Aggregate liegen im linearen Speicher mit natürlicher Ausrichtung.

| Turbo Pascal | In WASM-Werten | Im Speicher |
| --- | --- | --- |
| Byte, ShortInt, Char, Boolean | `i32` | 1 Byte; jede Zuweisung kürzt auf 8 Bit |
| Integer | `i32` | 4 Byte, Bereich wie LongInt (`MaxInt` = 2147483647) |
| Word | `i32` | 4 Byte, Bereich 0..65535 (kein 16-Bit-Überlauf) |
| LongInt | `i32` | 4 Byte |
| Aufzählungen, Teilbereiche | `i32` | 1 Byte, wenn alle Werte in 0..255 bzw. −128..127 passen, sonst 4 Byte |
| Real, Double, Extended, Comp | `f64` | 8 Byte |
| Single | `f32` | 4 Byte |
| Zeiger, `nil` | `i32` | 4 Byte |
| `string[n]` / `string` | Adresse (`i32`) | immer 256 Byte: Längenbyte + 255 Bytes UTF-8; beim Zuweisen auf `n` gekürzt |
| `set of T` | Adresse (`i32`) | immer 32 Byte (256 Bit); Operationen mit `v128` |
| Records, Arrays | Adresse (`i32`) | natürliche Ausrichtung; variante Teile überlagern sich wie eine Union |
| Prozedurale Typen | gibt es in 4.0 nicht | – |
| Dateivariablen | Adresse (`i32`) | Laufzeit-Record (Handle, Modus, Puffer) |

**Festlegungen zu Wertebereichen (nach Etappe 1):**

- `Integer` hat den Bereich von `LongInt`; `MaxInt` = `High(Integer)` = 2147483647. `Word` bleibt 0..65535; Konstanten außerhalb davon sind Compilerfehler, gerechnet wird aber in 32 Bit ohne 16-Bit-Kürzung.
- `Byte`, `ShortInt` und `Char` werden bei jeder Zuweisung auf 8 Bit gekürzt (`ShortInt` mit Vorzeichenerweiterung), egal ob die Variable in einem WASM-Local oder im Speicher liegt. `b := b + 1` ergibt bei `b = 255` also immer 0, wie in TP mit `{$R-}`. Teilbereiche mit 1 Byte Speicher werden ebenso gekürzt.
- Die Speichergröße eines Teilbereichs richtet sich nach dem Wertebereich, nicht nach der Anzahl der Werte: `1000..1100` belegt 4 Byte.

**Speicheraufteilung (linearer Speicher):**

1. Ab Adresse 0 eine freie Seite, die nie belegt wird. WASM kennt keinen Speicherschutz; deshalb prüft jede Dereferenzierung `p^` den Zeiger und bricht bei `nil` mit `Runtime error 216` ab.
2. Globale Variablen und typisierte Konstanten (Datensegment).
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

Elf Etappen (0 bis 10). Jede endet mit grünen Tests und einem Commit; die nächste beginnt erst danach. Nach jeder Etappe kurz berichten und auf Freigabe warten.

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
- `rtl/crt.pas` über ANSI: `ClrScr`, `ClrEol`, `TextColor`/`TextBackground` (CGA-Farbtabelle → ANSI, 8–15 als helle Farben, +16 = Blinken), `TextAttr`, `HighVideo`/`LowVideo`/`NormVideo`, `InsLine`/`DelLine`, `Delay` und `ReadKey` im Zeilenmodus: `ReadKey` wartet, bis eine Zeile mit Enter abgeschlossen ist, und liefert deren Zeichen dann einzeln, Enter als `#13`; Sondertasten gibt es nicht. Nicht enthalten sind `GotoXY`, `WhereX`, `WhereY`, `Window` und `KeyPressed`; ihre Verwendung ergibt den normalen Fehler für unbekannte Bezeichner.
- Kein Schatten-Bildschirm und kein Cursor-Tracking: Die Ausgabe geht direkt als Text und ANSI-Sequenzen ans Terminal, Umbruch und Scrollen übernimmt das Terminal. Alle Bytes gehen unverändert durch (UTF-8, siehe Zeichensatz).
- Nativer Runner: Das Terminal bleibt im normalen Zeilenmodus, Echo und Zeilenbearbeitung übernimmt das Terminal. Die CLI fängt Strg+C mit `os/signal` ab und gibt `ESC[0m` aus, damit keine Farbe hängen bleibt. Unterstützt werden Terminals mit ANSI-Unterstützung (Linux, macOS, Windows Terminal); die alte Windows-Konsole ohne VT-Modus wird nicht unterstützt.
- **Fertig, wenn:** Ein kleines Zahlenraten `zahlen.pas` (selbst geschrieben: farbige Ausgabe, `ClrScr`, Eingaben mit `ReadLn`, eine kurze Animation mit `Delay`, „Weiter mit Enter“ über `ReadKey`) im Terminal korrekt läuft. Außerdem besteht ein Testprogramm mit goto (Sprung aus verschachtelten Schleifen und zurück nach oben).
- *Erledigt (Nov 24, 2026).* `analysis` markiert anhaltefähige Funktionen über den Aufrufgraph der IR (Anhaltepunkt: Intrinsic `_Suspend`). Anhaltefähige Funktionen und Funktionen mit `goto` werden als Dispatch-Schleife (`loop` + `br_table`) übersetzt, gesplittet vor jedem Anhaltepunkt und jedem Aufruf einer anhaltefähigen Funktion; Abwickeln sichert das Segment im Frame, Fortsetzen springt dorthin zurück. Der WASI-Treiber `_start` wickelt bei jedem Anhalten wirklich ab und setzt fort. `rtl/crt.pas` hat jetzt `Delay` und `ReadKey`; die CLI fängt Strg+C ab. Tests: `testdata/etappe8/` mit `zahlen.pas` (Abnahme, reproduzierbar mit `-fest`), Anhalten tief in einer Rekursion mit inneren Prozeduren, Zeitscheiben und `goto` samt irreduziblem Sprung; alle Programme auch mit erzwungener Frame-Ablage.

### Etappe 9: Ziel `gojs`

- Importe aus Modul `gojs` mit Go-Stack-Layout (`sp`), Exporte `run`, `resume`, `getsp`, `mem`; Ausgabe über `runtime.wasmWrite`, Zeit, Timer, Zufall, Exit; Eingabe-Callback (mit einfachem Zeileneditor im Loader: Echo, Backspace, Enter) über `syscall/js`-Importe.
- `wasm_exec.js` der festgelegten Go-Version wird mitgeliefert. Ein kleiner Zusatz zum Loader übersetzt WASM-Traps über die Zeilentabelle in die Pascal-Fehlermeldung.
- **Fertig, wenn:** Das Zahlenraten unverändert mit `pascal build -target gojs` im Browser mit `wasm_exec.js` läuft.

### Etappe 10: Compiler als WASM und Browser-Demo

- `compiler` mit `GOOS=js GOARCH=wasm` bauen, eine JS-Funktion `pascalCompile(sources) → wasm | error` exportieren.
- `web/`: Editor-Textfeld, xterm.js-Terminal, Kompilieren und Ausführen im Browser.
- **Fertig, wenn:** Die Demo-Seite lokal über einen statischen Server das Zahlenraten kompiliert und startet.

## Arbeitsregeln für Claude Code

- **Eine Etappe nach der anderen.** Am Ende jeder Etappe: Tests grün, `go vet` sauber, `gofmt` angewendet, Commit mit aussagekräftiger Nachricht, kurzer Bericht (was geht, was fehlt, offene Fragen). Dann auf Freigabe warten.
- **Tests zuerst denken.** Jede neue Sprachfunktion bekommt mindestens ein Pascal-Testprogramm in `testdata/` mit erwarteter Ausgabe. Golden-Files nur mit `-update` neu schreiben und die Diffs prüfen.
- **Abhängigkeiten:** nur Go-Standardbibliothek im Paket `compiler`; wazero nur in `runner` und `cmd`. Nichts weiteres ohne Rückfrage.
- **WASM-Tauglichkeit des Compilers:** kein `os/exec`, keine cgo-Abhängigkeiten, Dateizugriff im Compiler nur über ein `fs.FS`-artiges Interface, damit der Browser Quelltexte aus dem Speicher liefern kann. Beide `GOOS=js`- und `GOOS=wasip1`-Builds in jedem Test-Lauf prüfen.
- **Fehlermeldungen:** Format `datei.pas(zeile,spalte): Error NN: Text`, angelehnt an TP. Abbruch beim ersten Fehler.
- **Codegenerator von Anfang an auf den Scheduler auslegen:** IR immer als CFG; jede Variable muss sich wahlweise in den Frame auslagern lassen. Auch wenn die Zustandsmaschinen erst in Etappe 8 kommen.
- **Keine eigenmächtigen Erweiterungen** des Sprachumfangs. Im Zweifel das Verhalten von TP 4.0 nachbilden und Abweichungen in `docs/abweichungen.md` festhalten.
- **Keine Raterei bei TP-Verhalten:** Wenn das genaue Verhalten unklar ist (Ausgabeformat, Randfälle), eine plausible Annahme treffen, im Code mit `// TP-ANNAHME:` markieren und im Bericht nennen.
- **Laufzeit in Pascal bevorzugen.** Neue Laufzeitfunktionen zuerst in `rtl/*.pas` schreiben; nur wenn das nicht geht, als Intrinsic in Go/WASM.
- **`PLAN.md` und `docs/abweichungen.md` pflegen:** erledigte Etappen abhaken, Entscheidungen kurz festhalten.
