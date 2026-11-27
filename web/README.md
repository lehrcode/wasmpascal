# web/

Browser-Demo und Laufzeitumgebung für Programme mit dem Ziel `gojs`.

| Datei | Zweck |
| --- | --- |
| `index.html` | Demo: Editor, Beispiele, Übersetzen im Browser (`compiler.wasm`), Terminal |
| `run.html` | führt ein fertiges `.wasm` (Ziel gojs) aus |
| `beispiele/` | Beispielprogramme der Demo |
| `wasm_exec.js` | unverändert aus Go 1.27.4 (`$(go env GOROOT)/lib/wasm/wasm_exec.js`); trägt Compiler und Programme |
| `loader.js` | `WasmPascal.run(wasm, {args, write, readLine, signal})` für Browser und Node.js; übersetzt Traps in `Runtime error NNN at datei.pas(zeile) in Prozedur`, vorher laufen über den Export `_exit` die Exit-Prozeduren |
| `terminal.js` | xterm.js-Terminal mit Zeileneditor für `index.html` und `run.html` |
| `node-compile.js` | `node web/node-compile.js compiler.wasm [-o verz] datei.pas …` übersetzt mit dem Browser-Compiler |
| `node-run.js` | `node web/node-run.js [--dir verz] [--env] [--setenv NAME=WERT …] prog.wasm [Argumente …]`, stdin/stdout wie unter WASI; mit `--dir` Dateizugriff unterhalb von `verz`, mit `--env` die Umgebung dieses Prozesses, mit `--setenv` einzelne Variablen |

## Demo starten

Den Compiler für den Browser bauen (etwa 4,7 MB):

```bash
GOOS=js GOARCH=wasm go build -ldflags=-s -trimpath -o web/compiler.wasm ./cmd/pascaljs
```

Einen beliebigen statischen Server im Verzeichnis `web/` starten:

```bash
python3 -m http.server -d web 8080
```

Dann `http://localhost:8080/` öffnen, ein Beispiel wählen und mit
„Übersetzen und starten“ (Strg+Enter) ausführen. xterm.js kommt vom CDN
(jsDelivr). Ein Klick auf eine Compilermeldung markiert die Fehlerstelle.

Ein schon übersetztes Programm läuft mit `run.html`:

```bash
go run ./cmd/pascal build -target gojs -o web/zahlen.wasm testdata/etappe8/zahlen.pas
```

`http://localhost:8080/run.html?prog=zahlen.wasm&args=-fest` öffnen; ohne
Server lässt sich `run.html` auch direkt öffnen und die Datei auswählen.

## pascalCompile

```js
const r = pascalCompile({ "zahlen.pas": text, "werkzeug.pas": unit }, { main: "zahlen.pas" });
if (r.error) { /* r.error, r.file, r.line, r.column, r.code */ }
else { await WasmPascal.run(r.wasm, { args: ["zahlen"], write, readLine }); }
```

Quelltexte sind Strings oder `Uint8Array`; weitere Optionen: `defines`
(Array), `stack` (Byte). Einzelheiten im Kommentar von `cmd/pascaljs/main.go`.

## Dateien

Unter `gojs` greift das Programm über WASI-Aufrufe (`path_open`, `fd_read`,
…) auf Dateien zu; `loader.js` setzt sie auf die Option `files` von
`WasmPascal.run` um: einen **synchronen** Host mit einer Teilmenge der
Node-API `fs` (`openSync`, `readSync`, `writeSync`, `closeSync`, `fstatSync`,
`statSync`, `ftruncateSync`, `unlinkSync`, `renameSync`, `mkdirSync`,
`rmdirSync`, `readdirSync`; `statSync`/`fstatSync` liefern auch `mtimeMs`). Der genaue Vertrag steht im Kopf von `loader.js`. Pfade kommen
normalisiert und relativ zur Wurzel an, Fehler meldet der Host als Exception
mit Node-Code (`err.code`). `node-run.js --dir` zeigt einen solchen Host über
Nodes `fs`; die Demo-Seiten haben keinen und melden bei Dateizugriffen
E/A-Fehler 5.

## Umgebung

`WasmPascal.run` gibt mit der Option `env` (Objekt Name → Wert) genau diese
Variablen an das Programm (Unit Dos: `GetEnv`, `EnvCount`, `EnvStr`). Ohne
`env` setzt der Loader nur `WASMPASCAL_UTC_OFFSET` aus
`getTimezoneOffset()`, damit `GetDate`/`GetTime` Ortszeit liefern.
`wasm_exec.js` sortiert die Variablen nach Namen und begrenzt Argumente plus
Umgebung auf etwa 8 KB.

## Schnittstelle zum Programm

Das Programm ruft für jede Eingabezeile `go.wasmPascalReadLine(f)` am
`Go`-Objekt auf; der Loader ruft `f(text)` später (nie innerhalb dieses
Aufrufs) mit dem nächsten Stück Eingabe auf, `""` bedeutet Dateiende.
Ausgaben kommen über das globale `fs.writeSync`, das der Loader auf das
gerade laufende Programm umleitet.
