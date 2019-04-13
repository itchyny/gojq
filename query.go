package gojq

// Query ...
type Query struct {
	Pipe *Pipe `@@`
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
	Expression  *Expression  `| @@ )`
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
	_ string `"."`
}

// Suffix ...
type Suffix struct {
	ObjectIndex *ObjectIndex `  @@`
	ArrayIndex  *ArrayIndex  `| @@`
	Array       *Array       `| @@`
	Optional    bool         `| @"?"`
}
