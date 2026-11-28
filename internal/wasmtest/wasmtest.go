// Package wasmtest baut winzige, handkodierte WASI-Module für Tests von Runner
// und Test-Harness, solange der Compiler noch keine Module erzeugt.
package wasmtest

// Print liefert ein Modul, das msg per fd_write auf fd (1 = stdout, 2 = stderr)
// schreibt und sich dann mit proc_exit(code) beendet.
func Print(fd int, msg string, code int) []byte {
	// Speicher: iovec {buf=16, len} bei 0, nwritten bei 8, Text ab 16.
	data := make([]byte, 16, 16+len(msg))
	putU32(data[0:], 16)
	putU32(data[4:], uint32(len(msg)))
	data = append(data, msg...)

	body := cat(
		[]byte{0x41}, sleb(int64(fd)), // i32.const fd
		[]byte{
			0x41, 0x00, // i32.const 0 (iovs)
			0x41, 0x01, // i32.const 1 (iovs_len)
			0x41, 0x08, // i32.const 8 (nwritten)
			0x10, 0x00, // call fd_write
			0x1a, // drop
		},
		[]byte{0x41}, sleb(int64(code)), // i32.const code
		[]byte{0x10, 0x01, 0x0b}, // call proc_exit; end
	)
	return module(body, data)
}

// Loop liefert ein Modul, dessen _start endlos läuft.
func Loop() []byte {
	return module([]byte{0x03, 0x40, 0x0c, 0x00, 0x0b, 0x0b}, nil) // loop br 0 end end
}

// Return liefert ein Modul, dessen _start sofort zurückkehrt (Exit-Code 0).
func Return() []byte {
	return module([]byte{0x0b}, nil)
}

// Environ liefert ein Modul, das die Umgebung per environ_sizes_get/environ_get
// liest und den environ-Puffer ("NAME=WERT\0" hintereinander) per fd_write auf
// stdout schreibt, dann proc_exit(0).
func Environ() []byte {
	// Speicher: Sizes bei 0, iovec bei 8, nwritten bei 16, Zeiger bei 32,
	// environ-Puffer ab 1024.
	body := cat(
		[]byte{0x41, 0x00, 0x41, 0x04, 0x10, 0x02, 0x1a},               // environ_sizes_get(0, 4)
		[]byte{0x41, 0x20, 0x41}, sleb(1024), []byte{0x10, 0x03, 0x1a}, // environ_get(32, 1024)
		[]byte{0x41, 0x08, 0x41}, sleb(1024), []byte{0x36, 0x02, 0x00}, // [8] = 1024 (iov.buf)
		[]byte{0x41, 0x0c, 0x41, 0x04, 0x28, 0x02, 0x00, 0x36, 0x02, 0x00},       // [12] = [4] (iov.len)
		[]byte{0x41, 0x01, 0x41, 0x08, 0x41, 0x01, 0x41, 0x10, 0x10, 0x00, 0x1a}, // fd_write(1, 8, 1, 16)
		[]byte{0x41, 0x00, 0x10, 0x01, 0x0b},                                     // proc_exit(0); end
	)
	m := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	m = section(m, 1, vec( // Typen
		[]byte{0x60, 0x04, 0x7f, 0x7f, 0x7f, 0x7f, 0x01, 0x7f}, // 0: fd_write
		[]byte{0x60, 0x01, 0x7f, 0x00},                         // 1: proc_exit
		[]byte{0x60, 0x00, 0x00},                               // 2: _start
		[]byte{0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f},             // 3: environ_sizes_get/environ_get
	))
	m = section(m, 2, vec( // Importe
		cat(name("wasi_snapshot_preview1"), name("fd_write"), []byte{0x00, 0x00}),
		cat(name("wasi_snapshot_preview1"), name("proc_exit"), []byte{0x00, 0x01}),
		cat(name("wasi_snapshot_preview1"), name("environ_sizes_get"), []byte{0x00, 0x03}),
		cat(name("wasi_snapshot_preview1"), name("environ_get"), []byte{0x00, 0x03}),
	))
	m = section(m, 3, vec([]byte{0x02}))       // eine definierte Funktion: _start (Index 4)
	m = section(m, 5, vec([]byte{0x00, 0x01})) // Speicher: 1 Seite
	m = section(m, 7, vec(                     // Exporte
		cat(name("memory"), []byte{0x02, 0x00}),
		cat(name("_start"), []byte{0x00, 0x04}),
	))
	fn := cat([]byte{0x00}, body) // keine Locals
	m = section(m, 10, vec(cat(uleb(uint64(len(fn))), fn)))
	return m
}

func module(body, data []byte) []byte {
	m := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	m = section(m, 1, vec( // Typen
		[]byte{0x60, 0x04, 0x7f, 0x7f, 0x7f, 0x7f, 0x01, 0x7f}, // fd_write
		[]byte{0x60, 0x01, 0x7f, 0x00},                         // proc_exit
		[]byte{0x60, 0x00, 0x00},                               // _start
	))
	m = section(m, 2, vec( // Importe
		cat(name("wasi_snapshot_preview1"), name("fd_write"), []byte{0x00, 0x00}),
		cat(name("wasi_snapshot_preview1"), name("proc_exit"), []byte{0x00, 0x01}),
	))
	m = section(m, 3, vec([]byte{0x02}))       // Funktionen
	m = section(m, 5, vec([]byte{0x00, 0x01})) // Speicher: 1 Seite
	m = section(m, 7, vec(                     // Exporte
		cat(name("memory"), []byte{0x02, 0x00}),
		cat(name("_start"), []byte{0x00, 0x02}),
	))
	fn := cat([]byte{0x00}, body) // keine Locals
	m = section(m, 10, vec(cat(uleb(uint64(len(fn))), fn)))
	if data != nil {
		m = section(m, 11, vec(cat([]byte{0x00, 0x41, 0x00, 0x0b}, uleb(uint64(len(data))), data)))
	}
	return m
}

func section(m []byte, id byte, content []byte) []byte {
	m = append(m, id)
	m = append(m, uleb(uint64(len(content)))...)
	return append(m, content...)
}

func vec(items ...[]byte) []byte {
	return cat(append([][]byte{uleb(uint64(len(items)))}, items...)...)
}

func name(s string) []byte { return cat(uleb(uint64(len(s))), []byte(s)) }

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func uleb(v uint64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func sleb(v int64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func putU32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
}
