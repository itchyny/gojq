package gojq

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

type freshNestedIter struct {
	remaining int
}

func (iter *freshNestedIter) Next() (any, bool) {
	if iter.remaining == 0 {
		return nil, false
	}
	iter.remaining--
	return []any{make([]any, 2000)}, true
}

// with MaxAlloc set, a memory bomb must error instead of allocating unboundedly.
func TestMaxAllocStopsBombs(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20 // 4 MiB

	for _, src := range []string{
		`"x" * 2000000000`,                      // string repeat
		`"xx"` + strings.Repeat(" | . + .", 30), // doubling chain
		`[range(100000000)]`,                    // huge array
		`null | setpath([50000000]; 1)`,         // huge sparse array
		`reduce range(60) as $i ("xx"; . + .)`,  // growing reduce
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		iter, gotErr := code.Run(nil), false
		for i := 0; i < 1000000; i++ {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if _, ok := v.(error); ok {
				gotErr = true
				break
			}
		}
		if !gotErr {
			t.Errorf("expected an allocation error for %q", src)
		}
	}
}

// with MaxAlloc set, ordinary expressions must still succeed.
func TestMaxAllocAllowsNormal(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{`"ab" * 3`, `[range(100)] | add`, `{a: 1, b: 2}`, `. + 1`} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(3).Next()
		if !ok {
			t.Errorf("%q: no result", src)
		} else if e, ok := v.(error); ok {
			t.Errorf("%q: unexpected error: %s", src, e)
		}
	}
}

// each of these builtins used to build an input-proportional value in one make()
// before the value meter could charge it. with MaxAlloc set they must now error.
func TestMaxAllocStopsSingleShotBuiltins(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 1 << 20 // 1 MiB

	bigStr := strings.Repeat("a", 1<<20)
	bigArr := make([]any, 1<<20)
	for i := range bigArr {
		bigArr[i] = float64(i)
	}
	for _, c := range []struct {
		src string
		in  any
	}{
		{`explode`, bigStr},   // string -> []any (x16)
		{`. / ""`, bigStr},    // split operator on empty separator
		{`split("")`, bigStr}, // split builtin
		{`reverse`, bigArr},   // input-sized array copy
		{`sort`, bigArr},      // sortItems make
		{`unique`, bigArr},    // via sort
	} {
		query, err := Parse(c.src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", c.src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", c.src, err)
		}
		v, ok := code.Run(c.in).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", c.src)
		} else if _, isErr := v.(error); !isErr {
			t.Errorf("%q: expected an allocation error, got a value", c.src)
		}
	}
}

// Every native callback result goes through the VM's ownership-aware result
// meter, including callbacks added in the future and custom callbacks unknown
// to the builtin-name switch. A shallow-only meter sees just the one-element
// outer array below and misses the fresh 2,000-slot child on every call.
func TestMaxAllocMetersEveryNativeReturn(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 1 << 20 // 1 MiB

	query, err := Parse(`[range(1000) | fresh_nested] | length`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query, WithFunction("fresh_nested", 0, 0, func(any, []any) any {
		return []any{make([]any, 2000)}
	}))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := code.Run(nil).Next()
	if !ok {
		t.Fatal("expected an allocation error, got no result")
	}
	if _, isAlloc := v.(*allocLimitError); !isAlloc {
		t.Errorf("expected *allocLimitError for an unknown nested producer, got %v", v)
	}

	iterQuery, err := Parse(`[fresh_nested_iter] | length`)
	if err != nil {
		t.Fatal(err)
	}
	iterCode, err := Compile(iterQuery, WithIterFunction("fresh_nested_iter", 0, 0, func(any, []any) Iter {
		return &freshNestedIter{remaining: 1000}
	}))
	if err != nil {
		t.Fatal(err)
	}
	v, ok = iterCode.Run(nil).Next()
	if !ok {
		t.Fatal("expected an iterator allocation error, got no result")
	}
	if _, isAlloc := v.(*allocLimitError); !isAlloc {
		t.Errorf("expected *allocLimitError for an unknown iterator producer, got %v", v)
	}
}

// Values pulled from input(s) belong to the caller, like the root value passed
// to Run, and must not consume the query's allocation budget merely by crossing
// the native-call return boundary.
func TestMaxAllocLeavesPulledInputFree(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 1 << 20 // 1 MiB

	pulled := make([]any, 100000) // larger than MaxAlloc by allocSize
	query, err := Parse(`input`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query, WithInputIter(NewIter(pulled)))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := code.Run(nil).Next()
	if !ok {
		t.Fatal("expected the pulled input")
	}
	if _, isErr := v.(error); isErr {
		t.Fatalf("pulled input was charged as query output: %v", v)
	}
	if got, isArray := v.([]any); !isArray || len(got) != len(pulled) {
		t.Fatalf("unexpected pulled input: %T len=%d", v, len(got))
	}
}

// deep or infinite recursion grows the interpreter's own stacks (operands,
// forks, scopes) without ever completing a value, so the value meter never
// charges it. and a big.Int can grow past the value meter because its size is
// not the size of the interface that holds it. both must error under MaxAlloc.
func TestMaxAllocStopsRecursionAndBigint(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	squareChain := "."
	for i := 0; i < 30; i++ {
		squareChain += " | (. * .)"
	}
	for _, c := range []struct {
		src string
		in  any
	}{
		{`def f: [f]; f`, nil}, // infinite nesting recursion
		{squareChain, 3},       // big.Int doubling by repeated squaring
	} {
		query, err := Parse(c.src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", c.src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", c.src, err)
		}
		v, ok := code.Run(c.in).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", c.src)
		} else if c.src == `def f: [f]; f` {
			switch v.(type) {
			case *stackLimitError, *allocLimitError:
			default:
				t.Errorf("%q: expected a stack or allocation limit error, got %v", c.src, v)
			}
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q: expected *allocLimitError, got %v", c.src, v)
		}
	}
}

// a value in a []any slot costs 16 bytes for the slot plus its own footprint,
// so an array of a million one-character strings is ~33 MB, not the ~1 MB its
// contents suggest. before charging the slot, such an array grew far past the
// limit (and `add` then churned 150+ MB the meter never saw). it must now error.
func TestMaxAllocCountsArraySlots(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`[range(1000000) | tostring]`,       // a million tiny strings
		`[range(1000000) | tostring] | add`, // ... then concatenated
		`[range(1000000)]`,                  // a million boxed numbers
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(nil).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isErr := v.(error); !isErr {
			t.Errorf("%q: expected an allocation error, got a value", src)
		}
	}
}

// fromjson builds a whole structure from one json.Decode; a 2 MB array string
// of a million ones would materialize ~16 MB before the value meter saw it.
// the charging streaming decoder must stop it during the parse.
func TestMaxAllocStopsFromJSON(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	src := "[" + strings.Repeat("1,", 999999) + "1]"
	query, err := Parse(`fromjson`)
	if err != nil {
		t.Fatalf("Parse: %s", err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatalf("Compile: %s", err)
	}
	v, ok := code.Run(src).Next()
	if !ok {
		t.Fatal("expected an error, got no result")
	}
	if _, isAlloc := v.(*allocLimitError); !isAlloc {
		t.Errorf("expected *allocLimitError, got %v", v)
	}
}

// a recursive function that binds many variables or takes many arguments grows
// the interpreter's value-slot slice (env.values) far faster than its scope
// stack, so the slot slice must be part of the live stack limit too.
func TestMaxAllocStopsValueSlotGrowth(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	var binds strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&binds, ". as $a%d | ", i)
	}
	for _, src := range []string{
		"def f: " + binds.String() + "[f]; f",                                    // many bindings per level
		"def f($a;$b;$c;$d;$e;$f;$g;$h): f(.;.;.;.;.;.;.;.); f(.;.;.;.;.;.;.;.)", // many args
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse: %s", err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile: %s", err)
		}
		v, ok := code.Run("x").Next()
		if !ok {
			t.Errorf("expected an error, got no result")
		} else {
			switch v.(type) {
			case *stackLimitError, *allocLimitError:
			default:
				t.Errorf("expected a stack or allocation limit error, got %v", v)
			}
		}
	}
}

