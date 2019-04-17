package gojq

var builtinFuncs = map[string]string{
	"map": `def map(f): [.[] | f];`,
	"recurse": `
		def recurse: recurse(.[]?);
		def recurse(f): def r: ., (f | r); r;
		def recurse(f; cond): def r: ., (f | select(cond) | r); r;`,
}
