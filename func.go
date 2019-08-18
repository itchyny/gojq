package gojq

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lestrrat-go/strftime"
	"github.com/pbnjay/strptime"
)

//go:generate go run _tools/gen_builtin.go -i builtin.jq -o builtin.go
var builtinFuncDefs map[string][]*FuncDef

const (
	argcount0 = 1 << iota
	argcount1
	argcount2
	argcount3
)

type function struct {
	argcount int
	callback func(interface{}, []interface{}) interface{}
}

func (fn function) accept(cnt int) bool {
	return fn.argcount&(1<<uint(cnt)) > 0
}

var internalFuncs map[string]function

func init() {
	internalFuncs = map[string]function{
		"empty":          argFunc0(nil),
		"path":           argFunc1(nil),
		"debug":          argFunc0(nil),
		"stderr":         argFunc0(nil),
		"halt":           argFunc0(nil),
		"length":         argFunc0(funcLength),
		"utf8bytelength": argFunc0(funcUtf8ByteLength),
		"keys":           argFunc0(funcKeys),
		"has":            argFunc1(funcHas),
		"tonumber":       argFunc0(funcToNumber),
		"tostring":       argFunc0(funcToString),
		"type":           argFunc0(funcType),
		"contains":       argFunc1(funcContains),
		"explode":        argFunc0(funcExplode),
		"implode":        argFunc0(funcImplode),
		"split":          function{argcount1 | argcount2, funcSplit},
		"tojson":         argFunc0(funcToJSON),
		"fromjson":       argFunc0(funcFromJSON),
		"_index":         argFunc2(funcIndex),
		"_slice":         argFunc3(funcSlice),
		"_break":         argFunc0(funcBreak),
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
		"nearbyint":      mathFunc("nearbyint", math.Round),
		"rint":           mathFunc("rint", math.Round),
		"ceil":           mathFunc("ceil", math.Ceil),
		"trunc":          mathFunc("trunc", math.Trunc),
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
		"fmax":           mathFunc2("fmax", math.Max),
		"fmin":           mathFunc2("fmin", math.Min),
		"fmod":           mathFunc2("fmod", math.Mod),
		"hypot":          mathFunc2("hypot", math.Hypot),
		"jn":             mathFunc2("jn", funcJn),
		"ldexp":          mathFunc2("ldexp", funcLdexp),
		"nextafter":      mathFunc2("nextafter", math.Nextafter),
		"nexttoward":     mathFunc2("nexttoward", math.Nextafter),
		"remainder":      mathFunc2("remainder", math.Remainder),
		"scalb":          mathFunc2("scalb", funcScalb),
		"scalbln":        mathFunc2("scalbln", funcScalbln),
		"yn":             mathFunc2("yn", funcYn),
		"pow":            mathFunc2("pow", math.Pow),
		"pow10":          mathFunc("pow10", funcExp10),
		"fma":            mathFunc3("fma", funcFma),
		"nan":            argFunc0(funcNan),
		"isnan":          argFunc0(funcIsnan),
		"setpath":        argFunc2(funcSetpath),
		"delpaths":       argFunc1(funcDelpaths),
		"getpath":        argFunc1(funcGetpath),
		"gmtime":         argFunc0(funcGmtime),
		"localtime":      argFunc0(funcLocaltime),
		"mktime":         argFunc0(funcMktime),
		"strftime":       argFunc1(funcStrftime),
		"strflocaltime":  argFunc1(funcStrflocaltime),
		"strptime":       argFunc1(funcStrptime),
		"now":            argFunc0(funcNow),
		"_match_impl":    argFunc3(funcMatchImpl),
		"error":          function{argcount0 | argcount1, funcError},
		"builtins":       argFunc0(funcBuiltins),
		"env":            argFunc0(funcEnv),
		"_type_error":    argFunc1(internalfuncTypeError),
	}
}

func argFunc0(fn func(interface{}) interface{}) function {
	return function{argcount0, func(v interface{}, _ []interface{}) interface{} {
		return fn(v)
	},
	}
}

