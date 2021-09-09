package gojq

func prependImport(xs []*Import, x *Import) []*Import {
	xs = append(xs, nil)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}

func prependFuncDef(xs []*FuncDef, x *FuncDef) []*FuncDef {
	xs = append(xs, nil)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}

func prependIfElif(xs []*IfElif, x *IfElif) []*IfElif {
	xs = append(xs, nil)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}
