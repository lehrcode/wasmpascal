// Package abi beschreibt die Ziel-ABIs wasi und gojs: die Importe, die die
// unterste Schicht der Laufzeit (rtl/system.pas) über Intrinsics des
// Compilers aufruft. Nur diese Schicht unterscheidet sich zwischen den Zielen
// (PLAN.md). Dateien und Verzeichnisse nutzen unter beiden Zielen die
// WASI-Intrinsics; unter gojs stellt web/loader.js diese Importe bereit.
package abi

// ParamKind ist die WASM-Darstellung eines Arguments. In Pascal sind alle
// Argumente LongInt; i64-Parameter werden erweitert.
type ParamKind int

// Arten von Parametern.
const (
	I32  ParamKind = iota
	I64S           // LongInt mit Vorzeichen auf i64 erweitert
	I64U           // LongInt ohne Vorzeichen auf i64 erweitert
)

// Intrinsic ist eine WASI-Funktion, die in Pascal als Standardfunktion mit
// LongInt-Parametern erscheint (Ergebnis: WASI-Fehlernummer, 0 = Erfolg).
type Intrinsic struct {
	Name   string // Name in Pascal
	Module string
	Import string
	Params []ParamKind
	Result bool
}

const wasi = "wasi_snapshot_preview1"

// WASI sind die Intrinsics für das Ziel wasi.
var WASI = []Intrinsic{
	{"_FdWrite", wasi, "fd_write", []ParamKind{I32, I32, I32, I32}, true},
	{"_FdRead", wasi, "fd_read", []ParamKind{I32, I32, I32, I32}, true},
	{"_FdClose", wasi, "fd_close", []ParamKind{I32}, true},
	{"_FdSeek", wasi, "fd_seek", []ParamKind{I32, I64S, I32, I32}, true},
	{"_FdFilestatGet", wasi, "fd_filestat_get", []ParamKind{I32, I32}, true},
	{"_FdFilestatSetSize", wasi, "fd_filestat_set_size", []ParamKind{I32, I64U}, true},
	{"_PathOpen", wasi, "path_open", []ParamKind{I32, I32, I32, I32, I32, I64U, I64U, I32, I32}, true},
	{"_PathUnlinkFile", wasi, "path_unlink_file", []ParamKind{I32, I32, I32}, true},
	{"_PathRename", wasi, "path_rename", []ParamKind{I32, I32, I32, I32, I32, I32}, true},
	{"_PathCreateDirectory", wasi, "path_create_directory", []ParamKind{I32, I32, I32}, true},
	{"_PathRemoveDirectory", wasi, "path_remove_directory", []ParamKind{I32, I32, I32}, true},
	{"_ArgsSizesGet", wasi, "args_sizes_get", []ParamKind{I32, I32}, true},
	{"_ArgsGet", wasi, "args_get", []ParamKind{I32, I32}, true},
	{"_ClockTimeGet", wasi, "clock_time_get", []ParamKind{I32, I64U, I32}, true},
	{"_ProcExit", wasi, "proc_exit", []ParamKind{I32}, false},
	{"_PollOneoff", wasi, "poll_oneoff", []ParamKind{I32, I32, I32, I32}, true},
}

// GoJS sind die Intrinsics für das Ziel gojs: Funktionen des Moduls "gojs"
// aus wasm_exec.js. Jede nimmt den Go-Stackzeiger sp; Argumente und
// Ergebnisse liegen ab sp+8 in 8-Byte-Fächern.
var GoJS = []Intrinsic{
	{"_GoWasmExit", "gojs", "runtime.wasmExit", []ParamKind{I32}, false},
	{"_GoWasmWrite", "gojs", "runtime.wasmWrite", []ParamKind{I32}, false},
	{"_GoResetMemory", "gojs", "runtime.resetMemoryDataView", []ParamKind{I32}, false},
	{"_GoNanotime", "gojs", "runtime.nanotime1", []ParamKind{I32}, false},
	{"_GoWalltime", "gojs", "runtime.walltime", []ParamKind{I32}, false},
	{"_GoScheduleTimeout", "gojs", "runtime.scheduleTimeoutEvent", []ParamKind{I32}, false},
	{"_GoClearTimeout", "gojs", "runtime.clearTimeoutEvent", []ParamKind{I32}, false},
	{"_GoFinalizeRef", "gojs", "syscall/js.finalizeRef", []ParamKind{I32}, false},
	{"_GoValueGet", "gojs", "syscall/js.valueGet", []ParamKind{I32}, false},
	{"_GoValueSet", "gojs", "syscall/js.valueSet", []ParamKind{I32}, false},
	{"_GoValueIndex", "gojs", "syscall/js.valueIndex", []ParamKind{I32}, false},
	{"_GoValueCall", "gojs", "syscall/js.valueCall", []ParamKind{I32}, false},
	{"_GoValuePrepareString", "gojs", "syscall/js.valuePrepareString", []ParamKind{I32}, false},
	{"_GoValueLoadString", "gojs", "syscall/js.valueLoadString", []ParamKind{I32}, false},
}

// Intrinsics sind alle Intrinsics beider Ziele; ihr Index ergibt die
// Nummer der Standardroutine im Compiler.
var Intrinsics = append(append([]Intrinsic{}, WASI...), GoJS...)
