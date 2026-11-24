package lexer

import (
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

// maxIncludeDepth begrenzt die Verschachtelung von {$I}-Dateien (fängt auch
// rekursive Includes ab).
const maxIncludeDepth = 16

// ctrlZ beendet wie unter DOS eine Quelldatei.
const ctrlZ = 0x1A

type fileState struct {
	f         *source.File
	off       int
	line      int
	lineStart int
}

func (s *fileState) eof() bool {
	return s.off >= len(s.f.Data) || s.f.Data[s.off] == ctrlZ
}

func (s *fileState) peek(n int) byte {
	if s.off+n < len(s.f.Data) {
		return s.f.Data[s.off+n]
	}
	return 0
}

// Lexer zerlegt eine Quelldatei samt {$I}-Includes in Tokens und wertet dabei
// die Direktiven aus.
type Lexer struct {
	loader    *source.Loader
	files     []*fileState // Include-Stapel, oben die aktuelle Datei
	defines   source.Defines
	cond      source.Conditionals
	switches  map[byte]bool
	stackSize int
	typeMode  bool
	prev      Kind
}

// defaultSwitches sind die Voreinstellungen der Schalter von TP 4.0. Außer I
// haben sie keine Wirkung, werden aber für {$IFOPT} mitgeführt.
var defaultSwitches = map[byte]bool{
	'B': false, 'D': true, 'E': true, 'F': false, 'I': true, 'L': true,
	'N': false, 'R': false, 'S': true, 'T': false, 'V': true,
}

// New öffnet die Datei name aus dem Loader. defines sind die bedingten Symbole
// zu Beginn (siehe source.NewDefines); der Lexer arbeitet auf einer Kopie.
func New(loader *source.Loader, name string, defines source.Defines) (*Lexer, error) {
	f, err := loader.Load(name)
	if err != nil {
		var se *source.Error
		if errors.As(err, &se) {
			return nil, se
		}
		return nil, source.Errorf(source.Pos{File: name}, source.ErrFileNotFound, "")
	}
	l := &Lexer{
		loader:   loader,
		files:    []*fileState{{f: f, line: 1}},
		defines:  defines.Clone(),
		switches: map[byte]bool{},
		prev:     -1,
	}
	for k, v := range defaultSwitches {
		l.switches[k] = v
	}
	return l, nil
}

// SetTypeMode legt fest, ob ein folgendes ^ als Zeigertyp gelesen wird. Der
// Parser setzt den Modus, bevor er an einer Stelle weiterliest, an der ein Typ
// beginnt (nach "=" im type-Teil, nach ":" in Deklarationen, nach "of"), weil
// ^C dort ein Zeiger auf den Typ C und kein Steuerzeichen ist.
func (l *Lexer) SetTypeMode(on bool) { l.typeMode = on }

// StackSize liefert den ersten Wert der letzten {$M}-Direktive, sonst 0.
func (l *Lexer) StackSize() int { return l.stackSize }

func (l *Lexer) cur() *fileState { return l.files[len(l.files)-1] }

func (l *Lexer) posAt(s *fileState, off int) source.Pos {
	return source.Pos{
		File: s.f.Name,
		Line: s.line,
		Col:  utf8.RuneCount(s.f.Data[s.lineStart:off]) + 1,
	}
}

func (l *Lexer) pos() source.Pos { s := l.cur(); return l.posAt(s, s.off) }

func (l *Lexer) errorf(code int, detail string, args ...any) error {
	return source.Errorf(l.pos(), code, detail, args...)
}

// Next liefert das nächste Token. Nach dem Ende kommt immer wieder EOF.
func (l *Lexer) Next() (Token, error) {
	tok, err := l.next()
	if err != nil {
		return Token{}, err
	}
	tok.IOChecks = l.switches['I']
	l.prev = tok.Kind
	return tok, nil
}

func (l *Lexer) next() (Token, error) {
	if err := l.skipTrivia(); err != nil {
		return Token{}, err
	}
	s := l.cur()
	pos := l.pos()
	if s.eof() {
		return Token{Kind: EOF, Pos: pos}, nil
	}
	start := s.off
	c := s.f.Data[s.off]
	tok := Token{Pos: pos}
	switch {
	case isLetter(c) || c == '_':
		for s.off < len(s.f.Data) && isIdentChar(s.f.Data[s.off]) {
			s.off++
		}
		tok.Text = string(s.f.Data[start:s.off])
		tok.Name = strings.ToLower(tok.Text)
		if k, ok := keywords[tok.Name]; ok {
			tok.Kind = k
		} else {
			tok.Kind = Ident
			if len(tok.Name) > MaxIdentLen {
				tok.Name = tok.Name[:MaxIdentLen]
			}
		}
		return tok, nil
	case isDigit(c):
		return l.number(tok)
	case c == '$':
		s.off++
		if !isHexDigit(s.peek(0)) {
			return Token{}, source.Errorf(pos, source.ErrIntConst, "")
		}
		for isHexDigit(s.peek(0)) {
			s.off++
		}
		tok.Kind, tok.Text = Int, string(s.f.Data[start:s.off])
		return tok, nil
	case c == '\'' || c == '#' || (c == '^' && l.ctrlCharAhead(true)):
		return l.stringConst(tok)
	case c >= utf8.RuneSelf:
		r, _ := utf8.DecodeRune(s.f.Data[s.off:])
		return Token{}, source.Errorf(pos, source.ErrNonASCII, "%q", r)
	}

	two := string([]byte{c, s.peek(1)})
	switch two {
	case ":=", "<=", ">=", "<>", "..", "(.", ".)":
		s.off += 2
		tok.Text = two
		tok.Kind = map[string]Kind{":=": Assign, "<=": Le, ">=": Ge, "<>": Ne, "..": DotDot, "(.": LBrack, ".)": RBrack}[two]
		return tok, nil
	}
	k, ok := map[byte]Kind{
		'+': Plus, '-': Minus, '*': Star, '/': Slash, '=': Eq, '<': Lt, '>': Gt,
		'[': LBrack, ']': RBrack, '.': Dot, ',': Comma, '(': LParen, ')': RParen,
		':': Colon, ';': Semicolon, '^': Caret, '@': At,
	}[c]
	if !ok {
		// TP-ANNAHME: Ein unzulässiges ASCII-Zeichen ergibt Error 5.
		return Token{}, source.Errorf(pos, source.ErrSyntax, "invalid character %q", rune(c))
	}
	s.off++
	tok.Kind, tok.Text = k, string(c)
	return tok, nil
}

// number liest eine Ganzzahl oder eine Real-Konstante.
func (l *Lexer) number(tok Token) (Token, error) {
	s := l.cur()
	start := s.off
	digits := func() int {
		n := 0
		for isDigit(s.peek(0)) {
			s.off++
			n++
		}
		return n
	}
	digits()
	tok.Kind = Int
	// "1..5" und "1.)" sind Ganzzahl plus Symbol; nur "1.5" ist eine Real-Zahl.
	if s.peek(0) == '.' && isDigit(s.peek(1)) {
		s.off++
		digits()
		tok.Kind = Real
	}
	if c := s.peek(0); c == 'e' || c == 'E' {
		s.off++
		if c := s.peek(0); c == '+' || c == '-' {
			s.off++
		}
		if digits() == 0 {
			return Token{}, source.Errorf(tok.Pos, source.ErrRealConst, "")
		}
		tok.Kind = Real
	}
	tok.Text = string(s.f.Data[start:s.off])
	return tok, nil
}

// ctrlCharAhead meldet, ob an der aktuellen Stelle (auf "^") ein Steuerzeichen
// wie ^M steht. atTokenStart gilt für den Beginn eines Tokens: Dann darf das ^ weder
// im Typmodus noch nach einem Operanden stehen (dort ist es ein Zeiger-Symbol).
func (l *Lexer) ctrlCharAhead(atTokenStart bool) bool {
	s := l.cur()
	c := s.peek(1)
	if !(c >= '@' && c <= '_' || c >= 'a' && c <= 'z') {
		return false
	}
	// ^Integer ist nie ein Steuerzeichen mit folgendem Bezeichner.
	if isIdentChar(s.peek(2)) {
		return false
	}
	if atTokenStart {
		if l.typeMode {
			return false
		}
		switch l.prev {
		case Ident, RParen, RBrack, Caret:
			return false
		}
	}
	return true
}

// stringConst liest eine Zeichenkettenkonstante: eine lückenlose Folge aus
// 'Text' (mit ” für ein Hochkomma), #nnn, #$hh und ^X.
func (l *Lexer) stringConst(tok Token) (Token, error) {
	s := l.cur()
	start := s.off
	var val []byte
loop:
	for !s.eof() {
		switch c := s.f.Data[s.off]; {
		case c == '\'':
			s.off++
			for {
				if s.eof() || s.f.Data[s.off] == '\n' || s.f.Data[s.off] == '\r' {
					return Token{}, source.Errorf(tok.Pos, source.ErrStringExceedsLine, "")
				}
				b := s.f.Data[s.off]
				s.off++
				if b == '\'' {
					if s.peek(0) != '\'' {
						break
					}
					s.off++
				}
				val = append(val, b)
			}
		case c == '#':
			numPos := l.pos()
			s.off++
			base, ok := 10, isDigit
			if s.peek(0) == '$' {
				s.off++
				base, ok = 16, isHexDigit
			}
			ds := s.off
			for ok(s.peek(0)) {
				s.off++
			}
			if ds == s.off {
				return Token{}, source.Errorf(numPos, source.ErrIntConst, "")
			}
			n, err := strconv.ParseUint(string(s.f.Data[ds:s.off]), base, 32)
			if err != nil || n > 255 {
				return Token{}, source.Errorf(numPos, source.ErrConstOutOfRange, "")
			}
			val = append(val, byte(n))
		case c == '^' && l.ctrlCharAhead(s.off == start):
			// TP-ANNAHME: ^a ist gleichbedeutend mit ^A.
			ch := s.peek(1)
			if ch >= 'a' && ch <= 'z' {
				ch -= 'a' - 'A'
			}
			val = append(val, ch^0x40)
			s.off += 2
		default:
			break loop
		}
	}
	tok.Kind = String
	tok.Text = string(s.f.Data[start:s.off])
	tok.Value = string(val)
	return tok, nil
}

// skipTrivia überspringt Leerraum, Kommentare und inaktive Bereiche, wertet
// Direktiven aus und kehrt am Ende einer Include-Datei in die einbindende zurück.
func (l *Lexer) skipTrivia() error {
	for {
		s := l.cur()
		if s.eof() {
			if len(l.files) > 1 {
				l.files = l.files[:len(l.files)-1]
				continue
			}
			if pos, open := l.cond.Open(); open {
				// TP-ANNAHME: ein fehlendes {$ENDIF} meldet Error 10 am Dateiende.
				return source.Errorf(l.pos(), source.ErrUnexpectedEOF, "missing {$ENDIF} for directive at %d,%d", pos.Line, pos.Col)
			}
			return nil
		}
		c := s.f.Data[s.off]
		switch {
		case c == '\n':
			s.off++
			s.line, s.lineStart = s.line+1, s.off
			continue
		case c <= ' ':
			s.off++
			continue
		case c == '{':
			if err := l.comment(1, "}"); err != nil {
				return err
			}
			continue
		case c == '(' && s.peek(1) == '*':
			if err := l.comment(2, "*)"); err != nil {
				return err
			}
			continue
		}
		if l.cond.Active() {
			return nil
		}
		// Inaktiver Bereich: Strings überspringen, damit darin keine
		// Kommentarzeichen gesehen werden; sonst Byte für Byte weiter.
		s.off++
		if c == '\'' {
			for !s.eof() && s.f.Data[s.off] != '\'' && s.f.Data[s.off] != '\n' {
				s.off++
			}
			if !s.eof() && s.f.Data[s.off] == '\'' {
				s.off++
			}
		}
	}
}

// comment überspringt einen Kommentar, der mit open Bytes beginnt und mit
// closer endet, und wertet ihn als Direktive aus, wenn er mit $ beginnt.
func (l *Lexer) comment(open int, closer string) error {
	s := l.cur()
	pos := l.pos()
	s.off += open
	bodyStart := s.off
	for {
		if s.eof() {
			return source.Errorf(pos, source.ErrUnexpectedEOF, "unterminated comment")
		}
		if strings.HasPrefix(string(s.f.Data[s.off:min(s.off+len(closer), len(s.f.Data))]), closer) {
			break
		}
		if s.f.Data[s.off] == '\n' {
			s.line, s.lineStart = s.line+1, s.off+1
		}
		s.off++
	}
	body := string(s.f.Data[bodyStart:s.off])
	s.off += len(closer)
	if strings.HasPrefix(body, "$") {
		return l.directive(body[1:], pos)
	}
	return nil
}

// directive wertet eine Compilerdirektive aus (Text nach dem $).
func (l *Lexer) directive(text string, pos source.Pos) error {
	n := 0
	for n < len(text) && isIdentChar(text[n]) {
		n++
	}
	name := strings.ToUpper(text[:n])
	rest := text[n:]
	arg := strings.TrimSpace(rest)
	bad := func(detail string) error {
		return source.Errorf(pos, source.ErrInvalidDirective, "%s", detail)
	}

	// Bedingte Übersetzung wird auch in inaktiven Bereichen verfolgt.
	switch name {
	case "IFDEF", "IFNDEF", "IFOPT":
		if !l.cond.Active() {
			l.cond.If(pos, false)
			return nil
		}
		var cond bool
		if name == "IFOPT" {
			if len(arg) < 2 || (arg[1] != '+' && arg[1] != '-') {
				return bad("{$IFOPT} expects a switch like I+")
			}
			val, ok := l.switches[upper(arg[0])]
			if !ok {
				return bad("unknown switch " + arg[:1])
			}
			cond = val == (arg[1] == '+')
		} else {
			sym := identPrefix(arg)
			if sym == "" {
				return bad("conditional symbol expected")
			}
			cond = l.defines.Defined(sym) == (name == "IFDEF")
		}
		l.cond.If(pos, cond)
		return nil
	case "ELSE":
		return l.cond.Else(pos)
	case "ENDIF":
		return l.cond.EndIf(pos)
	}
	if !l.cond.Active() {
		return nil
	}

	switch {
	case name == "DEFINE" || name == "UNDEF":
		sym := identPrefix(arg)
		if sym == "" {
			return bad("conditional symbol expected")
		}
		if name == "DEFINE" {
			l.defines.Define(sym)
		} else {
			l.defines.Undef(sym)
		}
	case len(name) == 1 && (strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-")):
		// Schalterliste wie {$I-,R+}
		for t := text; ; {
			if len(t) < 2 || !isLetter(t[0]) || (t[1] != '+' && t[1] != '-') {
				return bad("switch expected")
			}
			l.switches[upper(t[0])] = t[1] == '+'
			t = t[2:]
			if !strings.HasPrefix(t, ",") {
				break
			}
			t = t[1:]
		}
	case name == "I":
		return l.include(arg, pos)
	case name == "M":
		first, _, _ := strings.Cut(arg, ",")
		v, ok := parseNumber(strings.TrimSpace(first))
		if !ok || v <= 0 {
			return bad("stack size expected")
		}
		l.stackSize = v
	}
	// Alle übrigen Direktiven werden ignoriert.
	return nil
}

// include öffnet eine {$I datei}-Datei. Ohne Endung wird .pas angenommen.
func (l *Lexer) include(name string, pos source.Pos) error {
	if name == "" {
		return source.Errorf(pos, source.ErrInvalidFileName, "")
	}
	if path.Ext(strings.ReplaceAll(name, `\`, "/")) == "" {
		name += ".pas"
	}
	if len(l.files) >= maxIncludeDepth {
		return source.Errorf(pos, source.ErrTooManyNestedFiles, "")
	}
	found, err := l.loader.Resolve(name, path.Dir(l.cur().f.Name))
	if err != nil {
		if source.IsInvalidName(err) {
			return source.Errorf(pos, source.ErrInvalidFileName, "%s", name)
		}
		return source.Errorf(pos, source.ErrFileNotFound, "%s", name)
	}
	f, err := l.loader.Load(found)
	if err != nil {
		var se *source.Error
		if errors.As(err, &se) {
			return se
		}
		if errors.Is(err, fs.ErrNotExist) {
			return source.Errorf(pos, source.ErrFileNotFound, "%s", name)
		}
		return source.Errorf(pos, source.ErrFileNotFound, "%s: %v", name, err)
	}
	l.files = append(l.files, &fileState{f: f, line: 1})
	return nil
}

func parseNumber(s string) (int, bool) {
	base := 10
	if strings.HasPrefix(s, "$") {
		s, base = s[1:], 16
	}
	v, err := strconv.ParseInt(s, base, 32)
	return int(v), err == nil
}

func identPrefix(s string) string {
	n := 0
	for n < len(s) && isIdentChar(s[n]) {
		n++
	}
	if n == 0 || isDigit(s[0]) {
		return ""
	}
	return s[:n]
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

func isLetter(c byte) bool    { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool     { return c >= '0' && c <= '9' }
func isIdentChar(c byte) bool { return isLetter(c) || isDigit(c) || c == '_' }
func isHexDigit(c byte) bool  { return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
