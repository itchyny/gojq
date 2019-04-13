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

// Array ...
type Array struct {
	Pipe *Pipe `"[" @@? "]"`
}
