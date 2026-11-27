# WasmPascal

Ein Compiler für Turbo Pascal 5.0 (mit einigen Erweiterungen aus TP 7 und
Delphi) nach WebAssembly, geschrieben in Go. Die Laufzeit ist
selbst in Pascal geschrieben. Übersetzte Programme laufen unter WASI (z. B.
mit wazero oder Node.js) oder im Browser; der Compiler selbst läuft auch als
WebAssembly im Browser.

```pascal
program Mittelwert;
{ Liest Zahlen bis zur Eingabe 0 und gibt den Mittelwert aus. }
uses Crt;
var
  Zahl, Summe: Real;
  Anzahl: Integer;
begin
  TextColor(Yellow);
  WriteLn('Zahlen eingeben, 0 beendet:');
  NormVideo;
  Summe := 0;
  Anzahl := 0;
  repeat
    Write('> ');
    ReadLn(Zahl);
    if Zahl <> 0 then
    begin
      Summe := Summe + Zahl;
      Inc(Anzahl)
    end
  until Zahl = 0;
  if Anzahl > 0 then
    WriteLn('Mittelwert von ', Anzahl, ' Zahlen: ', Summe / Anzahl:0:2)
end.
```

## Umfang

- **Sprache:** Turbo Pascal 5.0 mit Units, `goto`, Mengen, Zeigern,
  typisierten Konstanten und prozeduralen Typen (`type TFunc = function(X:
  Real): Real`, Funktionsparameter, Tabellen von Routinen); Exit-Prozeduren
  über `ExitProc`, auch nach Laufzeitfehlern, und `RunError`. Dazu
  `Result`, `Break`/`Continue`, `Low`/`High`,
  `const`-Parameter, `for … in`, Funktionen mit Record- oder
  Array-Ergebnis, `SmallInt` und `WideChar`. `Integer` hat 32 Bit, `Word`
  und `SmallInt` 16 Bit wie in TP.
- **Unicode:** Texte sind UTF-8, `string` bleibt der ShortString mit 255
  Bytes. Ein `WideChar` hält ein Zeichen bis U+FFFF (`W := 'ä'`,
  `W := #$20AC`) und wird als String zu UTF-8; `UTF8Length`, `UTF8Next`,
  `UTF8Index`, `UTF8Copy` und `for w in s do` arbeiten zeichenweise statt
  byteweise.
- **Saubere Quelltexte:** Steuerzeichen stehen in Literalen nur als `#n` oder
  `^X` (`'Zeile'#10`, `#27'[34m'`), nicht roh; Bidi-Steuerzeichen, mit denen
  ein Editor Code anders anzeigen kann, als er übersetzt wird („Trojan
  Source“), lehnt der Compiler überall ab.
- **Units:** `System` (Ein- und Ausgabe, Text-, typisierte und untypisierte
  Dateien samt `FileMode`, Verzeichnisse, Strings, Mathematik, Heap), `Crt`
  (Farben, `ClrScr`, `GotoXY`, `Delay`, `ReadKey` zeilenweise) und `Dos`
  (Datum und Uhrzeit in Ortszeit, `FindFirst`/`FindNext`, `GetFAttr`,
  `GetFTime`, `FExpand`, `FSplit`, `FSearch`, `GetEnv`). Alles Nutzbare beschreibt
  [docs/units.md](docs/units.md); `WhereX`, `WhereY`, `Window` und
  `KeyPressed` fehlen.
- **Ziele:** WASI (CLI mit wazero, `node:wasi`, …) und `gojs` für Browser und
  Node.js. Eingaben, `Delay` und lange Ausgaben halten das Programm an, statt
  den Browser zu blockieren. Dateien gibt es unter beiden Zielen, unter `gojs`
  über einen Datei-Host des Loaders.
- **Robust für fremde Quelltexte:** einstellbare Grenzen für Quelltextmenge,
  Schachtelung, Tokens, statische Daten und Stack, Abbruch über `ctx`,
  geprüft mit Extremfällen und Fuzzing.

