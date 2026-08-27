package gojq

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"math"
	"math/big"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/itchyny/timefmt-go"
)

//go:generate go run -modfile=go.dev.mod _tools/gen_builtin.go -i builtin.jq -o builtin.go
var builtinFuncDefs map[string][]*FuncDef

const (
	argcount0 = 1 << iota
	argcount1
	argcount2
	argcount3
)

type function struct {
	argcount int
	iter     bool
	callback func(any, []any) any
}

func (fn function) accept(cnt int) bool {
	return fn.argcount&(1<<cnt) != 0
}

var internalFuncs map[string]function

func init() {
	internalFuncs = map[string]function{
		"empty":          argFunc0(nil),
		"path":           argFunc1(nil),
		"env":            argFunc0(nil),
		"builtins":       argFunc0(nil),
		"input":          argFunc0(nil),
		"modulemeta":     argFunc0(nil),
		"debug":          argFunc1(nil),
		"abs":            argFunc0(funcAbs),
		"length":         argFunc0(funcLength),
		"utf8bytelength": argFunc0(funcUtf8ByteLength),
		"keys":           argFunc0(funcKeys),
		"has":            argFunc1(funcHas),
		"add":            argFunc0(funcAdd),
		"toboolean":      argFunc0(funcToBoolean),
		"tonumber":       argFunc0(funcToNumber),
		"tostring":       argFunc0(funcToString),
		"type":           argFunc0(funcType),
		"reverse":        argFunc0(funcReverse),
		"contains":       argFunc1(funcContains),
		"inside":         argFunc1(funcInside),
		"indices":        argFunc1(funcIndices),
		"index":          argFunc1(funcIndex),
		"rindex":         argFunc1(funcRindex),
		"startswith":     argFunc1(funcStartsWith),
		"endswith":       argFunc1(funcEndsWith),
		"ltrimstr":       argFunc1(funcLtrimstr),
		"rtrimstr":       argFunc1(funcRtrimstr),
		"trimstr":        argFunc1(funcTrimstr),
		"ltrim":          argFunc0(funcLtrim),
		"rtrim":          argFunc0(funcRtrim),
		"trim":           argFunc0(funcTrim),
		"explode":        argFunc0(funcExplode),
		"implode":        argFunc0(funcImplode),
		"split":          argFunc1(funcSplit),
		"join":           argFunc1(funcJoin),
		"ascii_downcase": argFunc0(funcASCIIDowncase),
		"ascii_upcase":   argFunc0(funcASCIIUpcase),
		"tojson":         argFunc0(funcToJSON),
		"fromjson":       argFunc0(funcFromJSON),
		"format":         argFunc1(funcFormat),
		"_tohtml":        argFunc0(funcToHTML),
		"_touri":         argFunc0(funcToURI),
		"_tourid":        argFunc0(funcToURId),
		"_tocsv":         argFunc0(funcToCSV),
		"_totsv":         argFunc0(funcToTSV),
		"_tosh":          argFunc0(funcToSh),
		"_tobase64":      argFunc0(funcToBase64),
		"_tobase64d":     argFunc0(funcToBase64d),
		"_index":         argFunc2(funcIndex2),
		"_slice":         argFunc3(funcSlice),
		"_plus":          argFunc0(funcOpPlus),
		"_negate":        argFunc0(funcOpNegate),
		"_add":           argFunc2(funcOpAdd),
		"_subtract":      argFunc2(funcOpSub),
		"_multiply":      argFunc2(funcOpMul),
		"_divide":        argFunc2(funcOpDiv),
		"_modulo":        argFunc2(funcOpMod),
		"_alternative":   argFunc2(funcOpAlt),
		"_equal":         argFunc2(funcOpEq),
		"_notequal":      argFunc2(funcOpNe),
		"_greater":       argFunc2(funcOpGt),
		"_less":          argFunc2(funcOpLt),
		"_greatereq":     argFunc2(funcOpGe),
		"_lesseq":        argFunc2(funcOpLe),
		"flatten":        {argcount0 | argcount1, false, funcFlatten},
		"_range":         {argcount3, true, funcRange},
		"min":            argFunc0(funcMin),
		"_min_by":        argFunc1(funcMinBy),
		"max":            argFunc0(funcMax),
		"_max_by":        argFunc1(funcMaxBy),
		"sort":           argFunc0(funcSort),
		"_sort_by":       argFunc1(funcSortBy),
		"_group_by":      argFunc1(funcGroupBy),
		"unique":         argFunc0(funcUnique),
		"_unique_by":     argFunc1(funcUniqueBy),
		"sin":            mathFunc("sin", math.Sin),
		"cos":            mathFunc("cos", math.Cos),
		"tan":            mathFunc("tan", math.Tan),
		"asin":           mathFunc("asin", math.Asin),
		"acos":           mathFunc("acos", math.Acos),
		"atan":           mathFunc("atan", math.Atan),
		"sinh":           mathFunc("sinh", math.Sinh),
		"cosh":           mathFunc("cosh", math.Cosh),
		"tanh":           mathFunc("tanh", math.Tanh),
		"asinh":          mathFunc("asinh", math.Asinh),
		"acosh":          mathFunc("acosh", math.Acosh),
		"atanh":          mathFunc("atanh", math.Atanh),
		"floor":          mathFunc("floor", math.Floor),
		"round":          mathFunc("round", math.Round),
		"nearbyint":      mathFunc("nearbyint", math.RoundToEven),
		"rint":           mathFunc("rint", math.RoundToEven),
		"ceil":           mathFunc("ceil", math.Ceil),
		"trunc":          mathFunc("trunc", math.Trunc),
		"significand":    mathFunc("significand", funcSignificand),
		"fabs":           mathFunc("fabs", math.Abs),
		"sqrt":           mathFunc("sqrt", math.Sqrt),
		"cbrt":           mathFunc("cbrt", math.Cbrt),
		"exp":            mathFunc("exp", math.Exp),
		"exp10":          mathFunc("exp10", funcExp10),
		"exp2":           mathFunc("exp2", math.Exp2),
		"expm1":          mathFunc("expm1", math.Expm1),
		"frexp":          argFunc0(funcFrexp),
		"modf":           argFunc0(funcModf),
		"log":            mathFunc("log", math.Log),
		"log10":          mathFunc("log10", math.Log10),
		"log1p":          mathFunc("log1p", math.Log1p),
		"log2":           mathFunc("log2", math.Log2),
		"logb":           mathFunc("logb", math.Logb),
		"gamma":          mathFunc("gamma", math.Gamma),
		"tgamma":         mathFunc("tgamma", math.Gamma),
		"lgamma":         mathFunc("lgamma", funcLgamma),
		"erf":            mathFunc("erf", math.Erf),
		"erfc":           mathFunc("erfc", math.Erfc),
		"j0":             mathFunc("j0", math.J0),
		"j1":             mathFunc("j1", math.J1),
		"y0":             mathFunc("y0", math.Y0),
		"y1":             mathFunc("y1", math.Y1),
		"atan2":          mathFunc2("atan2", math.Atan2),
		"copysign":       mathFunc2("copysign", math.Copysign),
		"drem":           mathFunc2("drem", funcDrem),
		"fdim":           mathFunc2("fdim", math.Dim),
		"fmax":           mathFunc2("fmax", funcFmax),
		"fmin":           mathFunc2("fmin", funcFmin),
		"fmod":           mathFunc2("fmod", math.Mod),
		"hypot":          mathFunc2("hypot", math.Hypot),
		"jn":             mathFunc2("jn", funcJn),
		"nextafter":      mathFunc2("nextafter", math.Nextafter),
		"nexttoward":     mathFunc2("nexttoward", math.Nextafter),
		"remainder":      mathFunc2("remainder", math.Remainder),
		"ldexp":          mathFunc2("ldexp", funcLdexp),
		"scalb":          mathFunc2("scalb", funcLdexp),
		"scalbln":        mathFunc2("scalbln", funcLdexp),
		"yn":             mathFunc2("yn", funcYn),
		"pow":            mathFunc2("pow", math.Pow),
		"fma":            mathFunc3("fma", math.FMA),
		"infinite":       argFunc0(funcInfinite),
		"isfinite":       argFunc0(funcIsfinite),
		"isinfinite":     argFunc0(funcIsinfinite),
		"nan":            argFunc0(funcNan),
		"isnan":          argFunc0(funcIsnan),
		"isnormal":       argFunc0(funcIsnormal),
		"setpath":        argFunc2(funcSetpath),
		"delpaths":       argFunc1(funcDelpaths),
		"getpath":        argFunc1(funcGetpath),
		"transpose":      argFunc0(funcTranspose),
		"bsearch":        argFunc1(funcBsearch),
		"gmtime":         argFunc0(funcGmtime),
		"localtime":      argFunc0(funcLocaltime),
		"mktime":         argFunc0(funcMktime),
		"strftime":       argFunc1(funcStrftime),
		"strflocaltime":  argFunc1(funcStrflocaltime),
		"strptime":       argFunc1(funcStrptime),
		"now":            argFunc0(funcNow),
		"_match":         argFunc3(nil),
		"_captures":      argFunc0(funcCaptures),
		"error":          {argcount0 | argcount1, false, funcError},
		"halt":           argFunc0(funcHalt),
		"halt_error":     {argcount0 | argcount1, false, funcHaltError},
	}
}

