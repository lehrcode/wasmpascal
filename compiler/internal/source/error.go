package source

import "fmt"

// Pos ist eine Stelle im Quelltext. Line und Col sind 1-basiert; Col zählt
// Zeichen (UTF-8-Codepoints), nicht Bytes. 0 bedeutet „unbekannt“.
type Pos struct {
	File      string
	Line, Col int
}

// String formatiert die Stelle wie Turbo Pascal: "datei.pas(zeile,spalte)".
func (p Pos) String() string {
	switch {
	case p.Line > 0 && p.Col > 0:
		return fmt.Sprintf("%s(%d,%d)", p.File, p.Line, p.Col)
	case p.Line > 0:
		return fmt.Sprintf("%s(%d)", p.File, p.Line)
	default:
		return p.File
	}
}

// Fehlernummern. Sie folgen der Nummerierung von Turbo Pascal; Nummern ab 900
// sind eigene Fehler von WasmPascal ohne Entsprechung in TP.
//
// TP-ANNAHME: Die Nummern stammen aus der Fehlerliste von Turbo Pascal 7, die
// für die hier benutzten Fehler mit TP 4.0 übereinstimmen dürfte.
const (
	ErrIdentExpected      = 2
	ErrSyntax             = 5
	ErrRealConst          = 6
	ErrIntConst           = 7
	ErrStringExceedsLine  = 8
	ErrTooManyNestedFiles = 9
	ErrUnexpectedEOF      = 10
	ErrInvalidFileName    = 14
	ErrFileNotFound       = 15
	ErrInvalidDirective   = 17
	ErrTypeIdentExpected  = 12
	ErrInType             = 21
	ErrBeginExpected      = 36
	ErrEndExpected        = 37
	ErrInExpression       = 42
	ErrDoExpected         = 50
	ErrOfExpected         = 54
	ErrInterfaceExpected  = 55
	ErrThenExpected       = 57
	ErrToDowntoExpected   = 58
	ErrImplExpected       = 73
	ErrCircularUnit       = 68
	ErrUnitNameMismatch   = 69
	ErrUnitExpected       = 84
	ErrConstOutOfRange    = 76
	ErrSemicolonExpected  = 85
	ErrColonExpected      = 86
	ErrCommaExpected      = 87
	ErrLParenExpected     = 88
	ErrRParenExpected     = 89
	ErrEqExpected         = 90
	ErrAssignExpected     = 91
	ErrLBrackExpected     = 92
	ErrRBrackExpected     = 93
	ErrDotExpected        = 94
	ErrDotDotExpected     = 95
	ErrInStatement        = 113

	// Typprüfung
	ErrUnknownIdent        = 3
	ErrDuplicateIdent      = 4
	ErrUndefinedPtrType    = 19
	ErrVarIdentExpected    = 20
	ErrStructTooLarge      = 22
	ErrSetBaseRange        = 23
	ErrInvalidStringLen    = 25
	ErrTypeMismatch        = 26
	ErrInvalidSubrangeBase = 27
	ErrLowerGtUpper        = 28
	ErrOrdinalTypeExpected = 29
	ErrIntConstExpected    = 30
	ErrConstExpected       = 31
	ErrInvalidResultType   = 34
	ErrIntExprExpected     = 38
	ErrOrdinalExprExpected = 39
	ErrBoolExprExpected    = 40
	ErrOperandTypes        = 41
	ErrIllegalAssignment   = 43
	ErrFieldIdentExpected  = 44
	ErrUndefinedForward    = 59
	ErrInvalidTypecast     = 61
	ErrDivisionByZero      = 62
	ErrInvalidFileType     = 63
	ErrCannotReadWrite     = 64
	ErrPointerVarExpected  = 65
	ErrStringVarExpected   = 66
	ErrCaseConstType       = 74
	ErrRecordVarExpected   = 75
	ErrFileVarExpected     = 77
	ErrNumExprExpected     = 79
	ErrLabelNotInBlock     = 80
	ErrLabelDefined        = 81
	ErrInvalidAt           = 83
	ErrInvalidForVar       = 97
	ErrIntVarExpected      = 98
	ErrOrdinalVarExpected  = 104
	ErrInvalidVarRef       = 122
	ErrFilesMustBeVar      = 126
	ErrHeaderMismatch      = 131
	ErrInvalidFormat       = 135
	ErrInvalidProcRef      = 143

	ErrInvalidUTF8  = 900
	ErrNonASCII     = 901
	ErrNotSupported = 902
	ErrBreakOutside = 903
)

