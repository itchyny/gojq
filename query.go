package gojq

import (
	"fmt"
	"strconv"
	"strings"
)

// Query ...
type Query struct {
	FuncDefs []*FuncDef `@@*`
	Pipe     *Pipe      `@@?`
}

func (q *Query) String() string {
	var s strings.Builder
	for i, fd := range q.FuncDefs {
		if i > 0 {
			s.WriteByte(' ')
		}
		fmt.Fprint(&s, fd)
	}
	if q.Pipe != nil {
		if len(q.FuncDefs) > 0 {
			s.WriteByte(' ')
		}
		fmt.Fprint(&s, q.Pipe)
	}
	return s.String()
}

// Run query.
func (q *Query) Run(v interface{}) Iter {
	if code, err := compile(q); err == nil {
		return mapIterator(newEnv(nil).execute(code, v), normalizeValues)
	}
	return mapIterator(newEnv(nil).applyQuery(q, unitIterator(v)), normalizeValues)
}

// FuncDef ...
type FuncDef struct {
	Name string   `"def" @Ident`
	Args []string `("(" @Ident (";" @Ident)* ")")? ":"`
	Body *Query   `@@ ";"`
}

func (e *FuncDef) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "def %s", e.Name)
	if len(e.Args) > 0 {
		s.WriteByte('(')
		for i, e := range e.Args {
			if i > 0 {
				s.WriteString("; ")
			}
			fmt.Fprint(&s, e)
		}
		s.WriteByte(')')
	}
	fmt.Fprintf(&s, ": %s;", e.Body)
	return s.String()
}

// Pipe ...
type Pipe struct {
	Commas []*Comma `@@ ("|" @@)*`
}

func (e *Pipe) String() string {
	var s strings.Builder
	for i, e := range e.Commas {
		if i > 0 {
			s.WriteString(" | ")
		}
		fmt.Fprint(&s, e)
	}
	return s.String()
}

// Comma ...
type Comma struct {
	Alts []*Alt `@@ ("," @@)*`
}