func argFunc1(fn func(interface{}, interface{}) interface{}) function {
	return function{argcount1, func(v interface{}, args []interface{}) interface{} {
		return fn(v, args[0])
	},
	}
}

func argFunc2(fn func(interface{}, interface{}, interface{}) interface{}) function {
	return function{argcount2, func(v interface{}, args []interface{}) interface{} {
		return fn(v, args[0], args[1])
	},
	}
}

func argFunc3(fn func(interface{}, interface{}, interface{}, interface{}) interface{}) function {
	return function{argcount3, func(v interface{}, args []interface{}) interface{} {
		return fn(v, args[0], args[1], args[2])
	},
	}
}

func mathFunc(name string, f func(x float64) float64) function {
	return argFunc0(func(v interface{}) interface{} {
		x, ok := toFloat(v)
		if !ok {
			return &funcTypeError{name, v}
		}
		return f(x)
	})
}

func mathFunc2(name string, g func(x, y float64) float64) function {
	return argFunc2(func(_, x, y interface{}) interface{} {
		l, ok := toFloat(x)
		if !ok {
			return &funcTypeError{name, x}
		}
		r, ok := toFloat(y)
		if !ok {
			return &funcTypeError{name, y}
		}
		return g(l, r)
	})
}

func mathFunc3(name string, g func(x, y, z float64) float64) function {
	return argFunc3(func(_, a, b, c interface{}) interface{} {
		x, ok := toFloat(a)
		if !ok {
			return &funcTypeError{name, a}
		}
		y, ok := toFloat(b)
		if !ok {
			return &funcTypeError{name, b}
		}
		z, ok := toFloat(c)
		if !ok {
			return &funcTypeError{name, c}
		}
		return g(x, y, z)
	})
}

func funcLength(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return len(v)
	case map[string]interface{}:
		return len(v)
	case string:
		return len([]rune(v))
	case int:
		if v >= 0 {
			return v
		}
		return -v
	case float64:
		return math.Abs(v)
	case *big.Int:
		return new(big.Int).Abs(v)
	case nil:
		return 0
	default:
		return &funcTypeError{"length", v}
	}
}

func funcUtf8ByteLength(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		return len([]byte(v))
	default:
		return &funcTypeError{"utf8bytelength", v}
	}
}

func funcKeys(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		w := make([]interface{}, len(v))
		for i := range v {
			w[i] = i
		}
		return w
	case map[string]interface{}:
		w := make([]string, len(v))
		var i int
		for k := range v {
			w[i] = k
			i++
		}
		sort.Strings(w)
		u := make([]interface{}, len(v))
		for i, x := range w {
			u[i] = x
		}
		return u
	default:
		return &funcTypeError{"keys", v}
	}
}

func funcHas(v, x interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		if x, ok := toInt(x); ok {
			return 0 <= x && x < len(v)
		}
		return &hasKeyTypeError{v, x}
	case map[string]interface{}:
		switch x := x.(type) {
		case string:
			_, ok := v[x]
			return ok
		default:
			return &hasKeyTypeError{v, x}
		}
	default:
		return &hasKeyTypeError{v, x}
	}
}

func funcToNumber(v interface{}) interface{} {
	switch v := v.(type) {
	case int, float64, *big.Int:
		return v
	case string:
		var x float64
		if err := json.Unmarshal([]byte(v), &x); err != nil {
			return fmt.Errorf("%s: %q", err, v)
		}
		return x
	default:
		return &funcTypeError{"tonumber", v}
	}
}

func funcToString(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		return s
	}
	return funcToJSON(v)
}

func funcType(v interface{}) interface{} {
	return typeof(v)
}

