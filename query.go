package gojq

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// Query represents the abstract syntax tree of a jq query.
type Query struct {
	Meta     *ConstObject `json:"meta,omitempty"`
	Imports  []*Import    `json:"imports,omitempty"`
	FuncDefs []*FuncDef   `json:"func_defs,omitempty"`
	Term     *Term        `json:"term,omitempty"`
	Left     *Query       `json:"left,omitempty"`
	Op       Operator     `json:"op,omitempty"`
	Right    *Query       `json:"right,omitempty"`
	Func     *Token       `json:"func,omitempty"`
}

// Run the query.
//
// It is safe to call this method of a *Query in multiple goroutines.
func (e *Query) Run(v interface{}) Iter {
	return e.RunWithContext(context.Background(), v)
}

// RunWithContext runs the query with context.
func (e *Query) RunWithContext(ctx context.Context, v interface{}) Iter {
	code, err := Compile(e)
	if err != nil {
		return NewIter(err)
	}
	return code.RunWithContext(ctx, v)
}

func (e *Query) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Query) writeTo(s *strings.Builder) {
	if e.Meta != nil {
		s.WriteString("module ")
		e.Meta.writeTo(s)
		s.WriteString(";\n")
	}
	for _, im := range e.Imports {
		im.writeTo(s)
	}
	for i, fd := range e.FuncDefs {
		if i > 0 {
			s.WriteByte(' ')
		}
		fd.writeTo(s)
	}
	if len(e.FuncDefs) > 0 {
		s.WriteByte(' ')
	}
	if e.Func != nil {
		s.WriteString(e.Func.Str)
	} else if e.Term != nil {
		e.Term.writeTo(s)
	} else if e.Right != nil {
		e.Left.writeTo(s)
		if e.Op == OpComma {
			s.WriteString(", ")
		} else {
			s.WriteByte(' ')
			s.WriteString(e.Op.String())
			s.WriteByte(' ')
		}
		e.Right.writeTo(s)
	}
}

func (e *Query) minify() {
	for _, e := range e.FuncDefs {
		e.Minify()
	}
	if e.Term != nil {
		if name := e.Term.toFunc(); name != "" {
			e.Term = nil
			e.Func = &Token{Str: name}
		} else {
			e.Term.minify()
		}
	} else if e.Right != nil {
		e.Left.minify()
		e.Right.minify()
	}
}

func (e *Query) toIndices() []interface{} {
	if e.FuncDefs != nil || e.Right != nil || e.Term == nil {
		return nil
	}
	return e.Term.toIndices()
}

// Import ...
type Import struct {
	ImportPath  *Token       `json:"import_path,omitempty"`
	ImportAlias *Token       `json:"import_alias,omitempty"`
	IncludePath *Token       `json:"include_path,omitempty"`
	Meta        *ConstObject `json:"meta,omitempty"`
}

func (e *Import) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Import) writeTo(s *strings.Builder) {
	if e.ImportPath != nil {
		s.WriteString("import ")
		s.WriteString(strconv.Quote(e.ImportPath.Str))
		s.WriteString(" as ")
		s.WriteString(e.ImportAlias.Str)
	} else {
		s.WriteString("include ")
		s.WriteString(strconv.Quote(e.IncludePath.Str))
	}
	if e.Meta != nil {
		s.WriteByte(' ')
		e.Meta.writeTo(s)
	}
	s.WriteString(";\n")
}

// FuncDef ...
type FuncDef struct {
	Name *Token   `json:"name,omitempty"`
	Args []*Token `json:"args,omitempty"`
	Body *Query   `json:"body,omitempty"`
}

func (e *FuncDef) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *FuncDef) writeTo(s *strings.Builder) {
	s.WriteString("def ")
	s.WriteString(e.Name.Str)
	if len(e.Args) > 0 {
		s.WriteByte('(')
		for i, e := range e.Args {
			if i > 0 {
				s.WriteString("; ")
			}
			s.WriteString(e.Str)
		}
		s.WriteByte(')')
	}
	s.WriteString(": ")
	e.Body.writeTo(s)
	s.WriteByte(';')
}

// Minify ...
func (e *FuncDef) Minify() {
	e.Body.minify()
}

