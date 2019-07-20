package gojq

import (
	"reflect"
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
	}
	panic(op)
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
	}
	panic(op)
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
	}
	panic(op)
}

func binopTypeSwitch(
	l, r interface{},
	callbackInts func(int, int) interface{},
	callbackFloats func(float64, float64) interface{},
	callbackStrings func(string, string) interface{},
	callbackArrays func(l, r []interface{}) interface{},
	callbackMaps func(l, r map[string]interface{}) interface{},
	fallback func(interface{}, interface{}) interface{}) interface{} {
	switch l := l.(type) {
	case int:
		switch r := r.(type) {
		case int:
			return callbackInts(l, r)
		case float64:
			return callbackFloats(float64(l), r)
		default:
			return fallback(l, r)
		}
	case float64:
		switch r := r.(type) {
		case int:
			return callbackFloats(l, float64(r))
		case float64:
			return callbackFloats(l, r)
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
		func(l, r float64) interface{} { return l + r },
		func(l, r string) interface{} { return l + r },
		func(l, r []interface{}) interface{} { return append(l, r...) },
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
		func(l, r float64) interface{} { return l - r },
		func(l, r string) interface{} { return &binopTypeError{"subtract", l, r} },
		func(l, r []interface{}) interface{} {
			a := make([]interface{}, 0, len(l))
			for _, v := range l {
				var found bool
				for _, w := range r {
					if reflect.DeepEqual(v, w) {
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
		func(l, r float64) interface{} { return l * r },
		func(l, r string) interface{} { return &binopTypeError{"multiply", l, r} },
		func(l, r []interface{}) interface{} { return &binopTypeError{"multiply", l, r} },
		deepMergeObjects,
		func(l, r interface{}) interface{} {
			multiplyString := func(s string, cnt float64) interface{} {
				if cnt < 0.0 {
					return nil
				}
				if cnt < 1.0 {
					return l
				}
				return strings.Repeat(s, int(cnt))
			}
			if l, ok := l.(string); ok {
				switch r := r.(type) {
				case int:
					return multiplyString(l, float64(r))
				case float64:
					return multiplyString(l, r)
				}
			}
			if r, ok := r.(string); ok {
				switch l := l.(type) {
				case int:
					return multiplyString(r, float64(l))
				case float64:
					return multiplyString(r, l)
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
				return &zeroDivisionError{l, r}
			}
			return l / r
		},
		func(l, r float64) interface{} {
			if r == 0.0 {
				return &zeroDivisionError{l, r}
			}
			return l / r
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
		func(l, r float64) interface{} {
			if r == 0.0 {
				return &zeroModuloError{l, r}
			}
			return int(l) % int(r)
		},
		func(l, r string) interface{} { return &binopTypeError{"modulo", l, r} },
		func(l, r []interface{}) interface{} { return &binopTypeError{"modulo", l, r} },
		func(l, r map[string]interface{}) interface{} { return &binopTypeError{"modulo", l, r} },
		func(l, r interface{}) interface{} { return &binopTypeError{"modulo", l, r} },
	)
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