func funcContains(v, x interface{}) interface{} {
	switch v := v.(type) {
	case nil:
		if x == nil {
			return true
		}
	case bool:
		switch x := x.(type) {
		case bool:
			if v == x {
				return true
			}
		}
	}
	return binopTypeSwitch(v, x,
		func(l, r int) interface{} { return l == r },
		func(l, r float64) interface{} { return l == r },
		func(l, r *big.Int) interface{} { return l.Cmp(r) == 0 },
		func(l, r string) interface{} { return strings.Contains(l, r) },
		func(l, r []interface{}) interface{} {
			for _, x := range r {
				var found bool
				for _, y := range l {
					if funcContains(y, x) == true {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		},
		func(l, r map[string]interface{}) interface{} {
			for k, rk := range r {
				lk, ok := l[k]
				if !ok {
					return false
				}
				c := funcContains(lk, rk)
				if _, ok := c.(error); ok {
					return false
				}
				if c == false {
					return false
				}
			}
			return true
		},
		func(l, r interface{}) interface{} { return &funcContainsError{l, r} },
	)
}

func funcExplode(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		return explode(v)
	default:
		return &funcTypeError{"explode", v}
	}
}

func explode(s string) []interface{} {
	rs := []int32(s)
	xs := make([]interface{}, len(rs))
	for i, r := range rs {
		xs[i] = int(r)
	}
	return xs
}

func funcImplode(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return implode(v)
	default:
		return &funcTypeError{"implode", v}
	}
}

func funcSplit(v interface{}, args []interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return &funcTypeError{"split", v}
	}
	x, ok := args[0].(string)
	if !ok {
		return &funcTypeError{"split", x}
	}
	var ss []string
	if len(args) == 1 {
		ss = strings.Split(s, x)
	} else {
		var flags string
		if args[1] != nil {
			v, ok := args[1].(string)
			if !ok {
				return &funcTypeError{"split", args[1]}
			}
			flags = v
		}
		r, err := compileRegexp(x, flags)
		if err != nil {
			return err
		}
		ss = r.Split(s, -1)
	}
	xs := make([]interface{}, len(ss))
	for i, s := range ss {
		xs[i] = s
	}
	return xs
}

func implode(v []interface{}) interface{} {
	var rs []rune
	for _, r := range v {
		if r, ok := toInt(r); ok {
			rs = append(rs, rune(r))
			continue
		}
		return &funcTypeError{"implode", v}
	}
	return string(rs)
}

func funcToJSON(v interface{}) interface{} {
	xs, err := json.Marshal(v)
	if err != nil {
		xs, err = json.Marshal(normalizeValues(v))
		if err != nil {
			return err
		}
	}
	return string(xs)
}

func funcFromJSON(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		var w interface{}
		err := json.Unmarshal([]byte(v), &w)
		if err != nil {
			return err
		}
		return w
	default:
		return &funcTypeError{"fromjson", v}
	}
}

func funcIndex(_, v, x interface{}) interface{} {
	switch x := x.(type) {
	case string:
		switch v := v.(type) {
		case nil:
			return nil
		case map[string]interface{}:
			return v[x]
		default:
			return &expectedObjectError{v}
		}
	case int, float64, *big.Int:
		idx, _ := toInt(x)
		switch v := v.(type) {
		case nil:
			return nil
		case []interface{}:
			return funcIndexSlice(nil, nil, &idx, v)
		case string:
			switch v := funcIndexSlice(nil, nil, &idx, explode(v)).(type) {
			case []interface{}:
				return implode(v)
			case int:
				return implode([]interface{}{v})
			case nil:
				return ""
			default:
				panic(v)
			}
		default:
			return &expectedArrayError{v}
		}
	case []interface{}:
		switch v := v.(type) {
		case nil:
			return nil
		case []interface{}:
			var xs []interface{}
			if len(x) == 0 {
				return xs
			}
			for i := 0; i < len(v) && i < len(v)-len(x)+1; i++ {
				var neq bool
				for j, y := range x {
					if neq = compare(v[i+j], y) != 0; neq {
						break
					}
				}
				if !neq {
					xs = append(xs, i)
				}
			}
			return xs
		default:
			return &expectedArrayError{v}
		}
	default:
		return &objectKeyNotStringError{x}
	}
}