func argFunc0(f func(any) any) function {
	return function{
		argcount0, false, func(v any, _ []any) any {
			return f(v)
		},
	}
}

func argFunc1(f func(_, _ any) any) function {
	return function{
		argcount1, false, func(v any, args []any) any {
			return f(v, args[0])
		},
	}
}

func argFunc2(f func(_, _, _ any) any) function {
	return function{
		argcount2, false, func(v any, args []any) any {
			return f(v, args[0], args[1])
		},
	}
}

func argFunc3(f func(_, _, _, _ any) any) function {
	return function{
		argcount3, false, func(v any, args []any) any {
			return f(v, args[0], args[1], args[2])
		},
	}
}

func mathFunc(name string, f func(float64) float64) function {
	return argFunc0(func(v any) any {
		x, ok := toFloat(v)
		if !ok {
			return &func0TypeError{name, v}
		}
		return f(x)
	})
}

func mathFunc2(name string, f func(_, _ float64) float64) function {
	return argFunc2(func(_, x, y any) any {
		l, ok := toFloat(x)
		if !ok {
			return &func0TypeError{name, x}
		}
		r, ok := toFloat(y)
		if !ok {
			return &func0TypeError{name, y}
		}
		return f(l, r)
	})
}

func mathFunc3(name string, f func(_, _, _ float64) float64) function {
	return argFunc3(func(_, a, b, c any) any {
		x, ok := toFloat(a)
		if !ok {
			return &func0TypeError{name, a}
		}
		y, ok := toFloat(b)
		if !ok {
			return &func0TypeError{name, b}
		}
		z, ok := toFloat(c)
		if !ok {
			return &func0TypeError{name, c}
		}
		return f(x, y, z)
	})
}

func funcAbs(v any) any {
	switch v := v.(type) {
	case int:
		if v >= 0 {
			return v
		}
		return negate(v)
	case float64:
		return math.Abs(v)
	case *big.Int:
		if v.Sign() >= 0 {
			return v
		}
		return new(big.Int).Abs(v)
	case json.Number:
		if !strings.HasPrefix(v.String(), "-") {
			return v
		}
		return v[1:]
	default:
		return &func0TypeError{"abs", v}
	}
}

func funcLength(v any) any {
	switch v := v.(type) {
	case nil:
		return 0
	case int:
		if v >= 0 {
			return v
		}
		return negate(v)
	case float64:
		return math.Abs(v)
	case *big.Int:
		if v.Sign() >= 0 {
			return v
		}
		return new(big.Int).Abs(v)
	case json.Number:
		if !strings.HasPrefix(v.String(), "-") {
			return v
		}
		return v[1:]
	case string:
		return utf8.RuneCountInString(v)
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	default:
		return &func0TypeError{"length", v}
	}
}

func funcUtf8ByteLength(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"utf8bytelength", v}
	}
	return len(s)
}

func funcKeys(v any) any {
	switch v := v.(type) {
	case []any:
		if arrayTooLarge(len(v)) {
			return &allocLimitError{}
		}
		w := make([]any, len(v))
		for i := range v {
			w[i] = i
		}
		return w
	case map[string]any:
		if arrayTooLarge(len(v)) {
			return &allocLimitError{}
		}
		w := make([]any, len(v))
		for i, k := range keys(v) {
			w[i] = k
		}
		return w
	default:
		return &func0TypeError{"keys", v}
	}
}

func keys(v map[string]any) []string {
	w := make([]string, len(v))
	var i int
	for k := range v {
		w[i] = k
		i++
	}
	slices.Sort(w)
	return w
}

var errNotIterable = errors.New("not iterable")

func values(v any) ([]any, error) {
	switch v := v.(type) {
	case []any:
		return v, nil
	case map[string]any:
		if arrayTooLarge(len(v)) {
			return nil, &allocLimitError{}
		}
		vs := make([]any, len(v))
		for i, k := range keys(v) {
			vs[i] = v[k]
		}
		return vs, nil
	default:
		return nil, errNotIterable
	}
}

func funcHas(v, x any) any {
	switch v := v.(type) {
	case []any:
		if x, ok := toInt(x); ok {
			return 0 <= x && x < len(v)
		}
	case map[string]any:
		if x, ok := x.(string); ok {
			_, ok := v[x]
			return ok
		}
	case nil:
		return false
	}
	return &func1TypeError{"has", v, x}
}

func funcAdd(v any) any {
	vs, err := values(v)
	if err != nil {
		if err == errNotIterable {
			return &func0TypeError{"add", v}
		}
		return err
	}
	return add(slices.Values(vs))
}

func add(xs iter.Seq[any]) any {
	var v any
	for x := range xs {
		switch x := x.(type) {
		case nil:
			continue
		case string:
			switch w := v.(type) {
			case nil:
				var sb strings.Builder
				sb.WriteString(x)
				v = &sb
				continue
			case *strings.Builder:
				w.WriteString(x)
				if MaxAlloc > 0 && int64(w.Len()) > MaxAlloc {
					return &allocLimitError{}
				}
				continue
			}
		case []any:
			switch w := v.(type) {
			case nil:
				if arrayTooLarge(len(x)) {
					return &allocLimitError{}
				}
				s := make([]any, len(x))
				copy(s, x)
				v = s
				continue
			case []any:
				w = append(w, x...)
				if arrayTooLarge(len(w)) {
					return &allocLimitError{}
				}
				v = w
				continue
			}
		case map[string]any:
			switch w := v.(type) {
			case nil:
				if MaxAlloc > 0 && int64(len(x))*24 > MaxAlloc {
					return &allocLimitError{}
				}
				v = maps.Clone(x)
				continue
			case map[string]any:
				maps.Copy(w, x)
				if MaxAlloc > 0 && int64(len(w))*24 > MaxAlloc {
					return &allocLimitError{}
				}
				continue
			}
		}
		if sb, ok := v.(*strings.Builder); ok {
			v = sb.String()
		}
		v = funcOpAdd(nil, v, x)
		if err, ok := v.(error); ok {
			return err
		}
	}
	if sb, ok := v.(*strings.Builder); ok {
		v = sb.String()
	}
	return v
}

func funcToBoolean(v any) any {
	switch v := v.(type) {
	case bool:
		return v
	case string:
		switch v {
		case "true":
			return true
		case "false":
			return false
		default:
			return &func0WrapError{"toboolean", v, errors.New("invalid boolean")}
		}
	default:
		return &func0TypeError{"toboolean", v}
	}
}

func funcToNumber(v any) any {
	switch v := v.(type) {
	case int, float64, *big.Int, json.Number:
		return v
	case string:
		if !newLexer(v).validNumber() {
			return &func0WrapError{"tonumber", v, errors.New("invalid number")}
		}
		return toNumber(v)
	default:
		return &func0TypeError{"tonumber", v}
	}
}

func toNumber(v string) any {
	return parseNumber(json.Number(v))
}

func funcToString(v any) any {
	if s, ok := v.(string); ok {
		return s
	}
	return funcToJSON(v)
}

