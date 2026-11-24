# web/

Laufzeitumgebung für Programme mit dem Ziel `gojs`.

| Datei | Zweck |
| --- | --- |
| `wasm_exec.js` | unverändert aus Go 1.27.4 (`$(go env GOROOT)/lib/wasm/wasm_exec.js`) |
| `loader.js` | `WasmPascal.run(wasm, {args, write, readLine, signal})` für Browser und Node.js; übersetzt Traps in `Runtime error NNN at datei.pas(zeile) in Prozedur` |
| `run.html` | Testseite mit xterm.js und Zeileneditor |
| `node-run.js` | `node web/node-run.js prog.wasm [Argumente …]`, stdin/stdout wie unter WASI |

## Im Browser ausprobieren

```bash
go run ./cmd/pascal build -target gojs -o web/zahlen.wasm testdata/etappe8/zahlen.pas
```

```bash
python3 -m http.server -d web 8080
```

Dann `http://localhost:8080/run.html?prog=zahlen.wasm&args=-fest` öffnen. Ohne
Server lässt sich `run.html` auch direkt öffnen und die `.wasm`-Datei über
„Datei auswählen“ laden. xterm.js kommt vom CDN (jsDelivr).

## Schnittstelle zum Programm

Das Programm ruft für jede Eingabezeile `go.wasmPascalReadLine(f)` am
`Go`-Objekt auf; der Loader ruft `f(text)` später (nie innerhalb dieses
Aufrufs) mit dem nächsten Stück Eingabe auf, `""` bedeutet Dateiende.
Ausgaben kommen über das globale `fs.writeSync`, das der Loader auf das
gerade laufende Programm umleitet.