func funcSlice(_, v, end, start interface{}) (r interface{}) {
	if w, ok := v.(string); ok {
		v = explode(w)
		defer func() {
			switch s := r.(type) {
			case []interface{}:
				r = implode(s)
			case int:
				r = implode([]interface{}{s})
			case nil:
				r = ""
			default:
				panic(r)
			}
		}()
	}
	switch v := v.(type) {
	case nil:
		return nil
	case []interface{}:
		if start != nil {
			if start, ok := toInt(start); ok {
				if end != nil {
					if end, ok := toInt(end); ok {
						return funcIndexSlice(&start, &end, nil, v)
					}
					return &arrayIndexNotNumberError{end}
				}
				return funcIndexSlice(&start, nil, nil, v)
			}
			return &arrayIndexNotNumberError{start}
		}
		if end != nil {
			if end, ok := toInt(end); ok {
				return funcIndexSlice(nil, &end, nil, v)
			}
			return &arrayIndexNotNumberError{end}
		}
		return v
	default:
		return &expectedArrayError{v}
	}
}

func funcIndexSlice(start, end, index *int, a []interface{}) interface{} {
	aa := a
	if index != nil {
		i := toIndex(aa, *index)
		if i < 0 {
			return nil
		}
		return a[i]
	}
	if end != nil {
		i := toIndex(aa, *end)
		if i == -1 {
			i = len(a)
		} else if i == -2 {
			i = 0
		}
		a = a[:i]
	}
	if start != nil {
		i := toIndex(aa, *start)
		if i == -1 || len(a) < i {
			i = len(a)
		} else if i == -2 {
			i = 0
		}
		a = a[i:]
	}
	return a
}

func toIndex(a []interface{}, i int) int {
	l := len(a)
	switch {
	case i < -l:
		return -2
	case i < 0:
		return l + i
	case i < l:
		return i
	default:
		return -1
	}
}

func funcBreak(x interface{}) interface{} {
	return &breakError{x.(string)}
}

func funcExp10(v float64) float64 {
	return math.Pow(10, v)
}

func funcLgamma(v float64) float64 {
	v, _ = math.Lgamma(v)
	return v
}

func funcFrexp(v interface{}) interface{} {
	x, ok := toFloat(v)
	if !ok {
		return &funcTypeError{"frexp", v}
	}
	f, e := math.Frexp(x)
	return []interface{}{f, e}
}

func funcModf(v interface{}) interface{} {
	x, ok := toFloat(v)
	if !ok {
		return &funcTypeError{"modf", v}
	}
	i, f := math.Modf(x)
	return []interface{}{f, i}
}

func funcDrem(l, r float64) float64 {
	x := math.Remainder(l, r)
	if x == 0.0 {
		return math.Copysign(x, l)
	}
	return x
}

func funcJn(l, r float64) float64 {
	return math.Jn(int(l), r)
}

func funcLdexp(l, r float64) float64 {
	return math.Ldexp(l, int(r))
}

func funcScalb(l, r float64) float64 {
	return l * math.Pow(2, r)
}

func funcScalbln(l, r float64) float64 {
	return l * math.Pow(2, r)
}

func funcYn(l, r float64) float64 {
	return math.Yn(int(l), r)
}

func funcFma(x, y, z float64) float64 {
	return x*y + z
}

func funcNan(interface{}) interface{} {
	return math.NaN()
}

func funcIsnan(v interface{}) interface{} {
	x, ok := toFloat(v)
	if !ok {
		return &funcTypeError{"isnan", v}
	}
	return math.IsNaN(x)
}

func funcSetpath(v, p, w interface{}) interface{} {
	return updatePaths("setpath", clone(v), p, func(interface{}) interface{} {
		return w
	})
}

func funcDelpaths(v, p interface{}) interface{} {
	paths, ok := p.([]interface{})
	if !ok {
		return &funcTypeError{"delpaths", p}
	}
	for _, path := range paths {
		v = updatePaths("delpaths", clone(v), path, func(interface{}) interface{} {
			return struct{}{}
		})
		if _, ok := v.(error); ok {
			return v
		}
	}
	return deleteEmpty(v)
}

