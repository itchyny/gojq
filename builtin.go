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
	"flatten": `
		def _flatten($x): reduce .[] as $i ([]; if $i | type == "array" and $x != 0 then . + ($i | _flatten($x-1)) else . + [$i] end);
		def flatten($x): if $x < 0 then error("flatten depth must not be negative") else _flatten($x) end;
		def flatten: _flatten(-1);`,
	"min":    `def min: min_by(.);`,
	"min_by": `def min_by(f): reduce .[1:][] as $x (.[0]; if (.|f) > ($x|f) then $x end);`,
	"max":    `def max: max_by(.);`,
	"max_by": `def max_by(f): reduce .[1:][] as $x (.[0]; if (.|f) <= ($x|f) then $x end);`,
	"sort":   `def sort: sort_by(.);`,
	"sort_by": `
		def sort_by(f):
			def _sort_by:
				if length > 1 then
					.[0] as $x | .[1:] as $xs | ($x|[f]) as $fx
						| ([$xs[] | select([f] < $fx)] | _sort_by) + [$x] + ([$xs[] | select([f] >= $fx)] | _sort_by)
				end;
			_sort_by;`,
	"group_by": `
		def group_by(f):
			def _group_by:
				if length > 0 then
					.[0] as $x | .[1:] as $xs | ($x|[f]) as $fx
						| [$x, $xs[] | select([f] == $fx)], ([$xs[] | select([f] != $fx)] | _group_by)
				else
					empty
				end;
			sort_by(f) | [_group_by];`,
	"unique":    `def unique: group_by(.) | map(.[0]);`,
	"unique_by": `def unique_by(f): group_by(f) | map(.[0]);`,
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
	"indices": `
		def indices($x):
			if type == "array" and ($x|type) == "array" then .[$x]
			elif type == "array" then .[[$x]]
			elif type == "string" and ($x|type) == "string" then explode | .[$x|explode]
			else .[$x] end;`,
	"index":  `def index($x): indices($x) | .[0];`,
	"rindex": `def rindex($x): indices($x) | .[-1:][0];`,
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
	"join": `
		def join($x): reduce .[] as $i (null;
				(if . == null then "" else . + $x end) +
				($i | if type == "boolean" or type == "number" then tostring else . // "" end)
			) // "";`,
	"ascii_downcase": `
		def ascii_downcase:
			explode | map(if 65 <= . and . <= 90 then . + 32 end) | implode;`,
	"ascii_upcase": `
		def ascii_upcase:
			explode | map(if 97 <= . and . <= 122 then . - 32 end) | implode;`,
	"walk": `
		def walk(f):
			. as $in
				| if type == "object" then
						reduce keys[] as $key ({}; . + { ($key): ($in[$key] | walk(f)) }) | f
					elif type == "array" then
						map(walk(f)) | f
					else
						f
					end;`,
	"transpose": `
		def transpose:
			if . == [] then
				[]
			else
				. as $in
					| (map(length) | max) as $max
					| length as $length
					| reduce range(0; $max) as $j ([]; . + [reduce range(0; $length) as $i ([]; . + [ $in[$i][$j] ] )] )
			end;`,
	"first": `
		def first: .[0];
		def first(g): label $out | g | ., break $out;`,
	"last": `
		def last: .[-1];
		def last(g): reduce g as $item (null; $item);`,
	"isempty": `def isempty(g): first((g|false), true);`,
	"all": `
		def all: all(.[]; .);
		def all(y): all(.[]; y);
		def all(g; y): isempty(g|y and empty);`,
	"any": `
		def any: any(.[]; .);
		def any(y): any(.[]; y);
		def any(g; y): isempty(g|y or empty) | not;`,
	"limit": `
		def limit($n; g):
			if $n > 0 then
				label $out
					| foreach g as $item
						($n; .-1; $item, if . <= 0 then break $out else empty end)
			elif $n == 0 then
				empty
			else
				g
			end;`,
	"nth": `
		def nth($n): .[$n];
		def nth($n; g):
			if $n < 0 then
				error("nth doesn't support negative indices")
			else
				label $out
					| foreach g as $item
						($n; .-1; . < 0 or empty|$item, break $out)
			end;`,
}
