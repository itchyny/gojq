package gojq

// Program ...
type Program struct {
	FuncDefs []*FuncDef `@@*`
	Query    *Query     `@@?`
}

// Run program.
func (p *Program) Run(v interface{}) (interface{}, error) {
	return newEnv().run(p, v)
}

// FuncDef ...
type FuncDef struct {
	Name string   `"def" @Ident`
	Args []string `("(" @Ident (";" @Ident)* ")")? ":"`
	Body *Program `@@ ";"`
}