// Term ...
type Term struct {
	Type       TermType  `json:"type,omitempty"`
	Index      *Index    `json:"index,omitempty"`
	Func       *Func     `json:"func,omitempty"`
	Object     *Object   `json:"object,omitempty"`
	Array      *Array    `json:"array,omitempty"`
	Number     *Token    `json:"number,omitempty"`
	Unary      *Unary    `json:"unary,omitempty"`
	Format     *Token    `json:"format,omitempty"`
	Str        *String   `json:"str,omitempty"`
	If         *If       `json:"if,omitempty"`
	Try        *Try      `json:"try,omitempty"`
	Reduce     *Reduce   `json:"reduce,omitempty"`
	Foreach    *Foreach  `json:"foreach,omitempty"`
	Label      *Label    `json:"label,omitempty"`
	Break      *Token    `json:"break,omitempty"`
	Query      *Query    `json:"query,omitempty"`
	SuffixList []*Suffix `json:"suffix_list,omitempty"`
}

func (e *Term) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Term) writeTo(s *strings.Builder) {
	switch e.Type {
	case TermTypeIdentity:
		s.WriteByte('.')
	case TermTypeRecurse:
		s.WriteString("..")
	case TermTypeNull:
		s.WriteString("null")
	case TermTypeTrue:
		s.WriteString("true")
	case TermTypeFalse:
		s.WriteString("false")
	case TermTypeIndex:
		e.Index.writeTo(s)
	case TermTypeFunc:
		e.Func.writeTo(s)
	case TermTypeObject:
		e.Object.writeTo(s)
	case TermTypeArray:
		e.Array.writeTo(s)
	case TermTypeNumber:
		s.WriteString(e.Number.Str)
	case TermTypeUnary:
		e.Unary.writeTo(s)
	case TermTypeFormat:
		s.WriteString(e.Format.Str)
		if e.Str != nil {
			s.WriteByte(' ')
			e.Str.writeTo(s)
		}
	case TermTypeString:
		e.Str.writeTo(s)
	case TermTypeIf:
		e.If.writeTo(s)
	case TermTypeTry:
		e.Try.writeTo(s)
	case TermTypeReduce:
		e.Reduce.writeTo(s)
	case TermTypeForeach:
		e.Foreach.writeTo(s)
	case TermTypeLabel:
		e.Label.writeTo(s)
	case TermTypeBreak:
		s.WriteString("break ")
		s.WriteString(e.Break.Str)
	case TermTypeQuery:
		s.WriteByte('(')
		e.Query.writeTo(s)
		s.WriteByte(')')
	}
	for _, e := range e.SuffixList {
		e.writeTo(s)
	}
}

func (e *Term) minify() {
	switch e.Type {
	case TermTypeIndex:
		e.Index.minify()
	case TermTypeFunc:
		e.Func.minify()
	case TermTypeObject:
		e.Object.minify()
	case TermTypeArray:
		e.Array.minify()
	case TermTypeUnary:
		e.Unary.minify()
	case TermTypeFormat:
		if e.Str != nil {
			e.Str.minify()
		}
	case TermTypeString:
		e.Str.minify()
	case TermTypeIf:
		e.If.minify()
	case TermTypeTry:
		e.Try.minify()
	case TermTypeReduce:
		e.Reduce.minify()
	case TermTypeForeach:
		e.Foreach.minify()
	case TermTypeLabel:
		e.Label.minify()
	case TermTypeQuery:
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
	switch e.Type {
	case TermTypeIdentity:
		return "."
	case TermTypeRecurse:
		return ".."
	case TermTypeNull:
		return "null"
	case TermTypeTrue:
		return "true"
	case TermTypeFalse:
		return "false"
	case TermTypeFunc:
		return e.Func.toFunc()
	default:
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
	Op   Operator `json:"op,omitempty"`
	Term *Term    `json:"term,omitempty"`
}

func (e *Unary) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Unary) writeTo(s *strings.Builder) {
	s.WriteString(e.Op.String())
	e.Term.writeTo(s)
}

func (e *Unary) minify() {
	e.Term.minify()
}

// Pattern ...
type Pattern struct {
	Name   *Token           `json:"name,omitempty"`
	Array  []*Pattern       `json:"array,omitempty"`
	Object []*PatternObject `json:"object,omitempty"`
}

func (e *Pattern) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Pattern) writeTo(s *strings.Builder) {
	if e.Name != nil {
		s.WriteString(e.Name.Str)
	} else if len(e.Array) > 0 {
		s.WriteByte('[')
		for i, e := range e.Array {
			if i > 0 {
				s.WriteString(", ")
			}
			e.writeTo(s)
		}
		s.WriteByte(']')
	} else if len(e.Object) > 0 {
		s.WriteByte('{')
		for i, e := range e.Object {
			if i > 0 {
				s.WriteString(", ")
			}
			e.writeTo(s)
		}
		s.WriteByte('}')
	}
}

