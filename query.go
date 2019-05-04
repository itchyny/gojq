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
	Logic *Logic `  @@`
	If    *If    `| @@`
}

// Logic ...
type Logic struct {
	Left  *AndExpr `@@`
	Right []struct {
		Op    Operator `@"or"`
		Right *AndExpr `@@`
	} `@@*`
}

// AndExpr ...
type AndExpr struct {
	Left  *Compare `@@`
	Right []struct {
		Op    Operator `@"and"`
		Right *Compare `@@`
	} `@@*`
}

// Compare ...
type Compare struct {
	Left  *Arith `@@`
	Right *struct {
		Op    Operator `@CompareOp`
		Right *Arith   `@@`
	} `@@?`
}

// Arith ...
type Arith struct {
	Left  *Factor `( @@`
	Right []struct {
		Op    Operator `@("+" | "-")`
		Right *Factor  `@@`
	} `@@* )`
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
	Recurse     bool         `| @".."`
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
	Bind       *struct {
		Ident string `"as" @Ident`
		Body  *Pipe  `"|" @@`
	} `@@?`
}

// ObjectIndex ...
type ObjectIndex struct {
	Name string `"." @Ident`
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
		Key     string  `( ( ( @Ident | @String )`
		Pipe    *Pipe   `| "(" @@ ")" ) ":"`
		Val     *Expr   `@@`
		KeyOnly *string `| ( @Ident | @String ) )`
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
	Else *Pipe `("else" @@)? "end"`
}
