package gojq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Module ...
type Module struct {
	Meta     *ConstObject `( "module" @@ ";" )?`
	Imports  []*Import    `@@*`
	FuncDefs []*FuncDef   `@@*`
}

func (e *Module) String() string {
	var s strings.Builder
	if e.Meta != nil {
		fmt.Fprintf(&s, "module %s;\n", e.Meta)
	}
	for _, i := range e.Imports {
		fmt.Fprint(&s, i)
	}
	for i, fd := range e.FuncDefs {
		if i > 0 {
			s.WriteByte(' ')
		}
		fmt.Fprint(&s, fd)
	}
	return s.String()
}

// Import ...
type Import struct {
	ImportPath  string       `( "import" @String`
	ImportAlias string       `  "as" ( @Ident | @Variable )`
	IncludePath string       `| "include" @String )`
	Meta        *ConstObject `@@? ";"`
}

func (e *Import) String() string {
	var s strings.Builder
	if e.ImportPath != "" {
		s.WriteString("import ")
		s.WriteString(e.ImportPath)
		s.WriteString(" as ")
		s.WriteString(e.ImportAlias)
	} else {
		s.WriteString("include ")
		s.WriteString(e.IncludePath)
	}
	if e.Meta != nil {
		fmt.Fprintf(&s, " %s", e.Meta)
	}
	s.WriteString(";\n")
	return s.String()
}