// compileRegexp caches every distinct pattern; a run generating millions of
// them once grew the cache to 70-95 MB (and it persisted after the run). the
// cache must stop retaining once it reaches MaxAlloc, while patterns still work.
func TestRegexpCacheBounded(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 1 << 20 // 1 MiB

	var cache reCache
	for i := 0; i < 50000; i++ {
		if _, err := compileRegexp(fmt.Sprintf("distinct-pattern-%d", i), "", &cache); err != nil {
			t.Fatalf("compile %d: %s", i, err)
		}
	}
	if got := cache.bytes.Load(); got > MaxAlloc+4096 {
		t.Errorf("cache retained %d bytes for 50000 patterns, want bounded near MaxAlloc (%d)", got, int64(MaxAlloc))
	}
	// still correct after the cache filled: a pattern compiles and matches.
	r, err := compileRegexp("^abc$", "", &cache)
	if err != nil || !r.MatchString("abc") || r.MatchString("xyz") {
		t.Errorf("regex broken after cache filled: r=%v err=%v", r, err)
	}
}

// match with the "g" flag builds one map per match (plus one per capture group)
// in a single make, and the result array is charged only shallowly, so a global
// match on a moderate string materialized tens of MB under the limit. it must
// now error, while a small match still returns correct results.
func TestMaxAllocBoundsGlobalMatch(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := strings.Repeat("ab", 500000)
	query, err := Parse(`[match("(a)(b)"; "g")] | length`)
	if err != nil {
		t.Fatalf("Parse: %s", err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatalf("Compile: %s", err)
	}
	v, ok := code.Run(big).Next()
	if !ok {
		t.Fatal("expected an error, got no result")
	}
	if _, isAlloc := v.(*allocLimitError); !isAlloc {
		t.Errorf("expected *allocLimitError for a huge global match, got %v", v)
	}

	// a small match must still return its results under the same limit.
	query2, _ := Parse(`[match("a"; "g") | .offset]`)
	code2, _ := Compile(query2)
	v2, ok := code2.Run("banana").Next()
	if !ok {
		t.Fatal("no result for small match")
	}
	if got, isArr := v2.([]any); !isArr || len(got) != 3 {
		t.Errorf("small match: expected 3 offsets, got %v", v2)
	}
}

// the gas meter must not count transient scalars: a streaming query that
// produces millions of numbers but holds one accumulator (a counting reduce)
// would otherwise trip the limit though it holds almost nothing live. it must
// complete with the right value, while collecting the same range still errors.
func TestMaxAllocAllowsStreaming(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	query, err := Parse(`reduce range(2000000) as $x (0; . + 1)`)
	if err != nil {
		t.Fatalf("Parse: %s", err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatalf("Compile: %s", err)
	}
	v, ok := code.Run(nil).Next()
	if !ok {
		t.Fatal("counting reduce: no result")
	}
	if e, isErr := v.(error); isErr {
		t.Fatalf("counting reduce should complete, got error: %s", e)
	}
	if got := fmt.Sprint(v); got != "2000000" {
		t.Errorf("counting reduce: got %v, want 2000000", v)
	}

	// collecting the same range into an array retains it, so it must still error.
	q2, _ := Parse(`[range(2000000)] | length`)
	c2, _ := Compile(q2)
	v2, _ := c2.Run(nil).Next()
	if _, isAlloc := v2.(*allocLimitError); !isAlloc {
		t.Errorf("collecting 2M elements should error, got %v", v2)
	}
}

// fromjson yields json.Number values, a distinct type from string. allocSize and
// the streaming decoder must size them by their digit length, or a JSON document
// of a few huge numbers slips through the parse bound (each number charged as a
// 16-byte scalar) and downstream ops run on tens of MB of unmetered data.
func TestMaxAllocSizesJSONNumbers(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < 30; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(strings.Repeat("9", 500000)) // a 500,000-digit number
	}
	b.WriteString("]")

	query, err := Parse(`fromjson | add | tostring`)
	if err != nil {
		t.Fatalf("Parse: %s", err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatalf("Compile: %s", err)
	}
	v, ok := code.Run(b.String()).Next()
	if !ok {
		t.Fatal("expected an error, got no result")
	}
	if _, isAlloc := v.(*allocLimitError); !isAlloc {
		t.Errorf("fromjson of ~15 MB of numbers should error, got %v", v)
	}

	// a small number document still decodes and sums correctly.
	q2, _ := Parse(`fromjson | add`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run("[1,2,3,4]").Next(); !ok || fmt.Sprint(v2) != "10" {
		t.Errorf("small fromjson: expected 10, got %v", v2)
	}
}

// index/indices/rindex on a string explode the whole haystack to a []any of
// runes (x16); on a big string that materialized tens of MB the meter never saw
// because the result is one number. indices on an array builds its result via an
// internal append. both must be bounded, while small inputs still work.
func TestMaxAllocBoundsIndex(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	bigStr := strings.Repeat("hello world ", 400000) // ~4.8 MB -> ~76 MB of runes
	for _, src := range []string{`index("world")`, `indices("o")`, `rindex("d")`} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(bigStr).Next()
		if !ok {
			t.Errorf("%q: no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q on a big string: expected an allocation error, got %v", src, v)
		}
	}

	all := make([]any, 1000000)
	for i := range all {
		all[i] = float64(5)
	}
	query, _ := Parse(`indices(5)`)
	code, _ := Compile(query)
	if v, _ := code.Run(all).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
		t.Errorf("indices on an all-matches array: expected an allocation error, got a value")
	}

	q2, _ := Parse(`index("world")`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run("hello world").Next(); !ok || fmt.Sprint(v2) != "6" {
		t.Errorf("small index: expected 6, got %v", v2)
	}
}

// shared references (`[., .]` repeated) make a value whose own footprint is tiny
// expand to an exponentially larger JSON encoding. tojson / tostring / @json
// build the whole string before the meter, which only sees the result, can
// charge it, so a tiny expression produced gigabytes. the encoder must bound it.
func TestMaxAllocBoundsEncoding(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`reduce range(30) as $i (0; [., .]) | tojson`, // O(30) DAG, 2^30 expansion
		`reduce range(30) as $i (0; [., .]) | tostring`,
		`reduce range(25) as $i ({}; {a: ., b: .}) | tojson`,
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(nil).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q: expected an allocation error, got a value", src)
		}
	}

	// ordinary encoding still works.
	q2, _ := Parse(`tojson`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run([]any{1.0, "x", nil}).Next(); !ok || fmt.Sprint(v2) != `[1,"x",null]` {
		t.Errorf(`tojson: expected [1,"x",null], got %v`, v2)
	}
}

// A single string whose escaped form is far larger than the string itself
// (each control byte becomes \u00XX, six bytes) amplifies inside encodeString,
// which escapes the whole string in one uninterrupted loop. The between-values
// guard in encode never fires mid-string, so tojson / @json built the entire
// escaped blob (a 2 MB input reaching ~12 MB, peaking far higher through buffer
// doubling) before the value meter, seeing only the finished string, could
// charge it. The guard inside encodeString stops the escape past MaxAlloc.
func TestMaxAllocBoundsStringEscaping(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// 2 MB of control bytes: the input is under the limit, its escaping is not.
	ctrl := strings.Repeat(string(rune(1)), 2<<20)
	for _, src := range []string{`tojson`, `@json`} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(ctrl).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// a clean string under the limit still encodes: no escapes, guard never trips.
	q2, _ := Parse(`tojson`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(strings.Repeat("x", 1<<20)).Next(); !ok {
		t.Errorf("clean string: expected a result, got none")
	} else if _, isAlloc := v2.(*allocLimitError); isAlloc {
		t.Errorf("clean string under the limit should encode, got alloc error")
	}
}

// add accumulates the whole sum internally (a strings.Builder for strings, an
// append for arrays, a merge for maps) and is charged only at the end, so on a
// large input it built 1+ GB before the meter fired. bound the accumulation.
func TestMaxAllocBoundsAdd(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	bigStr := strings.Repeat("a", 200000)
	sharedStrs := make([]any, 2000)
	for i := range sharedStrs {
		sharedStrs[i] = bigStr
	}
	bigArr := make([]any, 50000)
	for i := range bigArr {
		bigArr[i] = float64(i)
	}
	sharedArrs := make([]any, 2000)
	for i := range sharedArrs {
		sharedArrs[i] = bigArr
	}
	for _, in := range []any{sharedStrs, sharedArrs} {
		query, _ := Parse(`add | length`)
		code, _ := Compile(query)
		if v, _ := code.Run(in).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
			t.Errorf("add of a large shared-reference input: expected an allocation error, got a value")
		}
	}

	// small add still returns the right values.
	for _, c := range []struct {
		src, in, want string
	}{
		{`add`, `[1,2,3,4]`, "10"},
		{`add`, `["a","b","c"]`, "abc"},
	} {
		q, _ := Parse(c.src)
		code, _ := Compile(q)
		var iv any
		jq, _ := Parse(c.in)
		jc, _ := Compile(jq)
		iv, _ = jc.Run(nil).Next()
		if v, ok := code.Run(iv).Next(); !ok || fmt.Sprint(v) != c.want {
			t.Errorf("add %s: expected %s, got %v", c.in, c.want, v)
		}
	}
}

// flatten builds its flat result with an internal append; keys / to_entries make
// an array of the input length. all are charged only at the end, so on a large
// input flatten alone hit 5.8 GB. they must error while small inputs still work.
func TestMaxAllocBoundsFlattenKeys(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	nested := make([]any, 20)
	for i := range nested {
		inner := make([]any, 20000)
		for j := range inner {
			inner[j] = float64(j)
		}
		nested[i] = inner
	}
	bigObj := map[string]any{}
	for i := 0; i < 400000; i++ {
		bigObj[fmt.Sprintf("k%d", i)] = float64(i)
	}
	for _, c := range []struct {
		src string
		in  any
	}{
		{`flatten`, nested},
		{`keys`, bigObj},
		{`to_entries`, bigObj},
	} {
		query, _ := Parse(c.src)
		code, _ := Compile(query)
		if v, _ := code.Run(c.in).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
			t.Errorf("%q on a large input: expected an allocation error, got a value", c.src)
		}
	}

	q, _ := Parse(`flatten`)
	code, _ := Compile(q)
	if v, ok := code.Run([]any{[]any{1.0, 2.0}, []any{3.0}}).Next(); !ok || fmt.Sprint(v) != "[1 2 3]" {
		t.Errorf("flatten: expected [1 2 3], got %v", v)
	}
}

