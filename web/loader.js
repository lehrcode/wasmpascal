// WasmPascal-Loader für Programme, die mit `pascal build -target gojs`
// übersetzt wurden. Er setzt auf wasm_exec.js aus der Go-Distribution auf
// (vorher laden; im Browser als <script>, in Node.js per require) und läuft
// im Browser wie in Node.js.
//
//   const result = await WasmPascal.run(wasmBytes, {
//     args: ["prog", "a", "b"],          // argv[0] ist der Programmname
//     write(fd, bytes) { … },            // Ausgabe (UTF-8), fd 1 oder 2
//     readLine() { return Promise },     // nächstes Stück Eingabe, "" oder null = Ende
//     files,                             // Dateisystem (optional, siehe unten)
//     env,                               // Umgebung { NAME: "WERT" } (optional)
//     signal,                            // AbortSignal zum Abbrechen (optional)
//   });
//   // result = { exitCode, error }  (error: WasmPascal.RuntimeError oder null)
//
// Ein Laufzeitfehler erscheint wie unter WASI als Meldung
// "Runtime error NNN at datei.pas(zeile) in Prozedur" auf fd 2; exitCode ist
// dann die Fehlernummer. Mehrere Programme dürfen gleichzeitig laufen.
//
// Dateien: Das Programm greift über WASI-Aufrufe (fd_read, path_open, …) auf
// Dateien zu; der Loader setzt sie auf files um, einen synchronen Host mit
// einer Teilmenge der Node-API fs. Pfade sind relativ zur Wurzel des
// Programms, mit "/" getrennt und schon normalisiert ("." ist die Wurzel):
//
//   openSync(path, flags) → fd          flags: "r", "r+", "w", "w+"
//   readSync(fd, buf, offset, length, position) → Anzahl
//   writeSync(fd, buf, offset, length, position) → Anzahl
//   closeSync(fd)
//   fstatSync(fd), statSync(path) → { size, isDirectory(), mtimeMs }
//   readdirSync(path) → Namen (ohne "." und "..")
//   ftruncateSync(fd, length)
//   unlinkSync(path), renameSync(from, to)
//   mkdirSync(path), rmdirSync(path)
//
// Fehler wirft der Host als Exception mit dem Node-Code in err.code (ENOENT,
// EACCES, EEXIST, …). Ohne files scheitert jeder Dateizugriff mit E/A-Fehler 5.
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

	// ------------------------------------------------------------ Dateien

	// WASI-Fehlernummern (wasi_snapshot_preview1)
	const ESUCCESS = 0;
	const EBADF = 8;
	const EINVAL = 28;
	const EIO = 29;
	const EISDIR = 31;
	const ENOTDIR = 54;
	const ENOTCAPABLE = 76;
	const errnoOf = {
		ENOENT: 44, EACCES: 2, EPERM: 63, EBADF: 8, EEXIST: 20, EISDIR: 31,
		ENOTDIR: 54, ENOTEMPTY: 55, EMFILE: 33, ENOSPC: 51, EINVAL: 28,
	};
	const ROOT_FD = 3; // vorgeöffnete Wurzel (RootFd in rtl/system.pas)

	// wasiFiles liefert die WASI-Importe für Dateien. memory() gibt den
	// aktuellen Speicher zurück (nach memory.grow ein neuer Puffer), host ist
	// opts.files oder undefined. Die Handle-Tabelle gehört zu diesem Lauf.
	function wasiFiles(memory, host) {
		const handles = new Map([[ROOT_FD, { dir: true, path: "." }]]);
		let nextFd = ROOT_FD + 1;

		const view = () => new DataView(memory().buffer);
		const bytes = (ptr, len) => new Uint8Array(memory().buffer, ptr, len);
		const str = (ptr, len) => decoder.decode(bytes(ptr, len));

		// guard führt f aus und übersetzt Exceptions des Hosts in errno
		const guard = (f) => {
			if (!host) {
				return ENOTCAPABLE;
			}
			try {
				return f();
			} catch (err) {
				if (typeof err === "number") {
					return err;
				}
				return (err && errnoOf[err.code]) || EIO;
			}
		};

		// resolve bildet einen Pfad relativ zum Verzeichnis-Handle dirFd auf
		// einen Pfad relativ zur Wurzel ab; ".." über die Wurzel hinaus ist
		// nicht erlaubt (wie unter wazero).
		const resolve = (dirFd, ptr, len) => {
			const dir = handles.get(dirFd);
			if (!dir || !dir.dir) {
				throw EBADF;
			}
			const parts = dir.path === "." ? [] : dir.path.split("/");
			const p = str(ptr, len);
			if (p.startsWith("/")) {
				throw ENOTCAPABLE;
			}
			for (const part of p.split("/")) {
				if (part === "" || part === ".") {
					continue;
				}
				if (part === "..") {
					if (parts.length === 0) {
						throw ENOTCAPABLE;
					}
					parts.pop();
				} else {
					parts.push(part);
				}
			}
			return parts.length === 0 ? "." : parts.join("/");
		};

		const file = (fd) => {
			const h = handles.get(fd);
			if (!h) {
				throw EBADF;
			}
			if (h.dir) {
				throw EISDIR;
			}
			return h;
		};

		// iovecs: je 8 Byte (Adresse, Länge)
		const iovecs = (ptr, n) => {
			const v = view();
			const list = [];
			for (let i = 0; i < n; i++) {
				list.push([v.getUint32(ptr + 8 * i, true), v.getUint32(ptr + 8 * i + 4, true)]);
			}
			return list;
		};

		// filestat (64 Byte): filetype bei 16, size bei 32, mtim (ns) bei 48
		const putStat = (ptr, st) => {
			bytes(ptr, 64).fill(0);
			const v = view();
			v.setUint8(ptr + 16, st.isDirectory() ? 3 : 4);
			v.setBigUint64(ptr + 32, BigInt(st.size), true);
			v.setBigUint64(ptr + 48, BigInt(Math.max(0, Math.round((st.mtimeMs || 0) * 1e6))), true);
		};

		return {
			fd_read(fd, iovs, iovsLen, nread) {
				return guard(() => {
					const h = file(fd);
					let total = 0;
					for (const [buf, len] of iovecs(iovs, iovsLen)) {
						const n = host.readSync(h.hostFd, bytes(buf, len), 0, len, h.pos);
						h.pos += n;
						total += n;
						if (n < len) {
							break;
						}
					}
					view().setUint32(nread, total, true);
					return ESUCCESS;
				});
			},

			fd_write(fd, iovs, iovsLen, nwritten) {
				return guard(() => {
					const h = file(fd);
					let total = 0;
					for (const [buf, len] of iovecs(iovs, iovsLen)) {
						if (h.append) {
							h.pos = host.fstatSync(h.hostFd).size;
						}
						let done = 0;
						while (done < len) {
							const n = host.writeSync(h.hostFd, bytes(buf, len), done, len - done, h.pos);
							if (n <= 0) {
								throw EIO;
							}
							h.pos += n;
							done += n;
						}
						total += done;
					}
					view().setUint32(nwritten, total, true);
					return ESUCCESS;
				});
			},

			fd_close(fd) {
				return guard(() => {
					const h = handles.get(fd);
					if (!h || fd === ROOT_FD) {
						return EBADF;
					}
					handles.delete(fd);
					if (!h.dir) {
						host.closeSync(h.hostFd);
					}
					return ESUCCESS;
				});
			},

			fd_seek(fd, offset, whence, newOffset) {
				return guard(() => {
					const h = file(fd);
					const off = Number(offset); // i64 kommt als BigInt
					let base;
					switch (whence) {
					case 0: base = 0; break;
					case 1: base = h.pos; break;
					case 2: base = host.fstatSync(h.hostFd).size; break;
					default: return EINVAL;
					}
					if (base + off < 0) {
						return EINVAL;
					}
					h.pos = base + off;
					view().setBigUint64(newOffset, BigInt(h.pos), true);
					return ESUCCESS;
				});
			},

			fd_filestat_get(fd, ptr) {
				return guard(() => {
					const h = handles.get(fd);
					if (!h) {
						return EBADF;
					}
					putStat(ptr, h.dir ? host.statSync(h.path) : host.fstatSync(h.hostFd));
					return ESUCCESS;
				});
			},

			fd_filestat_set_size(fd, size) {
				return guard(() => {
					const h = file(fd);
					host.ftruncateSync(h.hostFd, Number(size));
					return ESUCCESS;
				});
			},

			path_open(dirFd, dirFlags, pathPtr, pathLen, oflags, rightsBase, rightsInherit, fdflags, fdPtr) {
				return guard(() => {
					const p = resolve(dirFd, pathPtr, pathLen);
					let h;
					if (oflags & 2) { // O_DIRECTORY
						if (!host.statSync(p).isDirectory()) {
							return ENOTDIR;
						}
						h = { dir: true, path: p };
					} else {
						const canRead = (BigInt(rightsBase) & 0x2n) !== 0n;
						const canWrite = (BigInt(rightsBase) & 0x40n) !== 0n;
						let flags;
						if (oflags & 1) { // O_CREAT (die RTL setzt es nur mit O_TRUNC)
							flags = canRead ? "w+" : "w";
						} else if (canWrite) {
							flags = "r+";
						} else {
							flags = "r";
						}
						const hostFd = host.openSync(p, flags);
						h = { dir: false, hostFd, pos: 0, append: (fdflags & 1) !== 0 };
					}
					const fd = nextFd++;
					handles.set(fd, h);
					view().setUint32(fdPtr, fd, true);
					return ESUCCESS;
				});
			},

			path_filestat_get(dirFd, flags, ptr, len, statPtr) {
				return guard(() => {
					putStat(statPtr, host.statSync(resolve(dirFd, ptr, len)));
					return ESUCCESS;
				});
			},

			// Einträge ab cookie (Index); je Eintrag 24 Byte Kopf (d_next,
			// d_ino, d_namlen, d_type) und der Name. Ein Eintrag, der nicht
			// mehr ganz passt, wird abgeschnitten, wie WASI es vorsieht.
			fd_readdir(fd, buf, bufLen, cookie, bufused) {
				return guard(() => {
					const h = handles.get(fd);
					if (!h || !h.dir) {
						return ENOTDIR;
					}
					const names = host.readdirSync(h.path);
					let used = 0;
					for (let i = Number(cookie); i < names.length && used < bufLen; i++) {
						const name = encoder.encode(names[i]);
						const full = h.path === "." ? names[i] : h.path + "/" + names[i];
						let type = 0;
						try {
							type = host.statSync(full).isDirectory() ? 3 : 4;
						} catch (err) {
							type = 0;
						}
						const ent = new Uint8Array(24 + name.length);
						const dv = new DataView(ent.buffer);
						dv.setBigUint64(0, BigInt(i + 1), true);
						dv.setUint32(16, name.length, true);
						dv.setUint8(20, type);
						ent.set(name, 24);
						const n = Math.min(ent.length, bufLen - used);
						bytes(buf + used, n).set(ent.subarray(0, n));
						used += n;
					}
					view().setUint32(bufused, used, true);
					return ESUCCESS;
				});
			},

			path_unlink_file(dirFd, ptr, len) {
				return guard(() => {
					host.unlinkSync(resolve(dirFd, ptr, len));
					return ESUCCESS;
				});
			},

			path_rename(oldFd, oldPtr, oldLen, newFd, newPtr, newLen) {
				return guard(() => {
					host.renameSync(resolve(oldFd, oldPtr, oldLen), resolve(newFd, newPtr, newLen));
					return ESUCCESS;
				});
			},

			path_create_directory(dirFd, ptr, len) {
				return guard(() => {
					host.mkdirSync(resolve(dirFd, ptr, len));
					return ESUCCESS;
				});
			},

			path_remove_directory(dirFd, ptr, len) {
				return guard(() => {
					host.rmdirSync(resolve(dirFd, ptr, len));
					return ESUCCESS;
				});
			},
		};
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
		// Umgebung: genau opts.env; ohne opts.env nur der UTC-Versatz für die
		// Ortszeit der Unit Dos (wie Go unter GOOS=js: einmal beim Start).
		go.env = opts.env ? Object.assign({}, opts.env)
			: { WASMPASCAL_UTC_OFFSET: String(-new Date().getTimezoneOffset()) };
		let memory = null;
		go.importObject.wasi_snapshot_preview1 = wasiFiles(() => memory, opts.files);
		const { module, instance } = await instantiate(source, go.importObject);
		memory = instance.exports.mem;
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
