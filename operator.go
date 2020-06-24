package gojq

import (
	"math"
	"math/big"
	"strings"
)

// Operator ...
type Operator int

// Operators ...
const (
	OpAdd Operator = iota
	OpSub
	OpMul
	OpDiv
	OpMod
	OpEq
	OpNe
	OpGt
	OpLt
	OpGe
	OpLe
	OpAnd
	OpOr
	OpAlt
	OpAssign
	OpModify
	OpUpdateAdd
	OpUpdateSub
	OpUpdateMul
	OpUpdateDiv
	OpUpdateMod
	OpUpdateAlt
)

var operatorMap = map[string]Operator{
	"+":   OpAdd,
	"-":   OpSub,
	"*":   OpMul,
	"/":   OpDiv,
	"%":   OpMod,
	"==":  OpEq,
	"!=":  OpNe,
	">":   OpGt,
	"<":   OpLt,
	">=":  OpGe,
	"<=":  OpLe,
	"and": OpAnd,
	"or":  OpOr,
	"//":  OpAlt,
	"=":   OpAssign,
	"|=":  OpModify,
	"+=":  OpUpdateAdd,
	"-=":  OpUpdateSub,
	"*=":  OpUpdateMul,
	"/=":  OpUpdateDiv,
	"%=":  OpUpdateMod,
	"//=": OpUpdateAlt,
}

// Capture implements  participle.Capture.
func (op *Operator) Capture(s []string) error {
	var ok bool
	*op, ok = operatorMap[s[0]]
	if !ok {
		panic("operator: " + s[0])
	}
	return nil
}

// String implements Stringer.
func (op Operator) String() string {
	switch op {
	case OpAdd:
		return "+"
	case OpSub:
		return "-"
	case OpMul:
		return "*"
	case OpDiv:
		return "/"
	case OpMod:
		return "%"
	case OpEq:
		return "=="
	case OpNe:
		return "!="
	case OpGt:
		return ">"
	case OpLt:
		return "<"
	case OpGe:
		return ">="
	case OpLe:
		return "<="
	case OpAnd:
		return "and"
	case OpOr:
		return "or"
	case OpAlt:
		return "//"
	case OpAssign:
		return "="
	case OpModify:
		return "|="
	case OpUpdateAdd:
		return "+="
	case OpUpdateSub:
		return "-="
	case OpUpdateMul:
		return "*="
	case OpUpdateDiv:
		return "/="
	case OpUpdateMod:
		return "%="
	case OpUpdateAlt:
		return "//="
	default:
		panic(op)
	}
}

// GoString implements GoStringer.
func (op Operator) GoString() string {
	switch op {
	case OpAdd:
		return "OpAdd"
	case OpSub:
		return "OpSub"
	case OpMul:
		return "OpMul"
	case OpDiv:
		return "OpDiv"
	case OpMod:
		return "OpMod"
	case OpEq:
		return "OpEq"
	case OpNe:
		return "OpNe"
	case OpGt:
		return "OpGt"
	case OpLt:
		return "OpLt"
	case OpGe:
		return "OpGe"
	case OpLe:
		return "OpLe"
	case OpAnd:
		return "OpAnd"
	case OpOr:
		return "OpOr"
	case OpAlt:
		return "OpAlt"
	case OpAssign:
		return "OpAssign"
	case OpModify:
		return "OpModify"
	case OpUpdateAdd:
		return "OpUpdateAdd"
	case OpUpdateSub:
		return "OpUpdateSub"
	case OpUpdateMul:
		return "OpUpdateMul"
	case OpUpdateDiv:
		return "OpUpdateDiv"
	case OpUpdateMod:
		return "OpUpdateMod"
	case OpUpdateAlt:
		return "OpUpdateAlt"
	default:
		panic(op)
	}
}

