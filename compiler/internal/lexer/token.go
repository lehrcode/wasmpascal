package lexer

import "github.com/lehrcode/wasmpascal/compiler/internal/source"

// Kind ist die Art eines Tokens.
type Kind int

// Token-Arten.
const (
	EOF Kind = iota
	Ident
	Int    // 123 oder $FF; Text enthält die Schreibweise
	Real   // 1.5, 2E10, 1.5e-3
	String // 'abc', #13, ^M und Verkettungen davon; Value enthält die Bytes

	// Symbole
	Plus      // +
	Minus     // -
	Star      // *
	Slash     // /
	Eq        // =
	Ne        // <>
	Lt        // <
	Le        // <=
	Gt        // >
	Ge        // >=
	LBrack    // [ oder (.
	RBrack    // ] oder .)
	Dot       // .
	DotDot    // ..
	Comma     // ,
	LParen    // (
	RParen    // )
	Colon     // :
	Semicolon // ;
	Assign    // :=
	Caret     // ^ (Zeiger)
	At        // @

	keywordStart
	And
	Array
	Begin
	Case
	Const
	Div
	Do
	Downto
	Else
	End
	File
	For
	Function
	Goto
	If
	Implementation
	In
	Inline
	Interface
	Label
	Mod
	Nil
	Not
	Of
	Or
	Packed
	Procedure
	Program
	Record
	Repeat
	Set
	Shl
	Shr
	StringKw
	Then
	To
	Type
	Unit
	Until
	Uses
	Var
	While
	With
	Xor
	// Für eine spätere OOP-Erweiterung reserviert, derzeit ohne Bedeutung.
	Object
	Constructor
	Destructor
	Inherited
	keywordEnd
)

var kindNames = [...]string{
	EOF:       "end of file",
	Ident:     "identifier",
	Int:       "integer constant",
	Real:      "real constant",
	String:    "string constant",
	Plus:      "+",
	Minus:     "-",
	Star:      "*",
	Slash:     "/",
	Eq:        "=",
	Ne:        "<>",
	Lt:        "<",
	Le:        "<=",
	Gt:        ">",
	Ge:        ">=",
	LBrack:    "[",
	RBrack:    "]",
	Dot:       ".",
	DotDot:    "..",
	Comma:     ",",
	LParen:    "(",
	RParen:    ")",
	Colon:     ":",
	Semicolon: ";",
	Assign:    ":=",
	Caret:     "^",
	At:        "@",

	And:            "and",
	Array:          "array",
	Begin:          "begin",
	Case:           "case",
	Const:          "const",
	Div:            "div",
	Do:             "do",
	Downto:         "downto",
	Else:           "else",
	End:            "end",
	File:           "file",
	For:            "for",
	Function:       "function",
	Goto:           "goto",
	If:             "if",
	Implementation: "implementation",
	In:             "in",
	Inline:         "inline",
	Interface:      "interface",
	Label:          "label",
	Mod:            "mod",
	Nil:            "nil",
	Not:            "not",
	Of:             "of",
	Or:             "or",
	Packed:         "packed",
	Procedure:      "procedure",
	Program:        "program",
	Record:         "record",
	Repeat:         "repeat",
	Set:            "set",
	Shl:            "shl",
	Shr:            "shr",
	StringKw:       "string",
	Then:           "then",
	To:             "to",
	Type:           "type",
	Unit:           "unit",
	Until:          "until",
	Uses:           "uses",
	Var:            "var",
	While:          "while",
	With:           "with",
	Xor:            "xor",
	Object:         "object",
	Constructor:    "constructor",
	Destructor:     "destructor",
	Inherited:      "inherited",
}

// String liefert die Schreibweise (Symbole, Schlüsselwörter) oder eine
// Beschreibung (übrige Arten), passend für Meldungen wie `";" expected`.
func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return "?"
}

// IsKeyword meldet, ob k ein reserviertes Wort ist.
func (k Kind) IsKeyword() bool { return k > keywordStart && k < keywordEnd }

// TP-ANNAHME: Reserviert sind die Wörter, die auch spätere TP-Versionen
// reservieren (einschließlich inline). absolute, external, forward und
// interrupt gelten wie ab TP 5 als Standarddirektiven, also normale Bezeichner,
// die der Parser im Kontext erkennt.
var keywords = func() map[string]Kind {
	m := map[string]Kind{}
	for k := keywordStart + 1; k < keywordEnd; k++ {
		m[kindNames[k]] = k
	}
	return m
}()

// Keywords liefert alle reservierten Wörter in Kleinschreibung.
func Keywords() map[string]Kind {
	m := make(map[string]Kind, len(keywords))
	for k, v := range keywords {
		m[k] = v
	}
	return m
}

// MaxIdentLen ist die Zahl signifikanter Zeichen eines Bezeichners (wie TP).
const MaxIdentLen = 63

// Token ist ein Token mit Position.
type Token struct {
	Kind Kind
	Pos  source.Pos
	// Text ist die Schreibweise im Quelltext.
	Text string
	// Name ist bei Bezeichnern und Schlüsselwörtern die normalisierte Form
	// (Kleinbuchstaben, höchstens MaxIdentLen Zeichen) für Vergleiche.
	Name string
	// Value enthält bei String-Tokens die Bytes des Strings (UTF-8).
	Value string
	// CodePoint ist bei String-Tokens aus genau einem #n mit n über 255 der
	// Codepoint (eine WideChar-Konstante), sonst 0.
	CodePoint rune
	// IOChecks ist der Zustand des Schalters {$I+}/{$I-} an dieser Stelle.
	IOChecks bool
}
