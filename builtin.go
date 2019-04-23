package gojq

var builtinFuncs = map[string]string{
	"map": `def map(f): [.[] | f];`,
	"recurse": `
		def recurse: recurse(.[]?);
		def recurse(f): def r: ., (f | r); r;
		def recurse(f; cond): def r: ., (f | select(cond) | r); r;`,
	"while": `
		def while(cond; update):
			def _while: if cond then ., (update | _while) else empty end;
			_while;`,
	"until": `
		def until(cond; next):
			def _until: if cond then . else (next|_until) end;
			_until;`,
}
