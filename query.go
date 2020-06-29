package gojq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Query ...
type Query struct {
	Meta     *ConstObject
	Imports  []*Import
	FuncDefs []*FuncDef
	Term     *Term
	Left     *Query
	Op       Operator
	Right    *Query
	Bind     *Bind
	Func     string
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

func (e *Query) String() string {
	var s strings.Builder
	if e.Meta != nil {
		fmt.Fprintf(&s, "module %s;\n", e.Meta)
	}
	for _, im := range e.Imports {
		fmt.Fprint(&s, im)
	}
	for i, fd := range e.FuncDefs {
		if i > 0 {
			s.WriteByte(' ')
		}
		fmt.Fprint(&s, fd)
	}
	if len(e.FuncDefs) > 0 {
		s.WriteByte(' ')
	}
	if e.Func != "" {
		s.WriteString(e.Func)
	} else if e.Term != nil {
		fmt.Fprint(&s, e.Term)
		if e.Bind != nil {
			fmt.Fprint(&s, e.Bind)
		}
	} else if e.Right != nil {
		fmt.Fprintf(&s, "%s %s %s", e.Left, e.Op, e.Right)
	}
	return s.String()
}

func (e *Query) minify() {
	for _, e := range e.FuncDefs {
		e.Minify()
	}
	if e.Term != nil {
		if e.Bind == nil {
			if name := e.Term.toFunc(); name != "" {
				e.Term = nil
				e.Func = name
			} else {
				e.Term.minify()
			}
		} else {
			e.Term.minify()
			e.Bind.minify()
		}
	} else if e.Right != nil {
		e.Left.minify()
		e.Right.minify()
	}
}

func (e *Query) toIndices() []interface{} {
	if e.FuncDefs != nil || e.Right != nil || e.Bind != nil || e.Term == nil {
		return nil
	}
	return e.Term.toIndices()
}

func (e *Query) countCommaQueries() int {
	if e.Op == OpComma {
		return e.Left.countCommaQueries() + e.Right.countCommaQueries()
	}
	return 1
}

// Import ...
type Import struct {
	ImportPath  string
	ImportAlias string
	IncludePath string
	Meta        *ConstObject
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
	Name string
	Args []string
	Body *Query
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

// Bind ...
type Bind struct {
	Patterns []*Pattern
	Body     *Query
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

// Term ...
type Term struct {
	Index      *Index
	Identity   bool
	Recurse    bool
	Null       bool
	True       bool
	False      bool
	Func       *Func
	Object     *Object
	Array      *Array
	Number     string
	Unary      *Unary
	Format     string
	FormatStr  string
	Str        string
	RawStr     string
	If         *If
	Try        *Try
	Reduce     *Reduce
	Foreach    *Foreach
	Label      *Label
	Break      string
	Query      *Query
	SuffixList []*Suffix
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
	} else if e.Label != nil {
		fmt.Fprint(&s, e.Label)
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
	} else if e.Label != nil {
		e.Label.minify()
	} else if e.Query != nil {
		e.Query.minify()
	}
	for _, e := range e.SuffixList {
		e.minify()
	}
}

func (e *Term) toFunc() string {
	if len(e.SuffixList) != 0 {
		return ""
	}
	// ref: compiler#compileQuery
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
	Op   Operator
	Term *Term
}

func (e *Unary) String() string {
	return fmt.Sprintf("%s%s", e.Op, e.Term)
}

func (e *Unary) minify() {
	e.Term.minify()
}

// Pattern ...
type Pattern struct {
	Name   string
	Array  []*Pattern
	Object []*PatternObject
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
			fmt.Fprint(&s, e)
		}
		s.WriteRune('}')
	}
	return s.String()
}

// PatternObject ...
type PatternObject struct {
	Key       string
	KeyString string
	Query     *Query
	Val       *Pattern
	KeyOnly   string
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
	Name    string
	Str     string
	Start   *Query
	IsSlice bool
	End     *Query
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
	Name string
	Args []*Query
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
	KeyVals []*ObjectKeyVal
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
		fmt.Fprint(&s, kv)
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
	Key           string
	KeyString     string
	Query         *Query
	Val           *ObjectVal
	KeyOnly       *string
	KeyOnlyString string
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
	Queries []*Query
}

func (e *ObjectVal) String() string {
	var s strings.Builder
	for i, e := range e.Queries {
		if i > 0 {
			s.WriteString(" | ")
		}
		fmt.Fprint(&s, e)
	}
	return s.String()
}

func (e *ObjectVal) minify() {
	for _, e := range e.Queries {
		e.minify()
	}
}

// Array ...
type Array struct {
	Query *Query
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
	Index       *Index
	SuffixIndex *SuffixIndex
	Iter        bool
	Optional    bool
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
	Start   *Query
	IsSlice bool
	End     *Query
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
	Cond *Query
	Then *Query
	Elif []*IfElif
	Else *Query
}

func (e *If) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "if %s then %s", e.Cond, e.Then)
	for _, e := range e.Elif {
		fmt.Fprintf(&s, " %s", e)
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
	Cond *Query
	Then *Query
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
	Body  *Query
	Catch *Term
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
	Term    *Term
	Pattern *Pattern
	Start   *Query
	Update  *Query
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
	Term    *Term
	Pattern *Pattern
	Start   *Query
	Update  *Query
	Extract *Query
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
	Ident string
	Body  *Query
}

func (e *Label) String() string {
	return fmt.Sprintf("label %s | %s", e.Ident, e.Body)
}

func (e *Label) minify() {
	e.Body.minify()
}

// ConstTerm ...
type ConstTerm struct {
	Object *ConstObject
	Array  *ConstArray
	Number string
	Str    string
	Null   bool
	True   bool
	False  bool
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
	KeyVals []*ConstObjectKeyVal
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
		fmt.Fprint(&s, kv)
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
	Key       string
	KeyString string
	Val       *ConstTerm
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
	Elems []*ConstTerm
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