func updatePaths(name string, v, p interface{}, f func(interface{}) interface{}) interface{} {
	keys, ok := p.([]interface{})
	if !ok {
		return &funcTypeError{name, p}
	}
	if len(keys) == 0 {
		return f(v)
	}
	u := v
	g := func(w interface{}) interface{} { v = w; return w }
loop:
	for i, x := range keys {
		switch x := x.(type) {
		case string:
			if u == nil {
				if name == "delpaths" {
					break loop
				}
				u = g(make(map[string]interface{}))
			}
			switch uu := u.(type) {
			case map[string]interface{}:
				if _, ok := uu[x]; !ok && name == "delpaths" {
					break loop
				}
				if i < len(keys)-1 {
					u = uu[x]
					g = func(w interface{}) interface{} { uu[x] = w; return w }
				} else {
					uu[x] = f(uu[x])
				}
			default:
				return &expectedObjectError{u}
			}
		case int, float64, *big.Int:
			if u == nil {
				u = g([]interface{}{})
			}
			y, _ := toInt(x)
			switch uu := u.(type) {
			case []interface{}:
				l := len(uu)
				if y >= len(uu) && name == "setpath" {
					l = y + 1
				} else if y < -len(uu) {
					if name == "delpaths" {
						break loop
					}
					return &funcTypeError{name, y}
				} else if y < 0 {
					y = len(uu) + y
				}
				ys := make([]interface{}, l)
				copy(ys, uu)
				uu = ys
				g(uu)
				if y >= len(uu) {
					break loop
				}
				if i < len(keys)-1 {
					u = uu[y]
					g = func(w interface{}) interface{} { uu[y] = w; return w }
				} else {
					uu[y] = f(uu[y])
				}
			default:
				return &expectedArrayError{u}
			}
		case map[string]interface{}:
			if len(x) == 0 {
				switch u.(type) {
				case []interface{}:
					return &arrayIndexNotNumberError{x}
				default:
					return &objectKeyNotStringError{x}
				}
			}
			if u == nil {
				u = g([]interface{}{})
			}
			switch uu := u.(type) {
			case []interface{}:
				var start, end int
				if x, ok := toInt(x["start"]); ok {
					x := toIndex(uu, x)
					if x > len(uu) || x == -1 {
						start = len(uu)
					} else if x == -2 {
						start = 0
					} else {
						start = x
					}
				}
				if x, ok := toInt(x["end"]); ok {
					x := toIndex(uu, x)
					if x < start {
						end = start
					} else {
						end = x
					}
				} else {
					end = len(uu)
				}
				if i < len(keys)-1 {
					u = uu[start]
					g = func(w interface{}) interface{} { uu[start] = w; return w }
				} else if name == "delpaths" {
					for y := start; y < end; y++ {
						uu[y] = f(nil)
					}
				} else {
					switch v := f(nil).(type) {
					case []interface{}:
						vv := make([]interface{}, start+len(v)+len(uu)-end)
						copy(vv, uu[:start])
						copy(vv[start:], v)
						copy(vv[start+len(v):], uu[end:])
						g(vv)
					default:
						return &expectedArrayError{v}
					}
				}
			default:
				return &expectedArrayError{u}
			}
		default:
			switch u.(type) {
			case []interface{}:
				return &arrayIndexNotNumberError{x}
			default:
				return &objectKeyNotStringError{x}
			}
		}
	}
	return v
}

func funcGetpath(v, p interface{}) interface{} {
	keys, ok := p.([]interface{})
	if !ok {
		return &funcTypeError{"getpath", p}
	}
	u := v
	for _, x := range keys {
		switch v.(type) {
		case map[string]interface{}:
		case []interface{}:
		case nil:
		default:
			return &getpathError{u, p}
		}
		v = funcIndex(nil, v, x)
		if _, ok := v.(error); ok {
			return &getpathError{u, p}
		}
	}
	return v
}

func funcGmtime(v interface{}) interface{} {
	if v, ok := toFloat(v); ok {
		return epochToArray(v, time.UTC)
	}
	return &funcTypeError{"gmtime", v}
}

func funcLocaltime(v interface{}) interface{} {
	if v, ok := toFloat(v); ok {
		return epochToArray(v, time.Local)
	}
	return &funcTypeError{"localtime", v}
}

