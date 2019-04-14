package gojq

// Expression ...
type Expression struct {
	Null   *Null   `  @@`
	True   *True   `| @@`
	False  *False  `| @@`
	Object *Object `| @@`
	Array  *Array  `| @@`
}

// Null ...
type Null struct {
	_ bool `"null"`
}

// True ...
type True struct {
	_ bool `"true"`
}

// False ...
type False struct {
	_ bool `"false"`
}

// Object ...
type Object struct {
	KeyVals []*ObjKeyVal `"{" (@@ ("," @@)*)? "}"`
}

// ObjKeyVal ...
type ObjKeyVal struct {
	Key string `( @Ident | @String ) ":"`
	Val *Term  `@@`
}

// Array ...
type Array struct {
	Pipe *Pipe `"[" @@? "]"`
}
