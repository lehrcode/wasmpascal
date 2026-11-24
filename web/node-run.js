// Führt ein mit `pascal build -target gojs` übersetztes Programm in Node.js aus:
//
//   node web/node-run.js prog.wasm [Argumente …]
//
// Eingaben kommen von stdin, Ausgaben gehen nach stdout/stderr. Der
// Exit-Code ist der des Programms (bei einem Laufzeitfehler die Fehlernummer).
"use strict";

const fs = require("fs");
const path = require("path");

require("./wasm_exec.js");
const WasmPascal = require("./loader.js");

// lines liefert stdin stückweise: jede Zeile mit ihrem Zeilenende, am Ende
// den Rest ohne Zeilenende, danach null.
function stdinLines() {
	const queue = [];
	const waiting = [];
	let rest = "";
	let ended = false;
	const push = (v) => {
		if (waiting.length > 0) {
			waiting.shift()(v);
		} else {
			queue.push(v);
		}
	};
	process.stdin.setEncoding("utf8");
	process.stdin.on("data", (chunk) => {
		rest += chunk;
		let nl;
		while ((nl = rest.indexOf("\n")) >= 0) {
			push(rest.slice(0, nl + 1));
			rest = rest.slice(nl + 1);
		}
	});
	process.stdin.on("end", () => {
		if (rest !== "") {
			push(rest);
			rest = "";
		}
		ended = true;
		push(null);
	});
	process.stdin.pause();
	return () => {
		if (queue.length > 0) {
			const v = queue.shift();
			if (v === null) {
				queue.unshift(null); // Ende bleibt Ende
			}
			return Promise.resolve(v);
		}
		if (ended) {
			return Promise.resolve(null);
		}
		process.stdin.resume();
		return new Promise((resolve) => waiting.push((v) => {
			process.stdin.pause();
			resolve(v);
		}));
	};
}

async function main() {
	const [file, ...args] = process.argv.slice(2);
	if (!file) {
		process.stderr.write("Aufruf: node node-run.js prog.wasm [Argumente …]\n");
		return 2;
	}
	const name = path.basename(file, path.extname(file));
	const result = await WasmPascal.run(fs.readFileSync(file), {
		args: [name, ...args],
		write(fd, bytes) {
			(fd === 2 ? process.stderr : process.stdout).write(bytes);
		},
		readLine: stdinLines(),
	});
	return result.exitCode;
}

main().then(
	(code) => {
		process.exitCode = code;
		process.stdin.destroy();
	},
	(err) => {
		process.stderr.write(String(err && err.stack ? err.stack : err) + "\n");
		process.exitCode = 1;
		process.stdin.destroy();
	},
);
