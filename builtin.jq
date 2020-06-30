def not: if . then false else true end;
def in(xs): . as $x | xs | has($x);
def map(f): [.[] | f];
def to_entries: [keys[] as $k | {key: $k, value: .[$k]}];
def from_entries:
  map({ (.key // .Key // .name // .Name): (if has("value") then .value else .Value end) })
    | add | . //= {};
def with_entries(f): to_entries | map(f) | from_entries;
def select(f): if f then . else empty end;
def recurse: recurse(.[]?);
def recurse(f): def r: ., (f | r); r;
def recurse(f; cond): def r: ., (f | select(cond) | r); r;

def while(cond; update):
  def _while: if cond then ., (update | _while) else empty end;
  _while;
def until(cond; next):
  def _until: if cond then . else (next | _until) end;
  _until;
def repeat(f):
  def _repeat: f, _repeat;
  _repeat;
def range($x): range(0; $x);
def range($start; $end):
  $start | while(. < $end; . + 1);
def range($start; $end; $step):
  if $step > 0 then $start|while(. < $end; . + $step)
  elif $step < 0 then $start|while(. > $end; . + $step)
  else empty end;

def _flatten($x):
  reduce .[] as $i
    ( [];
      if $i | type == "array" and $x != 0
      then . + ($i | _flatten($x-1))
      else . + [$i]
      end);
def flatten($x):
  if $x < 0
  then error("flatten depth must not be negative")
  else _flatten($x) end;
def flatten: _flatten(-1);
def min: min_by(.);
def min_by(f): _min_by(map([f]));
def max: max_by(.);
def max_by(f): _max_by(map([f]));
def sort: sort_by(.);
def sort_by(f): _sort_by(map([f]));
def group_by(f): _group_by(map([f]));
def unique: unique_by(.);
def unique_by(f): _unique_by(map([f]));

def arrays: select(type == "array");
def objects: select(type == "object");
def iterables: select(type |. == "array" or . == "object");
def booleans: select(type == "boolean");
def numbers: select(type == "number");
def finites: select(isfinite);
def normals: select(isnormal);
def strings: select(type == "string");
def nulls: select(. == null);
def values: select(. != null);
def scalars: select(type |. != "array" and . != "object");
def leaf_paths: paths(scalars);

def indices($x):
  if type == "array" and ($x|type) == "array" then .[$x]
  elif type == "array" then .[[$x]]
  elif type == "string" and ($x|type) == "string" then explode | .[$x|explode]
  else .[$x] end;
def index($x): indices($x) | .[0];
def rindex($x): indices($x) | .[-1:][0];
def inside(xs): . as $x | xs | contains($x);
def startswith($x):
  if type == "string" then
    if $x|type == "string" then
      .[:$x | length] == $x
    else
      $x | _type_error("startswith")
    end
  else
    _type_error("startswith")
  end;
def endswith($x):
  if type == "string" then
    if $x|type == "string" then
      .[- ($x | length):] == $x
    else
      $x | _type_error("endswith")
    end
  else
    _type_error("endswith")
  end;
def ltrimstr($x):
  if type == "string" and ($x|type == "string") and startswith($x) then
    .[$x | length:]
  end;
def rtrimstr($x):
  if type == "string" and ($x|type == "string") and endswith($x) then
    .[:- ($x | length)]
  end;

def combinations:
  if length == 0 then
    []
  else
    .[0][] as $x | (.[1:] | combinations) as $y | [$x] + $y
  end;
def combinations(n):
  . as $dot | [range(n) | $dot] | combinations;
def join($x): reduce .[] as $i (null;
    (if . == null then "" else . + $x end) +
    ($i | if type == "boolean" or type == "number" then tostring else . // "" end)
  ) // "";
def ascii_downcase:
  explode | map(if 65 <= . and . <= 90 then . + 32 end) | implode;
def ascii_upcase:
  explode | map(if 97 <= . and . <= 122 then . - 32 end) | implode;
def walk(f):
  . as $in
    | if type == "object" then
        reduce keys[] as $key ({}; . + { ($key): ($in[$key] | walk(f)) }) | f
      elif type == "array" then
        map(walk(f)) | f
      else
        f
      end;
def transpose:
  if . == [] then
    []
  else
    . as $in
      | (map(length) | max) as $max
      | length as $length
      | reduce range(0; $max) as $j
          ([]; . + [reduce range(0; $length) as $i ([]; . + [ $in[$i][$j] ] )] )
  end;

def first: .[0];
def first(g): label $out | g | ., break $out;
def last: .[-1];
def last(g): reduce g as $item (null; $item);
def isempty(g): first((g|false), true);
def all: all(.[]; .);
def all(y): all(.[]; y);
def all(g; y): isempty(g|y and empty);
def any: any(.[]; .);
def any(y): any(.[]; y);
def any(g; y): isempty(g|y or empty) | not;
def limit($n; g):
  if $n > 0 then
    label $out
      | foreach g as $item
        ($n; .-1; $item, if . <= 0 then break $out else empty end)
  elif $n == 0 then
    empty
  else
    g
  end;
def nth($n): .[$n];
def nth($n; g):
  if $n < 0 then
    error("nth doesn't support negative indices")
  else
    label $out
      | foreach g as $item
        ($n; .-1; . < 0 or empty|$item, break $out)
  end;

def truncate_stream(f):
  . as $n | null | f | . as $input
    | if (.[0] | length) > $n then setpath([0]; $input[0][$n:]) else empty end;
def fromstream(f):
  { x: null, e: false } as $init
    | foreach f as $i
      ( $init;
        if .e then $init else . end
        | if $i | length == 2
          then setpath(["e"]; $i[0] | length==0) | setpath(["x"] + $i[0]; $i[1])
          else setpath(["e"]; $i[0] | length==1) end;
        if .e then .x else empty end);
def tostream:
  path(def r: (.[]? | r), .; r) as $p
    | getpath($p)
    | reduce path(.[]?) as $q ([$p, .]; [$p + $q]);

def _assign(ps; $v):
  reduce path(ps) as $p (.; setpath($p; $v));
def _modify(ps; f):
  reduce path(ps) as $p
    ([., []]; label $out | (([0] + $p) as $q | setpath($q; getpath($q) | f) | ., break $out), .[1] += [$p])
      | . as $x | $x[0] | delpaths($x[1]);
def map_values(f): .[] |= f;
def del(f): delpaths([path(f)]);
def paths:
  path(recurse(if (type | . == "array" or . == "object") then .[] else empty end))
    | select(length > 0);
def paths(f):
  . as $x | paths | select(. as $p | $x | getpath($p) | f);

def fromdateiso8601: strptime("%Y-%m-%dT%H:%M:%SZ") | mktime;
def todateiso8601: strftime("%Y-%m-%dT%H:%M:%SZ");
def fromdate: fromdateiso8601;
def todate: todateiso8601;

def match($re): match($re; null);
def match($re; $flags): _match($re; $flags; false) | .[];
def test($re): test($re; null);
def test($re; $flags): _match($re; $flags; true);
def capture($re): capture($re; null);
def capture($re; $flags):
  match($re; $flags)
    | [.captures | .[] | select(.name != null) | { (.name): .string }]
    | add // {};
def scan($re): scan($re; null);
def scan($re; $flags):
  match($re; "g" + $flags)
    | if (.captures|length > 0) then [ .captures | .[] | .string ] else .string end;
def splits($re): splits($re; null);
def splits($re; $flags): split($re; $flags) | .[];
def sub($re; str): sub($re; str; null);
def sub($re; str; $flags):
  . as $in
    | reduce match($re; $flags) as $r
      ( ["", 0];
        . as $x
          | [$r | .captures | .[] | select(.name != null) | { (.name) : .string }]
          | add // {}
          | [$x[0] + $in[$x[1]:$r.offset] + str, $r.offset + $r.length] )
    | .[0] + $in[.[1]:];
def gsub($re; str): sub($re; str; "g");
def gsub($re; str; $flags): sub($re; str; $flags + "g");

def inputs:
  try
    repeat(input)
  catch
    if . == "break" then empty else error end;

def INDEX(stream; idx_expr):
  reduce stream as $row ({}; .[$row|idx_expr|tostring] = $row);
def INDEX(idx_expr): INDEX(.[]; idx_expr);
def JOIN($idx; idx_expr):
  [.[] | [., $idx[idx_expr]]];
def JOIN($idx; stream; idx_expr):
  stream | [., $idx[idx_expr]];
def JOIN($idx; stream; idx_expr; join_expr):
  stream | [., $idx[idx_expr]] | join_expr;
def IN(s): any(s == .; .);
def IN(src; s): any(src == s; .);
