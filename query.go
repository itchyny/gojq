package gojq

// Query ...
type Query struct {
	FuncDefs []*FuncDef `@@*`
	Pipe     *Pipe      `@@?`
}

// Run query.
func (q *Query) Run(v interface{}) <-chan interface{} {
	return newEnv(nil).applyQuery(q, unitIterator(v))
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
	Exprs []*Expr `@@ ("," @@)*`
}

// Expr ...
type Expr struct {
	Left  *Compare `( @@`
	Right []struct {
		Op    Operator `@("+" | "-")`
		Right *Compare `@@`
	} `@@* )`
	If *If `| @@`
}

// Compare ...
type Compare struct {
	Left  *Factor `@@`
	Right []struct {
		Op    Operator `@CompareOp`
		Right *Factor  `@@`
	} `@@*`
}

// Factor ...
type Factor struct {
	Left  *Term `@@`
	Right []struct {
		Op    Operator `@("*" | "/" | "%")`
		Right *Term    `@@`
	} `@@*`
}

// Term ...
type Term struct {
	ObjectIndex *ObjectIndex `( @@`
	ArrayIndex  *ArrayIndex  `| @@`
	Identity    bool         `| @"."`
	Recurse     bool         `| @Recurse`
	Func        *Func        `| @@`
	Object      *Object      `| @@`
	Array       *Array       `| @@`
	Number      *float64     `| @Number`
	Unary       *struct {
		Op   Operator `@("+" | "-")`
		Term *Term    `@@`
	} `| @@`
	String     *string   `| @String`
	Pipe       *Pipe     `| "(" @@ ")" )`
	SuffixList []*Suffix `@@*`
}

// ObjectIndex ...
type ObjectIndex struct {
	Name string `"." ( @Ident | "[" @String "]" )`
}

// ArrayIndex ...
type ArrayIndex struct {
	Index   *Pipe `"." "[" ( @@`
	IsSlice bool  `( @":"`
	End     *Pipe `@@? )? | ":" @@ ) "]"`
}

// Func ...
type Func struct {
	Name string  `@Ident`
	Args []*Pipe `( "(" @@ (";" @@)* ")" )?`
}

// Object ...
type Object struct {
	KeyVals []struct {
		Key  string `( ( @Ident | @String )`
		Pipe *Pipe  `| "(" @@ ")" ) ":"`
		Val  *Expr  `@@`
	} `"{" (@@ ("," @@)*)? "}"`
}

// Array ...
type Array struct {
	Pipe *Pipe `"[" @@? "]"`
}

// Suffix ...
type Suffix struct {
	ObjectIndex *ObjectIndex `  @@`
	ArrayIndex  *ArrayIndex  `| @@`
	Array       *Array       `| @@`
	Optional    bool         `| @"?"`
}

// If ...
type If struct {
	Cond *Pipe `"if" @@`
	Then *Pipe `"then" @@`
	Elif []struct {
		Cond *Pipe `"elif" @@`
		Then *Pipe `"then" @@`
	} `@@*`
	Else *Pipe `"else" @@ "end"`
}
