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
	Identity    *Identity    `@@ |`
	ObjectIndex *ObjectIndex `@@`
}

// Identity ...
type Identity struct {
	_ string `"."`
}

// ObjectIndex ...
type ObjectIndex struct {
	Name string `@ObjectIndex`
}