func (op Operator) getFunc() string {
	switch op {
	case OpAdd:
		return "_add"
	case OpSub:
		return "_subtract"
	case OpMul:
		return "_multiply"
	case OpDiv:
		return "_divide"
	case OpMod:
		return "_modulo"
	case OpEq:
		return "_equal"
	case OpNe:
		return "_notequal"
	case OpGt:
		return "_greater"
	case OpLt:
		return "_less"
	case OpGe:
		return "_greatereq"
	case OpLe:
		return "_lesseq"
	case OpAnd:
		panic("unreachable")
	case OpOr:
		panic("unreachable")
	case OpAlt:
		panic("unreachable")
	case OpAssign:
		return "_assign"
	case OpModify:
		return "_modify"
	case OpUpdateAdd:
		return "_add"
	case OpUpdateSub:
		return "_subtract"
	case OpUpdateMul:
		return "_multiply"
	case OpUpdateDiv:
		return "_divide"
	case OpUpdateMod:
		return "_modulo"
	case OpUpdateAlt:
		return "_alternative"
	default:
		panic(op)
	}
}

func binopTypeSwitch(
	l, r interface{},
	callbackInts func(int, int) interface{},
	callbackInt64s func(int64, int64) interface{},
	callbackFloats func(float64, float64) interface{},
	callbackBigInts func(*big.Int, *big.Int) interface{},
	callbackStrings func(string, string) interface{},
	callbackArrays func(l, r []interface{}) interface{},
	callbackMaps func(l, r map[string]interface{}) interface{},
	fallback func(interface{}, interface{}) interface{}) interface{} {
	switch l := l.(type) {
	case int:
		switch r := r.(type) {
		case int:
			if minHalfInt <= l && l <= maxHalfInt &&
				minHalfInt <= r && r <= maxHalfInt {
				return callbackInts(l, r)
			}
			return callbackBigInts(big.NewInt(int64(l)), big.NewInt(int64(r)))
		case int64:
			if minHalfInt <= l && l <= maxHalfInt &&
				minHalfInt <= r && r <= maxHalfInt {
				return callbackInt64s(int64(l), r)
			}
			return callbackBigInts(big.NewInt(int64(l)), big.NewInt(r))
		case float64:
			return callbackFloats(float64(l), r)
		case *big.Int:
			return callbackBigInts(big.NewInt(int64(l)), r)
		default:
			return fallback(l, r)
		}
	case int64:
		switch r := r.(type) {
		case int:
			if minHalfInt <= l && l <= maxHalfInt &&
				minHalfInt <= r && r <= maxHalfInt {
				return callbackInt64s(l, int64(r))
			}
			return callbackBigInts(big.NewInt(l), big.NewInt(int64(r)))
		case int64:
			if minHalfInt <= l && l <= maxHalfInt &&
				minHalfInt <= r && r <= maxHalfInt {
				return callbackInt64s(l, r)
			}
			return callbackBigInts(big.NewInt(l), big.NewInt(r))
		case float64:
			return callbackFloats(float64(l), r)
		case *big.Int:
			return callbackBigInts(big.NewInt(l), r)
		default:
			return fallback(l, r)
		}
	case float64:
		switch r := r.(type) {
		case int:
			return callbackFloats(l, float64(r))
		case int64:
			return callbackFloats(l, float64(r))
		case float64:
			return callbackFloats(l, r)
		case *big.Int:
			return callbackFloats(l, bigToFloat(r))
		default:
			return fallback(l, r)
		}
	case *big.Int:
		switch r := r.(type) {
		case int:
			return callbackBigInts(l, big.NewInt(int64(r)))
		case int64:
			return callbackBigInts(l, big.NewInt(r))
		case float64:
			return callbackFloats(bigToFloat(l), r)
		case *big.Int:
			return callbackBigInts(l, r)
		default:
			return fallback(l, r)
		}
	case string:
		switch r := r.(type) {
		case string:
			return callbackStrings(l, r)
		default:
			return fallback(l, r)
		}
	case []interface{}:
		switch r := r.(type) {
		case []interface{}:
			return callbackArrays(l, r)
		default:
			return fallback(l, r)
		}
	case map[string]interface{}:
		switch r := r.(type) {
		case map[string]interface{}:
			return callbackMaps(l, r)
		default:
			return fallback(l, r)
		}
	default:
		return fallback(l, r)
	}
}

