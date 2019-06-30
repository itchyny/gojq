package gojq

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

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

func (fn function) accept(argcount int) bool {
	switch argcount {
	case 0:
		return fn.argcount&argcount0 > 0
	case 1:
		return fn.argcount&argcount1 > 0
	case 2:
		return fn.argcount&argcount2 > 0
	case 3:
		return fn.argcount&argcount3 > 0
	default:
		return false
	}
}

var internalFuncs map[string]function

func init() {
	internalFuncs = map[string]function{
		"empty":          noArgFunc(funcEmpty),
		"length":         noArgFunc(funcLength),
		"utf8bytelength": noArgFunc(funcUtf8ByteLength),
		"keys":           noArgFunc(funcKeys),
		"has":            argFunc1(funcHas),
		"tonumber":       noArgFunc(funcToNumber),
		"tostring":       noArgFunc(funcToString),
		"_toarray":       noArgFunc(funcToArray),
		"type":           noArgFunc(funcType),
		"explode":        noArgFunc(funcExplode),
		"implode":        noArgFunc(funcImplode),
		"tojson":         noArgFunc(funcToJSON),
		"fromjson":       noArgFunc(funcFromJSON),
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
		"rint":           mathFunc("rint", math.Round),
		"ceil":           mathFunc("ceil", math.Ceil),
		"trunc":          mathFunc("trunc", math.Trunc),
		"fabs":           mathFunc("fabs", math.Abs),
		"sqrt":           mathFunc("sqrt", math.Sqrt),
		"cbrt":           mathFunc("cbrt", math.Cbrt),
		"exp":            mathFunc("exp", math.Exp),
		"exp10":          mathFunc("exp10", func(v float64) float64 { return math.Pow(10, v) }),
		"exp2":           mathFunc("exp2", math.Exp2),
		"expm1":          mathFunc("expm1", math.Expm1),
		"frexp":          noArgFunc(funcFrexp),
		"modf":           noArgFunc(funcModf),
		"log":            mathFunc("log", math.Log),
		"log10":          mathFunc("log10", math.Log10),
		"log1p":          mathFunc("log1p", math.Log1p),
		"log2":           mathFunc("log2", math.Log2),
		"logb":           mathFunc("logb", math.Logb),
		"gamma":          mathFunc("gamma", math.Gamma),
		"tgamma":         mathFunc("tgamma", math.Gamma),
		"lgamma":         mathFunc("lgamma", func(v float64) float64 { v, _ = math.Lgamma(v); return v }),
		"erf":            mathFunc("erf", math.Erf),
		"erfc":           mathFunc("erfc", math.Erfc),
		"j0":             mathFunc("j0", math.J0),
		"j1":             mathFunc("j1", math.J1),
		"y0":             mathFunc("y0", math.Y0),
		"y1":             mathFunc("y1", math.Y1),
		"atan2":          mathFunc2("atan2", math.Atan2),
		"copysign":       mathFunc2("copysign", math.Copysign),
		"drem": mathFunc2("drem", func(l, r float64) float64 {
			x := math.Remainder(l, r)
			if x == 0.0 {
				return math.Copysign(x, l)
			}
			return x
		}),
		"fdim":        mathFunc2("fdim", math.Dim),
		"fmax":        mathFunc2("fmax", math.Max),
		"fmin":        mathFunc2("fmin", math.Min),
		"fmod":        mathFunc2("fmod", math.Mod),
		"hypot":       mathFunc2("hypot", math.Hypot),
		"jn":          mathFunc2("jn", func(l, r float64) float64 { return math.Jn(int(l), r) }),
		"ldexp":       mathFunc2("ldexp", func(l, r float64) float64 { return math.Ldexp(l, int(r)) }),
		"nextafter":   mathFunc2("nextafter", math.Nextafter),
		"nexttoward":  mathFunc2("nexttoward", math.Nextafter),
		"remainder":   mathFunc2("remainder", math.Remainder),
		"scalb":       mathFunc2("scalb", func(l, r float64) float64 { return l * math.Pow(2, r) }),
		"scalbln":     mathFunc2("scalbln", func(l, r float64) float64 { return l * math.Pow(2, r) }),
		"yn":          mathFunc2("yn", func(l, r float64) float64 { return math.Yn(int(l), r) }),
		"pow":         mathFunc2("pow", math.Pow),
		"fma":         mathFunc3("fma", func(x, y, z float64) float64 { return x*y + z }),
		"setpath":     argFunc2(funcSetpath),
		"error":       function{argcount0 | argcount1, funcError},
		"builtins":    noArgFunc(funcBuiltins),
		"env":         noArgFunc(funcEnv),
		"_type_error": argFunc1(internalfuncTypeError),
	}
}

