// Package abi beschreibt die Ziel-ABIs. Stand Etappe 6 nur wasi: die
// WASI-Funktionen, die die Laufzeit (rtl/system.pas) über Intrinsics des
// Compilers aufruft. In Etappe 9 kommt gojs dazu; nur diese unterste Schicht
// der Laufzeit unterscheidet sich zwischen den Zielen (PLAN.md).
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
