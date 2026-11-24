// Übersetzt Pascal-Programme in Node.js mit dem Compiler als WebAssembly
// (compiler.wasm aus cmd/pascaljs), wie es die Demo-Seite im Browser tut:
//
//   node web/node-compile.js compiler.wasm [-o verzeichnis] datei.pas …
//
// Jede Datei wird einzeln übersetzt; alle Dateien in ihrem Verzeichnis
// stehen als Units und Include-Dateien bereit. Das Ergebnis (Ziel gojs)
// landet als datei.wasm neben der Quelle bzw. unter -o mit demselben
// relativen Pfad. Compilerfehler gehen nach stderr; der Exit-Code ist dann 1.
"use strict";

const fs = require("fs");
const path = require("path");

require("./wasm_exec.js");

async function loadCompiler(file) {
	const go = new Go();
	const { instance } = await WebAssembly.instantiate(fs.readFileSync(file), go.importObject);
	go.run(instance); // kehrt zurück, sobald pascalCompile angelegt ist
	if (typeof globalThis.pascalCompile !== "function") {
		throw new Error(file + ": pascalCompile fehlt");
	}
	return globalThis.pascalCompile;
}

function sourcesNear(file) {
	const dir = path.dirname(file);
	const sources = {};
	for (const ent of fs.readdirSync(dir, { withFileTypes: true })) {
		if (ent.isFile() && !ent.name.endsWith(".wasm")) {
			sources[ent.name] = new Uint8Array(fs.readFileSync(path.join(dir, ent.name)));
		}
	}
	return sources;
}

async function main() {
	const args = process.argv.slice(2);
	const compilerFile = args.shift();
	let outDir = null;
	if (args[0] === "-o") {
		args.shift();
		outDir = args.shift();
	}
	if (!compilerFile || args.length === 0) {
		process.stderr.write("Aufruf: node node-compile.js compiler.wasm [-o verzeichnis] datei.pas …\n");
		return 2;
	}
	const pascalCompile = await loadCompiler(compilerFile);
	let code = 0;
	for (const file of args) {
		const r = pascalCompile(sourcesNear(file), { main: path.basename(file) });
		if (r.error) {
			process.stderr.write(r.error + "\n");
			code = 1;
			continue;
		}
		const rel = file.replace(/\.pas$/i, "") + ".wasm";
		const out = outDir ? path.join(outDir, path.relative(process.cwd(), path.resolve(rel))) : rel;
		fs.mkdirSync(path.dirname(out), { recursive: true });
		fs.writeFileSync(out, r.wasm);
	}
	return code;
}

main().then(
	(code) => process.exit(code),
	(err) => {
		process.stderr.write(String(err && err.stack ? err.stack : err) + "\n");
		process.exit(1);
	},
);
