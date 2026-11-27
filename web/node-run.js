// Führt ein mit `pascal build -target gojs` übersetztes Programm in Node.js aus:
//
//   node web/node-run.js [--dir verz] [--env] [--setenv NAME=WERT …] prog.wasm [Argumente …]
//
// Eingaben kommen von stdin, Ausgaben gehen nach stdout/stderr. Der
// Exit-Code ist der des Programms (bei einem Laufzeitfehler die Fehlernummer).
// Mit --dir bekommt das Programm das Verzeichnis verz als Wurzel für
// Dateizugriffe; ohne --dir scheitert jeder Dateizugriff mit E/A-Fehler 5.
// --env gibt die Umgebung dieses Prozesses weiter (dazu den UTC-Versatz in
// WASMPASCAL_UTC_OFFSET), --setenv einzelne Variablen, --emptyenv eine leere
// Umgebung (für Tests); ohne alles bekommt das Programm nur den UTC-Versatz.
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

// dirHost ist der Datei-Host für WasmPascal.run (Option files): Nodes fs,
// mit allen Pfaden unterhalb von root. Der Loader liefert nur normalisierte
// Pfade ohne ".." oberhalb der Wurzel.
function dirHost(root) {
	const abs = (p) => path.join(root, p);
	return {
		openSync: (p, flags) => fs.openSync(abs(p), flags),
		readSync: (fd, buf, off, len, pos) => fs.readSync(fd, buf, off, len, pos),
		writeSync: (fd, buf, off, len, pos) => fs.writeSync(fd, buf, off, len, pos),
		closeSync: (fd) => fs.closeSync(fd),
		fstatSync: (fd) => fs.fstatSync(fd),
		statSync: (p) => fs.statSync(abs(p)),
		readdirSync: (p) => fs.readdirSync(abs(p)),
		ftruncateSync: (fd, len) => fs.ftruncateSync(fd, len),
		unlinkSync: (p) => fs.unlinkSync(abs(p)),
		renameSync: (from, to) => fs.renameSync(abs(from), abs(to)),
		mkdirSync: (p) => fs.mkdirSync(abs(p)),
		rmdirSync: (p) => fs.rmdirSync(abs(p)),
	};
}

// Schließt der Leser die Pipe (z. B. "| head"), endet das Programm still.
for (const stream of [process.stdout, process.stderr]) {
	stream.on("error", (err) => {
		if (err.code === "EPIPE") {
			process.exit(0);
		}
		throw err;
	});
}

async function main() {
	const argv = process.argv.slice(2);
	let files;
	let env;
	while (argv.length > 0 && argv[0].startsWith("--")) {
		const opt = argv.shift();
		if (opt === "--emptyenv") {
			env = env || {};
			continue;
		}
		if (opt === "--env") {
			env = Object.assign(env || {}, process.env);
			if (env.WASMPASCAL_UTC_OFFSET === undefined) {
				env.WASMPASCAL_UTC_OFFSET = String(-new Date().getTimezoneOffset());
			}
			continue;
		}
		if (argv.length === 0) {
			process.stderr.write(`node-run.js: ${opt} braucht einen Wert\n`);
			return 2;
		}
		const value = argv.shift();
		if (opt === "--dir") {
			files = dirHost(path.resolve(value));
		} else if (opt === "--setenv") {
			const eq = value.indexOf("=");
			env = env || {};
			env[eq < 0 ? value : value.slice(0, eq)] = eq < 0 ? "" : value.slice(eq + 1);
		} else {
			process.stderr.write(`node-run.js: unbekannte Option ${opt}\n`);
			return 2;
		}
	}
	const [file, ...args] = argv;
	if (!file) {
		process.stderr.write("Aufruf: node node-run.js [--dir verz] [--env] [--setenv NAME=WERT …] prog.wasm [Argumente …]\n");
		return 2;
	}
	const name = path.basename(file, path.extname(file));
	const result = await WasmPascal.run(fs.readFileSync(file), {
		args: [name, ...args],
		write(fd, bytes) {
			(fd === 2 ? process.stderr : process.stdout).write(bytes);
		},
		readLine: stdinLines(),
		files,
		env,
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
