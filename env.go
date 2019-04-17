package gojq

type env struct {
}

func newEnv() *env {
	return &env{}
}

func (env *env) run(p *Program, v interface{}) (interface{}, error) {
	return env.applyQuery(p.Query, v)
}
