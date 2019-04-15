package gojq

type function struct {
	minArgs, maxArgs int
	callback         func(interface{}) (interface{}, error)
}

var funcMap = map[string]function{
	"null":  {0, 0, funcNull},
	"true":  {0, 0, funcTrue},
	"false": {0, 0, funcFalse},
}

func applyFunc(f *Func, v interface{}) (interface{}, error) {
	fn, ok := funcMap[f.Name]
	if !ok {
		return nil, &funcNotFoundError{f}
	}
	return fn.callback(v)
}

func funcNull(_ interface{}) (interface{}, error) {
	return nil, nil
}

func funcTrue(_ interface{}) (interface{}, error) {
	return true, nil
}

func funcFalse(_ interface{}) (interface{}, error) {
	return false, nil
}
