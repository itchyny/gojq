package gojq

// Expression ...
type Expression struct {
	Null  *Null  `  @@`
	True  *True  `| @@`
	False *False `| @@`
	Array *Array `| @@`
}

// Null ...
type Null struct {
	_ string `"null"`
}

// True ...
type True struct {
	_ string `"true"`
}

// False ...
type False struct {
	_ string `"false"`
}

// Array ...
type Array struct {
	Pipe *Pipe `"[" @@? "]"`
}