// PatternObject ...
type PatternObject struct {
	Key       *Token   `json:"key,omitempty"`
	KeyString *String  `json:"key_string,omitempty"`
	KeyQuery  *Query   `json:"key_query,omitempty"`
	Val       *Pattern `json:"val,omitempty"`
	KeyOnly   *Token   `json:"key_only,omitempty"`
}

func (e *PatternObject) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *PatternObject) writeTo(s *strings.Builder) {
	if e.Key != nil {
		s.WriteString(e.Key.Str)
	} else if e.KeyString != nil {
		e.KeyString.writeTo(s)
	} else if e.KeyQuery != nil {
		s.WriteByte('(')
		e.KeyQuery.writeTo(s)
		s.WriteByte(')')
	}
	if e.Val != nil {
		s.WriteString(": ")
		e.Val.writeTo(s)
	}
	if e.KeyOnly != nil {
		s.WriteString(e.KeyOnly.Str)
	}
}

// Index ...
type Index struct {
	Name    *Token  `json:"name,omitempty"`
	Str     *String `json:"str,omitempty"`
	Start   *Query  `json:"start,omitempty"`
	IsSlice bool    `json:"is_slice,omitempty"`
	End     *Query  `json:"end,omitempty"`
}

func (e *Index) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Index) writeTo(s *strings.Builder) {
	if l := s.Len(); l > 0 {
		// ". .x" != "..x" and "0 .x" != "0.x"
		if c := s.String()[l-1]; c == '.' || '0' <= c && c <= '9' {
			s.WriteByte(' ')
		}
	}
	s.WriteByte('.')
	e.writeSuffixTo(s)
}

func (e *Index) writeSuffixTo(s *strings.Builder) {
	if e.Name != nil {
		s.WriteString(e.Name.Str)
	} else {
		if e.Str != nil {
			e.Str.writeTo(s)
		} else {
			s.WriteByte('[')
			if e.Start != nil {
				e.Start.writeTo(s)
				if e.IsSlice {
					s.WriteByte(':')
					if e.End != nil {
						e.End.writeTo(s)
					}
				}
			} else if e.End != nil {
				s.WriteByte(':')
				e.End.writeTo(s)
			}
			s.WriteByte(']')
		}
	}
}

func (e *Index) minify() {
	if e.Str != nil {
		e.Str.minify()
	}
	if e.Start != nil {
		e.Start.minify()
	}
	if e.End != nil {
		e.End.minify()
	}
}

func (e *Index) toIndices() []interface{} {
	if e.Name == nil {
		return nil
	}
	return []interface{}{e.Name.Str}
}

// Func ...
type Func struct {
	Name *Token   `json:"name,omitempty"`
	Args []*Query `json:"args,omitempty"`
}

func (e *Func) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Func) writeTo(s *strings.Builder) {
	s.WriteString(e.Name.Str)
	if len(e.Args) > 0 {
		s.WriteByte('(')
		for i, e := range e.Args {
			if i > 0 {
				s.WriteString("; ")
			}
			e.writeTo(s)
		}
		s.WriteByte(')')
	}
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
	return e.Name.Str
}

// String ...
type String struct {
	Str     *Token   `json:"str,omitempty"`
	Queries []*Query `json:"queries,omitempty"`
}

