package gojq

// Query ...
type Query struct {
	Term *Term `@@`
}

// Term ...
type Term struct {
	Identity *Identity `@@`
}

// Identity ...
type Identity struct {
	X string `@"."`
}
