package gojq

var builtinFuncs = map[string]string{
	"not":        `def not: if . then false else true end;`,
	"in":         `def in(xs): . as $x | xs | has($x);`,
	"map":        `def map(f): [.[] | f];`,
	"add":        `def add: reduce .[] as $x (null; . + $x);`,
	"to_entries": `def to_entries: [keys[] as $k | {key: $k, value: .[$k]}];`,
	"select":     `def select(f): if f then . else empty end;`,
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
	"range": `
		def range($x): range(0; $x);
		def range($start; $end):
			$start | while(. < $end; . + 1);
		def range($start; $end; $step):
			if $step > 0 then $start|while(. < $end; . + $step)
			elif $step < 0 then $start|while(. > $end; . + $step)
			else empty end;`,
	"arrays":    `def arrays: select(type == "array");`,
	"objects":   `def objects: select(type == "object");`,
	"iterables": `def iterables: select(type |. == "array" or . == "object");`,
	"booleans":  `def booleans: select(type == "boolean");`,
	"numbers":   `def numbers: select(type == "number");`,
	"strings":   `def strings: select(type == "string");`,
	"nulls":     `def nulls: select(. == null);`,
	"values":    `def values: select(. != null);`,
	"scalars":   `def scalars: select(type |. != "array" and . != "object");`,
	"reverse":   `def reverse: [.[length - 1 - range(0;length)]];`,
	"startswith": `
		def startswith($x):
			if type == "string" then
				.[:$x | length] == $x
			else
				_type_error("startswith")
			end;`,
	"endswith": `
		def endswith($x):
			if type == "string" then
				.[- ($x | length):] == $x
			else
				_type_error("endswith")
			end;`,
	"ltrimstr": `
		def ltrimstr($x):
			if type == "string" then
				if startswith($x) then .[$x | length:] end
			else
				_type_error("ltrimstr")
			end;`,
	"rtrimstr": `
		def rtrimstr($x):
			if type == "string" then
				if endswith($x) then .[:- ($x | length)] end
			else
				_type_error("rtrimstr")
			end;`,
	"combinations": `
		def combinations:
			if length == 0 then
				[]
			else
				.[0][] as $x | (.[1:] | combinations) as $y | [$x] + $y
			end;
		def combinations(n):
			. as $dot | [range(n) | $dot] | combinations;`,
	"ascii_downcase": `
		def ascii_downcase:
			explode | map(if 65 <= . and . <= 90 then . + 32 end) | implode;`,
	"ascii_upcase": `
		def ascii_upcase:
			explode | map(if 97 <= . and . <= 122 then . - 32 end) | implode;`,
}