// .[] (opiter) builds a []pathValue of the input length to iterate, unmetered,
// so `.[] | select(false)` on a big map completed blind at 167 MB. transpose
// makes input-sized arrays. both must be bounded, iteration still correct.
func TestMaxAllocBoundsIterTranspose(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	bigMap := map[string]any{}
	for i := 0; i < 300000; i++ {
		bigMap[fmt.Sprintf("k%d", i)] = float64(i)
	}
	bigArr := make([]any, 300000)
	for i := range bigArr {
		bigArr[i] = float64(i)
	}
	row := func() []any {
		r := make([]any, 300000)
		for j := range r {
			r[j] = float64(j)
		}
		return r
	}
	matrix := []any{row(), row()}
	for _, c := range []struct {
		src string
		in  any
	}{
		{`.[] | select(false)`, bigMap}, // was 167 MB, meter blind
		{`.[] | select(false)`, bigArr},
		{`transpose`, matrix},
	} {
		query, _ := Parse(c.src)
		code, _ := Compile(query)
		v, ok := code.Run(c.in).Next()
		if !ok {
			t.Errorf("%q on a large input: expected an error, completed with no result", c.src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q on a large input: expected an allocation error, got %v", c.src, v)
		}
	}

	// small iteration and transpose still correct.
	q, _ := Parse(`[.[]]`)
	code, _ := Compile(q)
	if v, ok := code.Run(map[string]any{"b": 2.0, "a": 1.0}).Next(); !ok || fmt.Sprint(v) != "[1 2]" {
		t.Errorf("[.[]] on object: expected [1 2], got %v", v)
	}
	q2, _ := Parse(`transpose`)
	c2, _ := Compile(q2)
	if v, ok := c2.Run([]any{[]any{1.0, 2.0}, []any{3.0, 4.0}}).Next(); !ok || fmt.Sprint(v) != "[[1 3] [2 4]]" {
		t.Errorf("transpose: expected [[1 3] [2 4]], got %v", v)
	}
}

// object/array merge operators (. + . shallow, . * . deep) build the merged
// result internally before the opcall charges it; on a big input object they hit
// 160 MB. the flat makes and deepMergeObjects (recursive) must be bounded.
func TestMaxAllocBoundsMerge(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := map[string]any{}
	for i := 0; i < 1000000; i++ {
		big[fmt.Sprint(i)] = float64(i)
	}
	deep := any(map[string]any{"v": 0.0})
	for i := 0; i < 300000; i++ {
		deep = map[string]any{"a": deep}
	}
	for _, c := range []struct {
		src string
		in  any
	}{
		{`. + .`, big},          // shallow object merge
		{`. * .`, big},          // deep object merge, wide
		{`. * {"new": 1}`, big}, // deep merge copies the big object
		{`. * .`, deep},         // deep merge, deep
	} {
		query, _ := Parse(c.src)
		code, _ := Compile(query)
		if v, _ := code.Run(c.in).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
			t.Errorf("%q on a big object: expected an allocation error, got a value", c.src)
		}
	}

	// small merges still correct.
	for _, c := range []struct{ src, in, want string }{
		{`. + {"b": 2}`, `{"a": 1}`, "map[a:1 b:2]"},
		{`. * {"a": {"y": 2}}`, `{"a": {"x": 1}}`, "map[a:map[x:1 y:2]]"},
	} {
		q, _ := Parse(c.src)
		code, _ := Compile(q)
		var iv any
		jq, _ := Parse(c.in)
		jc, _ := Compile(jq)
		iv, _ = jc.Run(nil).Next()
		if v, ok := code.Run(iv).Next(); !ok || fmt.Sprint(v) != c.want {
			t.Errorf("%q: expected %s, got %v", c.src, c.want, v)
		}
	}
}

// array subtraction (. - r) pre-allocates a result with cap len(l); on a big
// input array that upfront allocation must be bounded.
func TestMaxAllocBoundsSubtract(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := make([]any, 2000000)
	for i := range big {
		big[i] = float64(i)
	}
	query, _ := Parse(`. - [1]`)
	code, _ := Compile(query)
	if v, _ := code.Run(big).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
		t.Errorf("array subtract on a big input: expected an allocation error, got a value")
	}

	// small subtraction still correct.
	q, _ := Parse(`. - [2, 3]`)
	c, _ := Compile(q)
	if v, ok := c.Run([]any{1.0, 2.0, 3.0, 4.0}).Next(); !ok || fmt.Sprint(v) != "[1 4]" {
		t.Errorf("subtract: expected [1 4], got %v", v)
	}
}

// @csv/@tsv/@sh build a []string of the whole row width (formatJoin) before the
// opcall charges the joined result; on a big row array that make is unbounded.
func TestMaxAllocBoundsFormatJoin(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	row := make([]any, 2000000)
	for i := range row {
		row[i] = float64(i)
	}
	for _, src := range []string{`@csv`, `@tsv`, `@sh`} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, _ := code.Run(row).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
			t.Errorf("%s on a big row: expected an allocation error, got a value", src)
		}
	}

	// small rows still format correctly.
	q, _ := Parse(`@csv`)
	code, _ := Compile(q)
	if v, ok := code.Run([]any{1.0, "a,b", true, nil}).Next(); !ok || fmt.Sprint(v) != `1,"a,b",true,` {
		t.Errorf("@csv: got %v", v)
	}
}

