package gojq

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
)

type expectedObjectError struct {
	v interface{}
}

func (err *expectedObjectError) Error() string {
	return fmt.Sprintf("expected an object but got: %s", typeErrorPreview(err.v))
}

type expectedArrayError struct {
	v interface{}
}

func (err *expectedArrayError) Error() string {
	return fmt.Sprintf("expected an array but got: %s", typeErrorPreview(err.v))
}

type iteratorError struct {
	v interface{}
}

func (err *iteratorError) Error() string {
	return fmt.Sprintf("cannot iterate over: %s", typeErrorPreview(err.v))
}

type objectKeyNotStringError struct {
	v interface{}
}

func (err *objectKeyNotStringError) Error() string {
	return fmt.Sprintf("expected a string for object key but got: %s", typeErrorPreview(err.v))
}

type arrayIndexNotNumberError struct {
	v interface{}
}

func (err *arrayIndexNotNumberError) Error() string {
	return fmt.Sprintf("expected a number for indexing an array but got: %s", typeErrorPreview(err.v))
}

type funcNotFoundError struct {
	f *Func
}

func (err *funcNotFoundError) Error() string {
	return fmt.Sprintf("function not defined: %s/%d", err.f.Name, len(err.f.Args))
}

type funcTypeError struct {
	name string
	v    interface{}
}

func (err *funcTypeError) Error() string {
	return fmt.Sprintf("%s cannot be applied to: %s", err.name, typeErrorPreview(err.v))
}

type funcContainsError struct {
	l, r interface{}
}

func (err *funcContainsError) Error() string {
	return fmt.Sprintf("cannot check contains(%s): %s", previewValue(err.r), typeErrorPreview(err.l))
}

type hasKeyTypeError struct {
	l, r interface{}
}

func (err *hasKeyTypeError) Error() string {
	return fmt.Sprintf("cannot check wether %s has a key: %s", typeErrorPreview(err.l), typeErrorPreview(err.r))
}

type unaryTypeError struct {
	name string
	v    interface{}
}

func (err *unaryTypeError) Error() string {
	return fmt.Sprintf("cannot %s: %s", err.name, typeErrorPreview(err.v))
}

type binopTypeError struct {
	name string
	l, r interface{}
}

func (err *binopTypeError) Error() string {
	return fmt.Sprintf("cannot %s: %s and %s", err.name, typeErrorPreview(err.l), typeErrorPreview(err.r))
}

type zeroDivisionError struct {
	l, r interface{}
}

func (err *zeroDivisionError) Error() string {
	return fmt.Sprintf("cannot divide %s by: %s", typeErrorPreview(err.l), typeErrorPreview(err.r))
}

type zeroModuloError struct {
	l, r interface{}
}

func (err *zeroModuloError) Error() string {
	return fmt.Sprintf("cannot modulo %s by: %s", typeErrorPreview(err.l), typeErrorPreview(err.r))
}

type variableNotFoundError struct {
	n string
}

func (err *variableNotFoundError) Error() string {
	return fmt.Sprintf("variable not defined: %s", err.n)
}

type bindVariableNameError struct {
	n string
}

func (err *bindVariableNameError) Error() string {
	return fmt.Sprintf(`variable should start with "$" but got: %q`, err.n)
}

type labelNameError struct {
	n string
}

func (err *labelNameError) Error() string {
	return fmt.Sprintf(`label should start with "$" but got: %q`, err.n)
}

type breakError struct {
	n string
}

func (err *breakError) Error() string {
	return fmt.Sprintf(`label not defined: %q`, err.n)
}

type stringLiteralError struct {
	s string
}

func (err *stringLiteralError) Error() string {
	return fmt.Sprintf("expected a string but got: %s", err.s)
}

type invalidPathError struct {
	v interface{}
}

func (err *invalidPathError) Error() string {
	return fmt.Sprintf("invalid path against: %s", typeErrorPreview(err.v))
}

type invalidPathIterError struct {
	v interface{}
}

func (err *invalidPathIterError) Error() string {
	return fmt.Sprintf("invalid path on iterating against: %s", typeErrorPreview(err.v))
}

type getpathError struct {
	v, path interface{}
}

func (err *getpathError) Error() string {
	return fmt.Sprintf("cannot getpath with %s against: %s", previewValue(err.path), typeErrorPreview(err.v))
}

func typeErrorPreview(v interface{}) string {
	p := preview(v)
	if p != "" {
		p = " (" + p + ")"
	}
	return typeof(v) + p
}

func typeof(v interface{}) (s string) {
	if v == nil {
		return "null"
	}
	k := reflect.TypeOf(v).Kind()
	switch k {
	case reflect.Array, reflect.Slice:
		return "array"
	case reflect.Map:
		return "object"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Uint, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Ptr:
		if _, ok := v.(*big.Int); ok {
			return "number"
		}
		return "ptr"
	default:
		return k.String()
	}
}

func preview(v interface{}) string {
	if v == nil {
		return ""
	}
	bs, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s, l := string(bs), 25
	if len(s) > l {
		s = s[:l-3] + " ..."
	}
	return s
}

func previewValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	return preview(v)
}
