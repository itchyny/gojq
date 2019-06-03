package gojq

import (
	"fmt"
	"sync"
)

type env struct {
	funcDefs  *sync.Map // map[string]*FuncDef
	variables *sync.Map // map[string]*Pipe
	values    *sync.Map // map[string]interface{}
	parent    *env
}

func newEnv(parent *env) *env {
	return &env{
		funcDefs:  new(sync.Map),
		variables: new(sync.Map),
		values:    new(sync.Map),
		parent:    parent,
	}
}

func (env *env) addFuncDef(fd *FuncDef) {
	env.funcDefs.Store(fd.Name+"/"+fmt.Sprint(len(fd.Args)), fd)
}

func (env *env) lookupFuncDef(name string, arg int) *FuncDef {
	if fd, ok := env.funcDefs.Load(name + "/" + fmt.Sprint(arg)); ok {
		return fd.(*FuncDef)
	}
	if env.parent != nil {
		return env.parent.lookupFuncDef(name, arg)
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
	fd, ok := env.funcDefs.Load(name + "/" + fmt.Sprint(arg))
	if !ok {
		return nil
	}
	return fd.(*FuncDef)
}

func (env *env) lookupVariable(name string) *Pipe {
	if p, ok := env.variables.Load(name); ok {
		return p.(*Pipe)
	}
	if env.parent != nil {
		return env.parent.lookupVariable(name)
	}
	return nil
}

func (env *env) lookupValue(name string) (interface{}, bool) {
	if p, ok := env.values.Load(name); ok {
		return p, true
	}
	if env.parent != nil {
		return env.parent.lookupValue(name)
	}
	return nil, false
}