func (e *String) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *String) writeTo(s *strings.Builder) {
	if e.Queries == nil {
		s.WriteString(strconv.Quote(e.Str.Str))
		return
	}
	s.WriteByte('"')
	for _, e := range e.Queries {
		if e.Term.Str == nil {
			s.WriteString(`\`)
			e.writeTo(s)
		} else {
			es := e.String()
			s.WriteString(es[1 : len(es)-1])
		}
	}
	s.WriteByte('"')
}

func (e *String) minify() {
	for _, e := range e.Queries {
		e.minify()
	}
}

// Object ...
type Object struct {
	KeyVals []*ObjectKeyVal `json:"key_vals,omitempty"`
}

func (e *Object) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Object) writeTo(s *strings.Builder) {
	if len(e.KeyVals) == 0 {
		s.WriteString("{}")
		return
	}
	s.WriteString("{ ")
	for i, kv := range e.KeyVals {
		if i > 0 {
			s.WriteString(", ")
		}
		kv.writeTo(s)
	}
	s.WriteString(" }")
}

func (e *Object) minify() {
	for _, e := range e.KeyVals {
		e.minify()
	}
}

// ObjectKeyVal ...
type ObjectKeyVal struct {
	Key           *Token     `json:"key,omitempty"`
	KeyString     *String    `json:"key_string,omitempty"`
	KeyQuery      *Query     `json:"key_query,omitempty"`
	Val           *ObjectVal `json:"val,omitempty"`
	KeyOnly       *Token     `json:"key_only,omitempty"`
	KeyOnlyString *String    `json:"key_only_string,omitempty"`
}

func (e *ObjectKeyVal) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *ObjectKeyVal) writeTo(s *strings.Builder) {
	if e.Key != nil {
		s.WriteString(e.Key.Str)
	} else if e.KeyString != nil {
		e.KeyString.writeTo(s)
	} else if e.KeyQuery != nil {
		s.WriteByte('(')
		e.KeyQuery.writeTo(s)
		s.WriteByte(')')
	}
	if e.Val != nil {
		s.WriteString(": ")
		e.Val.writeTo(s)
	}
	if e.KeyOnly != nil {
		s.WriteString(e.KeyOnly.Str)
	} else if e.KeyOnlyString != nil {
		e.KeyOnlyString.writeTo(s)
	}
}

func (e *ObjectKeyVal) minify() {
	if e.KeyString != nil {
		e.KeyString.minify()
	} else if e.KeyQuery != nil {
		e.KeyQuery.minify()
	}
	if e.Val != nil {
		e.Val.minify()
	}
	if e.KeyOnlyString != nil {
		e.KeyOnlyString.minify()
	}
}

// ObjectVal ...
type ObjectVal struct {
	Queries []*Query `json:"queries,omitempty"`
}

func (e *ObjectVal) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *ObjectVal) writeTo(s *strings.Builder) {
	for i, e := range e.Queries {
		if i > 0 {
			s.WriteString(" | ")
		}
		e.writeTo(s)
	}
}

func (e *ObjectVal) minify() {
	for _, e := range e.Queries {
		e.minify()
	}
}

// Array ...
type Array struct {
	Query *Query `json:"query,omitempty"`
}

func (e *Array) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Array) writeTo(s *strings.Builder) {
	s.WriteByte('[')
	if e.Query != nil {
		e.Query.writeTo(s)
	}
	s.WriteByte(']')
}

func (e *Array) minify() {
	if e.Query != nil {
		e.Query.minify()
	}
}

// Suffix ...
type Suffix struct {
	Index    *Index `json:"index,omitempty"`
	Iter     bool   `json:"iter,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Bind     *Bind  `json:"bind,omitempty"`
}

func (e *Suffix) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Suffix) writeTo(s *strings.Builder) {
	if e.Index != nil {
		if e.Index.Name != nil || e.Index.Str != nil {
			e.Index.writeTo(s)
		} else {
			e.Index.writeSuffixTo(s)
		}
	} else if e.Iter {
		s.WriteString("[]")
	} else if e.Optional {
		s.WriteByte('?')
	} else if e.Bind != nil {
		e.Bind.writeTo(s)
	}
}

func (e *Suffix) minify() {
	if e.Index != nil {
		e.Index.minify()
	} else if e.Bind != nil {
		e.Bind.minify()
	}
}

func (e *Suffix) toTerm() (*Term, bool) {
	if e.Index != nil {
		return &Term{Type: TermTypeIndex, Index: e.Index}, true
	} else if e.Iter {
		return &Term{Type: TermTypeIdentity, SuffixList: []*Suffix{{Iter: true}}}, true
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

// Bind ...
type Bind struct {
	Patterns []*Pattern `json:"patterns,omitempty"`
	Body     *Query     `json:"body,omitempty"`
}

func (e *Bind) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Bind) writeTo(s *strings.Builder) {
	for i, p := range e.Patterns {
		if i == 0 {
			s.WriteString(" as ")
			p.writeTo(s)
			s.WriteByte(' ')
		} else {
			s.WriteString("?// ")
			p.writeTo(s)
			s.WriteByte(' ')
		}
	}
	s.WriteString("| ")
	e.Body.writeTo(s)
}

func (e *Bind) minify() {
	e.Body.minify()
}

// If ...
type If struct {
	Cond *Query    `json:"cond,omitempty"`
	Then *Query    `json:"then,omitempty"`
	Elif []*IfElif `json:"elif,omitempty"`
	Else *Query    `json:"else,omitempty"`
}

func (e *If) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *If) writeTo(s *strings.Builder) {
	s.WriteString("if ")
	e.Cond.writeTo(s)
	s.WriteString(" then ")
	e.Then.writeTo(s)
	for _, e := range e.Elif {
		s.WriteByte(' ')
		e.writeTo(s)
	}
	if e.Else != nil {
		s.WriteString(" else ")
		e.Else.writeTo(s)
	}
	s.WriteString(" end")
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
	Cond *Query `json:"cond,omitempty"`
	Then *Query `json:"then,omitempty"`
}

func (e *IfElif) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *IfElif) writeTo(s *strings.Builder) {
	s.WriteString("elif ")
	e.Cond.writeTo(s)
	s.WriteString(" then ")
	e.Then.writeTo(s)
}

func (e *IfElif) minify() {
	e.Cond.minify()
	e.Then.minify()
}

// Try ...
type Try struct {
	Body  *Query `json:"body,omitempty"`
	Catch *Query `json:"catch,omitempty"`
}

func (e *Try) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Try) writeTo(s *strings.Builder) {
	s.WriteString("try ")
	e.Body.writeTo(s)
	if e.Catch != nil {
		s.WriteString(" catch ")
		e.Catch.writeTo(s)
	}
}