func funcType(v any) any {
	return TypeOf(v)
}

func funcReverse(v any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func0TypeError{"reverse", v}
	}
	if arrayTooLarge(len(vs)) {
		return &allocLimitError{}
	}
	ws := make([]any, len(vs))
	for i, v := range vs {
		ws[len(ws)-i-1] = v
	}
	return ws
}

func funcContains(v, x any) any {
	return containsDepth(v, x, 0)
}

// containsDepth is funcContains with a recursion-depth bound; contains/inside
// recurse on nested arrays and objects, so a deeply nested value would overflow
// the goroutine stack. Past the depth a value already exceeds the limit.
func containsDepth(v, x any, depth int) any {
	if MaxAlloc > 0 && (int64(depth)*16 > MaxAlloc || depth > maxRecursionDepth) {
		panic(&allocLimitError{})
	}
	return binopTypeSwitch(v, x,
		func(l, r int) any { return l == r },
		func(l, r float64) any { return l == r },
		func(l, r *big.Int) any { return l.Cmp(r) == 0 },
		func(l, r string) any { return strings.Contains(l, r) },
		func(l, r []any) any {
		R:
			for _, r := range r {
				for _, l := range l {
					if containsDepth(l, r, depth+1) == true {
						continue R
					}
				}
				return false
			}
			return true
		},
		func(l, r map[string]any) any {
			if len(l) < len(r) {
				return false
			}
			for k, r := range r {
				if l, ok := l[k]; !ok || containsDepth(l, r, depth+1) != true {
					return false
				}
			}
			return true
		},
		func(l, r any) any {
			if l == r {
				return true
			}
			return &func1TypeError{"contains", l, r}
		},
	)
}

func funcInside(v, x any) any {
	return funcContains(x, v)
}

// matchAt reports whether xs equals the len(xs)-run of vs starting at i. It
// compares element by element rather than reslicing vs on every position, which
// boxed a fresh slice header into Compare each step and turned indices / index /
// rindex over a large array into a big transient allocation ( tens of MB for one
// match ). Elements are already interface values, so this allocates nothing.
func matchAt(vs, xs []any, i int) bool {
	for j := range xs {
		if Compare(vs[i+j], xs[j]) != 0 {
			return false
		}
	}
	return true
}

func funcIndices(v, x any) any {
	return indexFunc("indices", v, x, indices)
}

func indices(vs, xs []any) any {
	rs := []any{}
	if len(xs) == 0 {
		return rs
	}
	for i := range len(vs) - len(xs) + 1 {
		if matchAt(vs, xs, i) {
			rs = append(rs, i)
			if arrayTooLarge(len(rs)) {
				return &allocLimitError{}
			}
		}
	}
	return rs
}

func funcIndex(v, x any) any {
	return indexFunc("index", v, x, func(vs, xs []any) any {
		if len(xs) == 0 {
			return nil
		}
		for i := range len(vs) - len(xs) + 1 {
			if matchAt(vs, xs, i) {
				return i
			}
		}
		return nil
	})
}

func funcRindex(v, x any) any {
	return indexFunc("rindex", v, x, func(vs, xs []any) any {
		if len(xs) == 0 {
			return nil
		}
		for i := len(vs) - len(xs); i >= 0; i-- {
			if matchAt(vs, xs, i) {
				return i
			}
		}
		return nil
	})
}

func indexFunc(name string, v, x any, f func(_, _ []any) any) any {
	switch v := v.(type) {
	case nil:
		return nil
	case []any:
		switch x := x.(type) {
		case []any:
			return f(v, x)
		default:
			return f(v, []any{x})
		}
	case string:
		if x, ok := x.(string); ok {
			if arrayTooLarge(utf8.RuneCountInString(v)) || arrayTooLarge(utf8.RuneCountInString(x)) {
				return &allocLimitError{}
			}
			return f(explode(v), explode(x))
		}
		return &func1TypeError{name, v, x}
	default:
		return &func1TypeError{name, v, x}
	}
}

func funcStartsWith(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func1TypeError{"startswith", v, x}
	}
	t, ok := x.(string)
	if !ok {
		return &func1TypeError{"startswith", v, x}
	}
	return strings.HasPrefix(s, t)
}

func funcEndsWith(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func1TypeError{"endswith", v, x}
	}
	t, ok := x.(string)
	if !ok {
		return &func1TypeError{"endswith", v, x}
	}
	return strings.HasSuffix(s, t)
}

func funcLtrimstr(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func1TypeError{"ltrimstr", v, x}
	}
	t, ok := x.(string)
	if !ok {
		return &func1TypeError{"ltrimstr", v, x}
	}
	return strings.TrimPrefix(s, t)
}

func funcRtrimstr(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func1TypeError{"rtrimstr", v, x}
	}
	t, ok := x.(string)
	if !ok {
		return &func1TypeError{"rtrimstr", v, x}
	}
	return strings.TrimSuffix(s, t)
}

func funcTrimstr(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func1TypeError{"trimstr", v, x}
	}
	t, ok := x.(string)
	if !ok {
		return &func1TypeError{"trimstr", v, x}
	}
	return strings.TrimSuffix(strings.TrimPrefix(s, t), t)
}

func funcLtrim(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"ltrim", v}
	}
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func funcRtrim(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"rtrim", v}
	}
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func funcTrim(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"trim", v}
	}
	return strings.TrimSpace(s)
}

func funcExplode(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"explode", v}
	}
	if arrayTooLarge(utf8.RuneCountInString(s)) {
		return &allocLimitError{}
	}
	return explode(s)
}

func explode(s string) []any {
	xs := make([]any, utf8.RuneCountInString(s))
	var i int
	for _, r := range s {
		xs[i] = int(r)
		i++
	}
	return xs
}

func funcImplode(v any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func0TypeError{"implode", v}
	}
	// The input array remains live while strings.Builder validates and encodes
	// every interface value. Builder growth can temporarily retain old and new
	// backing buffers, and each rune can occupy four output bytes. Cap that whole
	// decode/encode working set before starting it; the returned string is still
	// charged normally by the opcode meter.
	if implodeWorkingSetTooLarge(len(vs)) {
		return &allocLimitError{}
	}
	var sb strings.Builder
	// Grow is only a capacity hint; cap it at the limit so a huge input array
	// cannot force an upfront allocation past MaxAlloc before the per-rune check
	// below has a chance to fire.
	grow := len(vs)
	if MaxAlloc > 0 && int64(grow) > MaxAlloc {
		grow = int(MaxAlloc)
	}
	sb.Grow(grow)
	for _, v := range vs {
		if r, ok := toInt(v); ok {
			if 0 <= r && r <= utf8.MaxRune {
				sb.WriteRune(rune(r))
			} else {
				sb.WriteRune(utf8.RuneError)
			}
			if MaxAlloc > 0 && int64(sb.Len()) > MaxAlloc {
				return &allocLimitError{}
			}
		} else {
			return &func0TypeError{"implode", vs}
		}
	}
	return sb.String()
}

func funcSplit(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"split", v}
	}
	t, ok := x.(string)
	if !ok {
		return &func0TypeError{"split", x}
	}
	if arrayTooLarge(strings.Count(s, t) + 1) {
		return &allocLimitError{}
	}
	ss := strings.Split(s, t)
	xs := make([]any, len(ss))
	for i, s := range ss {
		xs[i] = s
	}
	return xs
}

func funcJoin(v, x any) any {
	vs, err := values(v)
	if err != nil {
		if err == errNotIterable {
			return &func1TypeError{"join", v, x}
		}
		return err
	}
	if len(vs) == 0 {
		return ""
	}
	return add(func(yield func(any) bool) {
		for i, v := range vs {
			s := x
			if i == 0 {
				s = ""
			}
			if !yield(s) {
				return
			}
			switch w := v.(type) {
			case bool, int, float64, *big.Int, json.Number:
				v = jsonMarshal(w)
			}
			if !yield(v) {
				return
			}
		}
	})
}

func funcASCIIDowncase(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"ascii_downcase", v}
	}
	return strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

