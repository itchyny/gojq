package gojq

type env struct {
	funcDefs  map[string]map[int]*FuncDef
	variables map[string]*Pipe
	values    map[string]interface{}
	parent    *env
}

func newEnv(parent *env) *env {
	return &env{
		funcDefs:  make(map[string]map[int]*FuncDef),
		variables: make(map[string]*Pipe),
		values:    make(map[string]interface{}),
		parent:    parent,
	}
}

func (env *env) addFuncDef(fd *FuncDef) {
	if _, ok := env.funcDefs[fd.Name]; !ok {
		env.funcDefs[fd.Name] = make(map[int]*FuncDef)
	}
	env.funcDefs[fd.Name][len(fd.Args)] = fd
}

func (env *env) lookupFuncDef(name string) map[int]*FuncDef {
	if fds, ok := env.funcDefs[name]; ok {
		return fds
	}
	if env.parent != nil {
		return env.parent.lookupFuncDef(name)
	}
	bfn, ok := builtinFuncs[name]
	if !ok {
		return nil
	}
	p, err := Parse(bfn)
	if err != nil {
		panic(err)
	}
	for _, fd := range p.FuncDefs {
		env.addFuncDef(fd)
	}
	return env.funcDefs[name]
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

func (env *env) lookupValues(name string) (interface{}, bool) {
	if p, ok := env.values[name]; ok {
		return p, true
	}
	if env.parent != nil {
		return env.parent.lookupValues(name)
	}
	return nil, false
}