func (e *Try) minify() {
	e.Body.minify()
	if e.Catch != nil {
		e.Catch.minify()
	}
}

// Reduce ...
type Reduce struct {
	Term    *Term    `json:"term,omitempty"`
	Pattern *Pattern `json:"pattern,omitempty"`
	Start   *Query   `json:"start,omitempty"`
	Update  *Query   `json:"update,omitempty"`
}

func (e *Reduce) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Reduce) writeTo(s *strings.Builder) {
	s.WriteString("reduce ")
	e.Term.writeTo(s)
	s.WriteString(" as ")
	e.Pattern.writeTo(s)
	s.WriteString(" (")
	e.Start.writeTo(s)
	s.WriteString("; ")
	e.Update.writeTo(s)
	s.WriteByte(')')
}

func (e *Reduce) minify() {
	e.Term.minify()
	e.Start.minify()
	e.Update.minify()
}

// Foreach ...
type Foreach struct {
	Term    *Term    `json:"term,omitempty"`
	Pattern *Pattern `json:"pattern,omitempty"`
	Start   *Query   `json:"start,omitempty"`
	Update  *Query   `json:"update,omitempty"`
	Extract *Query   `json:"extract,omitempty"`
}

func (e *Foreach) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Foreach) writeTo(s *strings.Builder) {
	s.WriteString("foreach ")
	e.Term.writeTo(s)
	s.WriteString(" as ")
	e.Pattern.writeTo(s)
	s.WriteString(" (")
	e.Start.writeTo(s)
	s.WriteString("; ")
	e.Update.writeTo(s)
	if e.Extract != nil {
		s.WriteString("; ")
		e.Extract.writeTo(s)
	}
	s.WriteByte(')')
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
	Ident *Token `json:"ident,omitempty"`
	Body  *Query `json:"body,omitempty"`
}

