package gojq

var builtinFuncs = map[string]string{
	"map":    `def map(f): [.[] | f];`,
	"select": `def select(f): if f then . else empty end;`,
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
			def _until: if cond then . else (next | _until) end;
			_until;`,
	"arrays":    `def arrays: select(type == "array");`,
	"objects":   `def objects: select(type == "object");`,
	"iterables": `def iterables: select(type |. == "array" or . == "object");`,
	"booleans":  `def booleans: select(type == "boolean");`,
	"numbers":   `def numbers: select(type == "number");`,
	"strings":   `def strings: select(type == "string");`,
	"nulls":     `def nulls: select(. == null);`,
	"values":    `def values: select(. != null);`,
	"scalars":   `def scalars: select(type |. != "array" and . != "object");`,
}
