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

// Identity ...
type Identity struct {
	_ bool `"."`
}

// Recurse ...
type Recurse struct {
	X bool `@Recurse`
}

// Func ...
type Func struct {
	Name string  `@Ident`
	Args []*Pipe `( "(" @@ (";" @@)* ")" )?`
}

// Object ...
type Object struct {
	KeyVals []*ObjKeyVal `"{" (@@ ("," @@)*)? "}"`
}

// ObjKeyVal ...
type ObjKeyVal struct {
	Key  string `( ( @Ident | @String )`
	Pipe *Pipe  `| "(" @@ ")" ) ":"`
	Val  *Term  `@@`
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