func funcASCIIUpcase(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"ascii_upcase", v}
	}
	return strings.Map(func(r rune) rune {
		if 'a' <= r && r <= 'z' {
			return r - ('a' - 'A')
		}
		return r
	}, s)
}

func funcToJSON(v any) any {
	s, err := marshalBounded(v)
	if err != nil {
		return err
	}
	return s
}

func funcFromJSON(v any) any {
	s, ok := v.(string)
	if !ok {
		return &func0TypeError{"fromjson", v}
	}
	var w any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if MaxAlloc > 0 {
		var size int64
		var err error
		if w, err = decodeJSONLimited(dec, &size, 0); err != nil {
			if _, ok := err.(*allocLimitError); ok {
				return err
			}
			return &func0WrapError{"fromjson", v, err}
		}
	} else if err := dec.Decode(&w); err != nil {
		return &func0WrapError{"fromjson", v, err}
	}
	if _, err := dec.Token(); err != io.EOF {
		return &func0TypeError{"fromjson", v}
	}
	return w
}

func funcFormat(v, x any) any {
	s, ok := x.(string)
	if !ok {
		return &func0TypeError{"format", x}
	}
	format := "@" + s
	f := formatToFunc(format)
	if f == nil {
		return &formatNotFoundError{format}
	}
	return internalFuncs[f.Name].callback(v, nil)
}

var htmlEscaper = strings.NewReplacer(
	`<`, "&lt;",
	`>`, "&gt;",
	`&`, "&amp;",
	`'`, "&apos;",
	`"`, "&quot;",
)

func funcToHTML(v any) any {
	switch x := funcToString(v).(type) {
	case string:
		s, ok := boundedReplace(htmlEscaper.Replace, x)
		if !ok {
			return &allocLimitError{}
		}
		return s
	default:
		return x
	}
}

func funcToURI(v any) any {
	switch x := funcToString(v).(type) {
	case string:
		s, ok := boundedReplace(func(s string) string {
			return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
		}, x)
		if !ok {
			return &allocLimitError{}
		}
		return s
	default:
		return x
	}
}

func funcToURId(v any) any {
	switch x := funcToString(v).(type) {
	case string:
		x, err := url.QueryUnescape(strings.ReplaceAll(x, "+", "%2B"))
		if err != nil {
			return &func0WrapError{"@urid", v, err}
		}
		return x
	default:
		return x
	}
}

var csvEscaper = strings.NewReplacer(
	`"`, `""`,
	"\x00", `\0`,
)

func funcToCSV(v any) any {
	return formatJoin("csv", v, ",", `"`, csvEscaper.Replace)
}

var tsvEscaper = strings.NewReplacer(
	"\t", `\t`,
	"\r", `\r`,
	"\n", `\n`,
	"\\", `\\`,
	"\x00", `\0`,
)

func funcToTSV(v any) any {
	return formatJoin("tsv", v, "\t", "", tsvEscaper.Replace)
}

var shEscaper = strings.NewReplacer(
	"'", `'\''`,
	"\x00", `\0`,
)

func funcToSh(v any) any {
	if _, ok := v.([]any); !ok {
		v = []any{v}
	}
	return formatJoin("sh", v, " ", "'", shEscaper.Replace)
}

func formatJoin(typ string, v any, sep, quote string, escape func(string) string) any {
	vs, ok := v.([]any)
	if !ok {
		return &func0TypeError{"@" + typ, v}
	}
	if arrayTooLarge(len(vs)) {
		return &allocLimitError{}
	}
	ss := make([]string, len(vs))
	var total int64
	for i, v := range vs {
		switch v := v.(type) {
		case []any, map[string]any:
			return &formatRowError{typ, v}
		case string:
			s, ok := boundedReplace(escape, v)
			if !ok {
				return &allocLimitError{}
			}
			ss[i] = quote + s + quote
		default:
			s, err := marshalBounded(v)
			if err != nil {
				return err
			}
			if s != "null" || typ == "sh" {
				ss[i] = s
			}
		}
		if MaxAlloc > 0 {
			if total += int64(len(ss[i])) + 16; total > MaxAlloc {
				return &allocLimitError{}
			}
		}
	}
	return strings.Join(ss, sep)
}

// boundedReplace applies a single-byte-replacement escaper to s, stopping once
// the escaped output would pass MaxAlloc. The csv/tsv/sh escapers each replace
// one byte at a time, so escaping fixed-size chunks and concatenating gives the
// same result as escaping the whole string, while never building more than the
// limit before the check fires. A quote-heavy field that would expand to many
// times its size is caught here instead of after it is fully materialized.
func boundedReplace(escape func(string) string, s string) (string, bool) {
	if MaxAlloc <= 0 {
		return escape(s), true
	}
	const chunk = 1 << 16
	if len(s) <= chunk {
		out := escape(s)
		return out, int64(len(out)) <= MaxAlloc
	}
	var b strings.Builder
	for i := 0; i < len(s); i += chunk {
		j := min(i+chunk, len(s))
		b.WriteString(escape(s[i:j]))
		if int64(b.Len()) > MaxAlloc {
			return "", false
		}
	}
	return b.String(), true
}

func funcToBase64(v any) any {
	switch x := funcToString(v).(type) {
	case string:
		if MaxAlloc > 0 && int64(base64.StdEncoding.EncodedLen(len(x))) > MaxAlloc {
			return &allocLimitError{}
		}
		return base64.StdEncoding.EncodeToString([]byte(x))
	default:
		return x
	}
}

func funcToBase64d(v any) any {
	switch x := funcToString(v).(type) {
	case string:
		if i := strings.IndexRune(x, base64.StdPadding); i >= 0 {
			x = x[:i]
		}
		y, err := base64.RawStdEncoding.DecodeString(x)
		if err != nil {
			return &func0WrapError{"@base64d", v, err}
		}
		return string(y)
	default:
		return x
	}
}

func funcIndex2(_, v, x any) any {
	switch x := x.(type) {
	case string:
		switch v := v.(type) {
		case nil:
			return nil
		case map[string]any:
			return v[x]
		default:
			return &expectedObjectError{v}
		}
	case int, float64, *big.Int, json.Number:
		i, _ := toInt(x)
		switch v := v.(type) {
		case nil:
			return nil
		case []any:
			return index(v, i)
		case string:
			return indexString(v, i)
		default:
			return &expectedArrayError{v}
		}
	case []any:
		switch v := v.(type) {
		case nil:
			return nil
		case []any:
			return indices(v, x)
		default:
			return &expectedArrayError{v}
		}
	case map[string]any:
		if v == nil {
			return nil
		}
		start, ok := x["start"]
		if !ok {
			return &expectedStartEndError{x}
		}
		end, ok := x["end"]
		if !ok {
			return &expectedStartEndError{x}
		}
		return funcSlice(nil, v, end, start)
	default:
		switch v.(type) {
		case []any:
			return &arrayIndexNotNumberError{x}
		case string:
			return &stringIndexNotNumberError{x}
		default:
			return &objectKeyNotStringError{x}
		}
	}
}

func index(vs []any, i int) any {
	i = clampIndex(i, -1, len(vs))
	if 0 <= i && i < len(vs) {
		return vs[i]
	}
	return nil
}

func indexString(s string, i int) any {
	l := utf8.RuneCountInString(s)
	i = clampIndex(i, -1, l)
	if 0 <= i && i < l {
		for _, r := range s {
			if i--; i < 0 {
				return string(r)
			}
		}
	}
	return nil
}

func funcSlice(_, v, e, s any) (r any) {
	switch v := v.(type) {
	case nil:
		return nil
	case []any:
		return slice(v, e, s)
	case string:
		return sliceString(v, e, s)
	default:
		return &expectedArrayError{v}
	}
}

func slice(vs []any, e, s any) any {
	var start, end int
	if s != nil {
		if i, ok := toInt(s); ok {
			start = clampIndex(i, 0, len(vs))
		} else {
			return &arrayIndexNotNumberError{s}
		}
	}
	if e != nil {
		if i, ok := toIntCeil(e); ok {
			end = clampIndex(i, start, len(vs))
		} else {
			return &arrayIndexNotNumberError{e}
		}
	} else {
		end = len(vs)
	}
	return vs[start:end]
}