func (e *Label) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *Label) writeTo(s *strings.Builder) {
	s.WriteString("label ")
	s.WriteString(e.Ident.Str)
	s.WriteString(" | ")
	e.Body.writeTo(s)
}

func (e *Label) minify() {
	e.Body.minify()
}

// ConstTerm ...
type ConstTerm struct {
	Object *ConstObject `json:"object,omitempty"`
	Array  *ConstArray  `json:"array,omitempty"`
	Number *Token       `json:"number,omitempty"`
	Str    *Token       `json:"str,omitempty"`
	Null   bool         `json:"null,omitempty"`
	True   bool         `json:"true,omitempty"`
	False  bool         `json:"false,omitempty"`
}

func (e *ConstTerm) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *ConstTerm) writeTo(s *strings.Builder) {
	if e.Object != nil {
		e.Object.writeTo(s)
	} else if e.Array != nil {
		e.Array.writeTo(s)
	} else if e.Number != nil {
		s.WriteString(e.Number.Str)
	} else if e.Str != nil {
		s.WriteString(strconv.Quote(e.Str.Str))
	} else if e.Null {
		s.WriteString("null")
	} else if e.True {
		s.WriteString("true")
	} else if e.False {
		s.WriteString("false")
	}
}

func (e *ConstTerm) toValue() interface{} {
	if e.Object != nil {
		return e.Object.ToValue()
	} else if e.Array != nil {
		return e.Array.toValue()
	} else if e.Number != nil {
		return normalizeNumbers(json.Number(e.Number.Str))
	} else if e.Null {
		return nil
	} else if e.True {
		return true
	} else if e.False {
		return false
	} else {
		return e.Str.Str
	}
}

// ConstObject ...
type ConstObject struct {
	KeyVals []*ConstObjectKeyVal `json:"keyvals,omitempty"`
}

func (e *ConstObject) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *ConstObject) writeTo(s *strings.Builder) {
	if len(e.KeyVals) == 0 {
		s.WriteString("{}")
		return
	}
	s.WriteString("{ ")
	for i, kv := range e.KeyVals {
		if i > 0 {
			s.WriteString(", ")
		}
		kv.writeTo(s)
	}
	s.WriteString(" }")
}

// ToValue converts the object to map[string]interface{}.
func (e *ConstObject) ToValue() map[string]interface{} {
	if e == nil {
		return nil
	}
	v := make(map[string]interface{}, len(e.KeyVals))
	for _, e := range e.KeyVals {
		var key string
		if e.Key != nil {
			key = e.Key.Str
		} else if e.KeyString != nil {
			key = e.KeyString.Str
		}
		v[key] = e.Val.toValue()
	}
	return v
}

// ConstObjectKeyVal ...
type ConstObjectKeyVal struct {
	Key       *Token     `json:"key,omitempty"`
	KeyString *Token     `json:"key_string,omitempty"`
	Val       *ConstTerm `json:"val,omitempty"`
}

func (e *ConstObjectKeyVal) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *ConstObjectKeyVal) writeTo(s *strings.Builder) {
	if e.Key != nil {
		s.WriteString(e.Key.Str)
	} else {
		s.WriteString(e.KeyString.Str)
	}
	s.WriteString(": ")
	e.Val.writeTo(s)
}

// ConstArray ...
type ConstArray struct {
	Elems []*ConstTerm `json:"elems,omitempty"`
}

func (e *ConstArray) String() string {
	var s strings.Builder
	e.writeTo(&s)
	return s.String()
}

func (e *ConstArray) writeTo(s *strings.Builder) {
	s.WriteByte('[')
	for i, e := range e.Elems {
		if i > 0 {
			s.WriteString(", ")
		}
		e.writeTo(s)
	}
	s.WriteByte(']')
}

func (e *ConstArray) toValue() []interface{} {
	v := make([]interface{}, len(e.Elems))
	for i, e := range e.Elems {
		v[i] = e.toValue()
	}
	return v
}
