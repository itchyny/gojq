package gojq

// Query ...
type Query struct {
	Term *Term `@@`
}

// Term ...
type Term struct {
	Identity    *Identity    `@@ |`
	ObjectIndex *ObjectIndex `@@`
}

// Identity ...
type Identity struct {
	X string `@"."`
}

// ObjectIndex ...
type ObjectIndex struct {
	Name string `@ObjectIndex`
}