func (e *Comma) String() string {
	var s strings.Builder
	for i, e := range e.Alts {
		if i > 0 {
			s.WriteString(", ")
		}
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Comma) toPipe() *Pipe {
	return &Pipe{[]*Comma{e}}
}

// Alt ...
type Alt struct {
	Left  *Expr      `@@`
	Right []AltRight `@@*`
}

func (e *Alt) String() string {
	var s strings.Builder
	fmt.Fprint(&s, e.Left)
	for _, e := range e.Right {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Alt) toPipe() *Pipe {
	return (&Comma{[]*Alt{e}}).toPipe()
}

// AltRight ...
type AltRight struct {
	Op    Operator `@"//"`
	Right *Expr    `@@`
}

func (e AltRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

// Expr ...
type Expr struct {
	Logic   *Logic    `( @@`
	If      *If       `| @@`
	Try     *Try      `| @@`
	Reduce  *Reduce   `| @@`
	Foreach *Foreach  `| @@ )`
	Bind    *ExprBind `@@?`
	Label   *Label    `| @@`
}

func (e *Expr) toPipe() *Pipe {
	return (&Alt{Left: e}).toPipe()
}

func (e *Expr) toAlt() *Alt {
	return &Alt{Left: e}
}

func (e *Expr) String() string {
	var s strings.Builder
	if e.Logic != nil {
		fmt.Fprint(&s, e.Logic)
	} else if e.If != nil {
		fmt.Fprint(&s, e.If)
	} else if e.Try != nil {
		fmt.Fprint(&s, e.Try)
	} else if e.Reduce != nil {
		fmt.Fprint(&s, e.Reduce)
	} else if e.Foreach != nil {
		fmt.Fprint(&s, e.Foreach)
	}
	if e.Bind != nil {
		fmt.Fprint(&s, e.Bind)
	} else if e.Label != nil {
		fmt.Fprint(&s, e.Label)
	}
	return s.String()
}

// ExprBind ...
type ExprBind struct {
	Pattern *Pattern `"as" @@`
	Body    *Pipe    `"|" @@`
}

func (e *ExprBind) String() string {
	return fmt.Sprintf(" as %s | %s", e.Pattern, e.Body)
}

// Logic ...
type Logic struct {
	Left  *AndExpr     `@@`
	Right []LogicRight `@@*`
}

func (e *Logic) String() string {
	var s strings.Builder
	fmt.Fprint(&s, e.Left)
	for _, e := range e.Right {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Logic) toPipe() *Pipe {
	return (&Expr{Logic: e}).toPipe()
}

func (e *Logic) toAlt() *Alt {
	return (&Expr{Logic: e}).toAlt()
}

// LogicRight ...
type LogicRight struct {
	Op    Operator `@"or"`
	Right *AndExpr `@@`
}

func (e LogicRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

// AndExpr ...
type AndExpr struct {
	Left  *Compare       `@@`
	Right []AndExprRight `@@*`
}

func (e *AndExpr) String() string {
	var s strings.Builder
	fmt.Fprint(&s, e.Left)
	for _, e := range e.Right {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *AndExpr) toPipe() *Pipe {
	return (&Logic{Left: e}).toPipe()
}

func (e *AndExpr) toAlt() *Alt {
	return (&Logic{Left: e}).toAlt()
}

func (e *AndExpr) toLogic() *Logic {
	return &Logic{Left: e}
}

// AndExprRight ...
type AndExprRight struct {
	Op    Operator `@"and"`
	Right *Compare `@@`
}

func (e AndExprRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

// Compare ...
type Compare struct {
	Left  *Arith        `@@`
	Right *CompareRight `@@?`
}

func (e *Compare) toPipe() *Pipe {
	return (&AndExpr{Left: e}).toPipe()
}

func (e *Compare) toAlt() *Alt {
	return (&AndExpr{Left: e}).toAlt()
}

func (e *Compare) toLogic() *Logic {
	return (&AndExpr{Left: e}).toLogic()
}

func (e *Compare) String() string {
	var s strings.Builder
	fmt.Fprint(&s, e.Left)
	if e.Right != nil {
		fmt.Fprint(&s, e.Right)
	}
	return s.String()
}

// CompareRight ...
type CompareRight struct {
	Op    Operator `@CompareOp`
	Right *Arith   `@@`
}

func (e *CompareRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

// Arith ...
type Arith struct {
	Left  *Factor      `@@`
	Right []ArithRight `@@*`
}

func (e *Arith) String() string {
	var s strings.Builder
	fmt.Fprint(&s, e.Left)
	for _, e := range e.Right {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Arith) toPipe() *Pipe {
	return (&Compare{Left: e}).toPipe()
}

func (e *Arith) toAlt() *Alt {
	return (&Compare{Left: e}).toAlt()
}

func (e *Arith) toLogic() *Logic {
	return (&Compare{Left: e}).toLogic()
}

// ArithRight ...
type ArithRight struct {
	Op    Operator `@("+" | "-")`
	Right *Factor  `@@`
}

func (e ArithRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

// Factor ...
type Factor struct {
	Left  *Term         `@@`
	Right []FactorRight `@@*`
}

func (e *Factor) String() string {
	var s strings.Builder
	fmt.Fprint(&s, e.Left)
	for _, e := range e.Right {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Factor) toPipe() *Pipe {
	return (&Arith{Left: e}).toPipe()
}

func (e *Factor) toAlt() *Alt {
	return (&Arith{Left: e}).toAlt()
}

func (e *Factor) toLogic() *Logic {
	return (&Arith{Left: e}).toLogic()
}

// FactorRight ...
type FactorRight struct {
	Op    Operator `@("*" | "/" | "%")`
	Right *Term    `@@`
}

func (e FactorRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

// Term ...
type Term struct {
	Index      *Index    `( @@`
	Identity   bool      `| @"."`
	Recurse    bool      `| @".."`
	Func       *Func     `| @@`
	Object     *Object   `| @@`
	Array      *Array    `| @@`
	Number     *float64  `| @Number`
	Unary      *Unary    `| @@`
	Str        string    `| @String`
	RawStr     string    `| @" "` // never matches, used in compiler
	Null       bool      `| @"null"`
	True       bool      `| @"true"`
	False      bool      `| @"false"`
	Break      string    `| "break" @Ident`
	Pipe       *Pipe     `| "(" @@ ")" )`
	SuffixList []*Suffix `@@*`
}

func (e *Term) String() string {
	var s strings.Builder
	if e.Index != nil {
		fmt.Fprint(&s, e.Index)
	} else if e.Identity {
		s.WriteString(".")
	} else if e.Recurse {
		s.WriteString("..")
	} else if e.Func != nil {
		fmt.Fprint(&s, e.Func)
	} else if e.Object != nil {
		fmt.Fprint(&s, e.Object)
	} else if e.Array != nil {
		fmt.Fprint(&s, e.Array)
	} else if e.Number != nil {
		fmt.Fprint(&s, *e.Number)
	} else if e.Unary != nil {
		fmt.Fprint(&s, e.Unary)
	} else if e.Str != "" {
		fmt.Fprint(&s, e.Str)
	} else if e.RawStr != "" {
		fmt.Fprint(&s, strconv.Quote(e.RawStr))
	} else if e.Null {
		s.WriteString("null")
	} else if e.True {
		s.WriteString("true")
	} else if e.False {
		s.WriteString("false")
	} else if e.Break != "" {
		fmt.Fprintf(&s, "break %s", e.Break)
	} else if e.Pipe != nil {
		fmt.Fprintf(&s, "(%s)", e.Pipe)
	}
	for _, e := range e.SuffixList {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Term) toPipe() *Pipe {
	return (&Factor{Left: e}).toPipe()
}

func (e *Term) toAlt() *Alt {
	return (&Factor{Left: e}).toAlt()
}

func (e *Term) toLogic() *Logic {
	return (&Factor{Left: e}).toLogic()
}

// Unary ...
type Unary struct {
	Op   Operator `@("+" | "-")`
	Term *Term    `@@`
}

func (e *Unary) String() string {
	return fmt.Sprintf("%s %s", e.Op, e.Term)
}

// Pattern ...
type Pattern struct {
	Name   string          `  @Ident`
	Array  []*Pattern      `| "[" @@ ("," @@)* "]"`
	Object []PatternObject `| "{" @@ ("," @@)* "}"`
}

func (e *Pattern) String() string {
	var s strings.Builder
	if e.Name != "" {
		s.WriteString(e.Name)
	}
	return s.String()
}

// PatternObject ...
type PatternObject struct {
	Key       string   `( ( @Ident | @Keyword )`
	KeyString string   `| @String ) ":"`
	Val       *Pattern `@@`
	KeyOnly   string   `| @Ident`
}

func (e *PatternObject) String() string {
	var s strings.Builder
	if e.Key != "" {
		s.WriteString(e.Key)
		s.WriteString(": ")
	}
	if e.KeyString != "" {
		s.WriteString(e.KeyString)
		s.WriteString(": ")
	}
	if e.Val != nil {
		fmt.Fprint(&s, e.Val)
	}
	if e.KeyOnly != "" {
		s.WriteString(e.KeyOnly)
	}
	return s.String()
}

// Index ...
type Index struct {
	Name    string `"." ( @Ident`
	Str     string `| @String`
	Start   *Pipe  `| "[" ( @@`
	IsSlice bool   `( @":"`
	End     *Pipe  `@@? )? | ":" @@ ) "]" )`
}

func (e *Index) String() string {
	var s strings.Builder
	s.WriteByte('.')
	if e.Name != "" {
		s.WriteString(e.Name)
	} else if e.Str != "" {
		s.WriteString(e.Str)
	} else {
		s.WriteByte('[')
		if e.Start != nil {
			fmt.Fprint(&s, e.Start)
			if e.IsSlice {
				s.WriteByte(':')
				if e.End != nil {
					fmt.Fprint(&s, e.End)
				}
			}
		} else if e.End != nil {
			s.WriteByte(':')
			fmt.Fprint(&s, e.End)
		}
		s.WriteByte(']')
	}
	return s.String()
}

// Func ...
type Func struct {
	Name    string  `@Ident`
	Args    []*Pipe `( "(" @@ (";" @@)* ")" )?`
	funcDef *FuncDef
}

func (e *Func) String() string {
	var s strings.Builder
	s.WriteString(e.Name)
	if len(e.Args) > 0 {
		s.WriteByte('(')
		for i, e := range e.Args {
			if i > 0 {
				s.WriteString("; ")
			}
			fmt.Fprint(&s, e)
		}
		s.WriteByte(')')
	}
	return s.String()
}

// Object ...
type Object struct {
	KeyVals []ObjectKeyVal `"{" (@@ ("," @@)*)? "}"`
}

func (e *Object) String() string {
	if len(e.KeyVals) == 0 {
		return "{}"
	}
	var s strings.Builder
	s.WriteString("{ ")
	for i, kv := range e.KeyVals {
		if i > 0 {
			s.WriteString(", ")
		}
		fmt.Fprint(&s, &kv)
	}
	s.WriteString(" }")
	return s.String()
}

// ObjectKeyVal ...
type ObjectKeyVal struct {
	Key           string  `( ( ( @Ident | @Keyword )`
	KeyString     string  `  | @String )`
	Pipe          *Pipe   `| "(" @@ ")" ) ":"`
	Val           *Expr   `@@`
	KeyOnly       *string `| @Ident`
	KeyOnlyString string  `| @String`
}

func (e *ObjectKeyVal) String() string {
	var s strings.Builder
	if e.Key != "" {
		s.WriteString(e.Key)
	} else if e.KeyString != "" {
		s.WriteString(e.KeyString)
	} else if e.Pipe != nil {
		fmt.Fprintf(&s, "(%s)", e.Pipe)
	}
	if e.Val != nil {
		fmt.Fprintf(&s, ": %s", e.Val)
	}
	if e.KeyOnly != nil {
		s.WriteString(*e.KeyOnly)
	}
	if e.KeyOnlyString != "" {
		s.WriteString(e.KeyOnlyString)
	}
	return s.String()
}

// Array ...
type Array struct {
	Pipe *Pipe `"[" @@? "]"`
}

func (e *Array) String() string {
	if e.Pipe == nil {
		return "[]"
	}
	return fmt.Sprintf("[%s]", e.Pipe)
}

// Suffix ...
type Suffix struct {
	Index       *Index       `  @@`
	SuffixIndex *SuffixIndex `| @@`
	Iter        bool         `| @("[" "]")`
	Optional    bool         `| @"?"`
}

func (e *Suffix) String() string {
	var s strings.Builder
	if e.Index != nil {
		fmt.Fprint(&s, e.Index)
	} else if e.SuffixIndex != nil {
		fmt.Fprint(&s, e.SuffixIndex)
	} else if e.Iter {
		s.WriteString("[]")
	} else if e.Optional {
		s.WriteString("?")
	}
	return s.String()
}

func (e *Suffix) toTerm() (*Term, bool) {
	if e.Index != nil {
		return &Term{Index: e.Index}, true
	} else if e.SuffixIndex != nil {
		return &Term{Index: e.SuffixIndex.toIndex()}, true
	} else if e.Iter {
		return &Term{Identity: true, SuffixList: []*Suffix{&Suffix{Iter: true}}}, true
	} else {
		return nil, false
	}
}

// SuffixIndex ...
type SuffixIndex struct {
	Start   *Pipe `"[" ( @@`
	IsSlice bool  `( @":"`
	End     *Pipe `@@? )? | ":" @@ ) "]"`
}

func (e *SuffixIndex) String() string {
	return e.toIndex().String()[1:]
}

func (e *SuffixIndex) toIndex() *Index {
	return &Index{
		Start:   e.Start,
		IsSlice: e.IsSlice,
		End:     e.End,
	}
}

// If ...
type If struct {
	Cond *Pipe    `"if" @@`
	Then *Pipe    `"then" @@`
	Elif []IfElif `@@*`
	Else *Pipe    `("else" @@)? "end"`
}

func (e *If) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "if %s then %s", e.Cond, e.Then)
	for _, e := range e.Elif {
		fmt.Fprintf(&s, " %s", &e)
	}
	if e.Else != nil {
		fmt.Fprintf(&s, " else %s", e.Else)
	}
	s.WriteString(" end")
	return s.String()
}

// IfElif ...
type IfElif struct {
	Cond *Pipe `"elif" @@`
	Then *Pipe `"then" @@`
}

func (e *IfElif) String() string {
	return fmt.Sprintf("elif %s then %s", e.Cond, e.Then)
}

// Try ...
type Try struct {
	Body  *Pipe `"try" @@`
	Catch *Pipe `("catch" @@)?`
}

func (e *Try) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "try %s", e.Body)
	if e.Catch != nil {
		fmt.Fprintf(&s, " catch %s", e.Catch)
	}
	return s.String()
}

// Reduce ...
type Reduce struct {
	Term    *Term    `"reduce" @@`
	Pattern *Pattern `"as" @@`
	Start   *Pipe    `"(" @@`
	Update  *Pipe    `";" @@ ")"`
}

func (e *Reduce) String() string {
	return fmt.Sprintf("reduce %s as %s (%s; %s)", e.Term, e.Pattern, e.Start, e.Update)
}

// Foreach ...
type Foreach struct {
	Term    *Term    `"foreach" @@`
	Pattern *Pattern `"as" @@`
	Start   *Pipe    `"(" @@`
	Update  *Pipe    `";" @@`
	Extract *Pipe    `(";" @@)? ")"`
}

func (e *Foreach) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "foreach %s as %s (%s; %s", e.Term, e.Pattern, e.Start, e.Update)
	if e.Extract != nil {
		fmt.Fprintf(&s, "; %s", e.Extract)
	}
	s.WriteByte(')')
	return s.String()
}

// Label ...
type Label struct {
	Ident string `"label" @Ident`
	Body  *Pipe  `"|" @@`
}

func (e *Label) String() string {
	return fmt.Sprintf("label %s | %s", e.Ident, e.Body)
}
