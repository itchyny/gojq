package gojq

func prependQuery(xs []*Query, x *Query) []*Query {
	xs = append(xs, nil)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}

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

func prependObjectKeyVal(xs []*ObjectKeyVal, x *ObjectKeyVal) []*ObjectKeyVal {
	xs = append(xs, nil)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}

func prependConstObjectKeyVal(xs []*ConstObjectKeyVal, x *ConstObjectKeyVal) []*ConstObjectKeyVal {
	xs = append(xs, nil)
	copy(xs[1:], xs)
	xs[0] = x
	return xs
}
