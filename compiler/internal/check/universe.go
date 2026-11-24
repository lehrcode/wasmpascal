package check

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/abi"
	"github.com/lehrcode/wasmpascal/compiler/internal/types"
)

// Standardprozeduren und -funktionen der System-Unit.
const (
	BWrite types.BuiltinID = iota + 1
	BWriteLn
	BRead
	BReadLn
	BInc
	BDec
	BNew
	BDispose
	BGetMem
	BFreeMem
	BMark
	BRelease
	BHalt
	BExit
	BBreak
	BContinue
	BAssign
	BReset
	BRewrite
	BAppend
	BClose
	BErase
	BRename
	BFlush
	BTruncate
	BSeek
	BBlockRead
	BBlockWrite
	BDelete
	BInsert
	BStr
	BVal
	BFillChar
	BMove
	BRandomize
	BChDir
	BMkDir
	BRmDir
	BGetDir
	// Funktionen
	BAbs
	BArcTan
	BCos
	BSin
	BExp
	BLn
	BSqrt
	BFrac
	BInt
	BSqr
	BTrunc
	BRound
	BChr
	BOrd
	BOdd
	BSucc
	BPred
	BHi
	BLo
	BSwap
	BHigh
	BLow
	BSizeOf
	BLength
	BCopy
	BConcat
	BPos
	BUpCase
	BEof
	BEoln
	BSeekEof
	BSeekEoln
	BFilePos
	BFileSize
	BIOResult
	BMemAvail
	BMaxAvail
	BParamCount
	BParamStr
	BRandom
	BPi
	BPtr
	BAddr
	BOfs
	BSeg
	// Intrinsics für die Laufzeit in rtl/system.pas
	BMemGrow
	BMemSize
	BHeapBase
	BRunError
	BSuspend
	BGoSetSP
	BGoArgc
	BGoArgv
	bUnsupported

	// BWasiBase + i ist abi.Intrinsics[i].
	BWasiBase types.BuiltinID = 1000
)

var builtinProcs = []struct {
	name   string
	id     types.BuiltinID
	isFunc bool
}{
	{"Write", BWrite, false}, {"WriteLn", BWriteLn, false}, {"Read", BRead, false}, {"ReadLn", BReadLn, false},
	{"Inc", BInc, false}, {"Dec", BDec, false}, {"New", BNew, false}, {"Dispose", BDispose, false},
	{"GetMem", BGetMem, false}, {"FreeMem", BFreeMem, false}, {"Mark", BMark, false}, {"Release", BRelease, false},
	{"Halt", BHalt, false}, {"Exit", BExit, false}, {"Break", BBreak, false}, {"Continue", BContinue, false},
	{"Assign", BAssign, false}, {"Reset", BReset, false}, {"Rewrite", BRewrite, false}, {"Append", BAppend, false},
	{"Close", BClose, false}, {"Erase", BErase, false}, {"Rename", BRename, false}, {"Flush", BFlush, false},
	{"Truncate", BTruncate, false}, {"Seek", BSeek, false}, {"BlockRead", BBlockRead, false}, {"BlockWrite", BBlockWrite, false},
	{"Delete", BDelete, false}, {"Insert", BInsert, false}, {"Str", BStr, false}, {"Val", BVal, false},
	{"FillChar", BFillChar, false}, {"Move", BMove, false}, {"Randomize", BRandomize, false},
	{"ChDir", BChDir, false}, {"MkDir", BMkDir, false}, {"RmDir", BRmDir, false}, {"GetDir", BGetDir, false},

	{"Abs", BAbs, true}, {"ArcTan", BArcTan, true}, {"Cos", BCos, true}, {"Sin", BSin, true},
	{"Exp", BExp, true}, {"Ln", BLn, true}, {"Sqrt", BSqrt, true}, {"Frac", BFrac, true},
	{"Int", BInt, true}, {"Sqr", BSqr, true}, {"Trunc", BTrunc, true}, {"Round", BRound, true},
	{"Chr", BChr, true}, {"Ord", BOrd, true}, {"Odd", BOdd, true}, {"Succ", BSucc, true},
	{"Pred", BPred, true}, {"Hi", BHi, true}, {"Lo", BLo, true}, {"Swap", BSwap, true},
	{"High", BHigh, true}, {"Low", BLow, true}, {"SizeOf", BSizeOf, true}, {"Length", BLength, true},
	{"Copy", BCopy, true}, {"Concat", BConcat, true}, {"Pos", BPos, true}, {"UpCase", BUpCase, true},
	{"Eof", BEof, true}, {"Eoln", BEoln, true}, {"SeekEof", BSeekEof, true}, {"SeekEoln", BSeekEoln, true},
	{"FilePos", BFilePos, true}, {"FileSize", BFileSize, true}, {"IOResult", BIOResult, true},
	{"MemAvail", BMemAvail, true}, {"MaxAvail", BMaxAvail, true}, {"ParamCount", BParamCount, true},
	{"ParamStr", BParamStr, true}, {"Random", BRandom, true}, {"Pi", BPi, true},
	{"Ptr", BPtr, true}, {"Addr", BAddr, true}, {"Ofs", BOfs, true}, {"Seg", BSeg, true},
	{"_MemGrow", BMemGrow, true}, {"_MemSize", BMemSize, true}, {"_HeapBase", BHeapBase, true},
	{"_RunError", BRunError, false}, {"_Suspend", BSuspend, true},
	{"_GoSetSP", BGoSetSP, false}, {"_GoArgc", BGoArgc, true}, {"_GoArgv", BGoArgv, true},
}

