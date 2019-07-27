package gojq

type env struct {
	pc        int
	stack     *stack
	scopes    *stack
	paths     *paths
	values    []interface{}
	codes     []*code
	codeinfos []codeinfo
	forks     []*fork
	backtrack bool
	offset    int
	expdepth  int
}

func newEnv() *env {
	return &env{stack: newStack(), scopes: newStack(), paths: &paths{newStack()}}
}

type scope struct {
	id        int
	offset    int
	pc        int
	saveindex int
}

type fork struct {
	op         opcode
	pc         int
	stackindex int
	stacklimit int
	scopeindex int
	scopelimit int
	pathindex  int
	pathlimit  int
	expdepth   int
}

type paths struct{ *stack }

func (ps *paths) collect() []interface{} {
	var xs []interface{}
	for {
		p := ps.pop().([2]interface{})
		if p[0] == nil {
			break
		}
		xs = append(xs, p[0])
	}
	for i := 0; i < len(xs)/2; i++ { // reverse
		j := len(xs) - 1 - i
		xs[i], xs[j] = xs[j], xs[i]
	}
	return xs
}