func noArgFunc(fn func(interface{}) interface{}) function {
	return function{argcount0, func(*env, *Func) func(interface{}) interface{} {
		return func(v interface{}) interface{} {
			// if len(f.Args) != 0 {
			// 	return &funcNotFoundError{f}
			// }
			return fn(v)
		}
	}}
}

func argFunc1(fn func(*env, *Func) func(interface{}) interface{}) function {
	return function{argcount1, func(env *env, f *Func) func(interface{}) interface{} {
		return func(v interface{}) interface{} {
			if len(f.Args) != 1 {
				return &funcNotFoundError{f}
			}
			return fn(env, f)(v)
		}
	}}
}

func argFunc2(fn func(*env, *Func) func(interface{}) interface{}) function {
	return function{argcount2, func(env *env, f *Func) func(interface{}) interface{} {
		return func(v interface{}) interface{} {
			if len(f.Args) != 2 {
				return &funcNotFoundError{f}
			}
			return fn(env, f)(v)
		}
	}}
}

func mathFunc(name string, f func(x float64) float64) function {
	return noArgFunc(func(v interface{}) interface{} {
		x, err := toFloat64(name, v)
		if err != nil {
			return err
		}
		return f(x)
	})
}

func mathFunc2(name string, g func(x, y float64) float64) function {
	return function{argcount2, func(env *env, f *Func) func(interface{}) interface{} {
		return func(v interface{}) interface{} {
			if len(f.Args) != 2 {
				return &funcNotFoundError{f}
			}
			l := reuseIterator(env.applyPipe(f.Args[0], unitIterator(v)))
			return mapIterator(env.applyPipe(f.Args[1], unitIterator(v)), func(v interface{}) interface{} {
				r, err := toFloat64(name, v)
				if err != nil {
					return err
				}
				return mapIterator(l(), func(v interface{}) interface{} {
					l, err := toFloat64(name, v)
					if err != nil {
						return err
					}
					return g(l, r)
				})
			})
		}
	}}
}

func mathFunc3(name string, g func(x, y, z float64) float64) function {
	return function{argcount3, func(env *env, f *Func) func(interface{}) interface{} {
		return func(v interface{}) interface{} {
			if len(f.Args) != 3 {
				return &funcNotFoundError{f}
			}
			x := reuseIterator(env.applyPipe(f.Args[0], unitIterator(v)))
			y := reuseIterator(env.applyPipe(f.Args[1], unitIterator(v)))
			return mapIterator(env.applyPipe(f.Args[2], unitIterator(v)), func(v interface{}) interface{} {
				z, err := toFloat64(name, v)
				if err != nil {
					return err
				}
				return mapIterator(y(), func(v interface{}) interface{} {
					y, err := toFloat64(name, v)
					if err != nil {
						return err
					}
					return mapIterator(x(), func(v interface{}) interface{} {
						x, err := toFloat64(name, v)
						if err != nil {
							return err
						}
						return g(x, y, z)
					})
				})
			})
		}
	}}
}

func toFloat64(name string, v interface{}) (float64, error) {
	switch v := v.(type) {
	case int:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, &funcTypeError{name, v}
	}
}

