package gojq

type env struct {
	funcDefs  map[string]*FuncDef
	variables map[string]*Pipe
	values    map[string]interface{}
	parent    *env
	pc        int
	stack     []interface{}
	value     []interface{}
	scopes    []*scope
	codes     []*code
	forks     []*fork
}

type scope struct {
	id     int
	offset int
}

type fork struct {
	op    opcode
	pc    int
	v     interface{}
	scope int
}

func newEnv(parent *env) *env {
	return &env{
		funcDefs:  make(map[string]*FuncDef),
		variables: make(map[string]*Pipe),
		values:    make(map[string]interface{}),
		parent:    parent,
		pc:        0,
		stack:     []interface{}{},
		value:     []interface{}{},
		scopes:    []*scope{},
		codes:     []*code{},
		forks:     []*fork{},
	}
}

func (env *env) addFuncDef(fd *FuncDef) {
	env.funcDefs[fd.Name+string(rune(len(fd.Args)))] = fd
}

func (env *env) lookupFuncDef(name string, arg int) *FuncDef {
	if fd, ok := env.funcDefs[name+string(rune(arg))]; ok {
		return fd
	}
	if env.parent != nil {
		return env.parent.lookupFuncDef(name, arg)
	}
	q, ok := builtinFuncs[name]
	if !ok {
		return nil
	}
	var f *FuncDef
	for _, fd := range q.FuncDefs {
		env.addFuncDef(fd)
		if len(fd.Args) == arg {
			f = fd
		}
	}
	return f
}

func (env *env) lookupVariable(name string) *Pipe {
	if p, ok := env.variables[name]; ok {
		return p
	}
	if env.parent != nil {
		return env.parent.lookupVariable(name)
	}
	return nil
}

func (env *env) lookupValue(name string) (interface{}, bool) {
	if p, ok := env.values[name]; ok {
		return p, true
	}
	if env.parent != nil {
		return env.parent.lookupValue(name)
	}
	return nil, false
}