func sliceString(v string, e, s any) any {
	var start, end int
	l := utf8.RuneCountInString(v)
	if s != nil {
		if i, ok := toInt(s); ok {
			start = clampIndex(i, 0, l)
		} else {
			return &stringIndexNotNumberError{s}
		}
	}
	if e != nil {
		if i, ok := toIntCeil(e); ok {
			end = clampIndex(i, start, l)
		} else {
			return &stringIndexNotNumberError{e}
		}
	} else {
		end = l
	}
	if start < l {
		for i := range v {
			if start--; start < 0 {
				start = i
				break
			}
		}
	} else {
		start = len(v)
	}
	if end < l {
		for i := range v {
			if end--; end < 0 {
				end = i
				break
			}
		}
	} else {
		end = len(v)
	}
	return v[start:end]
}

func clampIndex(i, minimum, maximum int) int {
	if i < 0 {
		i += maximum
	}
	if i < minimum {
		return minimum
	} else if i < maximum {
		return i
	} else {
		return maximum
	}
}

func funcFlatten(v any, args []any) (r any) {
	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(*allocLimitError); ok {
				r = &allocLimitError{}
				return
			}
			panic(e)
		}
	}()
	vs, err := values(v)
	if err != nil {
		if err == errNotIterable {
			return &func0TypeError{"flatten", v}
		}
		return err
	}
	var depth float64
	if len(args) == 0 {
		depth = -1
	} else {
		var ok bool
		depth, ok = toFloat(args[0])
		if !ok {
			return &func0TypeError{"flatten", args[0]}
		}
		if lt(depth, 0) {
			return &flattenDepthError{depth}
		}
	}
	return flatten([]any{}, vs, depth, 0)
}

func flatten(xs, vs []any, depth float64, rec int) []any {
	// bound the Go recursion depth so a deeply nested array cannot overflow the
	// goroutine stack; a value this deep already exceeds the limit.
	if MaxAlloc > 0 && (int64(rec)*16 > MaxAlloc || rec > maxRecursionDepth) {
		panic(&allocLimitError{})
	}
	for _, v := range vs {
		if vs, ok := v.([]any); ok && depth != 0 {
			xs = flatten(xs, vs, depth-1, rec+1)
		} else {
			xs = append(xs, v)
			if arrayTooLarge(len(xs)) {
				panic(&allocLimitError{})
			}
		}
	}
	return xs
}

type rangeIter struct {
	value, end, step any
}

func (iter *rangeIter) Next() (any, bool) {
	if Compare(iter.step, 0)*Compare(iter.value, iter.end) >= 0 {
		return nil, false
	}
	v := iter.value
	iter.value = funcOpAdd(nil, v, iter.step)
	return v, true
}

func funcRange(_ any, xs []any) any {
	for _, x := range xs {
		switch x.(type) {
		case int, float64, *big.Int, json.Number:
		default:
			return &func0TypeError{"range", x}
		}
	}
	return &rangeIter{xs[0], xs[1], xs[2]}
}

func funcMin(v any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func0TypeError{"min", v}
	}
	return minMaxBy(vs, vs, true)
}

func funcMinBy(v, x any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func1TypeError{"min_by", v, x}
	}
	xs, ok := x.([]any)
	if !ok {
		return &func1TypeError{"min_by", v, x}
	}
	if len(vs) != len(xs) {
		return &func1WrapError{"min_by", v, x, &lengthMismatchError{}}
	}
	return minMaxBy(vs, xs, true)
}

func funcMax(v any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func0TypeError{"max", v}
	}
	return minMaxBy(vs, vs, false)
}

func funcMaxBy(v, x any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func1TypeError{"max_by", v, x}
	}
	xs, ok := x.([]any)
	if !ok {
		return &func1TypeError{"max_by", v, x}
	}
	if len(vs) != len(xs) {
		return &func1WrapError{"max_by", v, x, &lengthMismatchError{}}
	}
	return minMaxBy(vs, xs, false)
}

func minMaxBy(vs, xs []any, isMin bool) any {
	if len(vs) == 0 {
		return nil
	}
	i, j, x := 0, 0, xs[0]
	for i++; i < len(xs); i++ {
		if Compare(x, xs[i]) > 0 == isMin {
			j, x = i, xs[i]
		}
	}
	return vs[j]
}

type sortItem struct {
	value, key any
}

func sortItems(name string, v, x any) ([]*sortItem, error) {
	vs, ok := v.([]any)
	if !ok {
		if strings.HasSuffix(name, "_by") {
			return nil, &func1TypeError{name, v, x}
		}
		return nil, &func0TypeError{name, v}
	}
	xs, ok := x.([]any)
	if !ok {
		return nil, &func1TypeError{name, v, x}
	}
	if len(vs) != len(xs) {
		return nil, &func1WrapError{name, v, x, &lengthMismatchError{}}
	}
	// Sorting holds an 8-byte pointer slice plus one 32-byte sortItem per input
	// and then a 16-byte result slot per input. Preflight the unavoidable peak so
	// a single call cannot allocate past MaxAlloc before its result is charged.
	if MaxAlloc > 0 && int64(len(vs)) > MaxAlloc/56 {
		return nil, &allocLimitError{}
	}
	items := make([]*sortItem, len(vs))
	for i, v := range vs {
		items[i] = &sortItem{v, xs[i]}
	}
	slices.SortStableFunc(items, func(x, y *sortItem) int {
		return Compare(x.key, y.key)
	})
	return items, nil
}

func funcSort(v any) any {
	return sortBy("sort", v, v)
}

func funcSortBy(v, x any) any {
	return sortBy("sort_by", v, x)
}

func sortBy(name string, v, x any) any {
	items, err := sortItems(name, v, x)
	if err != nil {
		return err
	}
	if arrayTooLarge(len(items)) {
		return &allocLimitError{}
	}
	rs := make([]any, len(items))
	for i, x := range items {
		rs[i] = x.value
	}
	return rs
}

func funcGroupBy(v, x any) any {
	items, err := sortItems("group_by", v, x)
	if err != nil {
		return err
	}
	rs := []any{}
	var last any
	for i, r := range items {
		if i == 0 || Compare(last, r.key) != 0 {
			rs, last = append(rs, []any{r.value}), r.key
		} else {
			rs[len(rs)-1] = append(rs[len(rs)-1].([]any), r.value)
		}
	}
	return rs
}

func funcUnique(v any) any {
	return uniqueBy("unique", v, v)
}

func funcUniqueBy(v, x any) any {
	return uniqueBy("unique_by", v, x)
}

func uniqueBy(name string, v, x any) any {
	items, err := sortItems(name, v, x)
	if err != nil {
		return err
	}
	rs := []any{}
	var last any
	for i, r := range items {
		if i == 0 || Compare(last, r.key) != 0 {
			rs, last = append(rs, r.value), r.key
		}
	}
	return rs
}

func funcSignificand(v float64) float64 {
	frac, _ := math.Frexp(v)
	return frac * 2
}

func funcExp10(v float64) float64 {
	return math.Pow(10, v)
}

func funcFrexp(v any) any {
	x, ok := toFloat(v)
	if !ok {
		return &func0TypeError{"frexp", v}
	}
	f, e := math.Frexp(x)
	return []any{f, e}
}

func funcModf(v any) any {
	x, ok := toFloat(v)
	if !ok {
		return &func0TypeError{"modf", v}
	}
	if math.IsInf(x, 0) {
		return []any{math.Copysign(0, x), x}
	}
	i, f := math.Modf(x)
	return []any{f, i}
}

func funcLgamma(v float64) float64 {
	v, _ = math.Lgamma(v)
	return v
}

func funcDrem(l, r float64) float64 {
	x := math.Remainder(l, r)
	if x == 0.0 {
		return math.Copysign(x, l)
	}
	return x
}

func funcFmax(l, r float64) float64 {
	if math.IsNaN(l) {
		return r
	}
	if math.IsNaN(r) {
		return l
	}
	return max(l, r)
}

func funcFmin(l, r float64) float64 {
	if math.IsNaN(l) {
		return r
	}
	if math.IsNaN(r) {
		return l
	}
	return min(l, r)
}

