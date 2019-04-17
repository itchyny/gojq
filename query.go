package gojq

// Query ...
type Query struct {
	FuncDefs []*FuncDef `@@*`
	Pipe     *Pipe      `@@?`
}

// Run query.
func (q *Query) Run(v interface{}) (interface{}, error) {
	return newEnv(nil).applyQuery(q, v)
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
	Terms []*Term `@@ ("," @@)*`
}

// Term ...
type Term struct {
	ObjectIndex *ObjectIndex `( @@`
	ArrayIndex  *ArrayIndex  `| @@`
	Identity    *Identity    `| @@`
	Recurse     *Recurse     `| @@`
	Expression  *Expression  `| @@`
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

// Identity ...
type Identity struct {
	_ bool `"."`
}

// Recurse ...
type Recurse struct {
	X bool `@Recurse`
}

// Suffix ...
type Suffix struct {
	ObjectIndex *ObjectIndex `  @@`
	ArrayIndex  *ArrayIndex  `| @@`
	Array       *Array       `| @@`
	Optional    bool         `| @"?"`
}
