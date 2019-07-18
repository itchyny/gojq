package gojq

type env struct {
	pc        int
	stack     *stack
	scopes    *stack
	values    []interface{}
	codes     []*code
	codeinfos []codeinfo
	forks     []*fork
	backtrack bool
	offset    int
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
}

func newEnv() *env {
	return &env{stack: newStack(), scopes: newStack()}
}