func funcOpPlus(v interface{}) interface{} {
	switch v := v.(type) {
	case int:
		return v
	case float64:
		return v
	case *big.Int:
		return v
	default:
		return &unaryTypeError{"plus", v}
	}
}

func funcOpNegate(v interface{}) interface{} {
	switch v := v.(type) {
	case int:
		return -v
	case float64:
		return -v
	case *big.Int:
		return new(big.Int).Neg(v)
	default:
		return &unaryTypeError{"negate", v}
	}
}

func funcOpAdd(_, l, r interface{}) interface{} {
	if l == nil {
		return r
	} else if r == nil {
		return l
	}
	return binopTypeSwitch(l, r,
		func(l, r int) interface{} { return l + r },
		func(l, r int64) interface{} { return l + r },
		func(l, r float64) interface{} { return l + r },
		func(l, r *big.Int) interface{} { return new(big.Int).Add(l, r) },
		func(l, r string) interface{} { return l + r },
		func(l, r []interface{}) interface{} {
			if len(r) == 0 {
				return l
			} else if len(l) == 0 {
				return r
			}
			v := make([]interface{}, 0, len(l)+len(r))
			return append(append(v, l...), r...)
		},
		func(l, r map[string]interface{}) interface{} {
			m := make(map[string]interface{})
			for k, v := range l {
				m[k] = v
			}
			for k, v := range r {
				m[k] = v
			}
			return m
		},
		func(l, r interface{}) interface{} { return &binopTypeError{"add", l, r} },
	)
}

func funcOpSub(_, l, r interface{}) interface{} {
	return binopTypeSwitch(l, r,
		func(l, r int) interface{} { return l - r },
		func(l, r int64) interface{} { return l - r },
		func(l, r float64) interface{} { return l - r },
		func(l, r *big.Int) interface{} { return new(big.Int).Sub(l, r) },
		func(l, r string) interface{} { return &binopTypeError{"subtract", l, r} },
		func(l, r []interface{}) interface{} {
			a := make([]interface{}, 0, len(l))
			for _, v := range l {
				var found bool
				for _, w := range r {
					if deepEqual(v, w) {
						found = true
						break
					}
				}
				if !found {
					a = append(a, v)
				}
			}
			return a
		},
		func(l, r map[string]interface{}) interface{} { return &binopTypeError{"subtract", l, r} },
		func(l, r interface{}) interface{} { return &binopTypeError{"subtract", l, r} },
	)
}

func funcOpMul(_, l, r interface{}) interface{} {
	return binopTypeSwitch(l, r,
		func(l, r int) interface{} { return l * r },
		func(l, r int64) interface{} { return l * r },
		func(l, r float64) interface{} { return l * r },
		func(l, r *big.Int) interface{} { return new(big.Int).Mul(l, r) },
		func(l, r string) interface{} { return &binopTypeError{"multiply", l, r} },
		func(l, r []interface{}) interface{} { return &binopTypeError{"multiply", l, r} },
		deepMergeObjects,
		func(l, r interface{}) interface{} {
			multiplyString := func(s string, cnt float64) interface{} {
				if cnt <= 0.0 {
					return nil
				}
				if cnt < 1.0 {
					return s
				}
				return strings.Repeat(s, int(cnt))
			}
			if l, ok := l.(string); ok {
				if f, ok := toFloat(r); ok {
					return multiplyString(l, f)
				}
			}
			if r, ok := r.(string); ok {
				if f, ok := toFloat(l); ok {
					return multiplyString(r, f)
				}
			}
			return &binopTypeError{"multiply", l, r}
		},
	)
}

func deepMergeObjects(l, r map[string]interface{}) interface{} {
	m := make(map[string]interface{})
	for k, v := range l {
		m[k] = v
	}
	for k, v := range r {
		if mk, ok := m[k]; ok {
			if mk, ok := mk.(map[string]interface{}); ok {
				if w, ok := v.(map[string]interface{}); ok {
					v = deepMergeObjects(mk, w)
				}
			}
		}
		m[k] = v
	}
	return m
}