func epochToArray(v float64, loc *time.Location) []interface{} {
	t := time.Unix(int64(v), int64((v-math.Floor(v))*1e9)).In(loc)
	return []interface{}{
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

func funcMktime(v interface{}) interface{} {
	if a, ok := v.([]interface{}); ok {
		t, err := arrayToTime("mktime", a, time.UTC)
		if err != nil {
			return err
		}
		return float64(t.Unix()) + float64(t.Nanosecond())/1e9
	}
	return &funcTypeError{"mktime", v}
}

func funcStrftime(v, x interface{}) interface{} {
	if w, ok := toFloat(v); ok {
		v = epochToArray(w, time.UTC)
	}
	if a, ok := v.([]interface{}); ok {
		if format, ok := x.(string); ok {
			t, err := arrayToTime("strftime", a, time.UTC)
			if err != nil {
				return err
			}
			got, err := strftime.Format(format, t)
			if err != nil {
				return err
			}
			return got
		}
		return &funcTypeError{"strftime", x}
	}
	return &funcTypeError{"strftime", v}
}

func funcStrflocaltime(v, x interface{}) interface{} {
	if w, ok := toFloat(v); ok {
		v = epochToArray(w, time.Local)
	}
	if a, ok := v.([]interface{}); ok {
		if format, ok := x.(string); ok {
			t, err := arrayToTime("strflocaltime", a, time.Local)
			if err != nil {
				return err
			}
			got, err := strftime.Format(format, t)
			if err != nil {
				return err
			}
			return got
		}
		return &funcTypeError{"strflocaltime", x}
	}
	return &funcTypeError{"strflocaltime", v}
}

func funcStrptime(v, x interface{}) interface{} {
	if v, ok := v.(string); ok {
		if format, ok := x.(string); ok {
			t, err := strptime.Parse(v, format)
			if err != nil {
				return err
			}
			var s time.Time
			if t == s {
				return &funcTypeError{"strptime", v}
			}
			return epochToArray(float64(t.Unix())+float64(t.Nanosecond())/1e9, time.UTC)
		}
		return &funcTypeError{"strptime", x}
	}
	return &funcTypeError{"strptime", v}
}

func arrayToTime(name string, a []interface{}, loc *time.Location) (time.Time, error) {
	var t time.Time
	if len(a) != 8 {
		return t, &funcTypeError{name, a}
	}
	var y, m, d, h, min, sec, nsec int
	if x, ok := toInt(a[0]); ok {
		y = x
	} else {
		return t, &funcTypeError{name, a}
	}
	if x, ok := toInt(a[1]); ok {
		m = x + 1
	} else {
		return t, &funcTypeError{name, a}
	}
	if x, ok := toInt(a[2]); ok {
		d = x
	} else {
		return t, &funcTypeError{name, a}
	}
	if x, ok := toInt(a[3]); ok {
		h = x
	} else {
		return t, &funcTypeError{name, a}
	}
	if x, ok := toInt(a[4]); ok {
		min = x
	} else {
		return t, &funcTypeError{name, a}
	}
	if x, ok := toFloat(a[5]); ok {
		sec = int(x)
		nsec = int((x - math.Floor(x)) * 1e9)
	} else {
		return t, &funcTypeError{name, a}
	}
	return time.Date(y, time.Month(m), d, h, min, sec, nsec, loc), nil
}

func funcNow(interface{}) interface{} {
	t := time.Now()
	return float64(t.Unix()) + float64(t.Nanosecond())/1e9
}

func funcMatchImpl(v, re, fs, testing interface{}) interface{} {
	var flags string
	if fs != nil {
		v, ok := fs.(string)
		if !ok {
			return &funcTypeError{"match", fs}
		}
		flags = v
	}
	s, ok := v.(string)
	if !ok {
		return &funcTypeError{"match", v}
	}
	restr, ok := re.(string)
	if !ok {
		return &funcTypeError{"match", v}
	}
	r, err := compileRegexp(restr, flags)
	if err != nil {
		return err
	}
	var xs [][]int
	if strings.ContainsRune(flags, 'g') && testing != true {
		xs = r.FindAllStringSubmatchIndex(s, -1)
	} else {
		got := r.FindStringSubmatchIndex(s)
		if testing == true {
			return got != nil
		}
		if got != nil {
			xs = [][]int{got}
		}
	}
	res, names := make([]interface{}, len(xs)), r.SubexpNames()
	for i, x := range xs {
		captures := make([]interface{}, (len(x)-2)/2)
		for j := 1; j < len(x)/2; j++ {
			var name interface{}
			if n := names[j]; n != "" {
				name = n
			}
			if x[j*2] < 0 {
				captures[j-1] = map[string]interface{}{
					"name":   name,
					"offset": -1,
					"length": 0,
					"string": nil,
				}
				continue
			}
			captures[j-1] = map[string]interface{}{
				"name":   name,
				"offset": len([]rune(s[:x[j*2]])),
				"length": len([]rune(s[:x[j*2+1]])) - len([]rune(s[:x[j*2]])),
				"string": s[x[j*2]:x[j*2+1]],
			}
		}
		res[i] = map[string]interface{}{
			"offset":   len([]rune(s[:x[0]])),
			"length":   len([]rune(s[:x[1]])) - len([]rune(s[:x[0]])),
			"string":   s[x[0]:x[1]],
			"captures": captures,
		}
	}
	return res
}

func compileRegexp(re, flags string) (*regexp.Regexp, error) {
	re = strings.ReplaceAll(re, "(?<", "(?P<")
	if strings.ContainsRune(flags, 'i') {
		re = "(?i)" + re
	}
	if strings.ContainsRune(flags, 'm') {
		re = "(?s)" + re
	}
	r, err := regexp.Compile(re)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression %q: %v", re, err)
	}
	return r, nil
}

func funcError(v interface{}, args []interface{}) interface{} {
	if len(args) == 0 {
		switch v := v.(type) {
		case string:
			return errors.New(v)
		default:
			return &funcTypeError{"error", v}
		}
	} else if len(args) == 1 {
		switch v := args[0].(type) {
		case string:
			return errors.New(v)
		default:
			return &funcTypeError{"error", v}
		}
	} else {
		return nil
	}
}

func funcBuiltins(interface{}) interface{} {
	var xs []string
	for name, fn := range internalFuncs {
		if name[0] != '_' {
			for i, cnt := 0, fn.argcount; cnt > 0; i, cnt = i+1, cnt>>1 {
				if cnt&1 > 0 {
					xs = append(xs, name+"/"+fmt.Sprint(i))
				}
			}
		}
	}
	for _, fds := range builtinFuncDefs {
		for _, fd := range fds {
			if fd.Name[0] != '_' {
				xs = append(xs, fd.Name+"/"+fmt.Sprint(len(fd.Args)))
			}
		}
	}
	sort.Strings(xs)
	ys := make([]interface{}, len(xs))
	for i, x := range xs {
		ys[i] = x
	}
	return ys
}

func funcEnv(interface{}) interface{} {
	env := make(map[string]interface{})
	for _, kv := range os.Environ() {
		xs := strings.SplitN(kv, "=", 2)
		env[xs[0]] = xs[1]
	}
	return env
}

func internalfuncTypeError(v, x interface{}) interface{} {
	return &funcTypeError{x.(string), v}
}

func toInt(x interface{}) (int, bool) {
	switch x := x.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	case *big.Int:
		if x.IsInt64() {
			return int(x.Int64()), true
		}
		if x.Sign() > 0 {
			return maxInt, true
		}
		return minInt, true
	default:
		return 0, false
	}
}

func toFloat(x interface{}) (float64, bool) {
	switch x := x.(type) {
	case int:
		return float64(x), true
	case float64:
		return x, true
	case *big.Int:
		return bigToFloat(x), true
	default:
		return 0.0, false
	}
}

func bigToFloat(x *big.Int) float64 {
	if x.IsInt64() {
		return float64(x.Int64())
	}
	bs, _ := json.Marshal(x)
	if f, err := json.Number(string(bs)).Float64(); err == nil {
		return f
	}
	return float64(x.Sign()) * math.MaxFloat64
}