// @csv / @tsv / @sh escape each string field ( a " becomes "" for csv , a '
// becomes the four bytes \'\\'\' for sh ) through a strings.Replacer that built
// the whole escaped field in one pass. A single quote-heavy field whose escaping
// is several times its own size therefore materialized fully before the value
// meter, seeing only the finished row, could charge it: a 4 MB field reached tens
// of MB and scaled linearly with input ( a 40 MB field peaked near 230 MB ), all
// under a 4 MB limit. boundedReplace escapes in chunks and stops past MaxAlloc.
func TestMaxAllocBoundsFormatFieldEscaping(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// a single field whose escaped form is far larger than the field itself.
	for _, tc := range []struct {
		src string
		in  any
	}{
		{`@csv`, []any{strings.Repeat(string(rune(34)), 3<<20)}}, // 3 MB of " -> ~6 MB
		{`@sh`, []any{strings.Repeat("'", 3<<20)}},               // 3 MB of ' -> ~12 MB
	} {
		query, _ := Parse(tc.src)
		code, _ := Compile(query)
		if v, ok := code.Run(tc.in).Next(); !ok {
			t.Errorf("%s: expected an error, got no result", tc.src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%s on a huge quote-heavy field: expected an allocation error, got %v", tc.src, v)
		}
	}

	// a clean field under the limit must still format: escaping does not expand it,
	// so the chunked check never trips ( no false positive on large clean content ).
	q, _ := Parse(`@csv`)
	code, _ := Compile(q)
	if v, ok := code.Run([]any{strings.Repeat("x", 2<<20)}).Next(); !ok {
		t.Errorf("clean field: expected a result, got none")
	} else if _, isAlloc := v.(*allocLimitError); isAlloc {
		t.Errorf("clean 2 MB field under the limit should format, got alloc error")
	}
}

// @html / @uri / @base64 each built the whole escaped or encoded output in one
// pass ( htmlEscaper.Replace up to 6x for ' -> &apos; , url.QueryEscape up to 3x,
// base64 4/3x ) with no in-loop bound, so a big input scaled the peak linearly
// ( @html on 40 MB reached ~460 MB ) under a 4 MB limit. @html/@uri now escape in
// chunks ( boundedReplace ) ; @base64 pre-checks its deterministic encoded length.
func TestMaxAllocBoundsFormatEscapers(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, tc := range []struct {
		src string
		in  string
	}{
		{`@html`, strings.Repeat("'", 3<<20)},             // ' -> &apos; (6x)
		{`@uri`, strings.Repeat(string(rune(34)), 3<<20)}, // " -> %22 (3x)
		{`@base64`, strings.Repeat("A", 4<<20)},           // 4/3x, over the limit
	} {
		query, _ := Parse(tc.src)
		code, _ := Compile(query)
		if v, ok := code.Run(tc.in).Next(); !ok {
			t.Errorf("%s: expected an error, got no result", tc.src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%s on a huge input: expected an allocation error, got %v", tc.src, v)
		}
	}

	// benign inputs still produce the exact escaping/encoding.
	for _, tc := range []struct{ src, in, want string }{
		{`@html`, "a<b>&'x", "a&lt;b&gt;&amp;&apos;x"},
		{`@uri`, "a b/c", "a%20b%2Fc"},
		{`@base64`, "hello", "aGVsbG8="},
	} {
		query, _ := Parse(tc.src)
		code, _ := Compile(query)
		if v, ok := code.Run(tc.in).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%s: got %v, want %s", tc.src, v, tc.want)
		}
	}

	// a clean large field does not expand, so it must still format ( no false positive ).
	q, _ := Parse(`@html`)
	code, _ := Compile(q)
	if v, ok := code.Run(strings.Repeat("x", 3<<20)).Next(); !ok {
		t.Errorf("clean @html: expected a result")
	} else if _, isAlloc := v.(*allocLimitError); isAlloc {
		t.Errorf("clean 3 MB @html should format, got alloc error")
	}
}

// implode builds a string from a codepoint array through a strings.Builder that
// is bounded per rune, but sb.Grow(len(vs)) pre-allocated len(vs) bytes upfront,
// so a huge input array forced that allocation before the per-rune check could
// fire ( a 60M-element array reserved 60 MB under a 4 MB limit ). The Grow is now
// capped at MaxAlloc; the per-rune check still bounds the actual writes.
func TestMaxAllocBoundsImplode(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// 2M four-byte codepoints -> an ~8 MB string, over the limit.
	big := make([]any, 2000000)
	for i := range big {
		big[i] = float64(0x10000)
	}
	query, _ := Parse(`implode`)
	code, _ := Compile(query)
	if v, _ := code.Run(big).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
		t.Errorf("implode on a huge codepoint array: expected an allocation error, got a value")
	}

	// a small implode still produces the exact string.
	if v, ok := code.Run([]any{float64(104), float64(105)}).Next(); !ok || fmt.Sprint(v) != "hi" {
		t.Errorf(`implode: got %v, want hi`, v)
	}
}

// setpath builds the spine of nested containers toward the target, one
// makeArray / makeObject per path element. A path of increasing indices
// [0,1,...,N-1] pads the array at each depth to its index, so the whole structure
// holds 1+2+...+N = N^2/2 slots while every single array stays small ( never
// tripping the per-array guard ) and the shallow return-charge counts only the top
// level. setpath([range(3000)];1) completed at 74 MB and [range(8000)] at 514 MB
// under a 4 MB limit. A cumulative counter threaded through update now bounds the
// whole spine build.
func TestMaxAllocBoundsSetpathPadding(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`setpath([range(8000)]; 1)`,    // quadratic array padding, many small arrays
		`setpath([200000, 200000]; 1)`, // two arrays each under the limit, cumulatively over
	} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		// setpath wraps the allocLimitError, so match the message not the type.
		if v, ok := code.Run(nil).Next(); !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if !strings.Contains(fmt.Sprint(v), "allocation exceeds") {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// benign setpath / assignment still build the exact structure.
	for _, tc := range []struct{ src, want string }{
		{`setpath(["a", "b"]; 5)`, `{"a":{"b":5}}`},
		{`[1, 2, 3, 4] | setpath([2]; 9)`, `[1,2,9,4]`},
		{`{} | .a.b.c = 7`, `{"a":{"b":{"c":7}}}`},
		{`[1, 2, 3] | del(.[1])`, `[1,3]`},
	} {
		query, _ := Parse(tc.src + " | tojson")
		code, _ := Compile(query)
		if v, ok := code.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// strftime / strflocaltime format a time with a format string taken as an
// argument, which can come from input ( strftime(.field) ). timefmt.Format builds
// the whole result in one pass and directives such as %A / %B / %c expand a
// two-byte directive to many bytes, so a big input format amplified the output far
// past the limit ( a 40 MB format of %A reached 358 MB ) before the value meter,
// seeing only the finished string, could charge it. A format-length pre-check now
// rejects a format whose worst-case expansion could exceed MaxAlloc.
func TestMaxAllocBoundsStrftime(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	bigfmt := strings.Repeat("%A", 2<<20) // 4 MB of weekday directives
	for _, src := range []string{
		`. as $f | 1700000000 | gmtime | strftime($f)`,
		`. as $f | 1700000000 | gmtime | strflocaltime($f)`,
	} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, ok := code.Run(bigfmt).Next(); !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q on a huge format: expected an allocation error, got %v", src, v)
		}
	}

	// realistic formats still produce the exact string.
	q, _ := Parse(`1700000000 | gmtime | strftime("%Y-%m-%dT%H:%M:%SZ")`)
	code, _ := Compile(q)
	if v, ok := code.Run(nil).Next(); !ok || fmt.Sprint(v) != "2023-11-14T22:13:20Z" {
		t.Errorf(`strftime: got %v, want 2023-11-14T22:13:20Z`, v)
	}
}

// indices / index / rindex compared xs against vs[i:i+len(xs)] on every position,
// which boxed a fresh slice header into Compare each step. Over a large array that
// was tens of MB of transient garbage for even a single match ( indices(5) on a 2M
// array peaked at 52 MB and scaled with input ), all invisible to the meter since
// the result is tiny. matchAt now compares element by element ( no reslice, no box ).
func TestMaxAllocBoundsIndices(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// many matches still hit the result cap.
	big := make([]any, 2000000)
	for i := range big {
		big[i] = float64(1)
	}
	query, _ := Parse(`indices(1)`)
	code, _ := Compile(query)
	if v, _ := code.Run(big).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
		t.Errorf("indices with 2M matches: expected an allocation error, got a value")
	}

	// the element-wise comparison preserves exact results.
	for _, tc := range []struct {
		src  string
		in   any
		want string
	}{
		{`indices(2)`, []any{1.0, 2.0, 3.0, 2.0, 2.0}, `[1,3,4]`},
		{`index(2)`, []any{1.0, 2.0, 3.0}, `1`},
		{`rindex(2)`, []any{1.0, 2.0, 3.0, 2.0}, `3`},
		{`indices([1, 2])`, []any{0.0, 1.0, 2.0, 1.0, 2.0}, `[1,3]`},
		{`indices("a")`, "banana", `[1,3,5]`},
	} {
		q, _ := Parse(tc.src + " | tojson")
		c, _ := Compile(q)
		if v, ok := c.Run(tc.in).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%s: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// jsonMarshal backs Error()/String() formatting and runs the encode guard, which
// panics once the buffer passes MaxAlloc. Unlike Marshal it did not recover that
// panic, so formatting a halt_error / error that carries a big non-string value
// ( e.g. halt_error on a large input array ) crashed the process with an uncaught
// panic. jsonMarshal now recovers and returns the bounded prefix.
func TestMaxAllocErrorFormattingBounded(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := make([]any, 2000000)
	for i := range big {
		big[i] = float64(i)
	}
	for _, src := range []string{`error`, `halt_error`, `halt_error(1)`} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		v, ok := code.Run(big).Next()
		if !ok {
			t.Fatalf("%q: expected an error value", src)
		}
		err, isErr := v.(error)
		if !isErr {
			t.Fatalf("%q: expected an error, got %T", src, v)
		}
		msg := err.Error() // must not panic
		if int64(len(msg)) > MaxAlloc+64 {
			t.Errorf("%q: error message len %d exceeds the limit", src, len(msg))
		}
	}
}

