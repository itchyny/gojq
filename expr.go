package gojq

// Expression ...
type Expression struct {
	Func   *Func   `  @@`
	Object *Object `| @@`
	Array  *Array  `| @@`
}

// Func ...
type Func struct {
	Name string `@Ident`
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
