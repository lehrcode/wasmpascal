// Package ast definiert die Knoten des abstrakten Syntaxbaums.
//
// Der Parser erzeugt den Baum ohne Namensauflösung: Ob a.b ein Recordfeld oder
// ein qualifizierter Bezeichner (Unit.Name) ist und ob f(x) ein Aufruf oder eine
// Typumwandlung ist, entscheidet erst die Typprüfung.
package ast

import (
	"github.com/lehrcode/wasmpascal/compiler/internal/lexer"
	"github.com/lehrcode/wasmpascal/compiler/internal/source"
)

// Node ist jeder Knoten des Baums.
type Node interface {
	Pos() source.Pos
}

// Base trägt die Position eines Knotens.
type Base struct {
	At source.Pos
}

// Pos liefert die Position des ersten Tokens des Knotens.
func (b Base) Pos() source.Pos { return b.At }

// At erzeugt eine Base für pos.
func At(pos source.Pos) Base { return Base{At: pos} }

// StmtBase ist die Basis aller Anweisungen. IOChecks ist der Zustand von
// {$I+}/{$I-} am Beginn der Anweisung.
type StmtBase struct {
	Base
	IOChecks bool
}

// ---------------------------------------------------------------------------
// Übersetzungseinheiten

// File ist ein Programm oder eine Unit.
type File interface {
	Node
	fileNode()
}

// Program ist ein Hauptprogramm. Name ist nil, wenn die Kopfzeile fehlt.
type Program struct {
	Base
	Name   *Ident
	Params []*Ident // program X(Input, Output); ohne Bedeutung
	Uses   []*Ident
	Block  *Block
}

// Unit ist eine Unit mit Interface- und Implementation-Teil.
type Unit struct {
	Base
	Name     *Ident
	IntfUses []*Ident
	Intf     []Decl // ProcDecl ohne Block
	ImplUses []*Ident
	Impl     []Decl
	Init     *CompoundStmt // nil, wenn die Unit nur mit end. schließt
}

func (*Program) fileNode() {}
func (*Unit) fileNode()    {}

// Block ist ein Deklarationsteil mit Anweisungsteil.
type Block struct {
	Base
	Decls []Decl
	Body  *CompoundStmt
}

// ---------------------------------------------------------------------------
// Bezeichner und Ausdrücke

// Expr ist ein Ausdruck.
type Expr interface {
	Node
	exprNode()
}

// Ident ist ein Bezeichner. Name ist normalisiert (Kleinbuchstaben, höchstens
// 63 Zeichen), Text die Schreibweise im Quelltext. Labels aus Ziffern sind
// ebenfalls Idents.
type Ident struct {
	Base
	Name string
	Text string
}

// IntLit ist eine Ganzzahlkonstante wie 42 oder $FF.
type IntLit struct {
	Base
	Text string
}

// RealLit ist eine Real-Konstante wie 1.5E3.
type RealLit struct {
	Base
	Text string
}

// StrLit ist eine Zeichenkettenkonstante; Value enthält die Bytes.
type StrLit struct {
	Base
	Value string
}

// NilLit ist nil.
type NilLit struct{ Base }

// SetLit ist ein Mengenkonstruktor [a, b..c].
type SetLit struct {
	Base
	Elems []SetElem
}

// SetElem ist ein Element oder (mit Hi) ein Bereich eines Mengenkonstruktors.
type SetElem struct {
	Lo, Hi Expr
}

// UnaryExpr ist +x, -x, not x oder @x.
type UnaryExpr struct {
	Base
	Op lexer.Kind
	X  Expr
}

// BinaryExpr ist ein zweistelliger Operator.
type BinaryExpr struct {
	Base
	Op   lexer.Kind
	X, Y Expr
}

// CallExpr ist f(args): Funktions-/Prozeduraufruf oder Typumwandlung T(x).
type CallExpr struct {
	Base
	Fun  Expr
	Args []Expr
}

// FormatArg ist ein Argument mit Formatangabe x:breite[:nachkomma], erlaubt
// nur bei Write/WriteLn/Str (geprüft in der Typprüfung).
type FormatArg struct {
	Base
	X, Width, Prec Expr
}

// IndexExpr ist a[i, j].
type IndexExpr struct {
	Base
	X       Expr
	Indices []Expr
}

