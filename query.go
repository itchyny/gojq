package gojq

// Query ...
type Query struct {
	Pipe *Pipe `@@`
}

// Pipe ...
type Pipe struct {
	Terms []*Term `@@ ("|" @@)*`
}

// Term ...
type Term struct {
	ObjectIndex *ObjectIndex `@@ |`
	ArrayIndex  *ArrayIndex  `@@ |`
	Iterator    *Iterator    `@@ |`
	Identity    *Identity    `@@ |`
	Expression  *Expression  `@@`
}

// ObjectIndex ...
type ObjectIndex struct {
	Name     string `"." ( @Ident | "[" @String "]" )`
	Optional bool   `@"?"?`
}

// ArrayIndex ...
type ArrayIndex struct {
	Start *int `"." "[" ( @Integer?`
	End   *int `":" @Integer? |`
	Index *int `@Integer ) "]"`
}

// Iterator ...
type Iterator struct {
	_ string `"." "[" "]"`
}

// Identity ...
type Identity struct {
	_ string `"."`
}