func funcOpDiv(_, l, r interface{}) interface{} {
	return binopTypeSwitch(l, r,
		func(l, r int) interface{} {
			if r == 0 {
				if l == 0 {
					return math.NaN()
				}
				return &zeroDivisionError{l, r}
			}
			return float64(l) / float64(r)
		},
		func(l, r int64) interface{} {
			if r == 0 {
				if l == 0 {
					return math.NaN()
				}
				return &zeroDivisionError{l, r}
			}
			return float64(l) / float64(r)
		},
		func(l, r float64) interface{} {
			if r == 0.0 {
				if l == 0.0 {
					return math.NaN()
				}
				return &zeroDivisionError{l, r}
			} else if isinf(r) {
				if isinf(l) {
					return math.NaN()
				}
				if (r >= 0) == (l >= 0) {
					return 0.0
				}
				return math.Copysign(0.0, -1)
			}
			return l / r
		},
		func(l, r *big.Int) interface{} {
			if r.Sign() == 0 {
				if l.Sign() == 0 {
					return math.NaN()
				}
				return &zeroDivisionError{l, r}
			}
			x := new(big.Int).Div(l, r)
			if new(big.Int).Mul(x, r).Cmp(l) == 0 {
				return x
			}
			rf := bigToFloat(r)
			if isinf(rf) {
				if l.Sign() == r.Sign() {
					return 0.0
				}
				return math.Copysign(0.0, -1)
			}
			return bigToFloat(l) / rf
		},
		func(l, r string) interface{} {
			if l == "" {
				return []interface{}{}
			}
			xs := strings.Split(l, r)
			vs := make([]interface{}, len(xs))
			for i, x := range xs {
				vs[i] = x
			}
			return vs
		},
		func(l, r []interface{}) interface{} { return &binopTypeError{"divide", l, r} },
		func(l, r map[string]interface{}) interface{} { return &binopTypeError{"divide", l, r} },
		func(l, r interface{}) interface{} { return &binopTypeError{"divide", l, r} },
	)
}

func funcOpMod(_, l, r interface{}) interface{} {
	return binopTypeSwitch(l, r,
		func(l, r int) interface{} {
			if r == 0 {
				return &zeroModuloError{l, r}
			}
			return l % r
		},
		func(l, r int64) interface{} {
			if r == 0 {
				return &zeroModuloError{l, r}
			}
			return l % r
		},
		func(l, r float64) interface{} {
			if r == 0.0 {
				return &zeroModuloError{l, r}
			}
			return int(l) % int(r)
		},
		func(l, r *big.Int) interface{} {
			if r.Sign() == 0 {
				return &zeroModuloError{l, r}
			}
			return new(big.Int).Mod(l, r)
		},
		func(l, r string) interface{} { return &binopTypeError{"modulo", l, r} },
		func(l, r []interface{}) interface{} { return &binopTypeError{"modulo", l, r} },
		func(l, r map[string]interface{}) interface{} { return &binopTypeError{"modulo", l, r} },
		func(l, r interface{}) interface{} { return &binopTypeError{"modulo", l, r} },
	)
}

func funcOpAlt(_, l, r interface{}) interface{} {
	if l == nil || l == false {
		return r
	}
	return l
}

func funcOpEq(_, l, r interface{}) interface{} {
	return compare(l, r) == 0
}

func funcOpNe(_, l, r interface{}) interface{} {
	return compare(l, r) != 0
}

func funcOpGt(_, l, r interface{}) interface{} {
	return compare(l, r) > 0
}

func funcOpLt(_, l, r interface{}) interface{} {
	return compare(l, r) < 0
}

func funcOpGe(_, l, r interface{}) interface{} {
	return compare(l, r) >= 0
}

func funcOpLe(_, l, r interface{}) interface{} {
	return compare(l, r) <= 0
}