// SelectorExpr ist x.sel (Recordfeld oder qualifizierter Bezeichner).
type SelectorExpr struct {
	Base
	X   Expr
	Sel *Ident
}

// DerefExpr ist p^.
type DerefExpr struct {
	Base
	X Expr
}

// ConstList ist (a, b, c) in einer typisierten Konstante (Array).
type ConstList struct {
	Base
	Elems []Expr
}

// RecordConst ist (x: 1; y: 2) in einer typisierten Konstante.
type RecordConst struct {
	Base
	Fields []*FieldInit
}

// FieldInit ist ein Feld einer RecordConst.
type FieldInit struct {
	Base
	Name  *Ident
	Value Expr
}

func (*Ident) exprNode()        {}
func (*IntLit) exprNode()       {}
func (*RealLit) exprNode()      {}
func (*StrLit) exprNode()       {}
func (*NilLit) exprNode()       {}
func (*SetLit) exprNode()       {}
func (*UnaryExpr) exprNode()    {}
func (*BinaryExpr) exprNode()   {}
func (*CallExpr) exprNode()     {}
func (*FormatArg) exprNode()    {}
func (*IndexExpr) exprNode()    {}
func (*SelectorExpr) exprNode() {}
func (*DerefExpr) exprNode()    {}
func (*ConstList) exprNode()    {}
func (*RecordConst) exprNode()  {}

// ---------------------------------------------------------------------------
// Typen

// TypeExpr ist ein Typausdruck.
type TypeExpr interface {
	Node
	typeNode()
}

// NamedType ist ein Typbezeichner, optional mit Unit (Crt.TColor).
type NamedType struct {
	Base
	Unit *Ident
	Name *Ident
}

// StringType ist string oder string[Len].
type StringType struct {
	Base
	Len Expr
}

// SubrangeType ist lo..hi.
type SubrangeType struct {
	Base
	Lo, Hi Expr
}

// EnumType ist (a, b, c).
type EnumType struct {
	Base
	Names []*Ident
}

// ArrayType ist [packed] array[I1, I2] of Elem.
type ArrayType struct {
	Base
	Packed bool
	Index  []TypeExpr
	Elem   TypeExpr
}

// RecordType ist [packed] record ... end.
type RecordType struct {
	Base
	Packed bool
	Fields *FieldList
}

// FieldList ist der Inhalt eines Records oder eines Variantenzweigs.
type FieldList struct {
	Fixed   []*FieldGroup
	Variant *VariantPart
}

// FieldGroup ist a, b: T.
type FieldGroup struct {
	Base
	Names []*Ident
	Type  TypeExpr
}

// VariantPart ist case [tag:] T of ... in einem Record.
type VariantPart struct {
	Base
	Tag     *Ident
	TagType *NamedType
	Cases   []*VariantCase
}

// VariantCase ist ein Zweig k1, k2: (Felder).
type VariantCase struct {
	Base
	Labels []Expr
	Fields *FieldList
}

// SetType ist [packed] set of Elem.
type SetType struct {
	Base
	Packed bool
	Elem   TypeExpr
}

// FileType ist file oder file of Elem.
type FileType struct {
	Base
	Packed bool
	Elem   TypeExpr // nil: untypisierte Datei
}

// PointerType ist ^T.
type PointerType struct {
	Base
	Elem TypeExpr // NamedType oder StringType
}

func (*NamedType) typeNode()    {}
func (*StringType) typeNode()   {}
func (*SubrangeType) typeNode() {}
func (*EnumType) typeNode()     {}
func (*ArrayType) typeNode()    {}
func (*RecordType) typeNode()   {}
func (*SetType) typeNode()      {}
func (*FileType) typeNode()     {}
func (*PointerType) typeNode()  {}

// ---------------------------------------------------------------------------
// Deklarationen

// Decl ist eine Deklaration.
type Decl interface {
	Node
	declNode()
}

// LabelDecl ist label 10, Ende;
type LabelDecl struct {
	Base
	Labels []*Ident
}

// ConstDecl ist eine Konstante; Type ist bei typisierten Konstanten gesetzt.
type ConstDecl struct {
	Base
	Name  *Ident
	Type  TypeExpr
	Value Expr
}

// TypeDecl ist Name = Typ.
type TypeDecl struct {
	Base
	Name *Ident
	Type TypeExpr
}

