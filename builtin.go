package gojq

//go:generate go run _tools/gen_builtin.go -i builtin.jq -o builtin_gen.go
var builtinFuncDefs map[string][]*FuncDef
