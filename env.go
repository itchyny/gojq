package gojq

import "context"

type env struct {
	pc        int
	stack     *stack[any]
	paths     *stack[pathValue]
	scopes    *stack[scope]
	values    []any
	codes     []*code
	codeinfos []codeinfo
	forks     []fork
	backtrack bool
	offset    int
	expdepth  int
	label     int
	args      [32]any // len(env.args) > maxarity
	ctx       context.Context
}

func newEnv(ctx context.Context) *env {
	return &env{
		stack:  newStack[any](),
		paths:  newStack[pathValue](),
		scopes: newStack[scope](),
		ctx:    ctx,
	}
}

type scope struct {
	id         int
	offset     int
	pc         int
	saveindex  int
	outerindex int
}

type fork struct {
	pc         int
	stackindex int
	stacklimit int
	scopeindex int
	scopelimit int
	pathindex  int
	pathlimit  int
	offset     int
	expdepth   int
}