// VarDecl ist a, b: T.
type VarDecl struct {
	Base
	Names []*Ident
	Type  TypeExpr
}

// ParamMode ist die Übergabeart eines Parameters.
type ParamMode int

// Übergabearten.
const (
	ByValue ParamMode = iota
	ByVar
	ByConst
)

// ParamGroup ist [var|const] a, b[: T]. Type ist nil bei untypisierten
// var-/const-Parametern.
type ParamGroup struct {
	Base
	Mode  ParamMode
	Names []*Ident
	Type  TypeExpr
}

// ProcDecl ist eine Prozedur oder Funktion. Block ist nil bei forward und bei
// Köpfen im Interface-Teil. Result ist bei Funktionen gesetzt, außer bei der
// verkürzten Wiederholung des Kopfes nach forward oder im Implementation-Teil.
type ProcDecl struct {
	Base
	IsFunc  bool
	Name    *Ident
	Params  []*ParamGroup
	Result  TypeExpr
	Forward bool
	Block   *Block
}

func (*LabelDecl) declNode() {}
func (*ConstDecl) declNode() {}
func (*TypeDecl) declNode()  {}
func (*VarDecl) declNode()   {}
func (*ProcDecl) declNode()  {}

// ---------------------------------------------------------------------------
// Anweisungen

// Stmt ist eine Anweisung.
type Stmt interface {
	Node
	stmtNode()
}

// CompoundStmt ist begin ... end.
type CompoundStmt struct {
	StmtBase
	Stmts []Stmt
}

// EmptyStmt ist die leere Anweisung.
type EmptyStmt struct{ StmtBase }

// AssignStmt ist lhs := rhs.
type AssignStmt struct {
	StmtBase
	Lhs, Rhs Expr
}

// CallStmt ist ein Prozeduraufruf (Ident, SelectorExpr oder CallExpr).
type CallStmt struct {
	StmtBase
	Call Expr
}

// IfStmt ist if Cond then Then [else Else].
type IfStmt struct {
	StmtBase
	Cond Expr
	Then Stmt
	Else Stmt // nil ohne else
}

// CaseStmt ist case X of ... [else ...] end.
type CaseStmt struct {
	StmtBase
	X       Expr
	Arms    []*CaseArm
	HasElse bool
	Else    []Stmt
}

// CaseArm ist ein Zweig k1, k2..k3: Body.
type CaseArm struct {
	Base
	Labels []CaseLabel
	Body   Stmt
}

// CaseLabel ist eine Marke oder (mit Hi) ein Bereich.
type CaseLabel struct {
	Lo, Hi Expr
}

// WhileStmt ist while Cond do Body.
type WhileStmt struct {
	StmtBase
	Cond Expr
	Body Stmt
}

// RepeatStmt ist repeat Body until Cond.
type RepeatStmt struct {
	StmtBase
	Body []Stmt
	Cond Expr
}

// ForStmt ist for Var := From to|downto To do Body.
type ForStmt struct {
	StmtBase
	Var      *Ident
	From, To Expr
	Down     bool
	Body     Stmt
}

// ForInStmt ist for Var in In do Body (Erweiterung aus Delphi).
type ForInStmt struct {
	StmtBase
	Var  *Ident
	In   Expr
	Body Stmt
}

// WithStmt ist with r1, r2 do Body.
type WithStmt struct {
	StmtBase
	Records []Expr
	Body    Stmt
}

// GotoStmt ist goto Label.
type GotoStmt struct {
	StmtBase
	Label *Ident
}

// LabeledStmt ist Label: Stmt.
type LabeledStmt struct {
	StmtBase
	Label *Ident
	Stmt  Stmt
}

func (*CompoundStmt) stmtNode() {}
func (*EmptyStmt) stmtNode()    {}
func (*AssignStmt) stmtNode()   {}
func (*CallStmt) stmtNode()     {}
func (*IfStmt) stmtNode()       {}
func (*CaseStmt) stmtNode()     {}
func (*WhileStmt) stmtNode()    {}
func (*RepeatStmt) stmtNode()   {}
func (*ForStmt) stmtNode()      {}
func (*ForInStmt) stmtNode()    {}
func (*WithStmt) stmtNode()     {}
func (*GotoStmt) stmtNode()     {}
func (*LabeledStmt) stmtNode()  {}
