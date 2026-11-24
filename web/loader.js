// WasmPascal-Loader für Programme, die mit `pascal build -target gojs`
// übersetzt wurden. Er setzt auf wasm_exec.js aus der Go-Distribution auf
// (vorher laden; im Browser als <script>, in Node.js per require) und läuft
// im Browser wie in Node.js.
//
//   const result = await WasmPascal.run(wasmBytes, {
//     args: ["prog", "a", "b"],          // argv[0] ist der Programmname
//     write(fd, bytes) { … },            // Ausgabe (UTF-8), fd 1 oder 2
//     readLine() { return Promise },     // nächstes Stück Eingabe, "" oder null = Ende
//     signal,                            // AbortSignal zum Abbrechen (optional)
//   });
//   // result = { exitCode, error }  (error: WasmPascal.RuntimeError oder null)
//
// Ein Laufzeitfehler erscheint wie unter WASI als Meldung
// "Runtime error NNN at datei.pas(zeile) in Prozedur" auf fd 2; exitCode ist
// dann die Fehlernummer. Mehrere Programme dürfen gleichzeitig laufen.
(function (root, factory) {
	"use strict";
	if (typeof module === "object" && module.exports) {
		module.exports = factory();
	} else {
		root.WasmPascal = factory();
	}
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
	"use strict";

	const encoder = new TextEncoder();
	const decoder = new TextDecoder("utf-8");

	// Laufzeitfehler eines Pascal-Programms.
	class RuntimeError extends Error {
		constructor(code, file, line, proc) {
			let msg = "Runtime error " + code;
			if (file) {
				msg += " at " + file;
				if (line > 0) {
					msg += "(" + line + ")";
				}
			}
			if (proc) {
				msg += " in " + proc;
			}
			super(msg);
			this.name = "RuntimeError";
			this.code = code;
			this.file = file;
			this.line = line;
			this.proc = proc;
		}
	}

	// Das gerade laufende Programm; wasm_exec.js schreibt über das globale
	// fs.writeSync, das hierher umgeleitet wird.
	let active = null;
	let hooked = false;

	function hookWrite() {
		if (hooked) {
			return;
		}
		if (typeof Go !== "function" || !globalThis.fs) {
			throw new Error("WasmPascal: wasm_exec.js muss vor loader.js geladen werden");
		}
		const fs = globalThis.fs;
		const orig = fs.writeSync;
		fs.writeSync = function (fd, buf) {
			if (active) {
				return active.write(fd, buf);
			}
			return orig.call(this, fd, buf);
		};
		hooked = true;
	}

	// trapCode ordnet einem WASM-Trap eine TP-Fehlernummer zu (wie runner/rterror.go).
	function trapCode(err) {
		const msg = String(err && err.message ? err.message : err);
		if (/divide by zero|division by zero/i.test(msg)) {
			return 200; // Division by zero
		}
		if (/call stack|too much recursion|stack overflow/i.test(msg)) {
			return 202; // Stack overflow
		}
		if (/unrepresentable|integer overflow|invalid conversion/i.test(msg)) {
			return 207; // Invalid floating point operation
		}
		return 216; // General protection fault
	}

	// readLines liest Datei- und Prozedurnamen aus der Custom Section
	// wasmpascal.lines.
	function readLines(module) {
		const sections = WebAssembly.Module.customSections(module, "wasmpascal.lines");
		if (sections.length === 0) {
			return null;
		}
		const b = new Uint8Array(sections[0]);
		let pos = 0;
		const u32 = () => {
			let v = 0;
			let shift = 0;
			for (;;) {
				if (pos >= b.length) {
					throw new Error("wasmpascal.lines: unerwartetes Ende");
				}
				const c = b[pos++];
				v += (c & 0x7f) * 2 ** shift;
				if ((c & 0x80) === 0) {
					return v;
				}
				shift += 7;
			}
		};
		const name = () => {
			const n = u32();
			const s = decoder.decode(b.subarray(pos, pos + n));
			pos += n;
			return s;
		};
		try {
			if (u32() !== 1) {
				return null;
			}
			const files = [];
			for (let n = u32(); n > 0; n--) {
				files.push(name());
			}
			const funcs = new Map();
			for (let n = u32(); n > 0; n--) {
				const idx = u32();
				funcs.set(idx, name());
				u32(); // Datei der Deklaration
				u32(); // Zeile der Deklaration
			}
			return { files, funcs };
		} catch (e) {
			return null;
		}
	}

	// runtimeError übersetzt einen Trap in einen Pascal-Laufzeitfehler; die
	// Fehlerstelle steht in den exportierten Globalen __rt_*.
	function runtimeError(module, exports, err) {
		const get = (name) => (exports[name] instanceof WebAssembly.Global ? exports[name].value >>> 0 : 0);
		let code = get("__rt_code");
		if (code === 0) {
			code = trapCode(err);
		}
		const file = get("__rt_file");
		const line = get("__rt_line");
		const fn = get("__rt_func");
		const lines = line > 0 ? readLines(module) : null;
		if (!lines) {
			return new RuntimeError(code, "", 0, "");
		}
		return new RuntimeError(code, lines.files[file] || "", line, lines.funcs.get(fn) || "");
	}

	// isTrap unterscheidet Traps des Programms von Fehlern im Loader.
	function isTrap(err) {
		return err instanceof WebAssembly.RuntimeError ||
			err instanceof RangeError ||
			(typeof InternalError === "function" && err instanceof InternalError); // Firefox: too much recursion
	}

	async function instantiate(source, importObject) {
		if (source instanceof WebAssembly.Module) {
			return { module: source, instance: await WebAssembly.instantiate(source, importObject) };
		}
		if (typeof Response === "function" && source instanceof Response) {
			source = await source.arrayBuffer();
		}
		return WebAssembly.instantiate(source, importObject);
	}

	// run lädt ein gojs-Modul und führt es aus (siehe Kopf der Datei).
	async function run(source, opts = {}) {
		hookWrite();
		const go = new Go();
		go.argv = (opts.args && opts.args.length > 0) ? opts.args.map(String) : ["program"];
		go.env = {};
		const { module, instance } = await instantiate(source, go.importObject);
		const exports = instance.exports;

		return new Promise((resolve, reject) => {
			let done = false;
			let lastByte = 10; // "\n": noch nichts geschrieben

			const write = (fd, bytes) => {
				if (bytes.length > 0) {
					lastByte = bytes[bytes.length - 1];
				}
				if (opts.write) {
					opts.write(fd, bytes);
				}
			};

			const stopTimers = () => {
				for (const t of go._scheduledTimeouts.values()) {
					clearTimeout(t);
				}
				go._scheduledTimeouts.clear();
			};

			const finish = (result) => {
				if (done) {
					return;
				}
				done = true;
				go.exited = true;
				stopTimers();
				if (opts.signal) {
					opts.signal.removeEventListener("abort", onAbort);
				}
				resolve(result);
			};

			const fail = (err) => {
				if (done) {
					return;
				}
				if (!isTrap(err)) {
					done = true;
					go.exited = true;
					stopTimers();
					reject(err);
					return;
				}
				const rte = runtimeError(module, exports, err);
				if (lastByte !== 10) {
					write(2, encoder.encode("\n"));
				}
				write(2, encoder.encode(rte.message + "\n"));
				finish({ exitCode: rte.code, error: rte });
			};

			const ctx = {
				write(fd, buf) {
					// buf zeigt in den Speicher des Moduls: kopieren
					write(fd, buf.slice());
					return buf.length;
				},
			};

			// führt einen Aufruf ins Modul aus; Ausgaben gehen an dieses Programm
			const enter = (f) => {
				const prev = active;
				active = ctx;
				try {
					return f();
				} finally {
					active = prev;
				}
			};

			const onAbort = () => finish({ exitCode: 130, error: null, aborted: true });
			if (opts.signal) {
				if (opts.signal.aborted) {
					onAbort();
					return;
				}
				opts.signal.addEventListener("abort", onAbort);
			}

			go.exit = (code) => finish({ exitCode: code, error: null });

			const resume = go._resume.bind(go);
			go._resume = () => {
				if (done) {
					return;
				}
				try {
					enter(resume);
				} catch (err) {
					fail(err);
				}
			};

			// vom Programm aufgerufen, wenn es eine Eingabe braucht; f setzt
			// es fort und darf nicht schon während dieses Aufrufs laufen
			go.wasmPascalReadLine = (f) => {
				Promise.resolve()
					.then(() => (opts.readLine ? opts.readLine() : null))
					.then((line) => {
						if (!done) {
							f(line == null ? "" : String(line));
						}
					}, fail);
			};

			let p;
			try {
				p = enter(() => go.run(instance));
			} catch (err) {
				fail(err);
				return;
			}
			p.catch(fail);
		});
	}

	return { run, RuntimeError };
});