func funcJn(l, r float64) float64 {
	return math.Jn(int(l), r)
}

func funcLdexp(l, r float64) float64 {
	return math.Ldexp(l, int(r))
}

func funcYn(l, r float64) float64 {
	return math.Yn(int(l), r)
}

func funcInfinite(any) any {
	return math.Inf(1)
}

func funcIsfinite(v any) any {
	x, ok := toFloat(v)
	return ok && !math.IsInf(x, 0)
}

func funcIsinfinite(v any) any {
	x, ok := toFloat(v)
	return ok && math.IsInf(x, 0)
}

func funcNan(any) any {
	return math.NaN()
}

func funcIsnan(v any) any {
	x, ok := toFloat(v)
	if !ok {
		if v == nil {
			return false
		}
		return &func0TypeError{"isnan", v}
	}
	return math.IsNaN(x)
}

func funcIsnormal(v any) any {
	if v, ok := toFloat(v); ok {
		e := (math.Float64bits(v) & 0x7ff0000000000000) >> 52
		return 0 < e && e < 0x7ff
	}
	return false
}

// An `allocator` creates new maps and slices, stores the allocated addresses.
// This allocator is used to reduce allocations on assignment operator (`=`),
// update-assignment operator (`|=`), and the `map_values`, `del`, `delpaths`
// functions.
type allocator map[uintptr]struct{}

func funcAllocator(any, []any) any {
	return allocator{}
}

func (a allocator) allocated(v any) bool {
	_, ok := a[reflect.ValueOf(v).Pointer()]
	return ok
}

func (a allocator) makeObject(l int) map[string]any {
	v := make(map[string]any, l)
	if a != nil {
		a[reflect.ValueOf(v).Pointer()] = struct{}{}
	}
	return v
}

func (a allocator) makeArray(l, c int) []any {
	v := make([]any, l, max(l, c))
	if a != nil {
		a[reflect.ValueOf(v).Pointer()] = struct{}{}
	}
	return v
}

func funcSetpath(v, p, n any) any {
	// There is no need to use an allocator on a single update.
	return setpath(v, p, n, nil)
}

// Used in compiler#compileAssign and compiler#compileModify.
func funcSetpathWithAllocator(v any, args []any) any {
	return setpath(v, args[0], args[1], args[2].(allocator))
}

func setpath(v, p, n any, a allocator) any {
	path, ok := p.([]any)
	if !ok {
		return &func1TypeError{"setpath", v, p}
	}
	var size int64
	u, err := update(v, path, n, a, &size)
	if err != nil {
		return &func2WrapError{"setpath", v, p, n, err}
	}
	return u
}

func funcDelpaths(v, p any) any {
	return delpaths(v, p, allocator{})
}

// Used in compiler#compileAssign and compiler#compileModify.
func funcDelpathsWithAllocator(v any, args []any) any {
	return delpaths(v, args[0], args[1].(allocator))
}

func delpaths(v, p any, a allocator) any {
	paths, ok := p.([]any)
	if !ok {
		return &func1TypeError{"delpaths", v, p}
	}
	if len(paths) == 0 {
		return v
	}
	// Fills the paths with an empty value and then delete them. We cannot delete
	// in each loop because array indices should not change. For example,
	//   jq -n "[0, 1, 2, 3] | delpaths([[1], [2]])" #=> [0, 3].
	var empty struct{}
	var err error
	var size int64
	u := v
	for _, q := range paths {
		path, ok := q.([]any)
		if !ok {
			return &func1WrapError{"delpaths", v, p, &expectedArrayError{q}}
		}
		u, err = update(u, path, empty, a, &size)
		if err != nil {
			return &func1WrapError{"delpaths", v, p, err}
		}
	}
	return deleteEmpty(u)
}

func update(v any, path []any, n any, a allocator, size *int64) (any, error) {
	// update recurses once per path element ( updateObject/Index/Slice call back
	// into update ), and the size counters only charge on the way UP - so a long
	// path from input, which is not metered, would overflow the goroutine stack on
	// the way DOWN before any charge fires. The path length is the recursion depth ,
	// and a path this long builds a structure past the limit ( >= 16 bytes/level ).
	if MaxAlloc > 0 && (int64(len(path))*16 > MaxAlloc || len(path) > maxRecursionDepth) {
		return nil, &allocLimitError{}
	}
	if len(path) == 0 {
		return n, nil
	}
	switch p := path[0].(type) {
	case string:
		switch v := v.(type) {
		case nil:
			return updateObject(nil, p, path[1:], n, a, size)
		case map[string]any:
			return updateObject(v, p, path[1:], n, a, size)
		case struct{}:
			return v, nil
		default:
			return nil, &expectedObjectError{v}
		}
	case int, float64, *big.Int, json.Number:
		i, _ := toInt(p)
		switch v := v.(type) {
		case nil:
			return updateArrayIndex(nil, i, path[1:], n, a, size)
		case []any:
			return updateArrayIndex(v, i, path[1:], n, a, size)
		case struct{}:
			return v, nil
		default:
			return nil, &expectedArrayError{v}
		}
	case map[string]any:
		switch v := v.(type) {
		case nil:
			return updateArraySlice(nil, p, path[1:], n, a, size)
		case []any:
			return updateArraySlice(v, p, path[1:], n, a, size)
		case struct{}:
			return v, nil
		default:
			return nil, &expectedArrayError{v}
		}
	default:
		switch v.(type) {
		case []any:
			return nil, &arrayIndexNotNumberError{p}
		default:
			return nil, &objectKeyNotStringError{p}
		}
	}
}

func updateObject(v map[string]any, k string, path []any, n any, a allocator, size *int64) (any, error) {
	x, ok := v[k]
	if !ok && n == struct{}{} {
		if v == nil {
			return nil, nil
		}
		return v, nil
	}
	u, err := update(x, path, n, a, size)
	if err != nil {
		return nil, err
	}
	if a.allocated(v) {
		v[k] = u
		return v, nil
	}
	if MaxAlloc > 0 {
		if *size += int64(len(v)+1)*24 + 16; *size > MaxAlloc {
			return nil, &allocLimitError{}
		}
	}
	w := a.makeObject(len(v) + 1)
	maps.Copy(w, v)
	w[k] = u
	return w, nil
}

func updateArrayIndex(v []any, i int, path []any, n any, a allocator, size *int64) (any, error) {
	var x any
	if j := clampIndex(i, -1, len(v)); j < 0 {
		if n == struct{}{} {
			if v == nil {
				return nil, nil
			}
			return v, nil
		}
		return nil, &arrayIndexNegativeError{i}
	} else if j < len(v) {
		i = j
		x = v[i]
	} else {
		if n == struct{}{} {
			if v == nil {
				return nil, nil
			}
			return v, nil
		}
		if i >= 0x20000000 {
			return nil, &arrayIndexTooLargeError{i}
		}
		if MaxAlloc > 0 && int64(i+1)*16 > MaxAlloc {
			return nil, &arrayIndexTooLargeError{i}
		}
	}
	u, err := update(x, path, n, a, size)
	if err != nil {
		return nil, err
	}
	l, c := len(v), cap(v)
	if a.allocated(v) {
		if i < c {
			if i >= l {
				v = v[:i+1]
			}
			v[i] = u
			return v, nil
		}
		c *= 2
	}
	if i >= l {
		l = i + 1
	}
	if MaxAlloc > 0 {
		if *size += int64(l)*16 + 16; *size > MaxAlloc {
			return nil, &allocLimitError{}
		}
	}
	w := a.makeArray(l, c)
	copy(w, v)
	w[i] = u
	return w, nil
}