// FuncDef ...
type FuncDef struct {
	Name string   `"def" @Ident`
	Args []string `("(" ( @Ident | @Variable ) (";" ( @Ident | @Variable ))* ")")? ":"`
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

// Minify ...
func (e *FuncDef) Minify() {
	e.Body.minify()
}

// Query ...
type Query struct {
	Imports []*Import `@@*`
	Commas  []*Comma  `@@ ("|" @@)*`
}

func (e *Query) String() string {
	var s strings.Builder
	for _, i := range e.Imports {
		fmt.Fprint(&s, i)
	}
	for i, e := range e.Commas {
		if i > 0 {
			s.WriteString(" | ")
		}
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Query) minify() {
	for _, x := range e.Commas {
		x.minify()
	}
}

func (e *Query) toIndices() []interface{} {
	var xs []interface{}
	for _, e := range e.Commas {
		x := e.toIndices()
		if x == nil {
			return nil
		}
		xs = append(xs, x...)
	}
	return xs
}

// Run query.
func (e *Query) Run(v interface{}) Iter {
	return e.RunWithContext(nil, v)
}

// RunWithContext query.
func (e *Query) RunWithContext(ctx context.Context, v interface{}) Iter {
	code, err := Compile(e)
	if err != nil {
		return unitIterator(err)
	}
	return code.RunWithContext(ctx, v)
}

// Comma ...
type Comma struct {
	Filters []*Filter `@@ ("," @@)*`
	Func    string    `| @" "` // never matches, used in minifier
}

func (e *Comma) String() string {
	if e.Func != "" {
		return e.Func
	}
	var s strings.Builder
	for i, e := range e.Filters {
		if i > 0 {
			s.WriteString(", ")
		}
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Comma) minify() {
	if len(e.Filters) == 1 {
		if e.Func = e.Filters[0].toFunc(); e.Func != "" {
			e.Filters = nil
		}
	}
	for _, e := range e.Filters {
		e.minify()
	}
}

func (e *Comma) toQuery() *Query {
	return &Query{Commas: []*Comma{e}}
}

func (e *Comma) toIndices() []interface{} {
	if len(e.Filters) != 1 {
		return nil
	}
	return e.Filters[0].toIndices()
}

// Filter ...
type Filter struct {
	FuncDefs []*FuncDef `@@*`
	Alt      *Alt       `@@`
}

func (e *Filter) String() string {
	var s strings.Builder
	for i, fd := range e.FuncDefs {
		if i > 0 {
			s.WriteByte(' ')
		}
		fmt.Fprint(&s, fd)
	}
	if len(e.FuncDefs) > 0 {
		s.WriteByte(' ')
	}
	fmt.Fprint(&s, e.Alt)
	return s.String()
}

func (e *Filter) minify() {
	for _, e := range e.FuncDefs {
		e.Minify()
	}
	e.Alt.minify()
}

func (e *Filter) toQuery() *Query {
	return (&Comma{Filters: []*Filter{e}}).toQuery()
}

func (e *Filter) toFunc() string {
	if len(e.FuncDefs) != 0 {
		return ""
	}
	return e.Alt.toFunc()
}

func (e *Filter) toIndices() []interface{} {
	if len(e.FuncDefs) != 0 {
		return nil
	}
	return e.Alt.toIndices()
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

func (e *Alt) minify() {
	e.Left.minify()
	for _, e := range e.Right {
		e.minify()
	}
}

func (e *Alt) toQuery() *Query {
	return (&Filter{Alt: e}).toQuery()
}

func (e *Alt) toFilter() *Filter {
	return &Filter{Alt: e}
}

func (e *Alt) toFunc() string {
	if len(e.Right) != 0 {
		return ""
	}
	return e.Left.toFunc()
}

func (e *Alt) toIndices() []interface{} {
	if len(e.Right) != 0 {
		return nil
	}
	return e.Left.toIndices()
}

// AltRight ...
type AltRight struct {
	Op    Operator `@"//"`
	Right *Expr    `@@`
}

func (e AltRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

func (e *AltRight) minify() {
	e.Right.minify()
}

// Expr ...
type Expr struct {
	Logic    *Logic   `@@`
	UpdateOp Operator `( ( @UpdateOp | @UpdateAltOp )`
	Update   *Alt     `  @@`
	Bind     *Bind    `| @@ )?`
	Label    *Label   `| @@`
}

func (e *Expr) String() string {
	var s strings.Builder
	if e.Logic != nil {
		fmt.Fprint(&s, e.Logic)
	}
	if e.Update != nil {
		fmt.Fprintf(&s, " %s %s", e.UpdateOp, e.Update)
	} else if e.Bind != nil {
		fmt.Fprint(&s, e.Bind)
	} else if e.Label != nil {
		fmt.Fprint(&s, e.Label)
	}
	return s.String()
}

func (e *Expr) minify() {
	if e.Logic != nil {
		e.Logic.minify()
	}
	if e.Update != nil {
		e.Update.minify()
	} else if e.Bind != nil {
		e.Bind.minify()
	} else if e.Label != nil {
		e.Label.minify()
	}
}

func (e *Expr) toQuery() *Query {
	return (&Alt{Left: e}).toQuery()
}

func (e *Expr) toFilter() *Filter {
	return (&Alt{Left: e}).toFilter()
}

func (e *Expr) toFunc() string {
	if e.Update != nil || e.Bind != nil || e.Logic == nil {
		return ""
	}
	return e.Logic.toFunc()
}

func (e *Expr) toIndices() []interface{} {
	if e.Update != nil || e.Bind != nil || e.Logic == nil {
		return nil
	}
	return e.Logic.toIndices()
}

// Bind ...
type Bind struct {
	Patterns []*Pattern `"as" @@ ("?//" @@)*`
	Body     *Query     `"|" @@`
}

func (e *Bind) String() string {
	var s strings.Builder
	for i, p := range e.Patterns {
		if i == 0 {
			fmt.Fprintf(&s, " as %s ", p)
		} else {
			fmt.Fprintf(&s, "?// %s ", p)
		}
	}
	fmt.Fprintf(&s, "| %s ", e.Body)
	return s.String()
}

func (e *Bind) minify() {
	e.Body.minify()
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

func (e *Logic) minify() {
	e.Left.minify()
	for _, e := range e.Right {
		e.minify()
	}
}

func (e *Logic) toQuery() *Query {
	return (&Expr{Logic: e}).toQuery()
}

func (e *Logic) toFilter() *Filter {
	return (&Expr{Logic: e}).toFilter()
}

func (e *Logic) toFunc() string {
	if len(e.Right) != 0 {
		return ""
	}
	return e.Left.toFunc()
}

func (e *Logic) toIndices() []interface{} {
	if len(e.Right) != 0 {
		return nil
	}
	return e.Left.toIndices()
}

// LogicRight ...
type LogicRight struct {
	Op    Operator `@"or"`
	Right *AndExpr `@@`
}

func (e LogicRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

func (e *LogicRight) minify() {
	e.Right.minify()
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

func (e *AndExpr) minify() {
	e.Left.minify()
	for _, e := range e.Right {
		e.minify()
	}
}

func (e *AndExpr) toQuery() *Query {
	return (&Logic{Left: e}).toQuery()
}

func (e *AndExpr) toFilter() *Filter {
	return (&Logic{Left: e}).toFilter()
}

func (e *AndExpr) toLogic() *Logic {
	return &Logic{Left: e}
}

func (e *AndExpr) toFunc() string {
	if len(e.Right) != 0 {
		return ""
	}
	return e.Left.toFunc()
}

func (e *AndExpr) toIndices() []interface{} {
	if len(e.Right) != 0 {
		return nil
	}
	return e.Left.toIndices()
}

// AndExprRight ...
type AndExprRight struct {
	Op    Operator `@"and"`
	Right *Compare `@@`
}

func (e AndExprRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

func (e *AndExprRight) minify() {
	e.Right.minify()
}

// Compare ...
type Compare struct {
	Left  *Arith        `@@`
	Right *CompareRight `@@?`
}

func (e *Compare) minify() {
	e.Left.minify()
	if e.Right != nil {
		e.Right.minify()
	}
}

func (e *Compare) toQuery() *Query {
	return (&AndExpr{Left: e}).toQuery()
}

func (e *Compare) toFilter() *Filter {
	return (&AndExpr{Left: e}).toFilter()
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

func (e *Compare) toFunc() string {
	if e.Right != nil {
		return ""
	}
	return e.Left.toFunc()
}

func (e *Compare) toIndices() []interface{} {
	if e.Right != nil {
		return nil
	}
	return e.Left.toIndices()
}

// CompareRight ...
type CompareRight struct {
	Op    Operator `@CompareOp`
	Right *Arith   `@@`
}

func (e *CompareRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

func (e *CompareRight) minify() {
	e.Right.minify()
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

func (e *Arith) minify() {
	e.Left.minify()
	for _, e := range e.Right {
		e.minify()
	}
}

func (e *Arith) toQuery() *Query {
	return (&Compare{Left: e}).toQuery()
}

func (e *Arith) toFilter() *Filter {
	return (&Compare{Left: e}).toFilter()
}

func (e *Arith) toLogic() *Logic {
	return (&Compare{Left: e}).toLogic()
}

func (e *Arith) toFunc() string {
	if len(e.Right) != 0 {
		return ""
	}
	return e.Left.toFunc()
}

func (e *Arith) toIndices() []interface{} {
	if len(e.Right) != 0 {
		return nil
	}
	return e.Left.toIndices()
}

// ArithRight ...
type ArithRight struct {
	Op    Operator `@("+" | "-")`
	Right *Factor  `@@`
}

func (e ArithRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

func (e *ArithRight) minify() {
	e.Right.minify()
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

func (e *Factor) minify() {
	e.Left.minify()
	for _, e := range e.Right {
		e.minify()
	}
}

func (e *Factor) toQuery() *Query {
	return (&Arith{Left: e}).toQuery()
}

func (e *Factor) toFilter() *Filter {
	return (&Arith{Left: e}).toFilter()
}

func (e *Factor) toLogic() *Logic {
	return (&Arith{Left: e}).toLogic()
}

func (e *Factor) toFunc() string {
	if len(e.Right) != 0 {
		return ""
	}
	return e.Left.toFunc()
}

func (e *Factor) toIndices() []interface{} {
	if len(e.Right) != 0 {
		return nil
	}
	return e.Left.toIndices()
}

// FactorRight ...
type FactorRight struct {
	Op    Operator `@("*" | "/" | "%")`
	Right *Term    `@@`
}

func (e FactorRight) String() string {
	return fmt.Sprintf(" %s %s", e.Op, e.Right)
}

func (e *FactorRight) minify() {
	e.Right.minify()
}

// Term ...
type Term struct {
	Index      *Index    `( @@`
	Identity   bool      `| @"."`
	Recurse    bool      `| @".."`
	Null       bool      `| @"null"`
	True       bool      `| @"true"`
	False      bool      `| @"false"`
	Func       *Func     `| @@`
	Object     *Object   `| @@`
	Array      *Array    `| @@`
	Number     string    `| @Number`
	Unary      *Unary    `| @@`
	Format     string    `| ( @Format`
	FormatStr  string    `    @String? )`
	Str        string    `| @String`
	RawStr     string    `| @" "` // never matches, used in compiler
	If         *If       `| @@`
	Try        *Try      `| @@`
	Reduce     *Reduce   `| @@`
	Foreach    *Foreach  `| @@`
	Break      string    `| "break" @Variable`
	Query      *Query    `| "(" @@ ")" )`
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
	} else if e.Null {
		s.WriteString("null")
	} else if e.True {
		s.WriteString("true")
	} else if e.False {
		s.WriteString("false")
	} else if e.Func != nil {
		fmt.Fprint(&s, e.Func)
	} else if e.Object != nil {
		fmt.Fprint(&s, e.Object)
	} else if e.Array != nil {
		fmt.Fprint(&s, e.Array)
	} else if e.Number != "" {
		fmt.Fprint(&s, e.Number)
	} else if e.Unary != nil {
		fmt.Fprint(&s, e.Unary)
	} else if e.Str != "" {
		fmt.Fprint(&s, e.Str)
	} else if e.RawStr != "" {
		fmt.Fprint(&s, strconv.Quote(e.RawStr))
	} else if e.If != nil {
		fmt.Fprint(&s, e.If)
	} else if e.Try != nil {
		fmt.Fprint(&s, e.Try)
	} else if e.Reduce != nil {
		fmt.Fprint(&s, e.Reduce)
	} else if e.Foreach != nil {
		fmt.Fprint(&s, e.Foreach)
	} else if e.Break != "" {
		fmt.Fprintf(&s, "break %s", e.Break)
	} else if e.Query != nil {
		fmt.Fprintf(&s, "(%s)", e.Query)
	}
	for _, e := range e.SuffixList {
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *Term) minify() {
	if e.Index != nil {
		e.Index.minify()
	} else if e.Func != nil {
		e.Func.minify()
	} else if e.Object != nil {
		e.Object.minify()
	} else if e.Array != nil {
		e.Array.minify()
	} else if e.Unary != nil {
		e.Unary.minify()
	} else if e.If != nil {
		e.If.minify()
	} else if e.Try != nil {
		e.Try.minify()
	} else if e.Reduce != nil {
		e.Reduce.minify()
	} else if e.Foreach != nil {
		e.Foreach.minify()
	} else if e.Query != nil {
		e.Query.minify()
	}
	for _, e := range e.SuffixList {
		e.minify()
	}
}

func (e *Term) toQuery() *Query {
	return (&Factor{Left: e}).toQuery()
}

func (e *Term) toFilter() *Filter {
	return (&Factor{Left: e}).toFilter()
}

func (e *Term) toLogic() *Logic {
	return (&Factor{Left: e}).toLogic()
}

func (e *Term) toFunc() string {
	if len(e.SuffixList) != 0 {
		return ""
	}
	// ref: compiler#compileComma
	if e.Identity {
		return "."
	} else if e.Recurse {
		return ".."
	} else if e.Null {
		return "null"
	} else if e.True {
		return "true"
	} else if e.False {
		return "false"
	} else if e.Func != nil {
		return e.Func.toFunc()
	} else {
		return ""
	}
}

func (e *Term) toIndices() []interface{} {
	if e.Index != nil {
		xs := e.Index.toIndices()
		if xs == nil {
			return nil
		}
		for _, s := range e.SuffixList {
			x := s.toIndices()
			if x == nil {
				return nil
			}
			xs = append(xs, x...)
		}
		return xs
	} else if e.Query != nil && len(e.SuffixList) == 0 {
		return e.Query.toIndices()
	} else {
		return nil
	}
}

// Unary ...
type Unary struct {
	Op   Operator `@("+" | "-")`
	Term *Term    `@@`
}

func (e *Unary) String() string {
	return fmt.Sprintf("%s%s", e.Op, e.Term)
}

func (e *Unary) minify() {
	e.Term.minify()
}

// Pattern ...
type Pattern struct {
	Name   string          `  @Variable`
	Array  []*Pattern      `| "[" @@ ("," @@)* "]"`
	Object []PatternObject `| "{" @@ ("," @@)* "}"`
}

func (e *Pattern) String() string {
	var s strings.Builder
	if e.Name != "" {
		s.WriteString(e.Name)
	} else if len(e.Array) > 0 {
		s.WriteRune('[')
		for i, e := range e.Array {
			if i > 0 {
				s.WriteString(", ")
			}
			fmt.Fprint(&s, e)
		}
		s.WriteRune(']')
	} else if len(e.Object) > 0 {
		s.WriteRune('{')
		for i, e := range e.Object {
			if i > 0 {
				s.WriteString(", ")
			}
			fmt.Fprint(&s, &e)
		}
		s.WriteRune('}')
	}
	return s.String()
}

// PatternObject ...
type PatternObject struct {
	Key       string   `( ( @Ident | @Variable | @Keyword )`
	KeyString string   `  | @String`
	Query     *Query   `  | "(" @@ ")" ) ":"`
	Val       *Pattern `@@`
	KeyOnly   string   `| @Variable`
}

func (e *PatternObject) String() string {
	var s strings.Builder
	if e.Key != "" {
		s.WriteString(e.Key)
	} else if e.KeyString != "" {
		s.WriteString(e.KeyString)
	} else if e.Query != nil {
		fmt.Fprintf(&s, "(%s)", e.Query)
	}
	if e.Val != nil {
		s.WriteString(": ")
		fmt.Fprint(&s, e.Val)
	}
	if e.KeyOnly != "" {
		s.WriteString(e.KeyOnly)
	}
	return s.String()
}

// Index ...
type Index struct {
	Name    string `@Index`
	Str     string `| "." ( @String`
	Start   *Query `| "[" ( @@`
	IsSlice bool   `( @":"`
	End     *Query `@@? )? | ":" @@ ) "]" )`
}

func (e *Index) String() string {
	var s strings.Builder
	if e.Name != "" {
		s.WriteString(e.Name)
	} else {
		s.WriteByte('.')
		if e.Str != "" {
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
	}
	return s.String()
}

func (e *Index) minify() {
	if e.Start != nil {
		e.Start.minify()
	}
	if e.End != nil {
		e.End.minify()
	}
}

func (e *Index) toIndices() []interface{} {
	if e.Name == "" {
		return nil
	}
	return []interface{}{e.Name[1:]}
}

// Func ...
type Func struct {
	Name string   `( @Ident | @Variable | @ModuleIdent )`
	Args []*Query `( "(" @@ (";" @@)* ")" )?`
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

func (e *Func) minify() {
	for _, x := range e.Args {
		x.minify()
	}
}

func (e *Func) toFunc() string {
	if len(e.Args) != 0 {
		return ""
	}
	return e.Name
}

// Object ...
type Object struct {
	KeyVals []ObjectKeyVal `"{" (@@ ("," @@)* ","?)? "}"`
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

func (e *Object) minify() {
	for _, e := range e.KeyVals {
		e.minify()
	}
}

// ObjectKeyVal ...
type ObjectKeyVal struct {
	Key           string     `( ( ( @Ident | @Variable | @Keyword )`
	KeyString     string     `  | @String )`
	Query         *Query     `| "(" @@ ")" ) ":"`
	Val           *ObjectVal `@@`
	KeyOnly       *string    `| @Ident | @Variable`
	KeyOnlyString string     `| @String`
}

func (e *ObjectKeyVal) String() string {
	var s strings.Builder
	if e.Key != "" {
		s.WriteString(e.Key)
	} else if e.KeyString != "" {
		s.WriteString(e.KeyString)
	} else if e.Query != nil {
		fmt.Fprintf(&s, "(%s)", e.Query)
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

func (e *ObjectKeyVal) minify() {
	if e.Query != nil {
		e.Query.minify()
	}
	if e.Val != nil {
		e.Val.minify()
	}
}

// ObjectVal ...
type ObjectVal struct {
	Alts []*Alt `@@ ("|" @@)*`
}

func (e *ObjectVal) String() string {
	var s strings.Builder
	for i, e := range e.Alts {
		if i > 0 {
			s.WriteString(" | ")
		}
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *ObjectVal) minify() {
	for _, e := range e.Alts {
		e.minify()
	}
}

// Array ...
type Array struct {
	Query *Query `"[" @@? "]"`
}

func (e *Array) String() string {
	if e.Query == nil {
		return "[]"
	}
	return fmt.Sprintf("[%s]", e.Query)
}

func (e *Array) minify() {
	if e.Query != nil {
		e.Query.minify()
	}
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

func (e *Suffix) minify() {
	if e.Index != nil {
		e.Index.minify()
	} else if e.SuffixIndex != nil {
		e.SuffixIndex.minify()
	}
}

func (e *Suffix) toTerm() (*Term, bool) {
	if e.Index != nil {
		return &Term{Index: e.Index}, true
	} else if e.SuffixIndex != nil {
		return &Term{Index: e.SuffixIndex.toIndex()}, true
	} else if e.Iter {
		return &Term{Identity: true, SuffixList: []*Suffix{{Iter: true}}}, true
	} else {
		return nil, false
	}
}

func (e *Suffix) toIndices() []interface{} {
	if e.Index == nil {
		return nil
	}
	return e.Index.toIndices()
}

// SuffixIndex ...
type SuffixIndex struct {
	Start   *Query `"[" ( @@`
	IsSlice bool   `( @":"`
	End     *Query `@@? )? | ":" @@ ) "]"`
}

func (e *SuffixIndex) String() string {
	return e.toIndex().String()[1:]
}

func (e *SuffixIndex) minify() {
	if e.Start != nil {
		e.Start.minify()
	}
	if e.End != nil {
		e.End.minify()
	}
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
	Cond *Query   `"if" @@`
	Then *Query   `"then" @@`
	Elif []IfElif `@@*`
	Else *Query   `("else" @@)? "end"`
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

func (e *If) minify() {
	e.Cond.minify()
	e.Then.minify()
	for _, x := range e.Elif {
		x.minify()
	}
	if e.Else != nil {
		e.Else.minify()
	}
}

// IfElif ...
type IfElif struct {
	Cond *Query `"elif" @@`
	Then *Query `"then" @@`
}

func (e *IfElif) String() string {
	return fmt.Sprintf("elif %s then %s", e.Cond, e.Then)
}

func (e *IfElif) minify() {
	e.Cond.minify()
	e.Then.minify()
}

// Try ...
type Try struct {
	Body  *Query `"try" @@`
	Catch *Term  `("catch" @@)?`
}

func (e *Try) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "try %s", e.Body)
	if e.Catch != nil {
		fmt.Fprintf(&s, " catch %s", e.Catch)
	}
	return s.String()
}

func (e *Try) minify() {
	e.Body.minify()
	if e.Catch != nil {
		e.Catch.minify()
	}
}

// Reduce ...
type Reduce struct {
	Term    *Term    `"reduce" @@`
	Pattern *Pattern `"as" @@`
	Start   *Query   `"(" @@`
	Update  *Query   `";" @@ ")"`
}

func (e *Reduce) String() string {
	return fmt.Sprintf("reduce %s as %s (%s; %s)", e.Term, e.Pattern, e.Start, e.Update)
}

func (e *Reduce) minify() {
	e.Term.minify()
	e.Start.minify()
	e.Update.minify()
}

// Foreach ...
type Foreach struct {
	Term    *Term    `"foreach" @@`
	Pattern *Pattern `"as" @@`
	Start   *Query   `"(" @@`
	Update  *Query   `";" @@`
	Extract *Query   `(";" @@)? ")"`
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

func (e *Foreach) minify() {
	e.Term.minify()
	e.Start.minify()
	e.Update.minify()
	if e.Extract != nil {
		e.Extract.minify()
	}
}

// Label ...
type Label struct {
	Ident string `"label" @Variable`
	Body  *Query `"|" @@`
}

func (e *Label) String() string {
	return fmt.Sprintf("label %s | %s", e.Ident, e.Body)
}

func (e *Label) minify() {
	e.Body.minify()
}

// ConstTerm ...
type ConstTerm struct {
	Object *ConstObject `  @@`
	Array  *ConstArray  `| @@`
	Number string       `| @Number`
	Str    string       `| @String`
	Null   bool         `| @"null"`
	True   bool         `| @"true"`
	False  bool         `| @"false"`
}

func (e *ConstTerm) String() string {
	var s strings.Builder
	if e.Object != nil {
		fmt.Fprint(&s, e.Object)
	} else if e.Array != nil {
		fmt.Fprint(&s, e.Array)
	} else if e.Number != "" {
		fmt.Fprint(&s, e.Number)
	} else if e.Str != "" {
		fmt.Fprint(&s, e.Str)
	} else if e.Null {
		s.WriteString("null")
	} else if e.True {
		s.WriteString("true")
	} else if e.False {
		s.WriteString("false")
	}
	return s.String()
}

func (e *ConstTerm) toValue() interface{} {
	if e.Object != nil {
		return e.Object.ToValue()
	} else if e.Array != nil {
		return e.Array.toValue()
	} else if e.Number != "" {
		return normalizeNumbers(json.Number(e.Number))
	} else if e.Str != "" {
		s, _ := strconv.Unquote(e.Str)
		return s
	} else if e.True {
		return true
	} else if e.False {
		return false
	} else {
		return nil
	}
}

// ConstObject ...
type ConstObject struct {
	KeyVals []ConstObjectKeyVal `"{" (@@ ("," @@)* ","?)? "}"`
}

func (e *ConstObject) String() string {
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

// ToValue converts the object to map[string]interface{}.
func (e *ConstObject) ToValue() map[string]interface{} {
	if e == nil {
		return nil
	}
	v := make(map[string]interface{}, len(e.KeyVals))
	for _, e := range e.KeyVals {
		key := e.Key
		if key == "" {
			key, _ = strconv.Unquote(e.KeyString)
		}
		v[key] = e.Val.toValue()
	}
	return v
}

// ConstObjectKeyVal ...
type ConstObjectKeyVal struct {
	Key       string     `( @Ident | @Keyword`
	KeyString string     `| @String ) ":"`
	Val       *ConstTerm `@@`
}

func (e *ConstObjectKeyVal) String() string {
	var s strings.Builder
	if e.Key != "" {
		s.WriteString(e.Key)
	} else {
		s.WriteString(e.KeyString)
	}
	s.WriteString(": ")
	fmt.Fprint(&s, e.Val)
	return s.String()
}

// ConstArray ...
type ConstArray struct {
	Elems []*ConstTerm `"[" (@@ ("," @@)*)? "]"`
}

func (e *ConstArray) String() string {
	var s strings.Builder
	s.WriteString("[")
	for i, e := range e.Elems {
		if i > 0 {
			s.WriteString(", ")
		}
		fmt.Fprint(&s, e)
	}
	s.WriteString("]")
	return s.String()
}

func (e *ConstArray) toValue() []interface{} {
	v := make([]interface{}, len(e.Elems))
	for i, e := range e.Elems {
		v[i] = e.toValue()
	}
	return v
}