// unsupported sind Bezeichner der System-Unit von TP 4.0, die es wegen des
// fehlenden DOS-Segmentmodells nicht gibt (PLAN.md, Adressfunktionen und
// Weggelassen). Ihre Verwendung ergibt Error 902.
var unsupported = []string{
	"CSeg", "DSeg", "SSeg", "SPtr", "PrefixSeg", "FreePtr", "FreeMin",
	"HeapOrg", "HeapPtr", "HeapError", "Mem", "MemW", "MemL", "Port", "PortW",
	"Intr", "MsDos", "OvrHeapOrg",
}

func newUniverse() *types.Scope {
	u := types.NewScope(types.UniverseScope, nil, nil)
	for _, t := range []*types.Basic{
		types.Boolean, types.Char, types.ShortInt, types.Byte, types.Integer, types.Word,
		types.LongInt, types.Real, types.Single, types.Double, types.Extended, types.Comp,
	} {
		u.Insert(types.NewTypeName(lower(t.String()), t.String(), noPos, t))
	}
	u.Insert(types.NewTypeName("text", "Text", noPos, types.Text))
	u.Insert(types.NewTypeName("pointer", "Pointer", noPos, types.PointerType))

	u.Insert(types.NewConst("false", "False", noPos, types.Boolean, types.IntVal(0)))
	u.Insert(types.NewConst("true", "True", noPos, types.Boolean, types.IntVal(1)))
	u.Insert(types.NewConst("maxint", "MaxInt", noPos, types.Integer, types.IntVal(types.MaxLongInt)))
	u.Insert(types.NewConst("maxlongint", "MaxLongInt", noPos, types.LongInt, types.IntVal(types.MaxLongInt)))

	// Input, Output, RandSeed, FileMode und ExitCode deklariert die
	// System-Unit (rtl/system.pas).
	for _, b := range builtinProcs {
		u.Insert(types.NewBuiltin(b.name, b.id, b.isFunc))
	}
	for i, w := range abi.Intrinsics {
		u.Insert(types.NewBuiltin(w.Name, BWasiBase+types.BuiltinID(i), w.Result))
	}
	for _, name := range unsupported {
		b := types.NewBuiltin(name, bUnsupported, false)
		b.Unsupported = true
		u.Insert(b)
	}
	u.Insert(types.NewUnit("System", u))
	return u
}

func lower(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'A' && ch <= 'Z' {
			b[i] = ch + 32
		}
	}
	return string(b)
}