func updateArraySlice(v []any, m map[string]any, path []any, n any, a allocator, size *int64) (any, error) {
	s, ok := m["start"]
	if !ok {
		return nil, &expectedStartEndError{m}
	}
	e, ok := m["end"]
	if !ok {
		return nil, &expectedStartEndError{m}
	}
	var start, end int
	if s != nil {
		if i, ok := toInt(s); ok {
			start = clampIndex(i, 0, len(v))
		} else {
			return nil, &arrayIndexNotNumberError{s}
		}
	}
	if e != nil {
		if i, ok := toIntCeil(e); ok {
			end = clampIndex(i, start, len(v))
		} else {
			return nil, &arrayIndexNotNumberError{e}
		}
	} else {
		end = len(v)
	}
	if start == end && n == struct{}{} {
		if v == nil {
			return nil, nil
		}
		return v, nil
	}
	u, err := update(v[start:end], path, n, a, size)
	if err != nil {
		return nil, err
	}
	switch u := u.(type) {
	case []any:
		var w []any
		if len(u) == end-start && a.allocated(v) {
			w = v
		} else {
			if MaxAlloc > 0 {
				if *size += int64(len(v)-(end-start)+len(u))*16 + 16; *size > MaxAlloc {
					return nil, &allocLimitError{}
				}
			}
			w = a.makeArray(len(v)-(end-start)+len(u), 0)
			copy(w, v[:start])
			copy(w[start+len(u):], v[end:])
		}
		copy(w[start:], u)
		return w, nil
	case struct{}:
		var w []any
		if a.allocated(v) {
			w = v
		} else {
			if MaxAlloc > 0 {
				if *size += int64(len(v))*16 + 16; *size > MaxAlloc {
					return nil, &allocLimitError{}
				}
			}
			w = a.makeArray(len(v), 0)
			copy(w, v)
		}
		for i := start; i < end; i++ {
			w[i] = u
		}
		return w, nil
	default:
		return nil, &expectedArrayError{u}
	}
}

func deleteEmpty(v any) any {
	return deleteEmptyDepth(v, 0)
}

// deleteEmptyDepth is deleteEmpty with a recursion-depth bound. delpaths walks
// the whole result to strip empty markers, recursing on nesting; a deeply nested
// sibling would overflow the goroutine stack. Past the depth the value already
// exceeds the limit ; the interpreter's Next recovers the panic.
func deleteEmptyDepth(v any, depth int) any {
	if MaxAlloc > 0 && (int64(depth)*16 > MaxAlloc || depth > maxRecursionDepth) {
		panic(&allocLimitError{})
	}
	switch v := v.(type) {
	case struct{}:
		return nil
	case map[string]any:
		for k, w := range v {
			if w == struct{}{} {
				delete(v, k)
			} else {
				v[k] = deleteEmptyDepth(w, depth+1)
			}
		}
		return v
	case []any:
		var j int
		for _, w := range v {
			if w != struct{}{} {
				v[j] = deleteEmptyDepth(w, depth+1)
				j++
			}
		}
		for i := j; i < len(v); i++ {
			v[i] = nil
		}
		return v[:j]
	default:
		return v
	}
}

func funcGetpath(v, p any) any {
	path, ok := p.([]any)
	if !ok {
		return &func1TypeError{"getpath", v, p}
	}
	u := v
	for _, x := range path {
		switch v.(type) {
		case nil, []any, map[string]any:
			v = funcIndex2(nil, v, x)
			if err, ok := v.(error); ok {
				return &func1WrapError{"getpath", u, p, err}
			}
		default:
			return &func1TypeError{"getpath", u, p}
		}
	}
	return v
}

func funcTranspose(v any) any {
	vss, ok := v.([]any)
	if !ok {
		return &func0TypeError{"transpose", v}
	}
	if len(vss) == 0 {
		return []any{}
	}
	var l int
	for _, vs := range vss {
		vs, ok := vs.([]any)
		if !ok {
			return &func0TypeError{"transpose", v}
		}
		if k := len(vs); l < k {
			l = k
		}
	}
	if MaxAlloc > 0 && int64(l)*int64(len(vss))*16 > MaxAlloc {
		return &allocLimitError{}
	}
	wss := make([][]any, l)
	xs := make([]any, l)
	for i, k := 0, len(vss); i < l; i++ {
		s := make([]any, k)
		wss[i] = s
		xs[i] = s
	}
	for i, vs := range vss {
		for j, v := range vs.([]any) {
			wss[j][i] = v
		}
	}
	return xs
}

func funcBsearch(v, t any) any {
	vs, ok := v.([]any)
	if !ok {
		return &func1TypeError{"bsearch", v, t}
	}
	i := sort.Search(len(vs), func(i int) bool {
		return Compare(vs[i], t) >= 0
	})
	if i < len(vs) && Compare(vs[i], t) == 0 {
		return i
	}
	return -i - 1
}

func funcGmtime(v any) any {
	if v, ok := toFloat(v); ok {
		return epochToArray(v, time.UTC)
	}
	return &func0TypeError{"gmtime", v}
}

func funcLocaltime(v any) any {
	if v, ok := toFloat(v); ok {
		return epochToArray(v, time.Local)
	}
	return &func0TypeError{"localtime", v}
}

func epochToArray(v float64, loc *time.Location) []any {
	t := time.Unix(int64(v), int64((v-math.Floor(v))*1e9)).In(loc)
	return []any{
		t.Year(),
		int(t.Month()) - 1,
		t.Day(),
		t.Hour(),
		t.Minute(),
		float64(t.Second()) + float64(t.Nanosecond())/1e9,
		int(t.Weekday()),
		t.YearDay() - 1,
	}
}

func funcMktime(v any) any {
	a, ok := v.([]any)
	if !ok {
		return &func0TypeError{"mktime", v}
	}
	t, err := arrayToTime(a, time.UTC)
	if err != nil {
		return &func0WrapError{"mktime", v, err}
	}
	return timeToEpoch(t)
}

func timeToEpoch(t time.Time) float64 {
	return float64(t.Unix()) + float64(t.Nanosecond())/1e9
}

// boundedStrftime formats t with format but rejects a format long enough that
// its expansion could pass MaxAlloc. timefmt.Format builds the whole result in
// one pass, and directives such as %A / %B / %c expand a two-byte directive to
// many bytes, so a big format taken from input ( strftime(.field) ) could
// amplify far past the limit before the value meter, which sees only the
// finished string, could charge it. The widest directive expands under 16x per
// format byte, so this keeps a passing format's output under the limit.
func boundedStrftime(t time.Time, format string) any {
	if MaxAlloc > 0 && int64(len(format))*16 > MaxAlloc {
		return &allocLimitError{}
	}
	return timefmt.Format(t, format)
}

func funcStrftime(v, x any) any {
	if w, ok := toFloat(v); ok {
		v = epochToArray(w, time.UTC)
	}
	a, ok := v.([]any)
	if !ok {
		return &func1TypeError{"strftime", v, x}
	}
	format, ok := x.(string)
	if !ok {
		return &func1TypeError{"strftime", v, x}
	}
	t, err := arrayToTime(a, time.UTC)
	if err != nil {
		return &func1WrapError{"strftime", v, x, err}
	}
	return boundedStrftime(t, format)
}

func funcStrflocaltime(v, x any) any {
	if w, ok := toFloat(v); ok {
		v = epochToArray(w, time.Local)
	}
	a, ok := v.([]any)
	if !ok {
		return &func1TypeError{"strflocaltime", v, x}
	}
	format, ok := x.(string)
	if !ok {
		return &func1TypeError{"strflocaltime", v, x}
	}
	t, err := arrayToTime(a, time.Local)
	if err != nil {
		return &func1WrapError{"strflocaltime", v, x, err}
	}
	return boundedStrftime(t, format)
}

func funcStrptime(v, x any) any {
	s, ok := v.(string)
	if !ok {
		return &func1TypeError{"strptime", v, x}
	}
	format, ok := x.(string)
	if !ok {
		return &func1TypeError{"strptime", v, x}
	}
	// timefmt.Parse reads the whole input and format into runes before matching,
	// so a huge input ( which almost always fails on extra text anyway ) allocates
	// several times its size. Reject one whose parse could pass MaxAlloc; real date
	// strings are tiny, so only absurd inputs are refused.
	if MaxAlloc > 0 && (int64(len(s))*8 > MaxAlloc || int64(len(format))*8 > MaxAlloc) {
		return &allocLimitError{}
	}
	t, err := timefmt.Parse(s, format)
	if err != nil {
		return &func1WrapError{"strptime", v, x, err}
	}
	if t.Equal(time.Time{}) {
		return &func1TypeError{"strptime", v, x}
	}
	return epochToArray(timeToEpoch(t), time.UTC)
}

