package ir

// Importierte Funktionen.
//
// Die Write-Funktionen im Modul "wasmpascal" sind vorläufig (PLAN.md,
// Etappe 4): Sie werden in Etappe 6 durch die Laufzeit in Pascal ersetzt, die
// direkt fd_write benutzt. Der Go-Runner stellt sie bereit.
var (
	HostProcExit = &Host{Module: "wasi_snapshot_preview1", Name: "proc_exit", Params: []ValType{I32}}

	HostWriteInt  = &Host{Module: "wasmpascal", Name: "write_int", Params: []ValType{I32, I32}}
	HostWriteReal = &Host{Module: "wasmpascal", Name: "write_real", Params: []ValType{F64, I32, I32}}
	HostWriteStr  = &Host{Module: "wasmpascal", Name: "write_str", Params: []ValType{I32, I32, I32}}
	HostWriteChar = &Host{Module: "wasmpascal", Name: "write_char", Params: []ValType{I32, I32}}
	HostWriteBool = &Host{Module: "wasmpascal", Name: "write_bool", Params: []ValType{I32, I32}}
	HostWriteLn   = &Host{Module: "wasmpascal", Name: "write_ln"}
)