func funcEmpty(_ interface{}) interface{} {
	return struct{}{}
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

func funcHas(env *env, f *Func) func(interface{}) interface{} {
	return func(v interface{}) interface{} {
		return mapIterator(env.applyPipe(f.Args[0], unitIterator(v)), func(x interface{}) interface{} {
			switch v := v.(type) {
			case []interface{}:
				switch x := x.(type) {
				case int:
					return 0 <= x && x < len(v)
				case float64:
					return 0 <= int(x) && int(x) < len(v)
				default:
					return &hasKeyTypeError{v, x}
				}
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
		})
	}
}

func funcToNumber(v interface{}) interface{} {
	switch v := v.(type) {
	case int, uint, float64:
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

func funcToArray(v interface{}) interface{} {
	if a, ok := v.([]interface{}); ok {
		return a
	} else if o, ok := v.(map[string]interface{}); ok {
		a := make([]interface{}, len(o))
		var i int
		for _, v := range o {
			a[i] = v
			i++
		}
		return a
	} else {
		return &iteratorError{v}
	}
}

func funcType(v interface{}) interface{} {
	return typeof(v)
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

func implode(v []interface{}) interface{} {
	var rs []rune
	for _, r := range v {
		switch r := r.(type) {
		case int:
			rs = append(rs, rune(r))
		case float64:
			rs = append(rs, rune(r))
		default:
			return &funcTypeError{"implode", v}
		}
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

func funcFrexp(v interface{}) interface{} {
	x, err := toFloat64("frexp", v)
	if err != nil {
		return err
	}
	f, e := math.Frexp(x)
	return []interface{}{f, e}
}

func funcModf(v interface{}) interface{} {
	x, err := toFloat64("modf", v)
	if err != nil {
		return err
	}
	i, f := math.Modf(x)
	return []interface{}{f, i}
}

func funcSetpath(env *env, f *Func) func(interface{}) interface{} {
	return func(v interface{}) interface{} {
		return mapIterator(env.applyPipe(f.Args[1], unitIterator(v)), func(w interface{}) interface{} {
			return mapIterator(env.applyPipe(f.Args[0], unitIterator(v)), func(p interface{}) interface{} {
				keys, ok := p.([]interface{})
				if !ok {
					return &funcTypeError{"setpath", v}
				}
				if len(keys) == 0 {
					return w
				}
				v := v
				u := v
				f := func(w interface{}) interface{} { v = w; return w }
				for i, x := range keys {
					switch x := x.(type) {
					case string:
						if u == nil {
							u = f(make(map[string]interface{}))
						}
						switch uu := u.(type) {
						case map[string]interface{}:
							if i < len(keys)-1 {
								u = uu[x]
								f = func(w interface{}) interface{} { uu[x] = w; return w }
							} else {
								uu[x] = w
							}
						default:
							return &expectedObjectError{u}
						}
					case int, float64:
						if u == nil {
							u = f([]interface{}{})
						}
						y, _ := toInt(x)
						switch uu := u.(type) {
						case []interface{}:
							if y >= len(uu) {
								ys := make([]interface{}, y+1)
								copy(ys, uu)
								uu = ys
								f(uu)
							}
							if i < len(keys)-1 {
								u = uu[y]
								f = func(w interface{}) interface{} { uu[y] = w; return w }
							} else {
								uu[y] = w
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
			})
		})
	}
}

func funcError(env *env, f *Func) func(interface{}) interface{} {
	return func(v interface{}) interface{} {
		if len(f.Args) == 0 {
			switch v := v.(type) {
			case string:
				return errors.New(v)
			default:
				return &funcTypeError{"error", v}
			}
		} else if len(f.Args) == 1 {
			return mapIterator(env.applyPipe(f.Args[0], unitIterator(v)), func(v interface{}) interface{} {
				switch v := v.(type) {
				case string:
					return errors.New(v)
				default:
					return &funcTypeError{"error", v}
				}
			})
		} else {
			return &funcNotFoundError{f}
		}
	}
}

func funcBuiltins(_ interface{}) interface{} {
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
	for _, q := range builtinFuncs {
		for _, fd := range q.FuncDefs {
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

func funcEnv(_ interface{}) interface{} {
	env := make(map[string]interface{})
	for _, kv := range os.Environ() {
		xs := strings.SplitN(kv, "=", 2)
		env[xs[0]] = xs[1]
	}
	return env
}

func internalfuncTypeError(env *env, f *Func) func(interface{}) interface{} {
	return func(v interface{}) interface{} {
		return mapIterator(env.applyPipe(f.Args[0], unitIterator(v)), func(x interface{}) interface{} {
			return &funcTypeError{x.(string), v}
		})
	}
}