func arrayToTime(a []any, loc *time.Location) (time.Time, error) {
	var t time.Time
	var year, month, day, hour, minute,
		second, nanosecond, weekday, yearday int
	for i, p := range []*int{
		&year, &month, &day, &hour, &minute,
		&second, &weekday, &yearday,
	} {
		if i >= len(a) {
			break
		}
		if i == 5 {
			if v, ok := toFloat(a[i]); ok {
				*p = int(v)
				nanosecond = int((v - math.Floor(v)) * 1e9)
			} else {
				return t, &timeArrayError{}
			}
		} else if v, ok := toInt(a[i]); ok {
			*p = v
		} else {
			return t, &timeArrayError{}
		}
	}
	return time.Date(year, time.Month(month+1), day,
		hour, minute, second, nanosecond, loc), nil
}

func funcNow(any) any {
	return timeToEpoch(time.Now())
}

func funcMatch(v, re, fs, testing any, cache *reCache) any {
	var name string
	if testing == true {
		name = "test"
	} else {
		name = "match"
	}
	var flags string
	if fs != nil {
		var ok bool
		flags, ok = fs.(string)
		if !ok {
			return &func2TypeError{name, v, re, fs}
		}
	}
	s, ok := v.(string)
	if !ok {
		return &func2TypeError{name, v, re, fs}
	}
	restr, ok := re.(string)
	if !ok {
		return &func2TypeError{name, v, re, fs}
	}
	r, err := compileRegexp(restr, flags, cache)
	if err != nil {
		return err
	}
	if testing == true {
		return r.MatchString(s)
	}
	var n int
	capped := false
	if strings.ContainsRune(flags, 'g') {
		n = -1
		if MaxAlloc > 0 {
			// bound the number of matches so the result array (a map per match,
			// plus one per capture group) cannot exceed MaxAlloc.
			if lim := int(MaxAlloc/int64(600+r.NumSubexp()*256)) + 1; lim > 0 {
				n, capped = lim, true
			}
		}
	} else {
		n = 1
	}
	xs := r.FindAllStringSubmatchIndex(s, n)
	if capped && len(xs) >= n {
		return &allocLimitError{}
	}
	res, names := make([]any, len(xs)), r.SubexpNames()
	for i, x := range xs {
		captures := make([]any, (len(x)-2)/2)
		for j := 1; j < len(x)/2; j++ {
			var name any
			if n := names[j]; n != "" {
				name = n
			}
			if x[j*2] < 0 {
				captures[j-1] = map[string]any{
					"name":   name,
					"offset": -1,
					"length": 0,
					"string": nil,
				}
				continue
			}
			captures[j-1] = map[string]any{
				"name":   name,
				"offset": utf8.RuneCountInString(s[:x[j*2]]),
				"length": utf8.RuneCountInString(s[:x[j*2+1]]) - utf8.RuneCountInString(s[:x[j*2]]),
				"string": s[x[j*2]:x[j*2+1]],
			}
		}
		res[i] = map[string]any{
			"offset":   utf8.RuneCountInString(s[:x[0]]),
			"length":   utf8.RuneCountInString(s[:x[1]]) - utf8.RuneCountInString(s[:x[0]]),
			"string":   s[x[0]:x[1]],
			"captures": captures,
		}
	}
	return res
}

// reCache holds compiled regexps and tracks their approximate retained bytes,
// so a run that generates millions of distinct patterns cannot grow the cache
// without bound. When MaxAlloc is set and the cache is full, new patterns are
// still compiled and returned, just not retained.
type reCache struct {
	m     sync.Map
	bytes atomic.Int64
}

func compileRegexp(re, flags string, cache *reCache) (*regexp.Regexp, error) {
	key := [2]string{re, flags}
	if r, ok := cache.m.Load(key); ok {
		return r.(*regexp.Regexp), nil
	}
	if strings.IndexFunc(flags, func(r rune) bool {
		return r != 'g' && r != 'i' && r != 'm'
	}) >= 0 {
		return nil, fmt.Errorf("unsupported regular expression flag: %q", flags)
	}
	if strings.ContainsRune(flags, 'i') {
		re = "(?i)" + re
	}
	if strings.ContainsRune(flags, 'm') {
		re = "(?s)" + re
	}
	r, err := regexp.Compile(re)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression %q: %s", re, err)
	}
	if MaxAlloc <= 0 || cache.bytes.Load() < MaxAlloc {
		if _, loaded := cache.m.LoadOrStore(key, r); !loaded {
			cache.bytes.Add(int64(len(re))*128 + 1024)
		}
	}
	return r, nil
}

func funcCaptures(v any) any {
	captures, ok := v.([]any)
	if !ok {
		return &expectedArrayError{v}
	}
	w := make(map[string]any, len(captures))
	for _, capture := range captures {
		if capture, ok := capture.(map[string]any); ok {
			if name, ok := capture["name"].(string); ok {
				w[name] = capture["string"]
			}
		}
	}
	return w
}

func funcError(v any, args []any) any {
	if len(args) > 0 {
		v = args[0]
	}
	return &exitCodeError{v, 5}
}

func funcHalt(any) any {
	return &HaltError{nil, 0}
}

func funcHaltError(v any, args []any) any {
	code := 5
	if len(args) > 0 {
		var ok bool
		if code, ok = toInt(args[0]); !ok {
			return &func0TypeError{"halt_error", args[0]}
		}
	}
	return &HaltError{v, code}
}

func toInt(x any) (int, bool) {
	switch x := x.(type) {
	case int:
		return x, true
	case float64:
		return floatToInt(x), true
	case *big.Int:
		if x.IsInt64() {
			if i := x.Int64(); math.MinInt <= i && i <= math.MaxInt {
				return int(i), true
			}
		}
		if x.Sign() > 0 {
			return math.MaxInt, true
		}
		return math.MinInt, true
	case json.Number:
		return toInt(parseNumber(x))
	default:
		return 0, false
	}
}

func toIntCeil(x any) (int, bool) {
	if f, ok := x.(float64); ok {
		x = math.Ceil(f)
	}
	return toInt(x)
}

func floatToInt(x float64) int {
	if math.MinInt <= x && x < math.MaxInt {
		return int(x)
	}
	if x > 0 {
		return math.MaxInt
	}
	return math.MinInt
}

func toFloat(x any) (float64, bool) {
	switch x := x.(type) {
	case int:
		return float64(x), true
	case float64:
		return x, true
	case *big.Int:
		return bigToFloat(x), true
	case json.Number:
		v, err := x.Float64()
		return v, err == nil
	default:
		return 0.0, false
	}
}

func bigToFloat(x *big.Int) float64 {
	if x.IsInt64() {
		return float64(x.Int64())
	}
	// Convert through big.Float (a binary copy) rather than x.String(), whose
	// base-10 conversion allocates large superlinear scratch inside math/big: a
	// 10 MB integer divided by a small number reached ~300 MB, none of it seen by
	// the value meter. Float64 already yields +/-Inf on overflow.
	f, _ := new(big.Float).SetPrec(53).SetInt(x).Float64()
	return f
}

func parseNumber(v json.Number) any {
	s := v.String()
	if len(s) <= 20 {
		if i, err := v.Int64(); err == nil && math.MinInt <= i && i <= math.MaxInt {
			return int(i)
		}
	}
	// Decimal conversion uses superlinear scratch inside math/big. Mirror the
	// encoder's 384-bytes-per-word safety factor in the reverse direction and
	// reject by O(1) digit count before SetString starts work. Counting a sign or
	// decimal syntax byte as a digit is deliberately conservative for absurdly
	// large numbers and avoids a linear pre-scan on the attack path.
	if MaxAlloc > 0 && len(s) > 20 {
		digits := int64(len(s))
		words := (digits + 18) / 19 // at most 19 decimal digits per 64-bit word
		if words > MaxAlloc/384 {
			panic(&allocLimitError{})
		}
	}
	if strings.ContainsAny(s, ".eE") {
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	if bi, ok := new(big.Int).SetString(s, 10); ok {
		return bi
	}
	if strings.HasPrefix(s, "-") {
		return math.Inf(-1)
	}
	return math.Inf(1)
}