Wo und warum WasmPascal von Turbo Pascal abweicht, steht in
[docs/abweichungen.md](docs/abweichungen.md), was sich zwischen den Versionen
geändert hat, in [CHANGELOG.md](CHANGELOG.md).

## Kommandozeile

Voraussetzung ist Go 1.27.4 oder neuer. Im ausgecheckten Repository:

```bash
go install ./cmd/pascal
```

```bash
pascal run mittel.pas
```

| Befehl | Wirkung |
| --- | --- |
| `pascal run datei.pas [arg …]` | übersetzt und führt aus (WASI mit wazero, Dateizugriff im Arbeitsverzeichnis) |
| `pascal build datei.pas` | erzeugt `datei.wasm`; `-target gojs` für den Browser, `-o` für den Namen |
| `pascal check datei.pas` | prüft nur |
| `pascal serve` | Compiler-Server: JSON-Anfragen zeilenweise auf stdin, Antworten auf stdout (siehe unten) |

Weitere Optionen: `-I pfad` (Units und Include-Dateien), `-D symbol`
(`{$DEFINE}`), `-stack n` (Schatten-Stack in Byte), `-nolines` (ohne
Zeilentabelle). Fehler erscheinen wie in Turbo Pascal, etwa
`mittel.pas(15,5): Error 85: ";" expected` für ein fehlendes Semikolon in
Zeile 14 oder `Runtime error 200 at mittel.pas(23) in Mittelwert`, wenn die
Abfrage `Anzahl > 0` fehlte und durch 0 geteilt würde.