var messages = map[int]string{
	ErrIdentExpected:     "Identifier expected",
	ErrTypeIdentExpected: "Type identifier expected",
	ErrInType:            "Error in type",
	ErrBeginExpected:     "BEGIN expected",
	ErrEndExpected:       "END expected",
	ErrInExpression:      "Error in expression",
	ErrDoExpected:        "DO expected",
	ErrOfExpected:        "OF expected",
	ErrInterfaceExpected: "INTERFACE expected",
	ErrThenExpected:      "THEN expected",
	ErrToDowntoExpected:  "TO or DOWNTO expected",
	ErrImplExpected:      "IMPLEMENTATION expected",
	ErrCircularUnit:      "Circular unit reference",
	ErrUnitNameMismatch:  "Unit name mismatch",
	ErrUnitExpected:      "UNIT expected",
	ErrSemicolonExpected: `";" expected`,
	ErrColonExpected:     `":" expected`,
	ErrCommaExpected:     `"," expected`,
	ErrLParenExpected:    `"(" expected`,
	ErrRParenExpected:    `")" expected`,
	ErrEqExpected:        `"=" expected`,
	ErrAssignExpected:    `":=" expected`,
	ErrLBrackExpected:    `"[" expected`,
	ErrRBrackExpected:    `"]" expected`,
	ErrDotExpected:       `"." expected`,
	ErrDotDotExpected:    `".." expected`,
	ErrInStatement:       "Error in statement",
	ErrNotSupported:      "Not supported by WasmPascal",
	ErrBreakOutside:      "BREAK or CONTINUE outside of loop",

	ErrUnknownIdent:        "Unknown identifier",
	ErrDuplicateIdent:      "Duplicate identifier",
	ErrUndefinedPtrType:    "Undefined type in pointer definition",
	ErrVarIdentExpected:    "Variable identifier expected",
	ErrStructTooLarge:      "Structure too large",
	ErrSetBaseRange:        "Set base type out of range",
	ErrInvalidStringLen:    "Invalid string length",
	ErrTypeMismatch:        "Type mismatch",
	ErrInvalidSubrangeBase: "Invalid subrange base type",
	ErrLowerGtUpper:        "Lower bound greater than upper bound",
	ErrOrdinalTypeExpected: "Ordinal type expected",
	ErrIntConstExpected:    "Integer constant expected",
	ErrConstExpected:       "Constant expected",
	ErrInvalidResultType:   "Invalid function result type",
	ErrIntExprExpected:     "Integer expression expected",
	ErrOrdinalExprExpected: "Ordinal expression expected",
	ErrBoolExprExpected:    "Boolean expression expected",
	ErrOperandTypes:        "Operand types do not match operator",
	ErrIllegalAssignment:   "Illegal assignment",
	ErrFieldIdentExpected:  "Field identifier expected",
	ErrUndefinedForward:    "Undefined forward",
	ErrInvalidTypecast:     "Invalid typecast",
	ErrDivisionByZero:      "Division by zero",
	ErrInvalidFileType:     "Invalid file type",
	ErrCannotReadWrite:     "Cannot Read or Write variables of this type",
	ErrPointerVarExpected:  "Pointer variable expected",
	ErrStringVarExpected:   "String variable expected",
	ErrCaseConstType:       "Constant and case types do not match",
	ErrRecordVarExpected:   "Record variable expected",
	ErrFileVarExpected:     "File variable expected",
	ErrNumExprExpected:     "Integer or real expression expected",
	ErrLabelNotInBlock:     "Label not within current block",
	ErrLabelDefined:        "Label already defined",
	ErrInvalidAt:           "Invalid @ argument",
	ErrInvalidForVar:       "Invalid FOR control variable",
	ErrIntVarExpected:      "Integer variable expected",
	ErrOrdinalVarExpected:  "Ordinal variable expected",
	ErrInvalidVarRef:       "Invalid variable reference",
	ErrFilesMustBeVar:      "Files must be var parameters",
	ErrHeaderMismatch:      "Header does not match previous definition",
	ErrInvalidFormat:       "Invalid format specifier",
	ErrInvalidProcRef:      "Invalid procedure or function reference",
	ErrSyntax:              "Syntax error",
	ErrRealConst:           "Error in real constant",
	ErrIntConst:            "Error in integer constant",
	ErrStringExceedsLine:   "String constant exceeds line",
	ErrTooManyNestedFiles:  "Too many nested files",
	ErrUnexpectedEOF:       "Unexpected end of file",
	ErrInvalidFileName:     "Invalid file name",
	ErrFileNotFound:        "File not found",
	ErrInvalidDirective:    "Invalid compiler directive",
	ErrConstOutOfRange:     "Constant out of range",
	ErrInvalidUTF8:         "Source file is not valid UTF-8; convert it with: iconv -f CP437 -t UTF-8",
	ErrNonASCII:            "Invalid character; identifiers and symbols must be ASCII",
}

// Error ist ein Compilerfehler mit Position und TP-Fehlernummer.
type Error struct {
	Pos  Pos
	Code int
	Msg  string
}

// Errorf erzeugt einen Fehler mit dem Standardtext zu code; detail wird, falls
// nicht leer, in Klammern angehängt.
func Errorf(pos Pos, code int, detail string, args ...any) *Error {
	msg := messages[code]
	if msg == "" {
		msg = fmt.Sprintf("Error %d", code)
	}
	if detail != "" {
		msg += " (" + fmt.Sprintf(detail, args...) + ")"
	}
	return &Error{Pos: pos, Code: code, Msg: msg}
}

// Error formatiert wie Turbo Pascal: "datei.pas(zeile,spalte): Error NN: Text".
func (e *Error) Error() string {
	return fmt.Sprintf("%s: Error %d: %s", e.Pos, e.Code, e.Msg)
}
