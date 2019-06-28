package gojq

// Query ...
type Query struct {
	FuncDefs []*FuncDef `@@*`
	Pipe     *Pipe      `@@?`
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

// Pipe ...
type Pipe struct {
	Commas []*Comma `@@ ("|" @@)*`
}

// Comma ...
type Comma struct {
	Alts []*Alt `@@ ("," @@)*`
}

// Alt ...
type Alt struct {
	Left  *Expr      `@@`
	Right []AltRight `@@*`
}

// AltRight ...
type AltRight struct {
	Op    Operator `@"//"`
	Right *Expr    `@@`
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

// ExprBind ...
type ExprBind struct {
	Pattern *Pattern `"as" @@`
	Body    *Pipe    `"|" @@`
}

// Logic ...
type Logic struct {
	Left  *AndExpr     `@@`
	Right []LogicRight `@@*`
}

// LogicRight ...
type LogicRight struct {
	Op    Operator `@"or"`
	Right *AndExpr `@@`
}

// AndExpr ...
type AndExpr struct {
	Left  *Compare       `@@`
	Right []AndExprRight `@@*`
}

// AndExprRight ...
type AndExprRight struct {
	Op    Operator `@"and"`
	Right *Compare `@@`
}

// Compare ...
type Compare struct {
	Left  *Arith        `@@`
	Right *CompareRight `@@?`
}

// CompareRight ...
type CompareRight struct {
	Op    Operator `@CompareOp`
	Right *Arith   `@@`
}

// Arith ...
type Arith struct {
	Left  *Factor      `( @@`
	Right []ArithRight `@@* )`
}

// ArithRight ...
type ArithRight struct {
	Op    Operator `@("+" | "-")`
	Right *Factor  `@@`
}

// Factor ...
type Factor struct {
	Left  *Term         `@@`
	Right []FactorRight `@@*`
}

// FactorRight ...
type FactorRight struct {
	Op    Operator `@("*" | "/" | "%")`
	Right *Term    `@@`
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
	String     string    `| @String`
	Null       bool      `| @"null"`
	True       bool      `| @"true"`
	False      bool      `| @"false"`
	Break      string    `| "break" @Ident`
	Pipe       *Pipe     `| "(" @@ ")" )`
	SuffixList []*Suffix `@@*`
}

// Unary ...
type Unary struct {
	Op   Operator `@("+" | "-")`
	Term *Term    `@@`
}

// Pattern ...
type Pattern struct {
	Name   string          `( @Ident`
	Array  []*Pattern      `| "[" @@ ("," @@)* "]"`
	Object []PatternObject `| "{" @@ ("," @@)* "}" )`
}

// PatternObject ...
type PatternObject struct {
	Key       string   `( ( @Ident | @Keyword )`
	KeyString string   `| @String ) ":"`
	Val       *Pattern `@@`
	KeyOnly   string   `| @Ident`
}

// Index ...
type Index struct {
	Name    string `"." ( @Ident`
	String  string `| @String`
	Start   *Pipe  `| "[" ( @@`
	IsSlice bool   `( @":"`
	End     *Pipe  `@@? )? | ":" @@ ) "]" )`
}

// Func ...
type Func struct {
	Name    string  `@Ident`
	Args    []*Pipe `( "(" @@ (";" @@)* ")" )?`
	funcDef *FuncDef
}

// Object ...
type Object struct {
	KeyVals []ObjectKeyVal `"{" (@@ ("," @@)*)? "}"`
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

// Array ...
type Array struct {
	Pipe *Pipe `"[" @@? "]"`
}

// Suffix ...
type Suffix struct {
	Index       *Index       `  @@`
	SuffixIndex *SuffixIndex `| @@`
	Iter        bool         `| @("[" "]")`
	Optional    bool         `| @"?"`
}

// SuffixIndex ...
type SuffixIndex struct {
	Start   *Pipe `"[" ( @@`
	IsSlice bool  `( @":"`
	End     *Pipe `@@? )? | ":" @@ ) "]"`
}

// If ...
type If struct {
	Cond *Pipe    `"if" @@`
	Then *Pipe    `"then" @@`
	Elif []IfElif `@@*`
	Else *Pipe    `("else" @@)? "end"`
}

// IfElif ...
type IfElif struct {
	Cond *Pipe `"elif" @@`
	Then *Pipe `"then" @@`
}

// Try ...
type Try struct {
	Body  *Pipe `"try" @@`
	Catch *Pipe `("catch" @@)?`
}

// Reduce ...
type Reduce struct {
	Term    *Term    `"reduce" @@`
	Pattern *Pattern `"as" @@`
	Start   *Pipe    `"(" @@`
	Update  *Pipe    `";" @@ ")"`
}

// Foreach ...
type Foreach struct {
	Term    *Term    `"foreach" @@`
	Pattern *Pattern `"as" @@`
	Start   *Pipe    `"(" @@`
	Update  *Pipe    `";" @@`
	Extract *Pipe    `(";" @@)? ")"`
}

// Label ...
type Label struct {
	Ident string `"label" @Ident`
	Body  *Pipe  `"|" @@`
}
