# WasmPascal

Ein Compiler für Turbo Pascal 4.0 (ohne OOP, mit einigen Erweiterungen aus
TP 7 und Delphi 1.0) nach WebAssembly, geschrieben in Go. Die Laufzeit ist
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

## In eigene Projekte einbinden

### Go

Das Paket `compiler` braucht nur die Standardbibliothek, liest Quelltexte über
ein `fs.FS` und baut auch für `GOOS=js` und `GOOS=wasip1`. Das Paket `runner`
führt WASI-Module mit [wazero](https://wazero.io) aus. Quelltexte aus fremder
Hand, etwa in einem Server, darf `compiler.Compile` direkt übersetzen: Arbeit
und Speicher sind über `compiler.Options.Limits` begrenzt, und das Ergebnis
ist immer ein Modul oder ein Fehler, nie ein Absturz (Abschnitt „Einbetten in
Server“ in `go doc ./compiler`).

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
ohne `runner`, z. B. mit `node:wasi`.

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

`web/index.html` ist eine fertige Demo mit Editor und Terminal (xterm.js);
`web/node-compile.js` und `web/node-run.js` machen dasselbe in Node.js. Bau,
Start und Schnittstellen beschreibt [web/README.md](web/README.md).

## Dokumentation

| Datei | Inhalt |
| --- | --- |
| [docs/units.md](docs/units.md) | Referenz der Units System, Crt und Dos: Typen, Routinen, Laufzeitfehler |
| [docs/abweichungen.md](docs/abweichungen.md) | Wo und warum WasmPascal von Turbo Pascal 4.0 abweicht |
| [PLAN.md](PLAN.md) | Sprachumfang, Erweiterungen, Typabbildung, Speichermodell, Architektur, Etappen |
| [web/README.md](web/README.md) | Browser-Demo, Loader, `pascalCompile` |
| [testdata/README.md](testdata/README.md) | Aufbau der Testprogramme |
| [CLAUDE.md](CLAUDE.md) | Arbeitsregeln für die Weiterentwicklung |

Die Go-Pakete sind mit Doku-Kommentaren und Beispielen versehen
(`go doc ./compiler`, `go doc ./runner`).

## Tests

```bash
go test ./...
```

Jedes Programm unter `testdata/` wird übersetzt, ausgeführt und mit der
erwarteten Ausgabe verglichen. Ist `node` installiert, laufen die Programme
zusätzlich mit dem Ziel `gojs`, und der Browser-Compiler muss byte-gleiche
Module liefern.

## Lizenz

MIT, siehe [LICENSE](LICENSE). Ausgenommen ist `web/wasm_exec.js`: Die Datei
stammt unverändert aus der Go-Distribution und steht unter der BSD-Lizenz der
Go-Autoren.