// Encoding a big integer to base 10 ( tojson / tostring / @json / interpolation )
// runs v.Append(buf, 10), which allocates large superlinear scratch inside
// math/big: a 4 MB integer peaks past 100 MB , none of it seen by the encoder's
// between-values guard. A ~512 KB integer builds fine ( under the limit ) yet its
// decimal form costs ~25 MB , so serializing it is now refused.
func TestMaxAllocBoundsBigintEncoding(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`(reduce range(22) as $x (2; . * .)) | tojson`,
		`(reduce range(22) as $x (2; . * .)) | tostring`,
		`(reduce range(22) as $x (2; . * .)) | @json`,
	} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, ok := code.Run(nil).Next(); !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// ordinary numbers, including a moderately large big integer, still serialize.
	for _, tc := range []struct{ src, want string }{
		{`12345 | tojson`, `12345`},
		{`(2 | . * . * .) | tojson`, `8`},
		{`(reduce range(10) as $x (2; . * .)) | tojson | length > 0`, `true`},
	} {
		q, _ := Parse(tc.src)
		c, _ := Compile(q)
		if v, ok := c.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// bigToFloat converted a big integer to float64 via strconv.ParseFloat(x.String()),
// and x.String() runs the same superlinear base-10 conversion as round 78: a 10 MB
// integer divided by a small number reached ~300 MB inside math/big, unseen by the
// meter. Division falls back to bigToFloat when not evenly divisible, and mixed
// int/float arithmetic on a big integer goes through it too. It now converts via
// big.Float ( binary , bounded , correctly rounded ).
func TestMaxAllocBigToFloatBounded(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// a big integer built by squaring, divided by a small number, exercises the
	// bigToFloat fallback and must complete without erroring or blowing up.
	q0, _ := Parse(`(reduce range(23) as $x (2; . * .)) / 7`)
	c0, _ := Compile(q0)
	if v, ok := c0.Run(nil).Next(); !ok {
		t.Errorf("big bigint / 7: expected a result")
	} else if _, isErr := v.(error); isErr {
		t.Errorf("big bigint / 7: unexpected error %v", v)
	}

	// the conversion stays correct: exact division yields the exact integer, and
	// comparisons against floats give the right result.
	for _, tc := range []struct{ src, want string }{
		{`(reduce range(6) as $x (2; . * .)) / 4`, `4611686018427387904`}, // 2^64 / 4 = 2^62, exact
		{`(reduce range(6) as $x (2; . * .)) < 1e20`, `true`},             // 2^64 < 1e20 via bigToFloat
		{`(reduce range(6) as $x (2; . * .)) > 1e19`, `true`},             // 2^64 > 1e19
	} {
		q, _ := Parse(tc.src)
		c, _ := Compile(q)
		if v, ok := c.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// timefmt.Parse reads the whole input and format into runes before matching, so
// strptime on a big input string ( which fails on extra text anyway ) allocated
// several times its size: an 8 MB input peaked at 49 MB. A length pre-check now
// refuses an input or format whose parse could pass MaxAlloc; real date strings
// are tiny, so only absurd inputs are refused.
func TestMaxAllocBoundsStrptime(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := strings.Repeat("2024", 2000000) // 8 MB
	for _, tc := range []struct {
		src string
		in  any
	}{
		{`strptime("%Y")`, big},
		{`. as $f | "2024" | strptime($f)`, big}, // big format
	} {
		query, _ := Parse(tc.src)
		code, _ := Compile(query)
		if v, ok := code.Run(tc.in).Next(); !ok {
			t.Errorf("%q: expected an error, got no result", tc.src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q on a huge input: expected an allocation error, got %v", tc.src, v)
		}
	}

	// realistic date strings still parse correctly.
	for _, tc := range []struct{ src, want string }{
		{`"2024-01-15" | strptime("%Y-%m-%d") | .[0]`, `2024`},
		{`"2024-01-15T10:30:00Z" | strptime("%Y-%m-%dT%H:%M:%SZ") | .[2]`, `15`},
	} {
		q, _ := Parse(tc.src)
		c, _ := Compile(q)
		if v, ok := c.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// encode is Go-recursive ( encode -> encodeArray -> encode ) and is NOT covered by
// the interpreter's overStackLimit ; the buffer guard does not fire on a deep-narrow
// value ( each level adds one byte on the way down ). A deeply nested value therefore
// overflowed the goroutine stack in tojson / tostring / @json - a fatal, unrecoverable
// crash, not a panic. A recursion-depth bound now refuses a value whose nesting alone
// exceeds the limit ( >= 16 bytes per level ).
func TestMaxAllocBoundsEncodeDepth(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	var deepArr any = 0.0
	for i := 0; i < 500000; i++ {
		deepArr = []any{deepArr}
	}
	var deepObj any = 0.0
	for i := 0; i < 500000; i++ {
		deepObj = map[string]any{"a": deepObj}
	}
	for _, tc := range []struct {
		src string
		in  any
	}{
		{`tojson`, deepArr}, {`tostring`, deepArr}, {`@json`, deepArr}, {`tojson`, deepObj},
	} {
		query, _ := Parse(tc.src)
		code, _ := Compile(query)
		if v, ok := code.Run(tc.in).Next(); !ok {
			t.Errorf("%s on a deep value: expected an error, got no result", tc.src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%s on a deep value: expected an allocation error, got %v", tc.src, v)
		}
	}

	// normal nesting still encodes correctly.
	q, _ := Parse(`tojson`)
	code, _ := Compile(q)
	if v, ok := code.Run([]any{[]any{1.0, []any{2.0}}, map[string]any{"a": 3.0}}).Next(); !ok || fmt.Sprint(v) != `[[1,[2]],{"a":3}]` {
		t.Errorf("tojson of a normal value: got %v", v)
	}
}

// Several value operations recurse in Go on the structure's nesting depth -
// Compare ( sort / unique / min / < / == / group_by ), contains / inside, and
// flatten - none covered by the interpreter's overStackLimit. A deeply nested
// value overflowed the goroutine stack ( a fatal crash, not a panic ). Depth
// bounds now make them error ; Next recovers the Compare / contains panic.
func TestMaxAllocBoundsDeepRecursion(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	var deep any = 0.0
	for i := 0; i < 500000; i++ {
		deep = []any{deep}
	}
	for _, src := range []string{
		`[., .] | sort`, `[., .] | unique`, `[., .] | group_by(.)`, `[., .] | min`,
		`. < .`, `. == .`, `contains(.)`, `inside(.)`, `flatten`,
	} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, ok := code.Run(deep).Next(); !ok {
			t.Errorf("%q on a deep value: expected an error, got no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q on a deep value: expected an allocation error, got %v", src, v)
		}
	}

	// normal nesting still compares and flattens correctly.
	for _, tc := range []struct{ src, want string }{
		{`[[3], [1], [2]] | sort | tojson`, `[[1],[2],[3]]`},
		{`[[1, [2, [3]]]] | flatten | tojson`, `[1,2,3]`},
		{`{"a": {"b": 1}} | contains({"a": {"b": 1}})`, `true`},
		{`([1] < [2])`, `true`},
	} {
		q, _ := Parse(tc.src)
		c, _ := Compile(q)
		if v, ok := c.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// delpaths strips empty markers by walking the whole result recursively in Go
// ( deleteEmpty ), so del of a shallow key on a value with a deeply nested sibling
// overflowed the goroutine stack ( a fatal crash ). A depth bound now makes it
// error ; the interpreter's Next recovers the panic.
func TestMaxAllocBoundsDeleteEmpty(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	var deep any = 0.0
	for i := 0; i < 500000; i++ {
		deep = []any{deep}
	}
	in := map[string]any{"deep": deep, "x": 1.0}
	for _, src := range []string{`del(.x)`, `delpaths([["x"]])`} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, ok := code.Run(in).Next(); !ok {
			t.Errorf("%q with a deep sibling: expected an error, got no result", src)
		} else if _, isAlloc := v.(*allocLimitError); !isAlloc {
			t.Errorf("%q with a deep sibling: expected an allocation error, got %v", src, v)
		}
	}

	// normal del still works.
	q, _ := Parse(`del(.b) | tojson`)
	code, _ := Compile(q)
	if v, ok := code.Run(map[string]any{"a": 1.0, "b": 2.0}).Next(); !ok || fmt.Sprint(v) != `{"a":1}` {
		t.Errorf(`del(.b): got %v`, v)
	}
}

// update recurses once per path element and the round-69 size counters only
// charge on the way UP, so a long path from input ( which is not metered )
// overflowed the goroutine stack on the way DOWN before any charge fired. The
// path length is the recursion depth, so it is now pre-checked at the top of
// update. getpath is iterative and unaffected.
func TestMaxAllocBoundsSetpathPathLength(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// a 300k-element path ( > MaxAlloc/16 ) - the recursion would be 300k deep.
	arrPath := make([]any, 300000)
	for i := range arrPath {
		arrPath[i] = float64(0)
	}
	objPath := make([]any, 300000)
	for i := range objPath {
		objPath[i] = "a"
	}
	for _, tc := range []struct {
		src string
		p   []any
	}{
		{`. as $p | null | setpath($p; 1)`, arrPath},
		{`. as $p | null | setpath($p; 1)`, objPath},
		{`. as $p | {a: {a: 1}} | delpaths([$p])`, objPath},
	} {
		query, _ := Parse(tc.src)
		code, _ := Compile(query)
		// setpath/delpaths wrap the error, so match the message.
		if v, ok := code.Run(tc.p).Next(); !ok {
			t.Errorf("%q with a huge path: expected an error, got no result", tc.src)
		} else if !strings.Contains(fmt.Sprint(v), "allocation exceeds") {
			t.Errorf("%q with a huge path: expected an allocation error, got %v", tc.src, v)
		}
	}

	// normal-length paths still work.
	q, _ := Parse(`null | setpath(["a", "b"]; 5) | tojson`)
	code, _ := Compile(q)
	if v, ok := code.Run(nil).Next(); !ok || fmt.Sprint(v) != `{"a":{"b":5}}` {
		t.Errorf("setpath: got %v", v)
	}
}

// add/flatten/join on a map go through values(), which makes a []any of the
// map's size; on a big map that make is unmetered, so `add` (result is a number)
// completed meter-blind. values() must error on a big map, keeping correctness.
func TestMaxAllocBoundsValues(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := map[string]any{}
	for i := 0; i < 2000000; i++ {
		big[fmt.Sprint(i)] = float64(i)
	}
	for _, src := range []string{`add`, `flatten`} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, _ := code.Run(big).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
			t.Errorf("%s on a big map: expected an allocation error, got a value", src)
		}
	}

	// small map ops still correct, and the type error is preserved.
	q, _ := Parse(`add`)
	code, _ := Compile(q)
	if v, ok := code.Run(map[string]any{"a": 1.0, "b": 2.0}).Next(); !ok || fmt.Sprint(v) != "3" {
		t.Errorf("add on small map: expected 3, got %v", v)
	}
	q2, _ := Parse(`add`)
	c2, _ := Compile(q2)
	if v, _ := c2.Run("x").Next(); func() bool { _, ok := v.(*allocLimitError); return ok }() {
		t.Errorf("add on a string should be a type error, not an alloc error")
	}
}

// encodeObject makes a []keyVal of the map's size to sort keys, at object entry -
// before the encoder's output-length check (round 22) - so tojson/tostring/@json
// of a big map spiked on it. must error, small objects still encode sorted.
func TestMaxAllocBoundsEncodeObject(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := map[string]any{}
	for i := 0; i < 1000000; i++ {
		big[fmt.Sprint(i)] = float64(i)
	}
	query, _ := Parse(`tojson`)
	code, _ := Compile(query)
	if v, _ := code.Run(big).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
		t.Errorf("tojson of a big map: expected an allocation error, got a value")
	}

	q, _ := Parse(`tojson`)
	c, _ := Compile(q)
	if v, ok := c.Run(map[string]any{"b": 2.0, "a": 1.0}).Next(); !ok || fmt.Sprint(v) != `{"a":1,"b":2}` {
		t.Errorf("tojson small object: got %v", v)
	}
}

// slice assignment (.[a:b] = x) rebuilds the array via a.makeArray(~len(v));
// on a big input array that make must be bounded. small slices still work.
func TestMaxAllocBoundsSliceAssign(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := make([]any, 2000000)
	for i := range big {
		big[i] = float64(i)
	}
	query, _ := Parse(`.[0:1] = [9]`)
	code, _ := Compile(query)
	// setpath wraps the allocLimitError, so match on the message.
	if v, _ := code.Run(big).Next(); !strings.Contains(fmt.Sprint(v), "allocation exceeds") {
		t.Errorf("slice assign on a big array: expected an allocation error, got %v", v)
	}

	q, _ := Parse(`.[1:3] = [9, 9]`)
	c, _ := Compile(q)
	if v, ok := c.Run([]any{0.0, 1.0, 2.0, 3.0, 4.0}).Next(); !ok || fmt.Sprint(v) != "[0 9 9 3 4]" {
		t.Errorf("slice assign: got %v", v)
	}
}

// setpath/modify/del on a map copy it via a.makeObject(len(v)+1); on a big map
// that copy is unbounded. must error (message-matched - setpath wraps it).
func TestMaxAllocBoundsObjectUpdate(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	big := map[string]any{}
	for i := 0; i < 1000000; i++ {
		big[fmt.Sprint(i)] = float64(i)
	}
	for _, src := range []string{`.new = 1`, `del(.["500000"])`} {
		query, _ := Parse(src)
		code, _ := Compile(query)
		if v, _ := code.Run(big).Next(); !strings.Contains(fmt.Sprint(v), "allocation exceeds") {
			t.Errorf("%s on a big map: expected an allocation error, got %v", src, v)
		}
	}

	q, _ := Parse(`.c = 3`)
	c, _ := Compile(q)
	if v, ok := c.Run(map[string]any{"a": 1.0, "b": 2.0}).Next(); !ok || fmt.Sprint(v) != "map[a:1 b:2 c:3]" {
		t.Errorf(".c=3: got %v", v)
	}
}

// add's map path clones the first map (maps.Clone) unguarded; a caller-provided
// array with a big map first element made add copy it (156 MB) before the merge
// guard. must error; the first-array case was already guarded in round 25.
func TestMaxAllocBoundsAddMapClone(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20
	big := map[string]any{}
	for i := 0; i < 2000000; i++ {
		big[fmt.Sprint(i)] = float64(i)
	}
	query, _ := Parse(`add`)
	code, _ := Compile(query)
	if v, _ := code.Run([]any{big, map[string]any{"z": 1.0}}).Next(); func() bool { _, ok := v.(*allocLimitError); return !ok }() {
		t.Errorf("add of [bigmap, ...]: expected an allocation error, got a value")
	}
	q, _ := Parse(`add`)
	c, _ := Compile(q)
	if v, ok := c.Run([]any{map[string]any{"a": 1.0}, map[string]any{"b": 2.0}}).Next(); !ok || fmt.Sprint(v) != "map[a:1 b:2]" {
		t.Errorf("add of small maps: got %v", v)
	}
}

// A memory-limit error can be caught by try/catch, //, or ?, but the meter is
// monotonic: catching it does not reset env.alloc, so a further allocation
// after the catch still fails. This is what stops a suppress-and-retry loop
// from defeating the limit by swallowing each error and allocating again.
func TestMaxAllocMonotonicAcrossCatch(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`try [range(1000000)] catch "caught" | [range(1000000)] | length`,
		`(([range(1000000)] | length) // 999) | . + ([range(1000000)] | length)`,
		`reduce range(1000) as $_ (0; . + (([range(1000000)] | length)? // 0))`,
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("%q: parse: %v", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("%q: compile: %v", src, err)
		}
		v, ok := code.Run(nil).Next()
		if !ok {
			t.Errorf("%q: expected a result", src)
			continue
		}
		if _, isErr := v.(error); !isErr {
			t.Errorf("%q: expected the limit to hold after the catch, got %v", src, v)
		}
	}
}

// The encoder is not the only thing that materializes a shared-reference DAG.
// walk, flatten, tostream, and recurse ( .. ) each rebuild or enumerate every
// logical node, so a tiny O(K) DAG with 2^K logical nodes expands under them
// as well. Each must charge as it expands and stop at the limit, the same way
// tojson does ( see TestMaxAllocBoundsEncoding ), not run on to OOM.
func TestMaxAllocBoundsDAGTraversal(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	objDAG := `reduce range(35) as $_ (1; {a: ., b: .})`
	arrDAG := `reduce range(35) as $_ ([1]; [., .])`
	for _, src := range []string{
		objDAG + ` | walk(.)`,
		objDAG + ` | [tostream]`,
		objDAG + ` | [..]`,
		arrDAG + ` | flatten`,
		arrDAG + ` | walk(.)`,
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(nil).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isErr := v.(error); !isErr || !strings.Contains(fmt.Sprint(v), "allocation exceeds") {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// the same traversals still work on an ordinary small value.
	q2, _ := Parse(`walk(if type == "number" then . + 1 else . end)`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run([]any{1.0, 2.0}).Next(); !ok || fmt.Sprint(v2) != "[2 3]" {
		t.Errorf("walk on small value: got %v", v2)
	}
}

// Each run must be independently bounded: env.alloc lives on the per-run env
// created by newEnv, not on a shared global, and no run mutates state another
// run can see. Run a memory bomb and a legit query concurrently on shared
// compiled code and assert every bomb errors and every legit run returns its
// own correct answer. Under `go test -race` this also guards against a data
// race on the per-Code regex cache or the package builtin tables.
func TestMaxAllocConcurrentRunsIsolated(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	bombQ, err := Parse(`[range(100000000)]`)
	if err != nil {
		t.Fatal(err)
	}
	bomb, err := Compile(bombQ)
	if err != nil {
		t.Fatal(err)
	}
	legitQ, err := Parse(`.a + .b`)
	if err != nil {
		t.Fatal(err)
	}
	legit, err := Compile(legitQ)
	if err != nil {
		t.Fatal(err)
	}

	const n = 32
	var wg sync.WaitGroup
	bombErrored := make([]bool, n)
	legitCorrect := make([]bool, n)
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			iter := bomb.Run(nil)
			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if _, isErr := v.(error); isErr {
					bombErrored[i] = true
					break
				}
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			if v, ok := legit.Run(map[string]any{"a": float64(i), "b": 1.0}).Next(); ok {
				if r, isNum := v.(float64); isNum && r == float64(i)+1 {
					legitCorrect[i] = true
				}
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if !bombErrored[i] {
			t.Errorf("run %d: bomb did not error under concurrency", i)
		}
		if !legitCorrect[i] {
			t.Errorf("run %d: legit query returned a wrong or missing result under concurrency", i)
		}
	}
}

// Assignment ( |= ) materializes a shared-reference DAG the same way the
// read-only traversals do ( see TestMaxAllocBoundsDAGTraversal ): de-sharing
// each branch to update its leaves expands the 2^K logical nodes. The update
// path also grows an allocator map that records every container it copies, but
// that map is bounded because each entry is a charge-preceded copy and repeated
// updates to an already-copied container reuse it in place. So the assignment
// stops at the limit instead of growing the value or the allocator to OOM.
func TestMaxAllocBoundsDAGAssignment(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`reduce range(25) as $_ (1; {a: ., b: .}) | (.. | numbers) |= . + 1`,
		`reduce range(25) as $_ ([0]; [., .]) | (.. | numbers) |= . + 1`,
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		v, ok := code.Run(nil).Next()
		if !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isErr := v.(error); !isErr || !strings.Contains(fmt.Sprint(v), "allocation exceeds") {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// an ordinary assignment still works.
	q2, _ := Parse(`.a.b |= . + 1`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(map[string]any{"a": map[string]any{"b": 1.0}}).Next(); !ok || fmt.Sprint(v2) != "map[a:map[b:2]]" {
		t.Errorf("assignment on small value: got %v", v2)
	}
}

// funcMatch builds each match object's nested captures array internally, not
// through the collect opcodes, so the shallow value meter charged only the
// top-level match map ( ~112 bytes ) while the object holds one capture map per
// group. A query collecting many matches, each with many capture groups,
// allocated hundreds of MB ( a short regex reached ~950 MB ) while the meter saw
// almost nothing. The universal native-result meter charges their real size.
func TestMaxAllocBoundsMatchCaptures(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// 10000 matches, each with 50 capture groups; the captures dominate the
	// object and must be charged, so this errors instead of building ~GB.
	src := `("(.)" * 50) as $re | ("a" * 50) as $s | [range(10000) | ($s | match($re))] | length`
	query, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := code.Run(nil).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected an allocation error for many capture-heavy matches, got %v", v)
	}

	// an ordinary match with capture groups still works.
	q2, _ := Parse(`"2024-01-15" | [match("([0-9]+)-([0-9]+)-([0-9]+)").captures[].string]`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(nil).Next(); !ok || fmt.Sprint(v2) != "[2024 01 15]" {
		t.Errorf("ordinary match: got %v", v2)
	}
}

// fromjson decodes a fresh nested tree. decodeJSONLimited bounds a single
// decode, but the result was otherwise charged at its top level only, so a
// query collecting many decodes of a deeply-nested JSON string ( a ~40 KB
// string reached ~3.7 GB ) allocated unbounded while the meter saw ~16 bytes
// per decode. The universal native-result meter charges the whole decoded tree.
func TestMaxAllocBoundsFromjsonCollection(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// a deeply-nested JSON string decoded a million times and collected: each
	// decode is tiny at the top but large nested, so this must error.
	src := `("[" * 20000 + "0" + "]" * 20000) as $j | [range(1000000) | ($j | fromjson)] | length`
	query, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := code.Run(nil).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected an allocation error collecting deep fromjson, got %v", v)
	}

	// ordinary fromjson still works.
	q2, _ := Parse(`"[1,2,3]" | fromjson | add`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(nil).Next(); !ok || fmt.Sprint(v2) != "6" {
		t.Errorf("ordinary fromjson: got %v", v2)
	}
}

// transpose builds a fresh outer array of inner arrays whose elements are refs
// into the input. The shallow meter charged only the outer array, so an N-by-2
// transpose ( two wide inner arrays ) collected in a loop reached ~2.2 GB while
// the meter saw ~48 bytes per result. The universal native-result meter charges
// the inner arrays' slots. ( Round 120 missed this: it tested the 2-by-N orientation,
// which is many NARROW inner arrays and stays bounded. )
func TestMaxAllocBoundsTransposeCollection(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// N-by-2 input transposes to two N-wide inner arrays; collecting a million
	// of them must error instead of building GB.
	src := `([range(2000)] | map([., .])) as $m | [range(1000000) | ($m | transpose)] | length`
	query, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := code.Run(nil).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected an allocation error collecting wide transposes, got %v", v)
	}

	// an ordinary transpose still works.
	q2, _ := Parse(`[[1, 2, 3], [4, 5, 6]] | transpose`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(nil).Next(); !ok || fmt.Sprint(v2) != "[[1 4] [2 5] [3 6]]" {
		t.Errorf("ordinary transpose: got %v", v2)
	}
}

// setpath ( and |= / = ) copies a fresh spine of containers from the root down
// to the target and shares the rest of the input by reference. The shallow
// meter charged only the top of the result, so a deep-path setpath collected in
// a loop ( a 1000-deep path reached ~2 GB ) allocated the spine unbounded.
// The universal native-result meter charges the copied spine, while leaving a shallow assignment
// charged as before ( its spine is just the top ).
func TestMaxAllocBoundsSetpathSpine(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`[range(1000) | 0] as $p | [range(1000000) | (null | setpath($p; 1))] | length`,
		`[range(1000) | 0] as $p | [range(1000000) | (null | setpath($p; 1) | getpath($p) |= . + 1)] | length`,
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		if v, ok := code.Run(nil).Next(); !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isErr := v.(error); !isErr {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// ordinary setpath and deep assignment still produce the right value.
	q2, _ := Parse(`setpath([0, 0, 0]; 5)`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(nil).Next(); !ok || fmt.Sprint(v2) != "[[[5]]]" {
		t.Errorf("setpath: got %v", v2)
	}
	q3, _ := Parse(`.a.b.c = 7`)
	c3, _ := Compile(q3)
	in := map[string]any{"a": map[string]any{"b": map[string]any{"c": 1.0}}}
	if v3, ok := c3.Run(in).Next(); !ok || fmt.Sprint(v3) != "map[a:map[b:map[c:7]]]" {
		t.Errorf("assignment: got %v", v3)
	}
}

// delpaths ( and del, and |= empty ) copies a fresh spine per deleted path via
// the same copy-on-write as setpath, but was not in the deep-charge switch: the
// run meter saw only the shallow top of the result, so collecting many deep-path
// deletions of a bound value allocated the copied spines unbounded ( a 1000-deep
// object deleted in a loop peaked ~75 MB at a 4 MiB limit ). The universal
// native-result meter charges every copied spine by identity. Both
// the native "delpaths" and the assignment "_delpaths" opcode are covered.
func TestMaxAllocBoundsDelpathsSpine(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	for _, src := range []string{
		`(reduce range(1000) as $i (0; {a: .})) as $x | ([range(1000) | "a"]) as $p | [range(1000000) | ($x | delpaths([$p]))] | length`,
		`(reduce range(1500) as $i (0; {a: .})) as $x | [range(1000000) | ($x | del(.a.a.a.a.a))] | length`,
		`(reduce range(1000) as $i (0; {a: .})) as $x | [range(1000000) | ($x | (getpath([range(999) | "a"])) |= empty)] | length`,
	} {
		query, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, err)
		}
		code, err := Compile(query)
		if err != nil {
			t.Fatalf("Compile(%q): %s", src, err)
		}
		if v, ok := code.Run(nil).Next(); !ok {
			t.Errorf("%q: expected an error, got no result", src)
		} else if _, isErr := v.(error); !isErr {
			t.Errorf("%q: expected an allocation error, got %v", src, v)
		}
	}

	// ordinary delpaths and del still produce the right value.
	q4, _ := Parse(`delpaths([["a", "b"], ["c"]])`)
	c4, _ := Compile(q4)
	in4 := map[string]any{"a": map[string]any{"b": 1.0}, "c": 2.0, "d": 3.0}
	if v4, ok := c4.Run(in4).Next(); !ok || fmt.Sprint(v4) != "map[a:map[] d:3]" {
		t.Errorf("delpaths: got %v", v4)
	}
	q5, _ := Parse(`del(.a.b)`)
	c5, _ := Compile(q5)
	in5 := map[string]any{"a": map[string]any{"b": 1.0, "c": 2.0}}
	if v5, ok := c5.Run(in5).Next(); !ok || fmt.Sprint(v5) != "map[a:map[c:2]]" {
		t.Errorf("del: got %v", v5)
	}

	// a wide sibling delete ( del(.[]) ) has every path share the one root, which
	// the allocator copies once. Charging each path's spine separately would be
	// quadratic and would falsely reject this legitimate small-result delete, so
	// the result meter charges the union of the spines. This must succeed and give [].
	q6, _ := Parse(`del(.[])`)
	c6, _ := Compile(q6)
	in6 := make([]any, 5000)
	for i := range in6 {
		in6[i] = i
	}
	if v6, ok := c6.Run(in6).Next(); !ok {
		t.Error("del(.[]) on a wide array: got no result")
	} else if arr, isArr := v6.([]any); !isArr || len(arr) != 0 {
		t.Errorf("del(.[]) on a wide array: got %v, want []", v6)
	}
}

// map * map ( deepmerge ) recursively builds fresh merged maps via make(), the
// same shape as setpath: update tracks the merged size in a per-call local
// counter that bounds a single merge, but the result was charged to the run
// meter only at its top level. A deep-object self-merge collected in a loop
// ( a 1000-deep object reached ~7 GB ) allocated the merged maps unbounded.
// The universal native-result meter charges every fresh merged-map spine.
func TestMaxAllocBoundsDeepmerge(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	src := `reduce range(1000) as $i ({}; {a: .}) as $d | [range(1000000) | ($d * $d)] | length`
	query, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := code.Run(nil).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected an allocation error collecting deep merges, got %v", v)
	}

	// ordinary merge, and numeric / string multiply, are unaffected.
	for _, tc := range []struct{ src, want string }{
		{`{a: {b: 1}, c: 2} * {a: {d: 3}, e: 4}`, "map[a:map[b:1 d:3] c:2 e:4]"},
		{`6 * 7`, "42"},
		{`"ab" * 3`, "ababab"},
	} {
		q, _ := Parse(tc.src)
		c, _ := Compile(q)
		if v, ok := c.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}

	// a large self-merge has full key overlap, so it builds a union-sized map
	// ( ~2.5 MB for 55k keys ), not a len(l)+len(r)-sized one. Charging both key
	// counts double-counted the overlap and falsely rejected it at ~2x the real
	// size. This must succeed and return the union key count.
	in := make(map[string]any, 55000)
	for i := 0; i < 55000; i++ {
		in[fmt.Sprint(i)] = i
	}
	qm, _ := Parse(`. as $o | $o * $o | length`)
	cm, _ := Compile(qm)
	if v, ok := cm.Run(in).Next(); !ok {
		t.Error("large self-merge: got no result")
	} else if _, isErr := v.(error); isErr {
		t.Errorf("large self-merge falsely rejected: %v", v)
	} else if fmt.Sprint(v) != "55000" {
		t.Errorf("large self-merge: got %v, want 55000", v)
	}
}

// .[] materializes a []pathValue of every element to drive the iteration. That
// slice was pre-checked per array but never charged to the run meter, so nested
// iteration of the same large array ( $a[] as $x | $a[] as $y | ... ) stacked
// the uncharged slices and reached hundreds of MB while the meter saw nothing.
// The iteration is now charged, so nested / looped iterations accumulate and trip.
func TestMaxAllocBoundsIterPathValues(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	// five nested iterations of a 50k-element array: each materializes a 1.6 MB
	// []pathValue, ~8 MB total, which must now trip. limit(1) keeps it from being
	// a CPU bomb - the slices are materialized on entry, before any deep looping.
	src := "[range(50000)] as $a | [limit(1; $a[] as $x0 | $a[] as $x1 | $a[] as $x2 | $a[] as $x3 | $a[] as $x4 | 1)] | length"
	query, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := code.Run(nil).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected an allocation error for nested iteration, got %v", v)
	}

	// a single scan of a large array is unchanged ( one []pathValue, under the
	// limit ), and small / nested iteration of small arrays must still work.
	for _, tc := range []struct{ src, want string }{
		{`[range(1000)] | any(. > 998)`, "true"},
		{`[[1, 2], [3, 4]] | [.[] | .[]] | add`, "10"},
		{`[range(100000)] | length`, "100000"},
		{"[range(1000)] as $a | [limit(1; $a[] as $y0 | $a[] as $y1 | $a[] as $y2 | 1)] | length", "1"},
	} {
		q, _ := Parse(tc.src)
		c, _ := Compile(q)
		if v, ok := c.Run(nil).Next(); !ok || fmt.Sprint(v) != tc.want {
			t.Errorf("%q: got %v, want %s", tc.src, v, tc.want)
		}
	}
}

// The Go-recursive builtins ( compare, encode, contains, flatten, deleteEmpty,
// update, deepmerge, and the JSON decoder ) bound recursion depth by MaxAlloc,
// which at a large MaxAlloc would let a deeply-nested input overflow the
// goroutine stack - a fatal, uncatchable crash. maxRecursionDepth caps them
// independently of MaxAlloc, turning that into a clean allocation error. This
// exercises the JSON decoder's cap ( they all share the constant ): a string
// nested past the cap errors instead of crashing even though MaxAlloc is huge.
func TestMaxAllocCapsRecursionDepth(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 64 << 20 // large enough that the per-level MaxAlloc bound does not fire first

	depth := maxRecursionDepth + 1000
	doc := strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth)
	query, _ := Parse(`fromjson`)
	code, _ := Compile(query)
	if v, ok := code.Run(doc).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected the recursion-depth cap to error, got a value")
	}

	// a normally-nested value still decodes.
	q2, _ := Parse(`fromjson`)
	c2, _ := Compile(q2)
	if v2, ok := c2.Run(`[[1,[2,[3]]]]`).Next(); !ok || fmt.Sprint(v2) != "[[1 [2 [3]]]]" {
		t.Errorf("ordinary fromjson: got %v", v2)
	}
}

// A Go map[string]any has a ~320-byte fixed minimum ( hmap header + a full first
// bucket ) that allocSize's old len*24+16 missed, so a chain or collection of
// tiny 1-key maps ran several times over the limit ( a 1M-deep 1-key chain
// peaked ~27 MB at a 4 MiB limit ). allocSize now charges that base. This 30000-
// deep 1-key chain is ~11 MB of real maps but was charged under 2 MB before.
func TestMaxAllocChargesMapBase(t *testing.T) {
	defer func(o int64) { MaxAlloc = o }(MaxAlloc)
	MaxAlloc = 4 << 20

	src := `reduce range(30000) as $i (0; {($i | tostring): .})`
	query, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := code.Run(nil).Next(); !ok {
		t.Fatal("expected a result")
	} else if _, isErr := v.(error); !isErr {
		t.Errorf("expected the map-base charge to error on a deep 1-key chain, got a value")
	}

	// an ordinary small object is still fine.
	q2, _ := Parse(`{a: .x, b: .y, c: .z}`)
	c2, _ := Compile(q2)
	in := map[string]any{"x": 1.0, "y": 2.0, "z": 3.0}
	if v2, ok := c2.Run(in).Next(); !ok || fmt.Sprint(v2) != "map[a:1 b:2 c:3]" {
		t.Errorf("small object: got %v", v2)
	}
}
