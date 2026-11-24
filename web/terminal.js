// Terminal für WasmPascal-Programme im Browser: xterm.js mit einem einfachen
// Zeileneditor. Vorher laden: xterm.js, addon-fit, wasm_exec.js, loader.js.
//
//   const t = WasmPascalTerminal.create(element);
//   const result = await t.run(wasmBytes, { name: "zahlen", args: ["-fest"] });
//   t.stop();      // bricht das laufende Programm ab
//   t.running      // true, solange ein Programm läuft
//
// Eingaben werden geechot und mit Enter an das Programm übergeben (ReadLn wie
// in der DOS-Konsole). Rücktaste löscht, Strg+D am Zeilenanfang beendet die
// Eingabe, Strg+C bricht ab. Cursortasten und Sondertasten gibt es nicht.
(function (root) {
	"use strict";

	function create(element) {
		// Farben wie in der DOS-Konsole: Schwarz ist wirklich schwarz.
		const term = new Terminal({
			convertEol: true,
			cursorBlink: true,
			fontSize: 15,
			theme: { background: "#000000", black: "#000000" },
		});
		const fit = new FitAddon.FitAddon();
		term.loadAddon(fit);
		term.open(element);
		fit.fit();
		window.addEventListener("resize", () => fit.fit());
		if (typeof ResizeObserver === "function") {
			new ResizeObserver(() => fit.fit()).observe(element);
		}

		let controller = null; // AbortController des laufenden Programms
		const lines = [];      // fertige Zeilen, die das Programm noch nicht gelesen hat
		let waiting = null;    // Resolver eines wartenden readLine
		let current = "";      // angefangene Zeile

		const deliver = (line) => {
			if (waiting) {
				const w = waiting;
				waiting = null;
				w(line);
			} else {
				lines.push(line);
			}
		};

		const readLine = () => {
			if (lines.length > 0) {
				return Promise.resolve(lines.shift());
			}
			return new Promise((resolve) => { waiting = resolve; });
		};

		const stop = () => {
			if (controller) {
				controller.abort();
			}
		};

		term.onData((data) => {
			if (!controller || data.startsWith("\x1b")) {
				return; // kein Programm oder Cursortasten usw.
			}
			for (const ch of data) {
				if (ch === "\r") {
					term.write("\r\n");
					deliver(current + "\n");
					current = "";
				} else if (ch === "\x7f" || ch === "\b") {
					const cps = Array.from(current);
					if (cps.length > 0) {
						cps.pop();
						current = cps.join("");
						term.write("\b \b");
					}
				} else if (ch === "\x03") {
					stop();
					return;
				} else if (ch === "\x04") {
					if (current === "") {
						deliver(null);
					}
				} else if (ch >= " ") {
					current += ch;
					term.write(ch);
				}
			}
		});

		// run führt ein gojs-Modul im Terminal aus; Ergebnis wie WasmPascal.run,
		// bei Abbruch mit aborted: true.
		const run = async (wasm, opts = {}) => {
			if (controller) {
				throw new Error("Es läuft schon ein Programm");
			}
			controller = new AbortController();
			lines.length = 0;
			waiting = null;
			current = "";
			term.reset();
			term.focus();
			try {
				const result = await WasmPascal.run(wasm, {
					args: [opts.name || "programm", ...(opts.args || [])],
					write: (fd, bytes) => term.write(bytes),
					readLine,
					signal: controller.signal,
				});
				if (result.aborted) {
					term.write("\x1b[0m\r\n^C\r\n");
				}
				return result;
			} finally {
				if (waiting) {
					waiting(null);
					waiting = null;
				}
				controller = null;
			}
		};

		return {
			term,
			run,
			stop,
			get running() {
				return controller !== null;
			},
		};
	}

	root.WasmPascalTerminal = { create };
})(globalThis);
