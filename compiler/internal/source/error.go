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
const (
	ErrSyntax             = 5
	ErrRealConst          = 6
	ErrIntConst           = 7
	ErrStringExceedsLine  = 8
	ErrTooManyNestedFiles = 9
	ErrUnexpectedEOF      = 10
	ErrInvalidFileName    = 14
	ErrFileNotFound       = 15
	ErrInvalidDirective   = 17
	ErrConstOutOfRange    = 76

	ErrInvalidUTF8 = 900
	ErrNonASCII    = 901
)

var messages = map[int]string{
	ErrSyntax:             "Syntax error",
	ErrRealConst:          "Error in real constant",
	ErrIntConst:           "Error in integer constant",
	ErrStringExceedsLine:  "String constant exceeds line",
	ErrTooManyNestedFiles: "Too many nested files",
	ErrUnexpectedEOF:      "Unexpected end of file",
	ErrInvalidFileName:    "Invalid file name",
	ErrFileNotFound:       "File not found",
	ErrInvalidDirective:   "Invalid compiler directive",
	ErrConstOutOfRange:    "Constant out of range",
	ErrInvalidUTF8:        "Source file is not valid UTF-8; convert it with: iconv -f CP437 -t UTF-8",
	ErrNonASCII:           "Invalid character; identifiers and symbols must be ASCII",
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