Das Programm sieht das Arbeitsverzeichnis als Wurzel: Pfade werden darin
aufgelöst (`\` gilt als `/`, `..` über die Wurzel hinaus ist ein Fehler).
Ob Groß-/Kleinschreibung in Dateinamen zählt, bestimmt das Dateisystem
(unter Linux ja, anders als in TP); Units und `{$I}`-Dateien findet der
Compiler unabhängig davon.

## In eigene Projekte einbinden

### Go

Das Paket `compiler` braucht nur die Standardbibliothek, liest Quelltexte über
ein `fs.FS` und baut auch für `GOOS=js` und `GOOS=wasip1`. Das Paket `runner`
führt WASI-Module mit [wazero](https://wazero.io) aus.

```go
src := fstest.MapFS{"hallo.pas": {Data: []byte("program Hallo; begin WriteLn('Hallo!') end.")}}

res, err := compiler.Compile(ctx, "hallo.pas", compiler.Options{FS: src})
if err != nil {
	// Compilerfehler sind *compiler.Diagnostic mit File, Line, Col, Code, Msg
	fmt.Println(err)
	return
}

code, err := runner.Run(ctx, res.Wasm, runner.Options{
	Args:   []string{"hallo"},
	Stdin:  os.Stdin,
	Stdout: os.Stdout,
	Dirs:   map[string]string{"/": "."}, // ohne Eintrag kein Dateizugriff
})
var rte *runner.RuntimeError
switch {
case errors.As(err, &rte):
	fmt.Println(rte) // Runtime error NNN at datei.pas(zeile) in Prozedur
case err != nil:
	fmt.Println(err)
default:
	fmt.Println("Exit-Code:", code)
}
```

Das Modul aus `compiler.Compile` (Standardziel WASI Preview 1) läuft auch
ohne `runner`, z. B. mit `node:wasi`. Nur Exit-Prozeduren nach einem
Laufzeitfehler brauchen `runner` oder den Loader: Sie rufen nach dem Trap den
Export `_exit` des Moduls.

Quelltexte aus fremder Hand darf `compiler.Compile` direkt übersetzen. Arbeit
und Speicher begrenzt `compiler.Options.Limits`; der Nullwert sind sichere
Voreinstellungen, jedes Feld lässt sich ändern oder mit −1 abschalten. Das
Ergebnis ist ein Modul, ein `*compiler.Diagnostic`, `ctx.Err()` oder ein
Fehler mit `compiler.ErrInternal` (abgefangene Panic im Compiler):

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
res, err := compiler.Compile(ctx, "hallo.pas", compiler.Options{
	FS:     src,
	Limits: compiler.Limits{MaxDepth: 100, MaxSourceBytes: 1 << 20},
})
var d *compiler.Diagnostic
switch {
case errors.As(err, &d):
	fmt.Println(d.Code, d) // z. B. 904 hallo.pas(…): Error 904: Nesting too deep
case errors.Is(err, compiler.ErrInternal):
	log.Println("Fehler im Compiler:", err)
case err != nil:
	fmt.Println(err) // context.DeadlineExceeded, ungültige Optionen
default:
	fmt.Println(len(res.Wasm), "Byte")
}
```

Was dabei garantiert ist und was nicht (keine harte Speichergrenze für den
ganzen Prozess), steht im Abschnitt „Einbetten in Server“ von
`go doc ./compiler`.

### Compiler-Server

`pascal serve` übersetzt im Dauerbetrieb: je Zeile auf stdin eine
JSON-Anfrage, je Anfrage genau eine JSON-Zeile auf stdout, in derselben
Reihenfolge. Quelltexte kommen nur aus der Anfrage, Hostdateien sind nicht
erreichbar. So lässt sich der Compiler aus jeder Sprache ansprechen, etwa als
Kindprozess eines Webservers.

```bash
echo '{"id": 1, "files": {"p.pas": "program P; begin WriteLn(6 * 7) end."}}' | pascal serve
```

```json
{"id":1,"wasm":"AGFzbQEAAAAB…"}
```

Anfrage: `files` (Dateiname → Quelltext, Pflicht), `main` (nötig bei mehreren
Dateien), `target` (`"wasi"` oder `"gojs"`), `defines`, `stack`, `limits` (die
Felder von `compiler.Limits` in camelCase, etwa `{"maxDepth": 100}`) und
optional `id`, die die Antwort unverändert enthält. Antwort: `wasm`
(base64) oder `kind` mit `error`; bei `"kind": "source"` zusätzlich `file`,
`line`, `column` und `code`. Weitere Arten sind `request` (kaputte Anfrage),
`internal` und `timeout`. Fehler beenden den Server nicht; er endet am Ende
der Eingabe. Optionen: `-timeout` je Anfrage (10 s), `-maxrequest` (64 MB je
Zeile), `-memory` (weiches Speicherziel des Prozesses).

In Go als Kindprozess:

```go
cmd := exec.Command("pascal", "serve", "-timeout", "5s")
in, _ := cmd.StdinPipe()
out, _ := cmd.StdoutPipe()
if err := cmd.Start(); err != nil {
	log.Fatal(err)
}
enc, dec := json.NewEncoder(in), json.NewDecoder(out)

type response struct {
	ID    int    `json:"id"`
	Wasm  []byte `json:"wasm"` // base64 im JSON, hier schon dekodiert
	Kind  string `json:"kind"` // leer bei Erfolg
	Error string `json:"error"`
}
quellen := []string{
	"program P; begin WriteLn(6 * 7) end.",
	"program P; begin x := 1 end.",
}
for i, src := range quellen {
	enc.Encode(map[string]any{"id": i, "target": "gojs", "files": map[string]string{"p.pas": src}})
	var r response
	if err := dec.Decode(&r); err != nil {
		log.Fatal(err)
	}
	if r.Kind != "" {
		fmt.Println(r.ID, r.Kind, r.Error) // 1 source p.pas(1,18): Error 3: Unknown identifier (x)
		continue
	}
	fmt.Println(r.ID, "Modul mit", len(r.Wasm), "Byte")
}
in.Close() // Ende der Eingabe beendet den Server
cmd.Wait()
```

Der Compiler läuft im Prozess des Servers. Die Grenzen aus `compiler.Limits`
halten Zeit und Speicher je Anfrage klein, eine harte Speichergrenze gibt es
aber nicht, und ein Absturz der Go-Laufzeit (etwa Speichermangel) beendet den
Server. Für Quelltexte aus fremder Hand den Server deshalb in einem Container
mit Speichergrenze und automatischem Neustart betreiben.

### Browser und Node.js

Mit `-target gojs` erzeugte Module nutzen die Schnittstelle von
`GOOS=js GOARCH=wasm` und laufen mit `web/wasm_exec.js` und `web/loader.js`.
Der Compiler selbst wird für den Browser so gebaut:

```bash
GOOS=js GOARCH=wasm go build -ldflags=-s -trimpath -o web/compiler.wasm ./cmd/pascaljs
```

```html
<script src="wasm_exec.js"></script>
<script src="loader.js"></script>
<script>
(async () => {
  const go = new Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch("compiler.wasm"), go.importObject);
  go.run(instance);                       // legt pascalCompile an

  const r = pascalCompile({ "hallo.pas": "program Hallo; begin WriteLn('Hallo!') end." });
  if (r.error) {
    console.log(r.error);                 // mit r.file, r.line, r.column, r.code
    return;
  }
  const result = await WasmPascal.run(r.wasm, {
    args: ["hallo"],
    write: (fd, bytes) => console.log(new TextDecoder().decode(bytes)),
    readLine: async () => prompt("Eingabe:"),   // "" oder null = Ende der Eingabe
  });
  console.log("Exit-Code:", result.exitCode);
})();
</script>
```

Dateizugriffe des Programms gehen unter `gojs` an die Option `files` von
`WasmPascal.run`: einen synchronen Host mit einer Teilmenge der Node-API `fs`
(`openSync`, `readSync`, `writeSync`, `statSync`, `mkdirSync`, …; Vertrag im
Kopf von `web/loader.js`). Ohne `files` ergibt jeder Dateizugriff E/A-Fehler 5.
In Node.js genügt:

```bash
pascal build -target gojs -o datei.wasm datei.pas
```

```bash
node web/node-run.js --dir daten datei.wasm
```

`web/index.html` ist eine fertige Demo mit Editor und Terminal (xterm.js),
`web/run.html` startet ein fertiges Modul; beide haben keinen Datei-Host.
`web/node-compile.js` übersetzt in Node.js mit dem Browser-Compiler. Bau,
Start und Schnittstellen beschreibt [web/README.md](web/README.md).

## Dokumentation

| Datei | Inhalt |
| --- | --- |
| [docs/units.md](docs/units.md) | Referenz der Units System, Crt und Dos: Typen, Routinen, Laufzeitfehler |
| [docs/abweichungen.md](docs/abweichungen.md) | Wo und warum WasmPascal von Turbo Pascal 5.0 abweicht |
| [CHANGELOG.md](CHANGELOG.md) | Änderungen je Version |
| [PLAN.md](PLAN.md) | Sprachumfang, Erweiterungen, Typabbildung, Speichermodell, Architektur, Etappen |
| [web/README.md](web/README.md) | Browser-Demo, Loader mit Datei-Host, `pascalCompile`, Node-Werkzeuge |
| [testdata/README.md](testdata/README.md) | Aufbau der Testprogramme, Fuzzing |
| [CLAUDE.md](CLAUDE.md) | Arbeitsregeln für die Weiterentwicklung |

Die Go-Pakete sind mit Doku-Kommentaren und Beispielen versehen
(`go doc ./compiler`, `go doc ./runner`).

## Tests

```bash
go test ./...
```

Jedes Programm unter `testdata/` wird übersetzt, ausgeführt und mit der
erwarteten Ausgabe verglichen. Ist `node` installiert, laufen die Programme
zusätzlich mit dem Ziel `gojs` (auch die Programme mit Dateien), und der
Browser-Compiler muss byte-gleiche Module liefern. `compiler/limits_test.go`
prüft die Grenzen mit Extremfällen (tiefe Schachtelung, riesige Arrays, lange
Zeilen, Abbruch über `ctx`). Fuzzing des Compilers:

```bash
go test ./compiler -run '^$' -fuzz=FuzzCompile -fuzztime=10m
```

## Lizenz

MIT, siehe [LICENSE](LICENSE). Ausgenommen ist `web/wasm_exec.js`: Die Datei
stammt unverändert aus der Go-Distribution und steht unter der BSD-Lizenz der
Go-Autoren.
