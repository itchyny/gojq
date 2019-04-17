package gojq

var builtinFuncs = map[string]string{
	"map": `def map(f): [.[] | f];`,
}
