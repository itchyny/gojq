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
	Left  *Factor `( @@`
	Right []struct {
		Op     Operator `@("+" | "-")`
		Factor *Factor  `@@`
	} `@@* )`
	If *If `| @@`
}

// Factor ...
type Factor struct {
	Left  *Term `@@`
	Right []struct {
		Op   Operator `@("*" | "/")`
		Term *Term    `@@`
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
	Pipe        *Pipe        `| "(" @@ ")" )`
	SuffixList  []*Suffix    `@@*`
}

// ObjectIndex ...
type ObjectIndex struct {
	Name string `"." ( @Ident | "[" @String "]" )`
}

// ArrayIndex ...
type ArrayIndex struct {
	Start *int `"." "[" ( @Integer?`
	End   *int `":" @Integer?`
	Index *int `| @Integer ) "]"`
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
